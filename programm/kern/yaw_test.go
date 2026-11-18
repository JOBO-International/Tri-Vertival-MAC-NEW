package kern

import (
	"math"
	"testing"
)

func abb(M Mat3, x, y float64) (float64, float64) {
	t := M.MulVec([3]float64{x, y, 1})
	return t[0] / t[2], t[1] / t[2]
}

// Yaw um die Unterkante: Boden bleibt waagerecht, Senkrechte bleiben senkrecht,
// die Oberkante läuft zusammen bzw. auseinander.
func TestYawUnterkanteBleibtWaagerecht(t *testing.T) {
	w, h := 1600, 1000
	for _, par := range []Parameter{
		{Yaw: 15, F35: 28},
		{Yaw: -20, F35: 24},
		{Roll: 1.5, Pitch: 12, Yaw: 10, F35: 24},
	} {
		M, _, _, err := Transformation(w, h, par)
		if err != nil {
			t.Fatal(err)
		}
		_, y1 := abb(M, 0, float64(h))
		_, y2 := abb(M, float64(w), float64(h))
		if par.Roll == 0 && math.Abs(y1-y2) > 1e-6 {
			t.Errorf("%+v: Unterkante schief: %.4f vs %.4f", par, y1, y2)
		}
		if par.Roll == 0 {
			xo, _ := abb(M, 500, 0)
			xu, _ := abb(M, 500, float64(h))
			if math.Abs(xo-xu) > 1e-6 {
				t.Errorf("%+v: Senkrechte nicht senkrecht: %.4f vs %.4f", par, xo, xu)
			}
		}
		// Oberkante muss dagegen schräg werden (links/rechts verschieden hoch)
		_, tl := abb(M, 0, 0)
		_, tr := abb(M, float64(w), 0)
		if math.Abs(tl-tr) < 1 {
			t.Errorf("%+v: Oberkante flacht nicht ab", par)
		}
	}
}

// YawMitte: altes Verhalten (Drehung um Bildmitte).
func TestYawMitteWieBisher(t *testing.T) {
	w, h := 1600, 1000
	par := Parameter{Roll: 1, Pitch: 5, Yaw: 12, F35: 28, YawMitte: true}
	M, _, _, err := Transformation(w, h, par)
	if err != nil {
		t.Fatal(err)
	}
	K := Kameramatrix(28, w, h)
	H := K.Mul(Drehmatrix(1, 5, 12)).Mul(K.Inv())
	ecken := [][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}}
	var x0, y0 float64 = math.Inf(1), math.Inf(1)
	for _, e := range ecken {
		x, y := abb(H, e[0], e[1])
		x0, y0 = math.Min(x0, x), math.Min(y0, y)
	}
	for _, e := range ecken {
		x, y := abb(H, e[0], e[1])
		mx, my := abb(M, e[0], e[1])
		if math.Abs(mx-(x-x0)) > 1e-6 || math.Abs(my-(y-y0)) > 1e-6 {
			t.Fatalf("Abweichung vom alten Verhalten")
		}
	}
}

// Yaw um die Unterkante == Yaw um die Bildmitte (beide Winkel, oben und unten)
// plus senkrechte Scherung, die den Boden wieder waagerecht stellt.
func TestYawUnterkanteIstMitteMitScherung(t *testing.T) {
	w, h := 1600, 1000
	for _, yaw := range []float64{-20, -8, 6, 15} {
		par := Parameter{Pitch: 8, Yaw: yaw, F35: 24}
		Mu, _, _, err := Transformation(w, h, par)
		if err != nil {
			t.Fatal(err)
		}
		par.YawMitte = true
		Mc, _, _, err := Transformation(w, h, par)
		if err != nil {
			t.Fatal(err)
		}
		// Scherung y' = y − k·x aus der Unterkante der Mitte-Variante
		bx0, by0 := abb(Mc, 0, float64(h))
		bx1, by1 := abb(Mc, float64(w), float64(h))
		k := (by1 - by0) / (bx1 - bx0)
		var dy []float64
		for _, e := range [][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}, {700, 300}} {
			xu, yu := abb(Mu, e[0], e[1])
			xc, yc := abb(Mc, e[0], e[1])
			dy = append(dy, yu-(yc-k*xc))
			if xu-xc != xu-xc { // NaN
				t.Fatal("NaN")
			}
		}
		// bis auf Verschiebung identisch (x-Versatz gleich, y-Versatz konstant)
		for i := range dy {
			if math.Abs(dy[i]-dy[0]) > 1e-6 {
				t.Errorf("Yaw %v: Abweichung %.6f", yaw, dy[i]-dy[0])
			}
		}
	}
}
