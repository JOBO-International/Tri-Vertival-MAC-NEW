package kern

import (
	"math"
	"testing"
	"time"
)

// fassade: frontales Bild mit Fensterraster und einem Rundfenster.
func fassade(w, h int) func(x, y float64) uint8 {
	return func(x, y float64) uint8 {
		if x < 0 || y < 0 || x >= float64(w) || y >= float64(h) {
			return 120
		}
		if y < 0.15*float64(h) {
			return 200 // Himmel
		}
		// Rundfenster
		if math.Hypot(x-0.5*float64(w), y-0.25*float64(h)) < 0.05*float64(w) {
			return 40
		}
		fx, fy := math.Mod(x, float64(w)/8), math.Mod(y-0.35*float64(h), float64(h)/6)
		if y > 0.35*float64(h) && fx > 0.25*float64(w)/8 && fx < 0.75*float64(w)/8 && fy > 0 && fy < 0.6*float64(h)/6 {
			// Fenster mit hellem Rahmen
			if fx < 0.28*float64(w)/8 || fx > 0.72*float64(w)/8 || fy < 0.03*float64(h)/6 || fy > 0.57*float64(h)/6 {
				return 245
			}
			return 50
		}
		return 170 // Wand
	}
}

func schiefesBild(w, h int, f35, roll, pitch, yaw float64) *Bild {
	K := Kameramatrix(f35, w, h)
	H := K.Mul(Drehmatrix(roll, pitch, yaw)).Mul(K.Inv())
	fr := fassade(w, h)
	b := NeuesBild(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := H.MulVec([3]float64{float64(x) + 0.5, float64(y) + 0.5, 1})
			v := fr(t[0]/t[2], t[1]/t[2])
			if t[2] <= 0 {
				v = 0
			}
			v = uint8(math.Max(0, math.Min(255, float64(v)+float64((x*7919+y*104729)%13)-6))) // etwas Rauschen
			o := (y*w + x) * 3
			b.Pix[o], b.Pix[o+1], b.Pix[o+2] = v, v, v
		}
	}
	return b
}

func TestAutoMarken(t *testing.T) {
	w, h, f35 := 2000, 1500, 28.0
	for _, fall := range []struct {
		r, p, y float64
		frontal bool
	}{{2.5, 12, 0, false}, {-3, 18, 8, true}, {1, -8, -12, true}, {2, 16, 25, true}, {2, 16, 25, false}} {
		b := schiefesBild(w, h, f35, fall.r, fall.p, fall.y)
		t0 := time.Now()
		e, err := AutoMarken(b, f35, fall.frontal)
		if err != nil {
			t.Fatalf("%+v: %v", fall, err)
		}
		l := e.Loesung
		t.Logf("soll %+.1f %+.1f %+.1f  ist %+.2f %+.2f %+.2f  (%d Formen, %d verwendet, %v)", fall.r, fall.p, fall.y, l.Roll, l.Pitch, l.Yaw, e.Gefunden, len(e.Marken), time.Since(t0))
		if math.Abs(l.Roll-fall.r) > 0.4 || math.Abs(l.Pitch-fall.p) > 0.7 || (fall.frontal && math.Abs(l.Yaw-fall.y) > 1.5) {
			t.Errorf("Abweichung zu groß")
		}
	}
}
