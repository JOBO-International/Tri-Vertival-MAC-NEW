package kern

import "math"

// KorrigiereImFormat korrigiert die Perspektive ohne Zuschnitt und liefert das
// Ergebnis in exakt der Größe (und damit dem Seitenverhältnis) des Originals.
// Der Bildausschnitt liegt mittig im umschließenden Rechteck des entzerrten
// Bildes. maske[i] = 255 markiert Pixel ohne Bildinhalt (zum Auffüllen).
func KorrigiereImFormat(b *Bild, p Parameter) (*Bild, []uint8, float64, error) {
	p.Zuschneiden = false
	M, ow, oh, err := Transformation(b.W, b.H, p)
	if err != nil {
		return nil, nil, 0, err
	}
	a := float64(b.W) / float64(b.H)
	fw := math.Min(float64(ow), float64(oh)*a)
	fh := fw / a
	x0, y0 := (float64(ow)-fw)/2, (float64(oh)-fh)/2
	s := float64(b.W) / fw
	// Ziel = S · T · M
	F := Mat3{{s, 0, -x0 * s}, {0, s, -y0 * s}, {0, 0, 1}}.Mul(M)
	Mi := F.Inv()
	out := NeuesBild(b.W, b.H)
	maske := make([]uint8, b.W*b.H)
	var leer int64
	zaehler := make([]int64, b.H)
	parallel(b.H, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			for x := 0; x < b.W; x++ {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				q0 := Mi[0][0]*fx + Mi[0][1]*fy + Mi[0][2]
				q1 := Mi[1][0]*fx + Mi[1][1]*fy + Mi[1][2]
				q2 := Mi[2][0]*fx + Mi[2][1]*fy + Mi[2][2]
				sx, sy := q0/q2-0.5, q1/q2-0.5
				i := y*b.W + x
				if q2 <= 0 || sx < 0.5 || sy < 0.5 || sx > float64(b.W)-1.5 || sy > float64(b.H)-1.5 {
					maske[i] = 255
					zaehler[y]++
					continue
				}
				bikubisch(b, sx, sy, out.Pix[i*3:i*3+3])
			}
		}
	})
	for _, z := range zaehler {
		leer += z
	}
	return out, maske, float64(leer) / float64(b.W*b.H), nil
}
