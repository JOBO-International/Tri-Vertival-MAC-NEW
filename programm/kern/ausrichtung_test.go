package kern

import (
	"math"
	"testing"
)

func TestWinkelNachAusrichtung(t *testing.T) {
	w, h := 4000, 3000
	f35 := 28.0
	roll, pitch, yaw := 3.0, 15.0, -7.0
	for _, s := range []string{DrehRechts, DrehLinks, SpiegelWaag, SpiegelSenk} {
		w2, h2 := w, h
		if s == DrehRechts || s == DrehLinks {
			w2, h2 = h, w
		}
		Q := AusrichtungsMatrix(s)
		r2, p2, y2 := WinkelNachAusrichtung(roll, pitch, yaw, Q)
		K := Kameramatrix(f35, w, h)
		K2 := Kameramatrix(f35, w2, h2)
		H := K.Mul(Drehmatrix(roll, pitch, yaw)).Mul(K.Inv())
		H2 := K2.Mul(Drehmatrix(r2, p2, y2)).Mul(K2.Inv())
		for _, pt := range [][2]float64{{100, 200}, {3900, 150}, {2000, 2900}, {1234, 1500}} {
			a := H.MulVec([3]float64{pt[0], pt[1], 1})
			ziel := AusrichtenPunkt([2]float64{a[0]/a[2] - float64(w)/2 + float64(w)/2, a[1] / a[2]}, w, h, s)
			p2pt := AusrichtenPunkt(pt, w, h, s)
			b := H2.MulVec([3]float64{p2pt[0], p2pt[1], 1})
			if math.Hypot(b[0]/b[2]-ziel[0], b[1]/b[2]-ziel[1]) > 1e-6 {
				t.Fatalf("%s: %v vs %v (Winkel %.2f %.2f %.2f)", s, ziel, [2]float64{b[0] / b[2], b[1] / b[2]}, r2, p2, y2)
			}
		}
		// Rückweg: Matrix wieder zerlegen
		r3, p3, y3 := WinkelAus(Drehmatrix(r2, p2, y2))
		if math.Abs(r3-r2)+math.Abs(p3-p2)+math.Abs(y3-y2) > 1e-9 {
			t.Fatal("WinkelAus")
		}
		t.Logf("%-6s → Roll %.2f Pitch %.2f Yaw %.2f", s, r2, p2, y2)
	}
}

func TestAusrichtenBild(t *testing.T) {
	b := NeuesBild(3, 2)
	b.Pix[0] = 9 // Pixel (0,0)
	r := Ausrichten(b, []string{DrehRechts})
	if r.W != 2 || r.H != 3 || r.Pix[(0*2+1)*3] != 9 { // oben rechts
		t.Fatalf("rechts: %v", r.Pix)
	}
	l := Ausrichten(b, []string{DrehLinks})
	if l.Pix[(2*2+0)*3] != 9 { // unten links
		t.Fatalf("links: %v", l.Pix)
	}
	if s := b.Skaliert(6, 4); s.W != 6 || s.H != 4 {
		t.Fatal("Skaliert")
	}
}
