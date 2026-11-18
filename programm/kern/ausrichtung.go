package kern

import "math"

// Drehen in 90°-Schritten und Spiegeln des Ausgangsbildes.
const (
	DrehLinks   = "links"  // 90° gegen den Uhrzeigersinn
	DrehRechts  = "rechts" // 90° im Uhrzeigersinn
	SpiegelWaag = "h"      // waagerecht spiegeln (links ↔ rechts)
	SpiegelSenk = "v"      // senkrecht spiegeln (oben ↔ unten)
)

// exifFuer: Schritt als EXIF-Ausrichtung für Gedreht.
func exifFuer(schritt string) int {
	switch schritt {
	case DrehRechts:
		return 6
	case DrehLinks:
		return 8
	case SpiegelWaag:
		return 2
	case SpiegelSenk:
		return 4
	}
	return 1
}

// Ausrichten wendet die Schritte der Reihe nach an.
func Ausrichten(b *Bild, schritte []string) *Bild {
	for _, s := range schritte {
		b = b.Gedreht(exifFuer(s))
	}
	return b
}

// AusrichtenPunkt: Bildpunkt (Pixelkoordinaten, Bildgröße w×h vor dem Schritt)
// nach dem Schritt.
func AusrichtenPunkt(pt [2]float64, w, h int, schritt string) [2]float64 {
	x, y := pt[0], pt[1]
	switch schritt {
	case DrehRechts:
		return [2]float64{float64(h) - y, x}
	case DrehLinks:
		return [2]float64{y, float64(w) - x}
	case SpiegelWaag:
		return [2]float64{float64(w) - x, y}
	case SpiegelSenk:
		return [2]float64{x, float64(h) - y}
	}
	return pt
}

// AusrichtungsMatrix: derselbe Schritt im Kamerasystem (x rechts, y unten).
func AusrichtungsMatrix(schritt string) Mat3 {
	switch schritt {
	case DrehRechts:
		return Mat3{{0, -1, 0}, {1, 0, 0}, {0, 0, 1}}
	case DrehLinks:
		return Mat3{{0, 1, 0}, {-1, 0, 0}, {0, 0, 1}}
	case SpiegelWaag:
		return Mat3{{-1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	case SpiegelSenk:
		return Mat3{{1, 0, 0}, {0, -1, 0}, {0, 0, 1}}
	}
	return Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
}

func transponiert(m Mat3) Mat3 {
	var t Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			t[i][j] = m[j][i]
		}
	}
	return t
}

// WinkelAus zerlegt eine Drehmatrix R = Ry(yaw)·Rx(pitch)·Rz(roll).
func WinkelAus(R Mat3) (roll, pitch, yaw float64) {
	pitch = grad(math.Asin(math.Max(-1, math.Min(1, -R[1][2]))))
	roll = grad(math.Atan2(R[1][0], R[1][1]))
	yaw = grad(math.Atan2(R[0][2], R[2][2]))
	return
}

// WinkelNachAusrichtung rechnet eine Korrektur (Roll, Pitch, Yaw) so um,
// dass sie auf dem gedrehten/gespiegelten Bild dasselbe bewirkt.
// Q ist das Produkt der AusrichtungsMatrix-Schritte (zuletzt links).
func WinkelNachAusrichtung(roll, pitch, yaw float64, Q Mat3) (float64, float64, float64) {
	R := Q.Mul(Drehmatrix(roll, pitch, yaw)).Mul(transponiert(Q))
	return WinkelAus(R)
}

// Skaliert liefert eine Kopie in der Größe w×h (verkleinern: Flächenmittel,
// vergrößern: bilinear).
func (b *Bild) Skaliert(w, h int) *Bild {
	if w <= 0 || h <= 0 || (w == b.W && h == b.H) {
		return b
	}
	out := NeuesBild(w, h)
	sx, sy := float64(b.W)/float64(w), float64(b.H)/float64(h)
	if sx >= 1 && sy >= 1 { // verkleinern
		parallel(h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				ya, yb := int(float64(y)*sy), int(math.Min(float64(b.H), float64(y+1)*sy))
				if yb <= ya {
					yb = ya + 1
				}
				for x := 0; x < w; x++ {
					xa, xb := int(float64(x)*sx), int(math.Min(float64(b.W), float64(x+1)*sx))
					if xb <= xa {
						xb = xa + 1
					}
					var r, g, bl, n int
					for yy := ya; yy < yb; yy++ {
						row := b.Pix[(yy*b.W+xa)*3:]
						for xx := 0; xx < xb-xa; xx++ {
							r += int(row[xx*3])
							g += int(row[xx*3+1])
							bl += int(row[xx*3+2])
							n++
						}
					}
					o := (y*w + x) * 3
					out.Pix[o], out.Pix[o+1], out.Pix[o+2] = uint8(r/n), uint8(g/n), uint8(bl/n)
				}
			}
		})
		return out
	}
	// vergrößern (oder gemischt): bilinear
	parallel(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			fy := math.Max(0, math.Min(float64(b.H-1), (float64(y)+0.5)*sy-0.5))
			ya := int(fy)
			yb := min(ya+1, b.H-1)
			ty := fy - float64(ya)
			for x := 0; x < w; x++ {
				fx := math.Max(0, math.Min(float64(b.W-1), (float64(x)+0.5)*sx-0.5))
				xa := int(fx)
				xb := min(xa+1, b.W-1)
				tx := fx - float64(xa)
				o := (y*w + x) * 3
				for k := 0; k < 3; k++ {
					a := float64(b.Pix[(ya*b.W+xa)*3+k])*(1-tx) + float64(b.Pix[(ya*b.W+xb)*3+k])*tx
					c := float64(b.Pix[(yb*b.W+xa)*3+k])*(1-tx) + float64(b.Pix[(yb*b.W+xb)*3+k])*tx
					out.Pix[o+k] = uint8(math.Round(a*(1-ty) + c*ty))
				}
			}
		}
	})
	return out
}

// BegrenzeKorrektur verringert Pitch und Yaw (Roll bleibt), bis das Ergebnis
// höchstens maxFaktor-mal so breit/hoch wird wie das Original. Sehr starke
// Korrekturen (z. B. steil nach oben fotografierte Hochhäuser) würden das Bild
// sonst extrem verzerren. Liefert die begrenzten Werte und den Anteil (0..1).
func BegrenzeKorrektur(w, h int, p Parameter, maxFaktor float64) (Parameter, float64) {
	geht := func(s float64) bool {
		q := p
		q.Pitch, q.Yaw = p.Pitch*s, p.Yaw*s
		q.Zuschneiden = false
		_, ow, oh, err := Transformation(w, h, q)
		if err != nil {
			return false
		}
		if float64(ow) > maxFaktor*float64(w) || float64(oh) > maxFaktor*float64(h) {
			return false
		}
		if p.Zuschneiden {
			q.Zuschneiden = true
			if _, _, _, err := Transformation(w, h, q); err != nil {
				return false
			}
		}
		return true
	}
	if geht(1) {
		return p, 1
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 30; i++ {
		m := (lo + hi) / 2
		if geht(m) {
			lo = m
		} else {
			hi = m
		}
	}
	q := p
	q.Pitch, q.Yaw = p.Pitch*lo, p.Yaw*lo
	return q, lo
}

// Zugeschnitten liefert den Ausschnitt x, y, w × h (wird auf das Bild begrenzt).
func (b *Bild) Zugeschnitten(x, y, w, h int) *Bild {
	x, y = max(0, min(x, b.W-1)), max(0, min(y, b.H-1))
	w, h = max(1, min(w, b.W-x)), max(1, min(h, b.H-y))
	if x == 0 && y == 0 && w == b.W && h == b.H {
		return b
	}
	out := NeuesBild(w, h)
	for j := 0; j < h; j++ {
		copy(out.Pix[j*w*3:(j+1)*w*3], b.Pix[((y+j)*b.W+x)*3:((y+j)*b.W+x+w)*3])
	}
	return out
}

// AusschnittMitRand liefert den Bereich x, y, w × h; Teile außerhalb des
// Bildes werden mit farbe gefüllt (Rahmen größer als das Foto).
func (b *Bild) AusschnittMitRand(x, y, w, h int, farbe [3]uint8) *Bild {
	if x >= 0 && y >= 0 && x+w <= b.W && y+h <= b.H {
		return b.Zugeschnitten(x, y, w, h)
	}
	w, h = max(1, w), max(1, h)
	out := NeuesBild(w, h)
	for i := 0; i < w*h; i++ {
		out.Pix[i*3], out.Pix[i*3+1], out.Pix[i*3+2] = farbe[0], farbe[1], farbe[2]
	}
	x0, x1 := max(0, x), min(b.W, x+w)
	if x1 <= x0 {
		return out
	}
	for j := max(0, y); j < min(b.H, y+h); j++ {
		copy(out.Pix[((j-y)*w+(x0-x))*3:((j-y)*w+(x1-x))*3], b.Pix[(j*b.W+x0)*3:(j*b.W+x1)*3])
	}
	return out
}
