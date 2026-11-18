// Package belichter: Bogen für den digitalen S/W-Belichter.
//
// Der Belichter erwartet eine Datei im Format 50 × 40 cm und skaliert sie auf
// seine tatsächliche Belichtungsfläche (z. B. 48,8 × 38,6 cm). Damit die Bilder
// in echter Größe herauskommen, wird der Bogen in echten Millimetern der
// Belichtungsfläche angelegt und beim Rendern auf die 50 × 40-Datei gestreckt –
// die Skalierung des Belichters hebt das wieder auf.
package belichter

import (
	"image"
	"image/color"
	"math"
	"sort"

	"vertical3/kern"
)

// Einrichtung des Belichters (einmal festgelegt, gespeichert).
type Einrichtung struct {
	DateiB   float64 `json:"datei_b_cm"` // Dateiformat, das die Software erwartet (cm)
	DateiH   float64 `json:"datei_h_cm"`
	FlaecheB float64 `json:"flaeche_b_cm"` // tatsächliche Belichtungsfläche (cm)
	FlaecheH float64 `json:"flaeche_h_cm"`
	DPI      float64 `json:"dpi"`
}

// Standard: Datei 50 × 40 cm, Fläche 48,8 × 38,6 cm, 300 dpi (quer).
func Standard() Einrichtung {
	return Einrichtung{DateiB: 50, DateiH: 40, FlaecheB: 48.8, FlaecheH: 38.6, DPI: 300}
}

// Pixel der Datei.
func (e Einrichtung) Pixel() (int, int) {
	return int(math.Round(e.DateiB / 2.54 * e.DPI)), int(math.Round(e.DateiH / 2.54 * e.DPI))
}

// Format: Abzugsgröße in mm (kurz × lang); 0 × 0 = ganzer Bogen.
type Format struct {
	Name       string
	Kurz, Lang float64
}

var Formate = []Format{
	{"Ganzer Bogen", 0, 0},
	{"9 × 13 cm", 90, 130},
	{"10 × 10 cm", 100, 100},
	{"10 × 15 cm", 100, 150},
	{"13 × 13 cm", 130, 130},
	{"13 × 18 cm", 130, 180},
	{"15 × 15 cm", 150, 150},
	{"15 × 20 cm", 150, 200},
	{"15 × 21 cm", 150, 210},
	{"18 × 24 cm", 180, 240},
	{"20 × 20 cm", 200, 200},
	{"20 × 25 cm", 200, 250},
	{"20 × 30 cm", 200, 300},
	{"24 × 30 cm", 240, 300},
	{"30 × 30 cm", 300, 300},
	{"30 × 40 cm", 300, 400},
	{"30 × 45 cm", 300, 450},
}

// Platz: ein Abzug auf dem Bogen (mm, Ursprung oben links der Belichtungsfläche).
type Platz struct {
	X, Y, B, H float64
}

// Anordnung berechnet die Plätze für ein Format auf der Fläche fb × fh (mm)
// mit Abstand a (mm) zwischen den Bildern. Probiert beide Ausrichtungen und
// einen Reststreifen mit gedrehten Bildern; das Ergebnis ist mittig.
func Anordnung(f Format, fb, fh, a float64) []Platz {
	if f.Kurz <= 0 {
		return []Platz{{0, 0, fb, fh}}
	}
	const eps = 1e-6
	anz := func(platz, groesse float64) int {
		if groesse > platz+eps {
			return 0
		}
		return int(math.Floor((platz + a + eps) / (groesse + a)))
	}
	raster := func(x0, y0, w, h float64, nx, ny int) []Platz {
		var p []Platz
		for j := 0; j < ny; j++ {
			for i := 0; i < nx; i++ {
				p = append(p, Platz{x0 + float64(i)*(w+a), y0 + float64(j)*(h+a), w, h})
			}
		}
		return p
	}
	var beste []Platz
	for _, o := range [][2]float64{{f.Kurz, f.Lang}, {f.Lang, f.Kurz}} {
		w, h := o[0], o[1]
		nx, ny := anz(fb, w), anz(fh, h)
		if nx == 0 || ny == 0 {
			continue
		}
		haupt := raster(0, 0, w, h, nx, ny)
		bw, bh := float64(nx)*(w+a)-a, float64(ny)*(h+a)-a
		kandidaten := [][]Platz{haupt}
		// Reststreifen rechts mit gedrehten Bildern
		if rb := fb - bw - a; rb > 0 {
			mx, my := anz(rb, h), anz(fh, w)
			if mx > 0 && my > 0 {
				kandidaten = append(kandidaten, append(append([]Platz{}, haupt...), raster(bw+a, 0, h, w, mx, my)...))
			}
		}
		// Reststreifen unten mit gedrehten Bildern
		if rh := fh - bh - a; rh > 0 {
			mx, my := anz(fb, h), anz(rh, w)
			if mx > 0 && my > 0 {
				kandidaten = append(kandidaten, append(append([]Platz{}, haupt...), raster(0, bh+a, h, w, mx, my)...))
			}
		}
		for _, k := range kandidaten {
			if len(k) > len(beste) {
				beste = k
			}
		}
	}
	if len(beste) == 0 {
		return nil
	}
	// mittig auf der Fläche
	var maxX, maxY float64
	for _, p := range beste {
		maxX, maxY = math.Max(maxX, p.X+p.B), math.Max(maxY, p.Y+p.H)
	}
	dx, dy := (fb-maxX)/2, (fh-maxY)/2
	for i := range beste {
		beste[i].X += dx
		beste[i].Y += dy
	}
	sort.SliceStable(beste, func(i, j int) bool {
		if math.Abs(beste[i].Y-beste[j].Y) > 1 {
			return beste[i].Y < beste[j].Y
		}
		return beste[i].X < beste[j].X
	})
	return beste
}

// Grau macht ein Bild exakt neutral (R = G = B) – der Belichter belichtet S/W.
func Grau(b *kern.Bild) *kern.Bild {
	out := kern.NeuesBild(b.W, b.H)
	for i := 0; i < b.W*b.H; i++ {
		r, g, bl := float64(b.Pix[i*3]), float64(b.Pix[i*3+1]), float64(b.Pix[i*3+2])
		v := uint8(math.Round(0.2126*r + 0.7152*g + 0.0722*bl))
		out.Pix[i*3], out.Pix[i*3+1], out.Pix[i*3+2] = v, v, v
	}
	return out
}

// Fuellen schneidet das Bild auf das Seitenverhältnis w:h zu (mittig) und
// dreht es um 90°, wenn Hoch-/Querformat nicht zum Platz passen.
func Fuellen(b *kern.Bild, w, h int) *kern.Bild {
	if (b.W > b.H) != (w > h) && b.W != b.H && w != h {
		b = b.Gedreht(6) // 90° im Uhrzeigersinn
	}
	ziel := float64(w) / float64(h)
	cw, ch := b.W, b.H
	if float64(b.W)/float64(b.H) > ziel {
		cw = int(math.Round(float64(b.H) * ziel))
	} else {
		ch = int(math.Round(float64(b.W) / ziel))
	}
	c := b.Zugeschnitten((b.W-cw)/2, (b.H-ch)/2, cw, ch)
	return c.Skaliert(w, h)
}

// Bogen: Inhalt eines Belichterbogens.
type Bogen struct {
	E             Einrichtung
	Plaetze       []Platz
	Bilder        []*kern.Bild // werden der Reihe nach auf die Plätze verteilt
	Schnittmarken bool
	Abstand       float64 // mm, für die Länge der Schnittmarken
}

// Rendern erzeugt die Datei in Pixelgröße pw × ph (Dateiformat). Die
// Belichtungsfläche füllt die ganze Datei; jeder echte Millimeter wird so gestreckt,
// dass er nach der Skalierung des Belichters wieder einen Millimeter ergibt.
func (bg Bogen) Rendern(pw, ph int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, pw, ph))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	sx := float64(pw) / (bg.E.FlaecheB * 10) // Dateipixel je echtem mm
	sy := float64(ph) / (bg.E.FlaecheH * 10)
	px := func(x, y float64) (int, int) { return int(math.Round(x * sx)), int(math.Round(y * sy)) }
	for i, p := range bg.Plaetze {
		if len(bg.Bilder) == 0 {
			break
		}
		x0, y0 := px(p.X, p.Y)
		x1, y1 := px(p.X+p.B, p.Y+p.H)
		w, h := x1-x0, y1-y0
		if w <= 0 || h <= 0 {
			continue
		}
		f := Fuellen(bg.Bilder[i%len(bg.Bilder)], w, h)
		for y := 0; y < h; y++ {
			if y0+y < 0 || y0+y >= ph {
				continue
			}
			for x := 0; x < w; x++ {
				if x0+x < 0 || x0+x >= pw {
					continue
				}
				s := (y*w + x) * 3
				d := img.PixOffset(x0+x, y0+y)
				img.Pix[d], img.Pix[d+1], img.Pix[d+2] = f.Pix[s], f.Pix[s+1], f.Pix[s+2]
			}
		}
	}
	if bg.Schnittmarken && len(bg.Plaetze) > 0 && bg.Plaetze[0].B > 0 {
		bg.markenZeichnen(img, sx, sy)
	}
	return img
}

// markenZeichnen: kurze Linien an den Ecken jedes Bildes, im Abstand zwischen
// den Bildern bzw. im Rand – so stören sie kein Bild.
func (bg Bogen) markenZeichnen(img *image.RGBA, sx, sy float64) {
	fb, fh := bg.E.FlaecheB*10, bg.E.FlaecheH*10
	frei := func(x, y float64) bool { // liegt der Punkt außerhalb aller Bilder?
		if x < 0 || y < 0 || x > fb || y > fh {
			return false
		}
		for _, p := range bg.Plaetze {
			if x > p.X-0.05 && x < p.X+p.B+0.05 && y > p.Y-0.05 && y < p.Y+p.H+0.05 {
				return false
			}
		}
		return true
	}
	dicke := 0.2 // mm
	linie := func(x0, y0, x1, y1 float64) {
		// in Schritten von 0,05 mm, nur wo frei
		n := int(math.Hypot(x1-x0, y1-y0)/0.05) + 1
		for k := 0; k <= n; k++ {
			t := float64(k) / float64(n)
			x, y := x0+(x1-x0)*t, y0+(y1-y0)*t
			if !frei(x, y) {
				continue
			}
			for _, dd := range []float64{-dicke / 2, 0, dicke / 2} {
				xx, yy := x, y
				if x0 == x1 {
					xx += dd
				} else {
					yy += dd
				}
				ix, iy := int(xx*sx), int(yy*sy)
				if image.Pt(ix, iy).In(img.Rect) {
					img.Set(ix, iy, color.Black)
				}
			}
		}
	}
	l := math.Min(4, math.Max(1.5, bg.Abstand)) // Länge der Marke (mm)
	ab := 0.5                                   // Abstand vom Bild (mm)
	for _, p := range bg.Plaetze {
		for _, e := range [][2]float64{{p.X, p.Y}, {p.X + p.B, p.Y}, {p.X, p.Y + p.H}, {p.X + p.B, p.Y + p.H}} {
			// waagerechte Marke (verlängert die obere/untere Kante nach außen)
			sxr := -1.0
			if e[0] > p.X {
				sxr = 1
			}
			linie(e[0]+sxr*ab, e[1], e[0]+sxr*(ab+l), e[1])
			// senkrechte Marke
			syr := -1.0
			if e[1] > p.Y {
				syr = 1
			}
			linie(e[0], e[1]+syr*ab, e[0], e[1]+syr*(ab+l))
		}
	}
}
