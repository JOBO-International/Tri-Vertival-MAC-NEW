// Package kern enthält die Rechenlogik der Perspektivkorrektur.
//
// Prinzip: Die Kamera war um Roll (Drehung um die Blickrichtung) und Pitch
// (Neigung nach oben/unten) verdreht. Sie wird rechnerisch in die Waagerechte
// zurückgedreht: H = K · R · K⁻¹ (K = Kameramatrix aus der Brennweite).
//
// Vorzeichen: Roll > 0 = Kamera im Uhrzeigersinn gedreht (von hinten gesehen),
// Pitch > 0 = Kamera nach oben geneigt, Yaw = virtuelle Drehung links/rechts.
package kern

import (
	"errors"
	"math"
)

// Mat3 ist eine 3×3-Matrix.
type Mat3 [3][3]float64

func (a Mat3) Mul(b Mat3) Mat3 {
	var c Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c[i][j] = a[i][0]*b[0][j] + a[i][1]*b[1][j] + a[i][2]*b[2][j]
		}
	}
	return c
}

func (a Mat3) MulVec(v [3]float64) [3]float64 {
	return [3]float64{
		a[0][0]*v[0] + a[0][1]*v[1] + a[0][2]*v[2],
		a[1][0]*v[0] + a[1][1]*v[1] + a[1][2]*v[2],
		a[2][0]*v[0] + a[2][1]*v[1] + a[2][2]*v[2],
	}
}

func (a Mat3) Inv() Mat3 {
	det := a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) -
		a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) +
		a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	var r Mat3
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

func rad(g float64) float64  { return g * math.Pi / 180 }
func grad(r float64) float64 { return r * 180 / math.Pi }

// BrennweitePx rechnet die Kleinbild-Brennweite über die Bilddiagonale in Pixel um.
func BrennweitePx(f35 float64, w, h int) float64 {
	return f35 * math.Hypot(float64(w), float64(h)) / math.Hypot(36, 24)
}

func Kameramatrix(f35 float64, w, h int) Mat3 {
	f := BrennweitePx(f35, w, h)
	return Mat3{{f, 0, float64(w) / 2}, {0, f, float64(h) / 2}, {0, 0, 1}}
}

// Drehmatrix = Ry(yaw) · Rx(pitch) · Rz(roll)
func Drehmatrix(roll, pitch, yaw float64) Mat3 {
	r, p, y := rad(roll), rad(pitch), rad(yaw)
	rz := Mat3{{math.Cos(r), -math.Sin(r), 0}, {math.Sin(r), math.Cos(r), 0}, {0, 0, 1}}
	rx := Mat3{{1, 0, 0}, {0, math.Cos(p), -math.Sin(p)}, {0, math.Sin(p), math.Cos(p)}}
	ry := Mat3{{math.Cos(y), 0, math.Sin(y)}, {0, 1, 0}, {-math.Sin(y), 0, math.Cos(y)}}
	return ry.Mul(rx).Mul(rz)
}

// KorrekturFehler: Winkel zu groß, Bild würde entarten.
var (
	FehlerWinkel    = errors.New("Winkel zu groß für diese Brennweite.")
	FehlerZuschnitt = errors.New("Nach dem Zuschneiden bleibt kein Bild übrig.")
	FehlerZuStark   = errors.New("Korrektur zu stark – das Bild würde extrem verzerrt.")
	FehlerKanten    = errors.New("Bitte mindestens zwei senkrechte Kanten markieren.")
	FehlerLinieKurz = errors.New("Eine Linie ist zu kurz.")
)

// Parameter einer Korrektur.
type Parameter struct {
	Roll, Pitch, Yaw float64
	F35              float64
	Zuschneiden      bool
	// YawMitte: Yaw dreht um die Bildmitte (Fluchtlinie in Bildmitte, wie die
	// automatische frontale Ausrichtung sie berechnet). Standard (false): Yaw
	// dreht um die Unterkante des bereits in Roll und Pitch korrigierten Bildes –
	// der Boden bleibt waagerecht, nur der obere Teil flacht ab.
	YawMitte bool
}

// homographie liefert H (Quelle → Ziel ohne Verschiebung) für p.
func homographie(w, h int, p Parameter) (Mat3, error) {
	K := Kameramatrix(p.F35, w, h)
	if p.Yaw == 0 || p.YawMitte {
		return K.Mul(Drehmatrix(p.Roll, p.Pitch, p.Yaw)).Mul(K.Inv()), nil
	}
	// 1. Roll und Pitch wie gewohnt
	H1 := K.Mul(Drehmatrix(p.Roll, p.Pitch, 0)).Mul(K.Inv())
	// 2. Unterkante des so korrigierten Bildes bestimmt den Drehpunkt (Horizont)
	ymax := math.Inf(-1)
	for _, e := range [4][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}} {
		t := H1.MulVec([3]float64{e[0], e[1], 1})
		if t[2] <= 1e-9 {
			return Mat3{}, FehlerWinkel
		}
		ymax = math.Max(ymax, t[1]/t[2])
	}
	Kb := K
	Kb[1][2] = ymax
	// 3. Yaw um die senkrechte Achse mit dem Horizont an der Unterkante
	return Kb.Mul(Drehmatrix(0, 0, p.Yaw)).Mul(Kb.Inv()).Mul(H1), nil
}

// Transformation liefert die Abbildungsmatrix (Quelle → Ziel) und die Zielgröße.
func Transformation(w, h int, p Parameter) (Mat3, int, int, error) {
	H, err := homographie(w, h, p)
	if err != nil {
		return Mat3{}, 0, 0, err
	}
	ecken := [4][2]float64{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}}
	var xs, ys [4]float64
	for i, e := range ecken {
		t := H.MulVec([3]float64{e[0], e[1], 1})
		if t[2] <= 1e-9 {
			return Mat3{}, 0, 0, FehlerWinkel
		}
		xs[i], ys[i] = t[0]/t[2], t[1]/t[2]
	}
	var x0, y0, x1, y1 float64
	if p.Zuschneiden {
		x0, y0, x1, y1 = GroesstesRechteck(xs, ys)
		if x1-x0 < 16 || y1-y0 < 16 {
			return Mat3{}, 0, 0, FehlerZuschnitt
		}
	} else {
		x0, x1, y0, y1 = minmax(xs[:])[0], minmax(xs[:])[1], minmax(ys[:])[0], minmax(ys[:])[1]
	}
	bw, bh := x1-x0, y1-y0
	if bw > 3*float64(w) || bh > 3*float64(h) {
		return Mat3{}, 0, 0, FehlerZuStark
	}
	T := Mat3{{1, 0, -x0}, {0, 1, -y0}, {0, 0, 1}}
	return T.Mul(H), int(math.Round(bw)), int(math.Round(bh)), nil
}

func minmax(v []float64) [2]float64 {
	lo, hi := v[0], v[0]
	for _, x := range v[1:] {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return [2]float64{lo, hi}
}

// raender: linker und rechter Rand des Vierecks auf Höhe y.
func raender(xs, ys [4]float64, y float64) (float64, float64) {
	l, r := math.Inf(1), math.Inf(-1)
	for i := 0; i < 4; i++ {
		xa, ya, xb, yb := xs[i], ys[i], xs[(i+1)%4], ys[(i+1)%4]
		if math.Abs(yb-ya) < 1e-9 {
			continue
		}
		t := (y - ya) / (yb - ya)
		if t >= -1e-9 && t <= 1+1e-9 {
			x := xa + t*(xb-xa)
			l, r = math.Min(l, x), math.Max(r, x)
		}
	}
	return l, r
}

// GroesstesRechteck: größtes achsparalleles Rechteck ohne Rand im (konvexen)
// Viereck. Zwischen zwei Höhen ist ein konvexes Viereck an einer der beiden
// am schmalsten – es genügt also, alle Höhenpaare zu prüfen.
func GroesstesRechteck(xs, ys [4]float64) (float64, float64, float64, float64) {
	ymin, ymax := minmax(ys[:])[0], minmax(ys[:])[1]
	suche := func(a []float64, b []float64) (float64, float64, float64, float64, float64) {
		la, ra := make([]float64, len(a)), make([]float64, len(a))
		lb, rb := make([]float64, len(b)), make([]float64, len(b))
		for i, y := range a {
			la[i], ra[i] = raender(xs, ys, y)
		}
		for j, y := range b {
			lb[j], rb[j] = raender(xs, ys, y)
		}
		best := -1.0
		var by0, by1, bx0, bx1 float64
		for i, y0 := range a {
			for j, y1 := range b {
				if y1 <= y0 {
					continue
				}
				L, R := math.Max(la[i], lb[j]), math.Min(ra[i], rb[j])
				if R <= L {
					continue
				}
				if f := (R - L) * (y1 - y0); f > best {
					best, by0, by1, bx0, bx1 = f, y0, y1, L, R
				}
			}
		}
		return by0, by1, bx0, bx1, best
	}
	n := 400
	ys1 := linspace(ymin, ymax, n)
	y0, y1, x0, x1, _ := suche(ys1, ys1)
	schritt := (ymax - ymin) / float64(n-1)
	for k := 0; k < 2; k++ {
		a := linspace(math.Max(ymin, y0-schritt), y0+schritt, 41)
		b := linspace(y1-schritt, math.Min(ymax, y1+schritt), 41)
		y0, y1, x0, x1, _ = suche(a, b)
		schritt /= 20
	}
	return x0 + 0.5, y0 + 0.5, x1 - 0.5, y1 - 0.5
}

func linspace(a, b float64, n int) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = a + (b-a)*float64(i)/float64(n-1)
	}
	return v
}

// Linie aus zwei Punkten.
type Linie struct{ X1, Y1, X2, Y2 float64 }

// WinkelAusSenkrechten bestimmt Roll und Pitch aus Kanten, die in Wirklichkeit senkrecht sind.
func WinkelAusSenkrechten(linien []Linie, w, h int, f35 float64) (float64, float64, error) {
	if len(linien) < 2 {
		return 0, 0, FehlerKanten
	}
	// Fluchtpunkt = Vektor, der L·v minimiert → kleinster Eigenvektor von LᵀL
	var A Mat3
	for _, l := range linien {
		a := [3]float64{l.X1, l.Y1, 1}
		b := [3]float64{l.X2, l.Y2, 1}
		c := kreuz(a, b)
		n := math.Hypot(c[0], c[1])
		if n < 1e-9 {
			return 0, 0, FehlerLinieKurz
		}
		for i := range c {
			c[i] /= n
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				A[i][j] += c[i] * c[j]
			}
		}
	}
	v := kleinsterEigenvektor(A)
	d := Kameramatrix(f35, w, h).Inv().MulVec(v)
	n := math.Sqrt(d[0]*d[0] + d[1]*d[1] + d[2]*d[2])
	for i := range d {
		d[i] /= n
	}
	if d[1] < 0 {
		d = [3]float64{-d[0], -d[1], -d[2]}
	}
	// Senkrechte im Kamerasystem: d = (sin r·cos p, cos r·cos p, −sin p)
	pitch := grad(math.Asin(math.Max(-1, math.Min(1, -d[2]))))
	roll := grad(math.Atan2(d[0], d[1]))
	return roll, pitch, nil
}

// NormiereRoll: Hochformat meldet ±90° – nur die Abweichung zählt.
func NormiereRoll(r float64) float64 {
	for r > 45 {
		r -= 90
	}
	for r < -45 {
		r += 90
	}
	return r
}

// RollAusHorizont: eine Kante, die waagerecht sein soll.
func RollAusHorizont(l Linie) float64 {
	x1, y1, x2, y2 := l.X1, l.Y1, l.X2, l.Y2
	if x2 < x1 {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}
	return NormiereRoll(-grad(math.Atan2(y2-y1, x2-x1)))
}

func kreuz(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// kleinsterEigenvektor einer symmetrischen 3×3-Matrix (Jacobi-Verfahren).
func kleinsterEigenvektor(a Mat3) [3]float64 {
	V := Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	for iter := 0; iter < 60; iter++ {
		p, q, maxv := 0, 1, math.Abs(a[0][1])
		if math.Abs(a[0][2]) > maxv {
			p, q, maxv = 0, 2, math.Abs(a[0][2])
		}
		if math.Abs(a[1][2]) > maxv {
			p, q, maxv = 1, 2, math.Abs(a[1][2])
		}
		if maxv < 1e-15 {
			break
		}
		theta := (a[q][q] - a[p][p]) / (2 * a[p][q])
		t := 1 / (math.Abs(theta) + math.Sqrt(theta*theta+1))
		if theta < 0 {
			t = -t
		}
		c := 1 / math.Sqrt(t*t+1)
		s := t * c
		var J Mat3 = Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
		J[p][p], J[q][q], J[p][q], J[q][p] = c, c, s, -s
		var Jt Mat3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				Jt[i][j] = J[j][i]
			}
		}
		a = Jt.Mul(a).Mul(J)
		V = V.Mul(J)
	}
	k := 0
	for i := 1; i < 3; i++ {
		if a[i][i] < a[k][k] {
			k = i
		}
	}
	return [3]float64{V[0][k], V[1][k], V[2][k]}
}
