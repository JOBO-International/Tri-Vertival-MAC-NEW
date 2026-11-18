package dng

import (
	"math"

	"vertical3/kern"
)

type mat3 [3][3]float64

func (a mat3) mul(b mat3) mat3 {
	var c mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c[i][j] = a[i][0]*b[0][j] + a[i][1]*b[1][j] + a[i][2]*b[2][j]
		}
	}
	return c
}

func (a mat3) vec(v [3]float64) [3]float64 {
	return [3]float64{
		a[0][0]*v[0] + a[0][1]*v[1] + a[0][2]*v[2],
		a[1][0]*v[0] + a[1][1]*v[1] + a[1][2]*v[2],
		a[2][0]*v[0] + a[2][1]*v[1] + a[2][2]*v[2],
	}
}

func (a mat3) inv() mat3 {
	det := a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) - a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) + a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	if det == 0 {
		return mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	}
	var r mat3
	r[0][0] = (a[1][1]*a[2][2] - a[1][2]*a[2][1]) / det
	r[0][1] = (a[0][2]*a[2][1] - a[0][1]*a[2][2]) / det
	r[0][2] = (a[0][1]*a[1][2] - a[0][2]*a[1][1]) / det
	r[1][0] = (a[1][2]*a[2][0] - a[1][0]*a[2][2]) / det
	r[1][1] = (a[0][0]*a[2][2] - a[0][2]*a[2][0]) / det
	r[1][2] = (a[0][2]*a[1][0] - a[0][0]*a[1][2]) / det
	r[2][0] = (a[1][0]*a[2][1] - a[1][1]*a[2][0]) / det
	r[2][1] = (a[0][1]*a[2][0] - a[0][0]*a[2][1]) / det
	r[2][2] = (a[0][0]*a[1][1] - a[0][1]*a[1][0]) / det
	return r
}

func matAus(v []float64) (mat3, bool) {
	if len(v) < 9 {
		return mat3{}, false
	}
	return mat3{{v[0], v[1], v[2]}, {v[3], v[4], v[5]}, {v[6], v[7], v[8]}}, true
}

// XYZ (D50) → lineares sRGB (D65), Bradford-angepasst
var xyzD50zuSRGB = mat3{
	{3.1338561, -1.6168667, -0.4906146},
	{-0.9787684, 1.9161415, 0.0334540},
	{0.0719453, -0.2289914, 1.4052427},
}

var bradford = mat3{{0.8951, 0.2664, -0.1614}, {-0.7502, 1.7135, 0.0367}, {0.0389, -0.0685, 1.0296}}
var d50 = [3]float64{0.96422, 1.0, 0.82521}

func anpassung(von, nach [3]float64) mat3 {
	a, b := bradford.vec(von), bradford.vec(nach)
	d := mat3{{b[0] / a[0], 0, 0}, {0, b[1] / a[1], 0}, {0, 0, b[2] / a[2]}}
	return bradford.inv().mul(d).mul(bradford)
}

// farbe bestimmt die Matrix (weißabgeglichene, normierte Kamerawerte → lineares
// sRGB) und die Weißabgleich-Faktoren (≥ 1, kleinster = 1).
func (t *tiff) farbe(ifd0, roh ifd) (mat3, [3]float64, float64) {
	hole := func(tag uint16) []float64 {
		if v := t.zahlen(roh, tag); len(v) > 0 {
			return v
		}
		return t.zahlen(ifd0, tag)
	}
	neutral := [3]float64{1, 1, 1}
	if n := hole(0xC628); len(n) >= 3 {
		neutral = [3]float64{n[0], n[1], n[2]}
	}
	for i := range neutral {
		if neutral[i] <= 0 {
			neutral[i] = 1
		}
	}
	// Lichtart wählen: Tageslicht (D65/D55) bevorzugen
	ill1, ill2 := t.wert(ifd0, 0xC65A, 0), t.wert(ifd0, 0xC65B, 0)
	tages := func(i float64) bool {
		return i == 21 || i == 20 || i == 1 || i == 23 || i == 22 || i == 4 || i == 9 || i == 10
	}
	idx := 1
	if _, ok := matAus(hole(0xC622)); ok && (tages(ill2) || !tages(ill1)) {
		idx = 2
	}
	tagCM, tagFM, tagCC := uint16(0xC621), uint16(0xC714), uint16(0xC623)
	if idx == 2 {
		tagCM, tagFM, tagCC = 0xC622, 0xC715, 0xC624
	}
	// AB·CC (Analog-Abgleich × Kamerakalibrierung) laut DNG-Spezifikation
	abcc := mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	if ab := hole(0xC627); len(ab) >= 3 {
		abcc = mat3{{ab[0], 0, 0}, {0, ab[1], 0}, {0, 0, ab[2]}}
	}
	if cc, ok := matAus(hole(tagCC)); ok {
		abcc = abcc.mul(cc)
	}
	var kameraZuXYZ mat3
	if fm, ok := matAus(hole(tagFM)); ok {
		// ForwardMatrix: XYZ(D50) = FM · D⁻¹ · Kamera, D = diag((AB·CC)⁻¹ · Neutral)
		dn := abcc.inv().vec(neutral)
		kameraZuXYZ = fm.mul(mat3{{1 / dn[0], 0, 0}, {0, 1 / dn[1], 0}, {0, 0, 1 / dn[2]}}).mul(abcc.inv())
	} else if cm, ok := matAus(hole(tagCM)); ok {
		cm = abcc.mul(cm)
		inv := cm.inv()
		weiss := inv.vec(neutral)
		if weiss[1] > 0 {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					inv[i][j] /= weiss[1]
				}
			}
			weiss = [3]float64{weiss[0] / weiss[1], 1, weiss[2] / weiss[1]}
		}
		kameraZuXYZ = anpassung(weiss, d50).mul(inv)
	} else {
		// ohne Farbmatrix: als sRGB behandeln
		kameraZuXYZ = xyzD50zuSRGB.inv().mul(mat3{{1 / neutral[0], 0, 0}, {0, 1 / neutral[1], 0}, {0, 0, 1 / neutral[2]}})
	}
	m := xyzD50zuSRGB.mul(kameraZuXYZ)
	// Weißabgleich-Faktoren 1/neutral, normiert auf kleinsten = 1
	wb := [3]float64{1 / neutral[0], 1 / neutral[1], 1 / neutral[2]}
	kl := math.Min(wb[0], math.Min(wb[1], wb[2]))
	for i := range wb {
		wb[i] /= kl
	}
	// m erwartet nicht abgeglichene Werte: m·v = m·diag(1/wb)·(v·wb)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			m[i][j] /= wb[j]
		}
	}
	return m, wb, t.wert(ifd0, 0xC62A, 0)
}

// Entwickle macht aus einer DNG-Datei ein fertiges RGB-Bild (sRGB, 8 Bit).
func Entwickle(datei []byte) (*kern.Bild, Info, error) {
	t, err := neu(datei)
	if err != nil {
		return nil, Info{}, err
	}
	rohIFD, ifd0, err := t.findeRohIFD()
	if err != nil {
		return nil, Info{}, err
	}
	r, err := t.ladeRoh(rohIFD)
	if err != nil {
		return nil, Info{}, err
	}
	m, wb, belichtung := t.farbe(ifd0, rohIFD)

	// Nachschlagetabellen: Rohwert → normierter, weißabgeglichener Wert
	// (je Position im Schwarzwert-Muster und Farbkanal)
	maxRoh := 65536
	lut := func(kanalWB float64, schwarz, weiss float64, ebene int) []float32 {
		l := make([]float32, maxRoh)
		for v := 0; v < maxRoh; v++ {
			x := float64(v)
			if r.linear != nil {
				x = float64(r.linear[min(v, len(r.linear)-1)])
			}
			n := math.Max(0, math.Min(1, (x-schwarz)/math.Max(1, weiss-schwarz)))
			for _, op := range r.opcodes[ebene] { // OpcodeList2, z. B. bei verlustbehafteter DNG
				n = op.wende(n)
			}
			l[v] = float32(math.Max(0, n) * kanalWB)
		}
		return l
	}
	sw := func(i int) float64 { return r.schwarz[min(i, len(r.schwarz)-1)] }
	ws := func(i int) float64 { return r.weiss[min(i, len(r.weiss)-1)] }

	ox, oy := r.aktiv[1]+int(math.Round(r.crop[0])), r.aktiv[0]+int(math.Round(r.crop[1]))
	bw, bh := int(math.Round(r.crop[2])), int(math.Round(r.crop[3]))
	bw, bh = min(bw, r.w-ox), min(bh, r.h-oy)
	if bw <= 0 || bh <= 0 {
		return nil, r.info, errors0("ungültiger Bildausschnitt")
	}
	// Belichtung: Vorgabe der Kamera/des Konverters (BaselineExposure), wie bei Adobe
	gain := math.Pow(2, belichtung)
	if NurGamma {
		gain = 1
	}

	// Ausgabe: Farbmatrix, Tonkurve, sRGB-Gamma
	var kurve [4097]uint8
	for i := range kurve {
		x := float64(i) / 4096
		if !NurGamma {
			x = 0.88*x + 0.12*(x*x*(3-2*x)) // leichte S-Kurve
		}
		var g float64
		if x <= 0.0031308 {
			g = 12.92 * x
		} else {
			g = 1.055*math.Pow(x, 1/2.4) - 0.055
		}
		kurve[i] = uint8(math.Max(0, math.Min(255, g*255+0.5)))
	}
	out := kern.NeuesBild(bw, bh)
	// schreibe: weißabgeglichene Kamerawerte → Farbmatrix, Belichtung, Kurve, 8 Bit
	schreibe := func(i int, c0, c1, c2 float32) {
		// abgeschnittene Lichter neutral halten
		c := [3]float64{math.Min(1, float64(c0)), math.Min(1, float64(c1)), math.Min(1, float64(c2))}
		rgb := m.vec(c)
		for k := 0; k < 3; k++ {
			v := rgb[k] * gain
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			out.Pix[i+k] = kurve[int(v*4096)]
		}
	}

	if r.photometric == cfa {
		// je CFA-Position (2×2, relativ zum aktiven Bereich) eine Tabelle
		var luts [4][]float32
		for i := 0; i < 4; i++ {
			sx, sy := i%2, i/2
			bi := (sy%r.schwarzDim[0])*r.schwarzDim[1] + sx%r.schwarzDim[1]
			luts[i] = lut(wb[r.cfaMuster[i]], sw(bi), ws(0), 0)
		}
		demosaic(r, luts, ox, oy, bw, bh, schreibe)
	} else {
		var luts [3][]float32
		for k := 0; k < 3; k++ {
			luts[k] = lut(wb[k], sw(k), ws(k), k)
		}
		parallel(bh, func(a, b int) {
			for y := a; y < b; y++ {
				for x := 0; x < bw; x++ {
					s := ((y+oy)*r.w + x + ox) * 3
					schreibe((y*bw+x)*3, luts[0][r.daten[s]], luts[1][r.daten[s+1]], luts[2][r.daten[s+2]])
				}
			}
		})
	}

	return out, r.info, nil
}

type fehlerText string

func (f fehlerText) Error() string { return string(f) }
func errors0(s string) error       { return fehlerText(s) }

// demosaic: Malvar-He-Cutler (gradientenkorrigiert, 5×5) für 2×2-Muster.
func demosaic(r *rohbild, luts [4][]float32, ox, oy, bw, bh int, schreibe func(i int, c0, c1, c2 float32)) {
	ax, ay := r.aktiv[1], r.aktiv[0]
	ax1, ay1 := r.aktiv[3]-1, r.aktiv[2]-1
	farbe := func(x, y int) int { return r.cfaMuster[((y-ay)&1)*2+((x-ax)&1)] }
	// am Rand um eine gerade Anzahl Pixel nach innen verschieben (gleiche Farbe)
	wert := func(x, y int) float32 {
		if x < ax {
			x += 2 * ((ax - x + 1) / 2)
		} else if x > ax1 {
			x -= 2 * ((x - ax1 + 1) / 2)
		}
		if y < ay {
			y += 2 * ((ay - y + 1) / 2)
		} else if y > ay1 {
			y -= 2 * ((y - ay1 + 1) / 2)
		}
		return luts[((y-ay)&1)*2+((x-ax)&1)][r.daten[y*r.w+x]]
	}
	parallel(bh, func(a, b int) {
		for yy := a; yy < b; yy++ {
			y := yy + oy
			for xx := 0; xx < bw; xx++ {
				x := xx + ox
				c0 := farbe(x, y)
				C := wert(x, y)
				N, S, W, E := wert(x, y-1), wert(x, y+1), wert(x-1, y), wert(x+1, y)
				NN, SS, WW, EE := wert(x, y-2), wert(x, y+2), wert(x-2, y), wert(x+2, y)
				NW, NE, SW, SE := wert(x-1, y-1), wert(x+1, y-1), wert(x-1, y+1), wert(x+1, y+1)
				var px [3]float32
				px[c0] = C
				if c0 != 1 { // R oder B: Grün schätzen, gegenüberliegende Farbe diagonal
					px[1] = (4*C + 2*(N+S+W+E) - (NN + SS + WW + EE)) / 8
					px[2-c0] = (6*C + 2*(NW+NE+SW+SE) - 1.5*(NN+SS+WW+EE)) / 8
				} else {
					h := farbe(x+1, y) // Farbe links/rechts
					v := 2 - h         // Farbe oben/unten
					zeile := (5*C + 4*(W+E) - (WW + EE) - (NW + NE + SW + SE) + 0.5*(NN+SS)) / 8
					spalte := (5*C + 4*(N+S) - (NN + SS) - (NW + NE + SW + SE) + 0.5*(WW+EE)) / 8
					px[h], px[v] = zeile, spalte
				}
				for k := 0; k < 3; k++ {
					if px[k] < 0 {
						px[k] = 0
					}
				}
				schreibe((yy*bw+xx)*3, px[0], px[1], px[2])
			}
		}
	})
}
