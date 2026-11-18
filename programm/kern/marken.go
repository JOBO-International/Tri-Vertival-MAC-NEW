package kern

// Bezugsmarken: Der Nutzer legt fest, was in Wirklichkeit senkrecht,
// waagerecht, rechtwinklig, rechteckig oder kreisrund ist. Aus allen Marken
// zusammen werden Roll, Pitch, Yaw (und auf Wunsch die Brennweite) per
// Ausgleichsrechnung bestimmt – das Programm entscheidet nichts selbst.

import (
	"errors"
	"math"
	"sort"
)

// Arten von Bezugsmarken
const (
	MarkeSenkrecht  = "senkrecht"   // 2 Punkte
	MarkeWaagerecht = "waagerecht"  // 2 Punkte
	MarkeEcke       = "ecke"        // 3 Punkte: oben, Ecke, seitlich (rechter Winkel)
	MarkeRechteck   = "rechteck"    // 4 Punkte im Umlauf
	MarkeKreisBoden = "kreis_boden" // ≥ 5 Randpunkte, Kreis liegt waagerecht
	MarkeKreisWand  = "kreis_wand"  // ≥ 5 Randpunkte, Kreis an senkrechter Wand
)

// PunkteFuer: Anzahl der Klicks je Markenart.
func PunkteFuer(art string) int {
	switch art {
	case MarkeSenkrecht, MarkeWaagerecht:
		return 2
	case MarkeEcke:
		return 3
	case MarkeRechteck:
		return 4
	case MarkeKreisBoden, MarkeKreisWand:
		return 5
	}
	return 0
}

// Marke: eine Bezugsmarke in Pixelkoordinaten des Originalbildes.
type Marke struct {
	Art         string       `json:"art"`
	P           [][2]float64 `json:"p"`
	Verhaeltnis float64      `json:"verhaeltnis,omitempty"` // Ecke: senkrechte : waagerechte Seite (0 = unbekannt)
	// NurSenkrecht: bei Rechteck/Ecke nur die senkrechten Seiten auswerten
	// (Gebäude über Eck – Yaw bleibt frei)
	NurSenkrecht bool    `json:"nur_senkrecht,omitempty"`
	Auto         bool    `json:"auto,omitempty"` // von der Automatik gefunden
	Gewicht      float64 `json:"-"`              // Gewicht in der Ausgleichsrechnung (0 = 1)
}

// Loesung der Ausgleichsrechnung.
type Loesung struct {
	Roll, Pitch, Yaw, F35 float64
	Abweichung            []float64 // je Marke größte Abweichung in Grad
	YawBestimmt           bool      // Yaw wird durch die Marken festgelegt
	BrennweiteBestimmt    bool
	Mehrdeutig            bool // mehrere Lösungen passen gleich gut – weitere Marke setzen
	GleichGross           bool // Automatik: gleich große Rechtecke mit ausgewertet
}

var (
	FehlerKeineMarken = errors.New("Noch keine vollständige Bezugsmarke gesetzt.")
	FehlerKreis       = errors.New("Durch die Punkte passt keine Ellipse – bitte Punkte gleichmäßig auf dem Rand verteilen.")
)

// ------------------------------------------------------------------ Geometrie-Hilfen

// eigenSym: Eigenwerte/-vektoren einer symmetrischen n×n-Matrix (Jacobi-Verfahren).
// Rückgabe absteigend sortiert; Vektoren als Spalten v[i][k] (k = Eigenwert-Index).
func eigenSym(a [][]float64) ([]float64, [][]float64) {
	n := len(a)
	m := make([][]float64, n)
	v := make([][]float64, n)
	for i := range m {
		m[i] = append([]float64(nil), a[i]...)
		v[i] = make([]float64, n)
		v[i][i] = 1
	}
	for durch := 0; durch < 100; durch++ {
		var aus float64
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				aus += m[i][j] * m[i][j]
			}
		}
		if aus < 1e-22 {
			break
		}
		for p := 0; p < n; p++ {
			for q := p + 1; q < n; q++ {
				if math.Abs(m[p][q]) < 1e-300 {
					continue
				}
				theta := (m[q][q] - m[p][p]) / (2 * m[p][q])
				t := 1 / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				if theta < 0 {
					t = -t
				}
				c := 1 / math.Sqrt(t*t+1)
				s := t * c
				for k := 0; k < n; k++ { // Zeilen/Spalten p, q drehen
					mkp, mkq := m[k][p], m[k][q]
					m[k][p], m[k][q] = c*mkp-s*mkq, s*mkp+c*mkq
				}
				for k := 0; k < n; k++ {
					mpk, mqk := m[p][k], m[q][k]
					m[p][k], m[q][k] = c*mpk-s*mqk, s*mpk+c*mqk
				}
				for k := 0; k < n; k++ {
					vkp, vkq := v[k][p], v[k][q]
					v[k][p], v[k][q] = c*vkp-s*vkq, s*vkp+c*vkq
				}
			}
		}
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return m[idx[a]][idx[a]] > m[idx[b]][idx[b]] })
	werte := make([]float64, n)
	vek := make([][]float64, n)
	for i := range vek {
		vek[i] = make([]float64, n)
	}
	for k, j := range idx {
		werte[k] = m[j][j]
		for i := 0; i < n; i++ {
			vek[i][k] = v[i][j]
		}
	}
	return werte, vek
}

// KegelschnittAus: Ellipse durch ≥ 5 Punkte (kleinste Quadrate).
// Ergebnis C mit (x, y, 1) C (x, y, 1)ᵀ = 0 in Pixelkoordinaten.
func KegelschnittAus(pts [][2]float64) (Mat3, error) {
	if len(pts) < 5 {
		return Mat3{}, FehlerKreis
	}
	// normieren: Schwerpunkt 0, mittlerer Abstand √2
	var mx, my float64
	for _, p := range pts {
		mx += p[0]
		my += p[1]
	}
	mx /= float64(len(pts))
	my /= float64(len(pts))
	var d float64
	for _, p := range pts {
		d += math.Hypot(p[0]-mx, p[1]-my)
	}
	d /= float64(len(pts))
	if d < 1e-9 {
		return Mat3{}, FehlerKreis
	}
	s := math.Sqrt2 / d
	A := make([][]float64, 6)
	for i := range A {
		A[i] = make([]float64, 6)
	}
	for _, p := range pts {
		x, y := (p[0]-mx)*s, (p[1]-my)*s
		z := []float64{x * x, x * y, y * y, x, y, 1}
		for i := 0; i < 6; i++ {
			for j := 0; j < 6; j++ {
				A[i][j] += z[i] * z[j]
			}
		}
	}
	_, vek := eigenSym(A)
	c := make([]float64, 6)
	for i := 0; i < 6; i++ {
		c[i] = vek[i][5] // kleinster Eigenwert
	}
	a, b, cc := c[0], c[1], c[2]
	if b*b-4*a*cc >= 0 {
		return Mat3{}, FehlerKreis
	}
	Cn := Mat3{{a, b / 2, c[3] / 2}, {b / 2, cc, c[4] / 2}, {c[3] / 2, c[4] / 2, c[5]}}
	// zurück in Pixel: x_n = T x, C = Tᵀ Cn T
	T := Mat3{{s, 0, -s * mx}, {0, s, -s * my}, {0, 0, 1}}
	C := transp(T).Mul(Cn).Mul(T)
	return C, nil
}

func transp(a Mat3) Mat3 {
	var t Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			t[i][j] = a[j][i]
		}
	}
	return t
}

// EllipsenPunkte: Punkte auf der Ellipse C (zum Zeichnen).
func EllipsenPunkte(C Mat3, n int) [][2]float64 {
	a, b, c := C[0][0], 2*C[0][1], C[1][1]
	d, e, f := 2*C[0][2], 2*C[1][2], C[2][2]
	den := b*b - 4*a*c
	if den >= 0 {
		return nil
	}
	x0 := (2*c*d - b*e) / den
	y0 := (2*a*e - b*d) / den
	// Wert im Mittelpunkt
	f0 := a*x0*x0 + b*x0*y0 + c*y0*y0 + d*x0 + e*y0 + f
	M := [][]float64{{a, b / 2}, {b / 2, c}}
	werte, vek := eigenSym(M)
	if f0 == 0 || werte[0]*f0 >= 0 || werte[1]*f0 >= 0 {
		return nil
	}
	r1, r2 := math.Sqrt(-f0/werte[0]), math.Sqrt(-f0/werte[1])
	out := make([][2]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		t := 2 * math.Pi * float64(i) / float64(n)
		u, v := r1*math.Cos(t), r2*math.Sin(t)
		out = append(out, [2]float64{x0 + u*vek[0][0] + v*vek[0][1], y0 + u*vek[1][0] + v*vek[1][1]})
	}
	return out
}

// kreisNormalen: die beiden möglichen Flächennormalen (Kamerakoordinaten)
// eines Kreises, dessen Bild die Ellipse C ist.
func kreisNormalen(C Mat3, K Mat3) ([2][3]float64, bool) {
	Q := transp(K).Mul(C).Mul(K)
	q := [][]float64{{Q[0][0], Q[0][1], Q[0][2]}, {Q[1][0], Q[1][1], Q[1][2]}, {Q[2][0], Q[2][1], Q[2][2]}}
	w, v := eigenSym(q)
	// Kegel: zwei Eigenwerte gleichen Vorzeichens, einer entgegengesetzt
	if w[1] < 0 { // zwei negative → Vorzeichen umdrehen
		for i := range q {
			for j := range q[i] {
				q[i][j] = -q[i][j]
			}
		}
		w, v = eigenSym(q)
	}
	l1, l2, l3 := w[0], w[1], w[2]
	if !(l1 >= l2 && l2 > 0 && l3 < 0) {
		return [2][3]float64{}, false
	}
	a := math.Sqrt((l1 - l2) / (l1 - l3))
	b := math.Sqrt((l2 - l3) / (l1 - l3))
	var n [2][3]float64
	for i := 0; i < 3; i++ {
		n[0][i] = a*v[i][0] + b*v[i][2]
		n[1][i] = -a*v[i][0] + b*v[i][2]
	}
	return n, true
}

// ------------------------------------------------------------------ Ausgleichsrechnung

type markenModell struct {
	w, h    int
	marken  []Marke
	kegel   []Mat3 // je Kreis-Marke
	gruppen []int  // Residuum → Markenindex (−1 = Vorgabe)
	f350    float64
	frei    bool
	// gleichGross: Gruppen von Rechtecken (Indizes), die in Wirklichkeit gleich
	// groß sind (z. B. gleiche Fenster einer Fassade) – nach der Korrektur
	// sollen sie auch gleich groß erscheinen
	gleichGross [][]int
}

func winkelGrad(dx, dy float64, senkrecht bool) float64 {
	if senkrecht {
		if dy < 0 {
			dx, dy = -dx, -dy
		}
		return grad(math.Atan2(dx, dy))
	}
	if dx < 0 {
		dx, dy = -dx, -dy
	}
	return grad(math.Atan2(dy, dx))
}

// residuen in Grad für Parameter x = (roll, pitch, yaw[, ln f35]).
func (m *markenModell) residuen(x []float64, mitGruppen bool) []float64 {
	f35 := m.f350
	if m.frei {
		f35 = math.Exp(x[3])
	}
	K := Kameramatrix(f35, m.w, m.h)
	R := Drehmatrix(x[0], x[1], x[2])
	H := K.Mul(R).Mul(K.Inv())
	ab := func(p [2]float64) (float64, float64) {
		t := H.MulVec([3]float64{p[0], p[1], 1})
		if math.Abs(t[2]) < 1e-12 {
			t[2] = 1e-12
		}
		return t[0] / t[2], t[1] / t[2]
	}
	var r []float64
	var g []int
	add := func(v float64, i int) {
		if i >= 0 && m.marken[i].Gewicht > 0 && !mitGruppen {
			v *= m.marken[i].Gewicht
		}
		r = append(r, v)
		if mitGruppen {
			g = append(g, i)
		}
	}
	strecke := func(a, b [2]float64, senkrecht bool) float64 {
		x1, y1 := ab(a)
		x2, y2 := ab(b)
		return winkelGrad(x2-x1, y2-y1, senkrecht)
	}
	ki := 0
	for i, mk := range m.marken {
		switch mk.Art {
		case MarkeSenkrecht:
			add(strecke(mk.P[0], mk.P[1], true), i)
		case MarkeWaagerecht:
			add(strecke(mk.P[0], mk.P[1], false), i)
		case MarkeEcke:
			add(strecke(mk.P[0], mk.P[1], true), i)
			if mk.NurSenkrecht {
				continue
			}
			add(strecke(mk.P[1], mk.P[2], false), i)
			if mk.Verhaeltnis > 0 {
				x0, y0 := ab(mk.P[0])
				x1, y1 := ab(mk.P[1])
				x2, y2 := ab(mk.P[2])
				s := math.Hypot(x1-x0, y1-y0)
				w := math.Hypot(x2-x1, y2-y1)
				if s > 0 && w > 0 {
					add(grad(math.Log((s/w)/mk.Verhaeltnis)), i)
				}
			}
		case MarkeRechteck:
			// welches Seitenpaar ist im Bild eher senkrecht?
			p := mk.P
			senk := func(a, b [2]float64) float64 {
				return math.Abs(b[1]-a[1]) / (math.Hypot(b[0]-a[0], b[1]-a[1]) + 1e-9)
			}
			erstesSenk := senk(p[0], p[1])+senk(p[2], p[3]) > senk(p[1], p[2])+senk(p[3], p[0])
			if mk.NurSenkrecht {
				if erstesSenk {
					add(strecke(p[0], p[1], true), i)
					add(strecke(p[2], p[3], true), i)
				} else {
					add(strecke(p[1], p[2], true), i)
					add(strecke(p[3], p[0], true), i)
				}
				continue
			}
			add(strecke(p[0], p[1], erstesSenk), i)
			add(strecke(p[2], p[3], erstesSenk), i)
			add(strecke(p[1], p[2], !erstesSenk), i)
			add(strecke(p[3], p[0], !erstesSenk), i)
		case MarkeKreisBoden, MarkeKreisWand:
			C := m.kegel[ki]
			ki++
			ns, ok := kreisNormalen(C, K)
			if !ok {
				add(0, i)
				add(0, i)
				continue
			}
			besteA, besteB, besteS := 0.0, 0.0, math.Inf(1)
			for _, n := range ns {
				d := R.MulVec(n)
				var a, b float64
				if mk.Art == MarkeKreisBoden { // Normale soll senkrecht (Bild-y) stehen
					a = grad(math.Atan2(d[0], math.Abs(d[1])))
					b = grad(math.Atan2(d[2], math.Abs(d[1])))
				} else { // Normale soll in Blickrichtung zeigen (Wand frontal)
					a = grad(math.Atan2(d[0], math.Abs(d[2])))
					b = grad(math.Atan2(d[1], math.Abs(d[2])))
				}
				if a*a+b*b < besteS {
					besteA, besteB, besteS = a, b, a*a+b*b
				}
			}
			add(besteA, i)
			add(besteB, i)
		}
	}
	// gleich große Rechtecke: Breiten und Höhen nach der Korrektur vergleichen
	for _, gr := range m.gleichGross {
		var bs, hs []float64
		for _, i := range gr {
			p := m.marken[i].P
			var q [4][2]float64
			for k := 0; k < 4; k++ {
				q[k][0], q[k][1] = ab(p[k])
			}
			l := func(a, b [2]float64) float64 { return math.Hypot(b[0]-a[0], b[1]-a[1]) }
			s1 := (l(q[0], q[1]) + l(q[2], q[3])) / 2
			s2 := (l(q[1], q[2]) + l(q[3], q[0])) / 2
			bs, hs = append(bs, math.Log(s1+1e-9)), append(hs, math.Log(s2+1e-9))
		}
		mb, mh := 0.0, 0.0
		for k := range bs {
			mb += bs[k] / float64(len(bs))
			mh += hs[k] / float64(len(hs))
		}
		for k := range bs {
			add(grad(bs[k]-mb), -1)
			add(grad(hs[k]-mh), -1)
		}
	}
	// schwache Vorgaben: unbestimmte Winkel bleiben bei 0, Brennweite beim Ausgangswert
	const vorgabe = 0.002
	add(vorgabe*x[0], -1)
	add(vorgabe*x[1], -1)
	add(vorgabe*x[2], -1)
	if m.frei {
		add(vorgabe*grad(x[3]-math.Log(m.f350)), -1)
	}
	if mitGruppen {
		m.gruppen = g
	}
	return r
}

func summeQ(r []float64) float64 {
	var s float64
	for _, v := range r {
		s += v * v
	}
	return s
}

// levenbergMarquardt: kleinste Quadrate mit numerischer Ableitung.
func (m *markenModell) levenbergMarquardt(x []float64) ([]float64, float64) {
	n := len(x)
	r := m.residuen(x, false)
	s := summeQ(r)
	lambda := 1e-3
	for iter := 0; iter < 200; iter++ {
		// Jacobi-Matrix
		J := make([][]float64, len(r))
		for i := range J {
			J[i] = make([]float64, n)
		}
		for j := 0; j < n; j++ {
			h := 1e-4
			xp := append([]float64(nil), x...)
			xp[j] += h
			rp := m.residuen(xp, false)
			for i := range r {
				J[i][j] = (rp[i] - r[i]) / h
			}
		}
		// Normalgleichungen
		A := make([][]float64, n)
		gv := make([]float64, n)
		for a := 0; a < n; a++ {
			A[a] = make([]float64, n)
			for b := 0; b < n; b++ {
				for i := range r {
					A[a][b] += J[i][a] * J[i][b]
				}
			}
			for i := range r {
				gv[a] -= J[i][a] * r[i]
			}
		}
		verbessert := false
		for versuch := 0; versuch < 10; versuch++ {
			B := make([][]float64, n)
			for a := range B {
				B[a] = append([]float64(nil), A[a]...)
				B[a][a] *= 1 + lambda
				B[a][a] += 1e-12
			}
			dx, ok := loeseLGS(B, gv)
			if !ok {
				lambda *= 10
				continue
			}
			xn := make([]float64, n)
			for j := range x {
				xn[j] = x[j] + dx[j]
			}
			rn := m.residuen(xn, false)
			sn := summeQ(rn)
			if sn < s {
				fertig := s-sn < 1e-12*(1+s)
				x, r, s = xn, rn, sn
				lambda = math.Max(lambda/5, 1e-9)
				verbessert = true
				if fertig {
					return x, s
				}
				break
			}
			lambda *= 10
		}
		if !verbessert {
			break
		}
	}
	return x, s
}

// loeseLGS: Gauß-Elimination mit Pivotsuche.
func loeseLGS(A [][]float64, b []float64) ([]float64, bool) {
	n := len(b)
	M := make([][]float64, n)
	for i := range M {
		M[i] = append(append([]float64(nil), A[i]...), b[i])
	}
	for c := 0; c < n; c++ {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(M[r][c]) > math.Abs(M[p][c]) {
				p = r
			}
		}
		if math.Abs(M[p][c]) < 1e-15 {
			return nil, false
		}
		M[c], M[p] = M[p], M[c]
		for r := 0; r < n; r++ {
			if r == c {
				continue
			}
			f := M[r][c] / M[c][c]
			for k := c; k <= n; k++ {
				M[r][k] -= f * M[c][k]
			}
		}
	}
	x := make([]float64, n)
	for i := range x {
		x[i] = M[i][n] / M[i][i]
	}
	return x, true
}

// yawBestimmt: Legen die Marken den Yaw fest?
func yawBestimmt(marken []Marke) bool {
	for _, m := range marken {
		switch m.Art {
		case MarkeWaagerecht, MarkeKreisWand:
			return true
		case MarkeEcke, MarkeRechteck:
			if !m.NurSenkrecht {
				return true
			}
		}
	}
	return false
}

// LoeseMarken berechnet aus allen vollständigen Marken Roll, Pitch, Yaw und –
// falls brennweiteFrei und genügend Marken vorhanden – die Brennweite.
// start: Ausgangswerte (roll, pitch, yaw).
func LoeseMarken(marken []Marke, w, h int, f35 float64, start [3]float64, brennweiteFrei bool) (Loesung, error) {
	var ms []Marke
	var kegel []Mat3
	for _, mk := range marken {
		if len(mk.P) < PunkteFuer(mk.Art) || PunkteFuer(mk.Art) == 0 {
			continue
		}
		if mk.Art == MarkeKreisBoden || mk.Art == MarkeKreisWand {
			C, err := KegelschnittAus(mk.P)
			if err != nil {
				return Loesung{}, err
			}
			kegel = append(kegel, C)
		}
		ms = append(ms, mk)
	}
	if len(ms) == 0 {
		return Loesung{}, FehlerKeineMarken
	}
	// Brennweite nur frei, wenn genug Bedingungen (mind. 5 Messgrößen)
	anzahl := 0
	for _, mk := range ms {
		switch mk.Art {
		case MarkeSenkrecht, MarkeWaagerecht:
			anzahl++
		case MarkeEcke:
			anzahl += 2
			if mk.NurSenkrecht {
				anzahl--
			} else if mk.Verhaeltnis > 0 {
				anzahl++
			}
		case MarkeRechteck:
			anzahl += 4
			if mk.NurSenkrecht {
				anzahl -= 2
			}
		default:
			anzahl += 2
		}
	}
	frei := brennweiteFrei && anzahl >= 5 && yawBestimmt(ms)
	m := &markenModell{w: w, h: h, marken: ms, kegel: kegel, f350: f35, frei: frei}
	// mehrere Startwerte: Ausgangswerte, 0 und ein Raster – manche Marken
	// (z. B. eine einzelne Ecke) lassen mehrere exakte Lösungen zu
	starts := [][]float64{{start[0], start[1], start[2]}, {0, 0, 0}}
	for _, pi := range []float64{-30, -15, 15, 30} {
		for _, yi := range []float64{-35, 0, 35} {
			starts = append(starts, []float64{0, pi, yi})
		}
	}
	if frei {
		for i := range starts {
			starts[i] = append(starts[i], math.Log(f35))
		}
	}
	type erg struct {
		x []float64
		s float64
	}
	var ergs []erg
	bestS := math.Inf(1)
	for _, s0 := range starts {
		x, s := m.levenbergMarquardt(s0)
		ergs = append(ergs, erg{x, s})
		bestS = math.Min(bestS, s)
	}
	// unter den gleich guten Lösungen die nächste zu den Ausgangswerten
	abstand := func(x []float64) float64 {
		return math.Hypot(math.Hypot(x[0]-start[0], x[1]-start[1]), x[2]-start[2])
	}
	var bestX []float64
	var gute [][]float64
	for _, e := range ergs {
		if e.s <= bestS+0.05*float64(len(ms))+1e-6 {
			gute = append(gute, e.x)
			if bestX == nil || abstand(e.x) < abstand(bestX) {
				bestX = e.x
			}
		}
	}
	mehrdeutig := false
	for _, g := range gute {
		d := math.Abs(g[0]-bestX[0]) + math.Abs(g[1]-bestX[1])
		if yawBestimmt(ms) {
			d += math.Abs(g[2] - bestX[2])
		}
		if d > 1 {
			mehrdeutig = true
		}
	}
	l := Loesung{Roll: bestX[0], Pitch: bestX[1], Yaw: bestX[2], F35: f35, YawBestimmt: yawBestimmt(ms), BrennweiteBestimmt: frei, Mehrdeutig: mehrdeutig}
	if !l.YawBestimmt {
		// Yaw spielt für diese Marken keine Rolle – Ausgangswert behalten
		l.Yaw = start[2]
	}
	if frei {
		l.F35 = math.Exp(bestX[3])
	}
	if math.Abs(l.Roll) > 45 || math.Abs(l.Pitch) > 60 || math.Abs(l.Yaw) > 60 || l.F35 < 5 || l.F35 > 2000 {
		return l, errors.New("Die Marken passen nicht zusammen – bitte prüfen, ob sie richtig gesetzt sind.")
	}
	// Abweichung je Marke (Index in der Liste aller übergebenen Marken)
	r := m.residuen(bestX, true)
	abw := make([]float64, len(ms))
	for i, gi := range m.gruppen {
		if gi >= 0 {
			abw[gi] = math.Max(abw[gi], math.Abs(r[i]))
		}
	}
	// auf die ursprüngliche Liste abbilden (unvollständige Marken: 0)
	l.Abweichung = make([]float64, len(marken))
	j := 0
	for i, mk := range marken {
		if len(mk.P) < PunkteFuer(mk.Art) || PunkteFuer(mk.Art) == 0 {
			continue
		}
		l.Abweichung[i] = abw[j]
		j++
	}
	return l, nil
}

// MarkenAbweichungen: größte Abweichung (Grad) jeder Marke bei gegebener
// Korrektur. Unvollständige oder fehlerhafte Marken: +Inf.
func MarkenAbweichungen(marken []Marke, w, h int, f35, roll, pitch, yaw float64) []float64 {
	out := make([]float64, len(marken))
	for i, mk := range marken {
		out[i] = math.Inf(1)
		if len(mk.P) < PunkteFuer(mk.Art) || PunkteFuer(mk.Art) == 0 {
			continue
		}
		var kegel []Mat3
		if mk.Art == MarkeKreisBoden || mk.Art == MarkeKreisWand {
			C, err := KegelschnittAus(mk.P)
			if err != nil {
				continue
			}
			kegel = []Mat3{C}
		}
		m := &markenModell{w: w, h: h, marken: []Marke{mk}, kegel: kegel, f350: f35}
		r := m.residuen([]float64{roll, pitch, yaw}, true)
		a := 0.0
		for j, g := range m.gruppen {
			if g >= 0 {
				a = math.Max(a, math.Abs(r[j]))
			}
		}
		out[i] = a
	}
	return out
}

// VerfeinereGleichGross: Feinausgleich für die Automatik. Neben den Formen
// selbst wird verlangt, dass gleich große Rechtecke (gleiche Fenster) nach
// der Korrektur auch gleich groß sind – das bestimmt den Yaw deutlich genauer.
func VerfeinereGleichGross(marken []Marke, w, h int, f35 float64, start [3]float64) ([3]float64, bool) {
	m := &markenModell{w: w, h: h, marken: marken, f350: f35}
	for _, mk := range marken {
		if mk.Art == MarkeKreisBoden || mk.Art == MarkeKreisWand {
			C, err := KegelschnittAus(mk.P)
			if err != nil {
				return start, false
			}
			m.kegel = append(m.kegel, C)
		}
	}
	// Gruppen nach Größe im (vorläufig) korrigierten Bild
	K := Kameramatrix(f35, w, h)
	H := K.Mul(Drehmatrix(start[0], start[1], start[2])).Mul(K.Inv())
	type gr struct{ b, h float64 }
	masse := map[int]gr{}
	for i, mk := range marken {
		if mk.Art != MarkeRechteck || mk.NurSenkrecht {
			continue
		}
		var q [4][2]float64
		for k := 0; k < 4; k++ {
			t := H.MulVec([3]float64{mk.P[k][0], mk.P[k][1], 1})
			q[k] = [2]float64{t[0] / t[2], t[1] / t[2]}
		}
		l := func(a, b [2]float64) float64 { return math.Hypot(b[0]-a[0], b[1]-a[1]) }
		masse[i] = gr{(l(q[0], q[1]) + l(q[2], q[3])) / 2, (l(q[1], q[2]) + l(q[3], q[0])) / 2}
	}
	vergeben := map[int]bool{}
	for i := range marken {
		mi, ok := masse[i]
		if !ok || vergeben[i] {
			continue
		}
		g := []int{i}
		for j := range marken {
			mj, ok := masse[j]
			if !ok || j == i || vergeben[j] {
				continue
			}
			if math.Abs(math.Log(mj.b/mi.b)) < 0.05 && math.Abs(math.Log(mj.h/mi.h)) < 0.05 {
				g = append(g, j)
			}
		}
		if len(g) >= 3 {
			for _, j := range g {
				vergeben[j] = true
			}
			m.gleichGross = append(m.gleichGross, g)
		}
	}
	if len(m.gleichGross) == 0 {
		return start, false
	}
	x, _ := m.levenbergMarquardt([]float64{start[0], start[1], start[2]})
	if math.Abs(x[0]-start[0]) > 5 || math.Abs(x[1]-start[1]) > 5 || math.Abs(x[2]-start[2]) > 10 {
		return start, false
	}
	return [3]float64{x[0], x[1], x[2]}, true
}
