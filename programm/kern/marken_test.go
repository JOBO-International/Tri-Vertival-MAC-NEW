package kern

import (
	"math"
	"testing"
)

// Szene: ideale (korrigierte) Punkte y → Quelle x = H⁻¹ y
func quellPunkte(ys [][2]float64, w, h int, f35, r, p, y float64) [][2]float64 {
	K := Kameramatrix(f35, w, h)
	Hi := K.Mul(Drehmatrix(r, p, y)).Mul(K.Inv()).Inv()
	var out [][2]float64
	for _, q := range ys {
		t := Hi.MulVec([3]float64{q[0], q[1], 1})
		out = append(out, [2]float64{t[0] / t[2], t[1] / t[2]})
	}
	return out
}

func pruefe(t *testing.T, name string, ms []Marke, f35wahr, fStart float64, frei bool, r, p, y float64, mitYaw bool) {
	l, err := LoeseMarken(ms, 4000, 3000, fStart, [3]float64{0, 0, 0}, frei)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	t.Logf("%-22s roll=%6.2f pitch=%6.2f yaw=%6.2f f35=%5.1f mehrdeutig=%v abw=%.3f", name, l.Roll, l.Pitch, l.Yaw, l.F35, l.Mehrdeutig, l.Abweichung)
	if l.Mehrdeutig {
		return
	}
	tol := 0.15
	if math.Abs(l.Roll-r) > tol || math.Abs(l.Pitch-p) > tol || (mitYaw && math.Abs(l.Yaw-y) > tol) {
		t.Errorf("%s: erwartet %.1f/%.1f/%.1f", name, r, p, y)
	}
	if frei && math.Abs(l.F35-f35wahr) > 1 {
		t.Errorf("%s: Brennweite %.1f statt %.1f", name, l.F35, f35wahr)
	}
}

func TestMarken(t *testing.T) {
	W, H := 4000, 3000
	r, p, y, f := 2.0, 14.0, -18.0, 28.0
	q := func(ys ...[2]float64) [][2]float64 { return quellPunkte(ys, W, H, f, r, p, y) }
	senk1 := Marke{Art: MarkeSenkrecht, P: q([2]float64{800, 500}, [2]float64{820 - 20, 2500})}
	senk2 := Marke{Art: MarkeSenkrecht, P: q([2]float64{3300, 400}, [2]float64{3300, 2600})}
	waag := Marke{Art: MarkeWaagerecht, P: q([2]float64{900, 2200}, [2]float64{3100, 2200})}
	ecke := Marke{Art: MarkeEcke, P: q([2]float64{1200, 800}, [2]float64{1200, 1600}, [2]float64{2400, 1600}), Verhaeltnis: 800.0 / 1200}
	recht := Marke{Art: MarkeRechteck, P: q([2]float64{1500, 700}, [2]float64{2600, 700}, [2]float64{2600, 1900}, [2]float64{1500, 1900})}
	var kw [][2]float64
	for i := 0; i < 7; i++ {
		a := float64(i) * 2 * math.Pi / 7
		kw = append(kw, [2]float64{2800 + 300*math.Cos(a), 900 + 300*math.Sin(a)})
	}
	kreisWand := Marke{Art: MarkeKreisWand, P: q(kw...)}
	// Kreis am Boden: 3D-Punkte unterhalb der (korrigierten) Kamera
	K := Kameramatrix(f, W, H)
	var kb [][2]float64
	for i := 0; i < 7; i++ {
		a := float64(i) * 2 * math.Pi / 7
		X := [3]float64{0.8 + 0.6*math.Cos(a), 1.6, 5 + 0.6*math.Sin(a)}
		t := K.MulVec(X)
		kb = append(kb, [2]float64{t[0] / t[2], t[1] / t[2]})
	}
	kreisBoden := Marke{Art: MarkeKreisBoden, P: q(kb...)}

	pruefe(t, "2 Senkrechte", []Marke{senk1, senk2}, f, f, false, r, p, y, false)
	pruefe(t, "Rechteck", []Marke{recht}, f, f, false, r, p, y, true)
	pruefe(t, "Ecke mit Verhältnis", []Marke{ecke}, f, f, false, r, p, y, true)
	pruefe(t, "Ecke + Senkrechte", []Marke{ecke, senk2}, f, f, false, r, p, y, true)
	pruefe(t, "Senkrechte+Waagerechte", []Marke{senk1, senk2, waag}, f, f, false, r, p, y, true)
	pruefe(t, "Kreis Wand + Senkrechte", []Marke{kreisWand, senk2}, f, f, false, r, p, y, true)
	pruefe(t, "Kreis Boden", []Marke{kreisBoden}, f, f, false, r, p, y, false)
	pruefe(t, "Brennweite frei", []Marke{recht, senk1, senk2, waag, ecke}, f, 40, true, r, p, y, true)
}
