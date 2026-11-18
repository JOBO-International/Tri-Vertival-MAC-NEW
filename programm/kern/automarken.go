package kern

// Automatik mit Bezugsmarken: Vertical3 sucht im Bild geschlossene Formen –
// Rechtecke (Fenster, Türen, Tafeln, Fliesen …) und Ellipsen (runde Dinge) –
// und behandelt sie wie von Hand gesetzte Bezugsmarken. Gesucht werden
// Flächen (Bereiche gleicher Helligkeit) und ihre Umrisse; es werden keine
// einzelnen Linien oder Fluchtpunkte ausgewertet. Aus allen Formen, die zu
// einer gemeinsamen Lösung passen, wird die Korrektur berechnet.

import (
	"errors"
	"math"
	"math/rand"
	"sort"
)

var FehlerZuWenigFormen = errors.New("Es wurden zu wenige zusammenpassende Formen gefunden, um das Bild sicher auszurichten. Bitte Bezugsmarken von Hand setzen (z. B. ein großes Rechteck).")

var FehlerKeineFormen = errors.New("Im Bild wurden keine geeigneten Formen gefunden (Rechtecke wie Fenster, Türen, Tafeln oder runde Gegenstände). Bitte Bezugsmarken von Hand setzen.")

// formKandidat: gefundene Form in Arbeitskoordinaten.
type formKandidat struct {
	art     string
	p       [][2]float64
	flaeche float64
	kante   bool // aus der Kantensuche (etwas ungenauer)
}

// graustufen einer verkleinerten Kopie (längste Seite max).
func graustufen(b *Bild, max int) ([]float64, int, int, float64) {
	k := b.Verkleinert(max)
	g := make([]float64, k.W*k.H)
	for i := range g {
		g[i] = 0.299*float64(k.Pix[i*3]) + 0.587*float64(k.Pix[i*3+1]) + 0.114*float64(k.Pix[i*3+2])
	}
	return g, k.W, k.H, float64(b.W) / float64(k.W)
}

func weichzeichnen(g []float64, w, h int) []float64 {
	kern := []float64{1, 4, 6, 4, 1}
	t := make([]float64, len(g))
	o := make([]float64, len(g))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s, n float64
			for i, kv := range kern {
				xx := x + i - 2
				if xx >= 0 && xx < w {
					s += kv * g[y*w+xx]
					n += kv
				}
			}
			t[y*w+x] = s / n
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s, n float64
			for i, kv := range kern {
				yy := y + i - 2
				if yy >= 0 && yy < h {
					s += kv * t[yy*w+x]
					n += kv
				}
			}
			o[y*w+x] = s / n
		}
	}
	return o
}

// lokalerMittelwert über ein Fenster (Integralbild).
func lokalerMittelwert(g []float64, w, h, r int) []float64 {
	I := make([]float64, (w+1)*(h+1))
	for y := 0; y < h; y++ {
		var zeile float64
		for x := 0; x < w; x++ {
			zeile += g[y*w+x]
			I[(y+1)*(w+1)+x+1] = I[y*(w+1)+x+1] + zeile
		}
	}
	o := make([]float64, len(g))
	for y := 0; y < h; y++ {
		y0, y1 := max(0, y-r), min(h, y+r+1)
		for x := 0; x < w; x++ {
			x0, x1 := max(0, x-r), min(w, x+r+1)
			s := I[y1*(w+1)+x1] - I[y0*(w+1)+x1] - I[y1*(w+1)+x0] + I[y0*(w+1)+x0]
			o[y*w+x] = s / float64((y1-y0)*(x1-x0))
		}
	}
	return o
}

// bereiche: zusammenhängende Flächen (4er-Nachbarschaft) einer Maske.
// Liefert für jede Fläche ein Startpixel am Rand und die Größe.
type bereich struct {
	start          int
	anzahl         int
	x0, y0, x1, y1 int
}

func bereicheFinden(m []bool, w, h int, label []int32) []bereich {
	for i := range label {
		label[i] = 0
	}
	var out []bereich
	stapel := make([]int, 0, 1024)
	nr := int32(0)
	for i := range m {
		if !m[i] || label[i] != 0 {
			continue
		}
		nr++
		b := bereich{start: i, x0: w, y0: h, x1: -1, y1: -1}
		label[i] = nr
		stapel = append(stapel[:0], i)
		for len(stapel) > 0 {
			j := stapel[len(stapel)-1]
			stapel = stapel[:len(stapel)-1]
			b.anzahl++
			x, y := j%w, j/w
			b.x0, b.y0, b.x1, b.y1 = min(b.x0, x), min(b.y0, y), max(b.x1, x), max(b.y1, y)
			if j < b.start {
				b.start = j
			}
			if x > 0 && m[j-1] && label[j-1] == 0 {
				label[j-1] = nr
				stapel = append(stapel, j-1)
			}
			if x < w-1 && m[j+1] && label[j+1] == 0 {
				label[j+1] = nr
				stapel = append(stapel, j+1)
			}
			if y > 0 && m[j-w] && label[j-w] == 0 {
				label[j-w] = nr
				stapel = append(stapel, j-w)
			}
			if y < h-1 && m[j+w] && label[j+w] == 0 {
				label[j+w] = nr
				stapel = append(stapel, j+w)
			}
		}
		out = append(out, b)
	}
	return out
}

// umriss: äußere Begrenzung einer Fläche (Moore-Nachbarschaft), Start ist das
// oberste-linke Pixel der Fläche.
func umriss(label []int32, w, h int, start int) [][2]float64 {
	nr := label[start]
	drin := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && label[y*w+x] == nr }
	dx := []int{1, 1, 0, -1, -1, -1, 0, 1}
	dy := []int{0, 1, 1, 1, 0, -1, -1, -1}
	sx, sy := start%w, start/w
	x, y := sx, sy
	richtung := 7
	var pts [][2]float64
	for schritte := 0; schritte < 4*w*h/8+10000; schritte++ {
		pts = append(pts, [2]float64{float64(x) + 0.5, float64(y) + 0.5})
		gefunden := false
		r0 := (richtung + 6) % 8 // links hinten beginnen
		for k := 0; k < 8; k++ {
			r := (r0 + k) % 8
			if drin(x+dx[r], y+dy[r]) {
				x, y, richtung = x+dx[r], y+dy[r], r
				gefunden = true
				break
			}
		}
		if !gefunden || (x == sx && y == sy) {
			break
		}
	}
	return pts
}

// douglasPeucker vereinfacht einen geschlossenen Umriss.
func douglasPeucker(pts [][2]float64, eps float64) [][2]float64 {
	n := len(pts)
	if n < 4 {
		return pts
	}
	// zwei weit auseinanderliegende Punkte als Start
	a := 0
	b, bd := 0, -1.0
	for i, p := range pts {
		if d := math.Hypot(p[0]-pts[a][0], p[1]-pts[a][1]); d > bd {
			b, bd = i, d
		}
	}
	a2, ad := 0, -1.0
	for i, p := range pts {
		if d := math.Hypot(p[0]-pts[b][0], p[1]-pts[b][1]); d > ad {
			a2, ad = i, d
		}
	}
	a = a2
	if a > b {
		a, b = b, a
	}
	var dp func(seg [][2]float64) [][2]float64
	dp = func(seg [][2]float64) [][2]float64 {
		if len(seg) < 3 {
			return seg
		}
		p, q := seg[0], seg[len(seg)-1]
		lx, ly := q[0]-p[0], q[1]-p[1]
		l := math.Hypot(lx, ly) + 1e-12
		mi, md := 0, -1.0
		for i := 1; i < len(seg)-1; i++ {
			d := math.Abs((seg[i][0]-p[0])*ly-(seg[i][1]-p[1])*lx) / l
			if d > md {
				mi, md = i, d
			}
		}
		if md <= eps {
			return [][2]float64{p, q}
		}
		l1 := dp(seg[:mi+1])
		l2 := dp(seg[mi:])
		return append(l1[:len(l1)-1], l2...)
	}
	teil1 := dp(pts[a : b+1])
	rest := append(append([][2]float64(nil), pts[b:]...), pts[:a+1]...)
	teil2 := dp(rest)
	out := append(teil1[:len(teil1)-1], teil2[:len(teil2)-1]...)
	return out
}

func vieleckFlaeche(p [][2]float64) float64 {
	var s float64
	for i := range p {
		j := (i + 1) % len(p)
		s += p[i][0]*p[j][1] - p[j][0]*p[i][1]
	}
	return s / 2
}

// vierEck prüft, ob ein vereinfachter Umriss ein brauchbares Viereck ist.
func vierEck(q [][2]float64, flaeche float64) ([][2]float64, bool) {
	if len(q) != 4 {
		return nil, false
	}
	a := vieleckFlaeche(q)
	if a < 0 { // im Uhrzeigersinn (Bildkoordinaten: y nach unten)
		q[1], q[3] = q[3], q[1]
		a = -a
	}
	if a <= 0 || flaeche/a < 0.85 || flaeche/a > 1.15 {
		return nil, false
	}
	// konvex, Winkel 35°…145°, keine sehr kurzen Seiten
	for i := 0; i < 4; i++ {
		p0, p1, p2 := q[(i+3)%4], q[i], q[(i+1)%4]
		ax, ay := p0[0]-p1[0], p0[1]-p1[1]
		bx, by := p2[0]-p1[0], p2[1]-p1[1]
		la, lb := math.Hypot(ax, ay), math.Hypot(bx, by)
		if la < 8 || lb < 8 {
			return nil, false
		}
		kreuz := ax*by - ay*bx
		if kreuz > 0 { // nicht konvex
			return nil, false
		}
		w := grad(math.Acos(math.Max(-1, math.Min(1, (ax*bx+ay*by)/(la*lb)))))
		if w < 35 || w > 145 {
			return nil, false
		}
	}
	// Startecke: oben links
	best, bs := 0, math.Inf(1)
	for i, p := range q {
		if s := p[0] + p[1]; s < bs {
			best, bs = i, s
		}
	}
	out := make([][2]float64, 4)
	for i := range out {
		out[i] = q[(best+i)%4]
	}
	return out, true
}

// ellipsePruefen: Umriss passt gut zu einer Ellipse?
func ellipsePruefen(pts [][2]float64, flaeche float64) ([][2]float64, bool) {
	if len(pts) < 30 {
		return nil, false
	}
	// 5 gleichmäßig verteilte Punkte bilden die Marke, alle Punkte prüfen
	n := len(pts)
	auswahl := make([][2]float64, 0, 24)
	for i := 0; i < 24; i++ {
		auswahl = append(auswahl, pts[i*n/24])
	}
	C, err := KegelschnittAus(auswahl)
	if err != nil {
		return nil, false
	}
	// Ellipse? (Determinante der 2×2-Matrix positiv)
	a, b, c := C[0][0], C[0][1], C[1][1]
	if a*c-b*b <= 0 {
		return nil, false
	}
	// geometrische Abweichung näherungsweise |f| / |∇f|
	var summe, maxd float64
	for _, p := range pts {
		v := [3]float64{p[0], p[1], 1}
		Cv := C.MulVec(v)
		f := v[0]*Cv[0] + v[1]*Cv[1] + v[2]*Cv[2]
		gx, gy := 2*Cv[0], 2*Cv[1]
		d := math.Abs(f) / (math.Hypot(gx, gy) + 1e-12)
		summe += d
		maxd = math.Max(maxd, d)
	}
	mittel := summe / float64(n)
	groesse := math.Sqrt(flaeche)
	if mittel > 0.6+0.01*groesse || maxd > 2+0.05*groesse {
		return nil, false
	}
	// Fläche der Ellipse vergleichen (Kreis vs. abgerundetes Rechteck)
	ep := EllipsenPunkte(C, 72)
	if len(ep) < 10 {
		return nil, false
	}
	ea := math.Abs(vieleckFlaeche(ep))
	if ea <= 0 || flaeche/ea < 0.9 || flaeche/ea > 1.1 {
		return nil, false
	}
	marke := make([][2]float64, 5)
	for i := range marke {
		marke[i] = ep[i*72/5%len(ep)]
	}
	return marke, true
}

// ellipsenLage: Achsenverhältnis und Neigung der großen Achse (Grad).
func ellipsenLage(p [][2]float64) (verh, winkel float64) {
	C, err := KegelschnittAus(p)
	if err != nil {
		return 1, 0
	}
	a, b, c := C[0][0], C[0][1], C[1][1]
	tr, det := a+c, a*c-b*b
	d := math.Sqrt(math.Max(0, tr*tr/4-det))
	l1, l2 := tr/2+d, tr/2-d // Halbachsen ~ 1/sqrt(l)
	if l1 <= 0 || l2 <= 0 {
		return 1, 0
	}
	verh = math.Sqrt(l2 / l1)
	// Richtung der großen Achse: Eigenvektor zu l2
	winkel = grad(0.5 * math.Atan2(2*b, a-c))
	winkel += 90
	for winkel > 90 {
		winkel -= 180
	}
	return verh, winkel
}

// formenFinden sucht Rechtecke und Ellipsen; Ergebnis in Arbeitskoordinaten.
func formenFinden(g []float64, w, h int) []formKandidat {
	gw := weichzeichnen(g, w, h)
	gesamt := float64(w * h)
	minF, maxF := gesamt*0.0004, gesamt*0.5
	var kand []formKandidat
	maske := make([]bool, w*h)
	label := make([]int32, w*h)
	ausKanten := false
	auswerten := func() {
		for _, b := range bereicheFinden(maske, w, h, label) {
			f := float64(b.anzahl)
			if f < minF || f > maxF || b.x0 <= 1 || b.y0 <= 1 || b.x1 >= w-2 || b.y1 >= h-2 {
				continue
			}
			bw, bh := float64(b.x1-b.x0+1), float64(b.y1-b.y0+1)
			if bw < 10 || bh < 10 || f/(bw*bh) < 0.3 {
				continue
			}
			u := umriss(label, w, h, b.start)
			if len(u) < 20 {
				continue
			}
			umfang := 0.0
			for i := range u {
				j := (i + 1) % len(u)
				umfang += math.Hypot(u[j][0]-u[i][0], u[j][1]-u[i][1])
			}
			q := douglasPeucker(u, math.Max(1.5, 0.02*umfang))
			if v, ok := vierEck(q, f); ok {
				if vf, ok := eckenVerfeinern(v, u); ok {
					v = vf
				}
				kand = append(kand, formKandidat{MarkeRechteck, v, f, ausKanten})
				continue
			}
			if e, ok := ellipsePruefen(u, f); ok {
				kand = append(kand, formKandidat{"ellipse", e, f, ausKanten})
			}
		}
	}
	// 1. feste Helligkeitsstufen (wie bei einer Posterisierung)
	for _, stufe := range []float64{40, 70, 100, 130, 160, 190, 220} {
		for i, v := range gw {
			maske[i] = v < stufe
		}
		auswerten()
		for i, v := range gw {
			maske[i] = v >= stufe
		}
		auswerten()
	}
	// 2. lokal dunkler bzw. heller als die Umgebung
	for _, r := range []int{max(8, w/40), max(16, w/15)} {
		mw := lokalerMittelwert(gw, w, h, r)
		for _, c := range []float64{8, -8} {
			for i, v := range gw {
				if c > 0 {
					maske[i] = v < mw[i]-c
				} else {
					maske[i] = v > mw[i]-c
				}
			}
			auswerten()
		}
	}
	// 3. Flächen, die von Kanten umschlossen sind (Fensterglas im Rahmen,
	//    Schilder, Fliesen): Kantenstärke als Grenze, dazwischen die Flächen
	gx := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			sx := gw[i-w+1] + 2*gw[i+1] + gw[i+w+1] - gw[i-w-1] - 2*gw[i-1] - gw[i+w-1]
			sy := gw[i+w-1] + 2*gw[i+w] + gw[i+w+1] - gw[i-w-1] - 2*gw[i-w] - gw[i-w+1]
			gx[i] = math.Hypot(sx, sy)
		}
	}
	ausKanten = true
	sortiert := append([]float64(nil), gx...)
	sort.Float64s(sortiert)
	for _, anteil := range []float64{0.80, 0.90} {
		schwelle := math.Max(20, sortiert[int(anteil*float64(len(sortiert)-1))])
		for i, v := range gx {
			maske[i] = v < schwelle
		}
		auswerten()
	}
	// doppelte Funde zusammenfassen (gleiche Form aus mehreren Stufen)
	sort.Slice(kand, func(i, j int) bool {
		if kand[i].kante != kand[j].kante {
			return !kand[i].kante
		}
		return kand[i].flaeche > kand[j].flaeche
	})
	var out []formKandidat
	for _, k := range kand {
		doppelt := false
		for _, o := range out {
			if o.art != k.art {
				continue
			}
			var d float64
			if k.art == MarkeRechteck {
				for i := range k.p {
					d = math.Max(d, math.Hypot(k.p[i][0]-o.p[i][0], k.p[i][1]-o.p[i][1]))
				}
			} else {
				ck, co := mitte(k.p), mitte(o.p)
				d = math.Hypot(ck[0]-co[0], ck[1]-co[1])
			}
			if d < 0.03*math.Sqrt(k.flaeche)+3 {
				doppelt = true
				break
			}
		}
		if !doppelt {
			out = append(out, k)
		}
	}
	return out
}

func mitte(p [][2]float64) [2]float64 {
	var m [2]float64
	for _, q := range p {
		m[0] += q[0]
		m[1] += q[1]
	}
	m[0] /= float64(len(p))
	m[1] /= float64(len(p))
	return m
}

// AutoErgebnisMarken: Ergebnis der Automatik.
type AutoErgebnisMarken struct {
	Loesung  Loesung
	Marken   []Marke // verwendete Formen (Vollbild-Koordinaten)
	Gefunden int     // Anzahl gefundener Formen insgesamt
}

// AutoMarken findet Formen und berechnet daraus die Korrektur.
// Im frontalen Modus wird – wenn genügend Rechtecke da sind – auch die
// Brennweite mitbestimmt (Ergebnis in Loesung.F35, BrennweiteBestimmt).
func AutoMarken(b *Bild, f35 float64, frontal bool) (AutoErgebnisMarken, error) {
	if !frontal {
		return autoMarkenIntern(b, f35, false)
	}
	direkt, err1 := autoMarkenIntern(b, f35, true)
	senk, err2 := autoMarkenIntern(b, f35, false)
	if err2 == nil {
		var ms []Marke
		for _, m := range senk.Marken {
			if m.Art == MarkeRechteck {
				m.NurSenkrecht = false
				ms = append(ms, m)
			}
		}
		if l, gut, ok := frontalMitBrennweite(ms, b.W, b.H, f35, [3]float64{senk.Loesung.Roll, senk.Loesung.Pitch, 0}); ok {
			if err1 != nil || len(gut) > len(direkt.Marken) {
				return AutoErgebnisMarken{Loesung: l, Marken: gut, Gefunden: senk.Gefunden}, nil
			}
		}
	}
	return direkt, err1
}

// frontalMitBrennweite: Rechtecke vollständig auswerten, Brennweite frei.
func frontalMitBrennweite(ms []Marke, w, h int, f35 float64, start [3]float64) (Loesung, []Marke, bool) {
	if len(ms) < 3 {
		return Loesung{}, nil, false
	}
	alle := ms
	akt := ms
	var l Loesung
	for runde := 0; runde < 3; runde++ {
		var err error
		l, err = LoeseMarken(akt, w, h, f35, start, true)
		if err != nil || !l.BrennweiteBestimmt {
			return Loesung{}, nil, false
		}
		abw := MarkenAbweichungen(alle, w, h, l.F35, l.Roll, l.Pitch, l.Yaw)
		var neu []Marke
		for i, a := range abw {
			if a < 0.8 {
				neu = append(neu, alle[i])
			}
		}
		if len(neu) < 3 {
			return Loesung{}, nil, false
		}
		akt = neu
		start = [3]float64{l.Roll, l.Pitch, l.Yaw}
	}
	if l.F35 < 0.5*f35 || l.F35 > 2*f35 {
		return Loesung{}, nil, false
	}
	if x, ok := VerfeinereGleichGross(akt, w, h, l.F35, [3]float64{l.Roll, l.Pitch, l.Yaw}); ok {
		l.Roll, l.Pitch, l.Yaw = x[0], x[1], x[2]
		l.GleichGross = true
	}
	l.Abweichung = MarkenAbweichungen(akt, w, h, l.F35, l.Roll, l.Pitch, l.Yaw)
	return l, akt, true
}

// autoMarkenIntern: Formen finden und robust (Konsens) auswerten.
// frontal: Rechtecke vollständig auswerten (Fläche wird frontal, inkl. Yaw);
// sonst nur deren senkrechte Seiten (Gebäude über Eck – Yaw bleibt 0).
func autoMarkenIntern(b *Bild, f35 float64, frontal bool) (AutoErgebnisMarken, error) {
	g, w, h, skal := graustufen(b, 1400)
	kand := formenFinden(g, w, h)
	// in Marken umwandeln (Vollbild)
	var marken []Marke
	var gewicht []float64
	for _, k := range kand {
		p := make([][2]float64, len(k.p))
		for i, q := range k.p {
			p[i] = [2]float64{q[0] * skal, q[1] * skal}
		}
		gw := math.Sqrt(k.flaeche)
		if k.kante {
			gw *= 0.5
		}
		switch k.art {
		case MarkeRechteck:
			marken = append(marken, Marke{Art: MarkeRechteck, P: p, NurSenkrecht: !frontal, Auto: true})
			gewicht = append(gewicht, gw)
		case "ellipse":
			verh, winkel := ellipsenLage(k.p)
			// flache, liegende Ellipse → vermutlich am Boden; fast rund → an der Wand
			if verh < 0.8 && math.Abs(winkel) < 30 {
				marken = append(marken, Marke{Art: MarkeKreisBoden, P: p, Auto: true})
				gewicht = append(gewicht, gw*0.5)
			}
			if frontal && verh > 0.6 {
				marken = append(marken, Marke{Art: MarkeKreisWand, P: p, Auto: true})
				gewicht = append(gewicht, gw*0.5) // Kreisnormale ist empfindlicher als Rechteckseiten
			}
		}
	}
	if len(marken) == 0 {
		return AutoErgebnisMarken{}, FehlerKeineFormen
	}
	// Hypothesen aus den größten Formen (einzeln und paarweise), beste Übereinstimmung gewinnt
	idx := make([]int, len(marken))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, c int) bool { return gewicht[idx[a]] > gewicht[idx[c]] })
	var hypo [][]int
	n := min(len(idx), 20)
	for i := 0; i < n; i++ {
		hypo = append(hypo, []int{idx[i]})
	}
	zufall := rand.New(rand.NewSource(1))
	for t := 0; t < 25 && n >= 2; t++ {
		a, c := zufall.Intn(n), zufall.Intn(n)
		if a != c {
			hypo = append(hypo, []int{idx[a], idx[c]})
		}
	}
	// erlaubte Abweichung je Form: kleine Formen sind ungenauer
	grenzeVon := func(i int) float64 { return 0.6 + 25/gewicht[i] }
	bewerten := func(l Loesung) (float64, []int) {
		abw := MarkenAbweichungen(marken, b.W, b.H, f35, l.Roll, l.Pitch, l.Yaw)
		var s float64
		var drin []int
		for i, a := range abw {
			if gr := grenzeVon(i); a < gr {
				s += gewicht[i] * (1 - a/gr/2)
				drin = append(drin, i)
			}
		}
		return s, drin
	}
	besteS := -1.0
	var besteDrin []int
	for _, hy := range hypo {
		var ms []Marke
		for _, i := range hy {
			ms = append(ms, marken[i])
		}
		l, err := LoeseMarken(ms, b.W, b.H, f35, [3]float64{}, false)
		if err != nil {
			continue
		}
		if !frontal {
			l.Yaw = 0
		}
		s, drin := bewerten(l)
		if s > besteS {
			besteS, besteDrin = s, drin
		}
	}
	if len(besteDrin) == 0 {
		return AutoErgebnisMarken{Gefunden: len(marken)}, FehlerKeineFormen
	}
	// eine einzelne Form reicht nur, wenn sie groß ist (z. B. eine ganze Fassade)
	if len(besteDrin) < 3 {
		gross := false
		for _, i := range besteDrin {
			if gewicht[i]*gewicht[i] > 0.04*float64(w*h) {
				gross = true
			}
		}
		if !gross {
			return AutoErgebnisMarken{Gefunden: len(marken)}, FehlerZuWenigFormen
		}
	}
	// Ausgleich über alle passenden Formen, danach noch einmal auslesen
	auswahl := func(drin []int) []Marke {
		sort.Slice(drin, func(a, c int) bool { return gewicht[drin[a]] > gewicht[drin[c]] })
		// pro Ellipse nur eine Deutung (Boden oder Wand)
		var ms []Marke
		belegt := map[[2]float64]bool{}
		for _, i := range drin {
			m := marken[i]
			if m.Art == MarkeKreisBoden || m.Art == MarkeKreisWand {
				k := mitte(m.P)
				k = [2]float64{math.Round(k[0]), math.Round(k[1])}
				if belegt[k] {
					continue
				}
				belegt[k] = true
			}
			m.Gewicht = gewicht[i] / gewicht[drin[0]] * 2
			ms = append(ms, m)
			if len(ms) >= 30 { // übersichtlich bleiben
				break
			}
		}
		return ms
	}
	ms := auswahl(besteDrin)
	l, err := LoeseMarken(ms, b.W, b.H, f35, [3]float64{}, false)
	if err != nil {
		return AutoErgebnisMarken{Gefunden: len(marken)}, err
	}
	// Ausreißer nach dem Ausgleich entfernen und neu rechnen
	var gut []Marke
	for i, a := range l.Abweichung {
		gr := 0.3 + 12/(ms[i].Gewicht/2*gewicht[besteDrin[0]])
		if a < gr {
			gut = append(gut, ms[i])
		}
	}
	if len(gut) > 0 && len(gut) < len(ms) {
		if l2, err := LoeseMarken(gut, b.W, b.H, f35, [3]float64{l.Roll, l.Pitch, l.Yaw}, false); err == nil {
			l, ms = l2, gut
		}
	}
	if !frontal {
		l.Yaw = 0 // nur senkrechte Seiten ausgewertet – Yaw bleibt unverändert
	} else if x, ok := VerfeinereGleichGross(ms, b.W, b.H, f35, [3]float64{l.Roll, l.Pitch, l.Yaw}); ok {
		l.Roll, l.Pitch, l.Yaw = x[0], x[1], x[2]
		l.GleichGross = true
	}
	l.Abweichung = MarkenAbweichungen(ms, b.W, b.H, f35, l.Roll, l.Pitch, l.Yaw)
	return AutoErgebnisMarken{Loesung: l, Marken: ms, Gefunden: len(marken)}, nil
}

// eckenVerfeinern: Geraden durch die Umrisspunkte jeder Seite legen und
// die Ecken als Schnittpunkte berechnen (genauer als ein Pixel).
func eckenVerfeinern(q [][2]float64, u [][2]float64) ([][2]float64, bool) {
	type gerade struct{ nx, ny, d float64 } // nx*x + ny*y = d
	var gs [4]gerade
	for i := 0; i < 4; i++ {
		a, b := q[i], q[(i+1)%4]
		lx, ly := b[0]-a[0], b[1]-a[1]
		l := math.Hypot(lx, ly)
		if l < 1 {
			return nil, false
		}
		ux, uy := lx/l, ly/l
		var pts [][2]float64
		for _, p := range u {
			t := ((p[0]-a[0])*ux + (p[1]-a[1])*uy) / l
			d := math.Abs((p[0]-a[0])*uy - (p[1]-a[1])*ux)
			if t > 0.12 && t < 0.88 && d < 2+0.02*l {
				pts = append(pts, p)
			}
		}
		if len(pts) < 5 {
			return nil, false
		}
		// Hauptachse der Punkte (kleinste Quadrate senkrecht zur Geraden)
		m := mitte(pts)
		var sxx, sxy, syy float64
		for _, p := range pts {
			dx, dy := p[0]-m[0], p[1]-m[1]
			sxx += dx * dx
			sxy += dx * dy
			syy += dy * dy
		}
		w := 0.5 * math.Atan2(2*sxy, sxx-syy) // Richtung der Geraden
		nx, ny := -math.Sin(w), math.Cos(w)
		gs[i] = gerade{nx, ny, nx*m[0] + ny*m[1]}
	}
	out := make([][2]float64, 4)
	for i := 0; i < 4; i++ {
		g1, g2 := gs[(i+3)%4], gs[i] // Seite davor und danach
		det := g1.nx*g2.ny - g1.ny*g2.nx
		if math.Abs(det) < 1e-6 {
			return nil, false
		}
		x := (g1.d*g2.ny - g1.ny*g2.d) / det
		y := (g1.nx*g2.d - g1.d*g2.nx) / det
		if math.Hypot(x-q[i][0], y-q[i][1]) > 6 {
			return nil, false
		}
		out[i] = [2]float64{x, y}
	}
	return out, true
}
