package kern

// Automatik über die Richtungsstatistik des ganzen Bildes.
//
// Für jeden Bildpunkt mit deutlicher Helligkeitsänderung wird die örtliche
// Kantenrichtung bestimmt (Strukturtensor, 5×5-Umgebung). Es werden KEINE
// Linien(-stücke) gesucht und KEINE Fluchtpunkte berechnet. Gesucht sind die
// Kamerawinkel, bei denen nach der Korrektur möglichst viel „Richtungsmasse“
// genau senkrecht (und im Frontal-Modus zusätzlich genau waagerecht) steht –
// ein statistisches Gütemaß über alle Bildpunkte (vgl. „Manhattan World“,
// Coughlan & Yuille 1999).

import (
	"errors"
	"math"
	"sort"
)

var arbeitsGroesse = 1000

var FehlerKeineRichtung = errors.New("Im Bild ist keine eindeutige senkrechte Struktur zu erkennen (z. B. Landschaft, Porträt, Pflanzen). Bitte von Hand ausrichten oder Bezugsmarken setzen.")

// richtungsProbe: ein Bildpunkt mit seiner Kantenrichtung (Kamerasystem).
type richtungsProbe struct {
	r, d [3]float64 // Sehstrahl und Richtung entlang der Kante (K⁻¹-normiert)
	w    float64
}

// OrientierungsErgebnis der Automatik.
type OrientierungsErgebnis struct {
	Roll, Pitch, Yaw float64
	Guete            float64 // Anteil der Kantenmasse, der nach der Korrektur senkrecht/waagerecht steht (0..1)
	NurRoll          bool    // keine senkrechten Strukturen – nur nach dem Horizont gedreht
	Unsicher         bool
	Proben           [][2]float64 // Bildpunkte, die nach der Korrektur senkrecht stehen (Vollbild, zur Anzeige)
}

// richtungsProben: Kantenpunkte mit Richtung und Gewicht aus einer Arbeitskopie.
func richtungsProben(b *Bild, f35 float64) ([]richtungsProbe, [][2]float64, int, int, float64) {
	g, w, h, skal := graustufenGanzzahlig(b, arbeitsGroesse)
	g = weichzeichnen(g, w, h)
	gx := make([]float64, w*h)
	gy := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			// Scharr-Operator: richtungstreuer als Sobel
			gx[i] = 3*(g[i-w+1]+g[i+w+1]) + 10*g[i+1] - 3*(g[i-w-1]+g[i+w-1]) - 10*g[i-1]
			gy[i] = 3*(g[i+w-1]+g[i+w+1]) + 10*g[i+w] - 3*(g[i-w-1]+g[i-w+1]) - 10*g[i-w]
		}
	}
	// Strukturtensor, mit Binomialfilter (nahezu richtungsunabhängig) geglättet
	jxx := make([]float64, w*h)
	jxy := make([]float64, w*h)
	jyy := make([]float64, w*h)
	for i := range gx {
		jxx[i], jxy[i], jyy[i] = gx[i]*gx[i], gx[i]*gy[i], gy[i]*gy[i]
	}
	box := func(a []float64) []float64 {
		t := make([]float64, w*h)
		o := make([]float64, w*h)
		for y := 0; y < h; y++ {
			for x := 2; x < w-2; x++ {
				i := y*w + x
				t[i] = a[i-2] + 4*a[i-1] + 6*a[i] + 4*a[i+1] + a[i+2]
			}
		}
		for y := 2; y < h-2; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				o[i] = t[i-2*w] + 4*t[i-w] + 6*t[i] + 4*t[i+w] + t[i+2*w]
			}
		}
		return o
	}
	for k := 0; k < tensorGlaettung; k++ {
		jxx, jxy, jyy = box(jxx), box(jxy), box(jyy)
	}
	// Zweite, größere Umgebung: Blattwerk und Gras haben nur im Kleinen eine
	// Richtung, Bauwerkskanten auch im Größeren. So zählen Bäume vor einer
	// Fassade nicht mehr als „Zufallsrichtungen“ gegen die Fassade.
	gxx, gxy, gyy := jxx, jxy, jyy
	for k := 0; k < grossGlaettung; k++ {
		gxx, gxy, gyy = box(gxx), box(gxy), box(gyy)
	}
	// Schwelle: die kräftigsten 15 % der Punkte
	energie := make([]float64, 0, w*h/4)
	for y := 4; y < h-4; y += 2 {
		for x := 4; x < w-4; x += 2 {
			energie = append(energie, jxx[y*w+x]+jyy[y*w+x])
		}
	}
	if len(energie) == 0 {
		return nil, nil, w, h, skal
	}
	sort.Float64s(energie)
	schwelle := math.Max(energie[int(float64(len(energie))*schwellAnteil)], 1e-6)
	Ki := Kameramatrix(f35, b.W, b.H).Inv() // im Vollbild rechnen
	var proben []richtungsProbe
	var orte [][2]float64
	for y := 4; y < h-4; y += 2 {
		for x := 4; x < w-4; x += 2 {
			i := y*w + x
			e := jxx[i] + jyy[i]
			if e < schwelle {
				continue
			}
			diff := math.Hypot(jxx[i]-jyy[i], 2*jxy[i])
			koh := diff / (e + 1e-9) // 1 = eine klare Richtung, 0 = Textur
			if koh < kohMin {
				continue
			}
			eg := gxx[i] + gyy[i]
			kohG := math.Hypot(gxx[i]-gyy[i], 2*gxy[i]) / (eg + 1e-9)
			if kohG < kohGrossMin {
				continue
			}
			// Gradientenrichtung → Kantenrichtung (90° gedreht)
			th := 0.5*math.Atan2(2*jxy[i], jxx[i]-jyy[i]) + math.Pi/2
			tx, ty := math.Cos(th), math.Sin(th)
			xv, yv := (float64(x)+0.5)*skal, (float64(y)+0.5)*skal
			r := Ki.MulVec([3]float64{xv, yv, 1})
			d := Ki.MulVec([3]float64{tx, ty, 0})
			proben = append(proben, richtungsProbe{r: r, d: d, w: koh * koh * kohG * kohG * math.Sqrt(math.Min(e/schwelle, 25))})
			orte = append(orte, [2]float64{xv, yv})
		}
	}
	return proben, orte, w, h, skal
}

// winkelFehler: Abweichung der Kante von senkrecht bzw. waagerecht (Grad) nach Drehung R.
func winkelFehler(p richtungsProbe, R Mat3) (senk, waag float64, ok bool) {
	X := R.MulVec(p.r)
	if X[2] <= 1e-6 {
		return 0, 0, false
	}
	D := R.MulVec(p.d)
	vx := D[0]*X[2] - X[0]*D[2]
	vy := D[1]*X[2] - X[1]*D[2]
	ax, ay := math.Abs(vx), math.Abs(vy)
	return grad(math.Atan2(ax, ay)), grad(math.Atan2(ay, ax)), true
}

type orientModell struct {
	proben        []richtungsProbe
	senk, waag    float64 // Gewichte der beiden Anteile
	sigma         float64
	summe         float64
	vorwahlPitch  float64
	schrittProben int
	vorliebe      float64 // Vorliebe für kleine Korrekturen (nur Grobsuche)
}

func (m *orientModell) wert(roll, pitch, yaw float64) float64 {
	R := Drehmatrix(roll, pitch, yaw)
	s2 := 2 * m.sigma * m.sigma
	var s float64
	schritt := max(1, m.schrittProben)
	for i := 0; i < len(m.proben); i += schritt {
		p := m.proben[i]
		a, b, ok := winkelFehler(p, R)
		if !ok {
			continue
		}
		if m.senk > 0 && a < 4*m.sigma {
			s += m.senk * p.w * math.Exp(-a*a/s2)
		}
		if m.waag > 0 && b < 4*m.sigma {
			s += m.waag * p.w * math.Exp(-b*b/s2)
		}
	}
	// leichte Vorliebe für kleine Korrekturen (bei gleich guten Lösungen)
	return s*float64(schritt) - m.vorliebe*m.summe*(roll*roll+pitch*pitch+yaw*yaw)
}

// grenzen der Suche: Roll, Pitch, Yaw (Grad)
var grenzen = [3]float64{15, 60, 40}

// schwellAnteil: Anteil der schwächsten Kantenpunkte, die verworfen werden
var schwellAnteil = 0.75

// waagerechtGewicht: Gewicht der waagerechten Kanten neben den senkrechten bei
// der Ausrichtung ohne Yaw (Roll und Pitch). 0 = nur senkrechte Kanten.
var waagerechtGewicht = 0.5

// waagSpitzeMin: Die waagerechten Kanten zählen nur, wenn sie nach der
// senkrechten Ausrichtung eine klare Spitze bilden (sonst: Zufall, Rundungen).
var waagSpitzeMin = 0.0

var tensorGlaettung = 3
var kohMin = 0.7

// Mindest-Kohärenz in der größeren Umgebung (grossGlaettung Durchgänge)
var kohGrossMin = 0.4
var grossGlaettung = 8

// verfeinern: Mustersuche mit kleiner werdender Schrittweite (innerhalb der Grenzen).
func (m *orientModell) verfeinern(x [3]float64, frei [3]bool, schritt, ende float64) [3]float64 {
	best := m.wert(x[0], x[1], x[2])
	for schritt >= ende {
		besser := false
		for k := 0; k < 3; k++ {
			if !frei[k] {
				continue
			}
			for _, s := range []float64{schritt, -schritt} {
				y := x
				y[k] += s
				if math.Abs(y[k]) > grenzen[k] {
					continue
				}
				if v := m.wert(y[0], y[1], y[2]); v > best {
					best, x, besser = v, y, true
				}
			}
		}
		if !besser {
			schritt /= 2
		}
	}
	return x
}

// AutoOrientierung bestimmt Roll und Pitch (frontal: auch Yaw) aus der
// Richtungsstatistik des Bildes.
func AutoOrientierung(b *Bild, f35 float64, frontal bool) (OrientierungsErgebnis, error) {
	proben, orte, _, _, _ := richtungsProben(b, f35)
	if len(proben) < 200 {
		return OrientierungsErgebnis{}, FehlerKeineRichtung
	}
	var summe float64
	for _, p := range proben {
		summe += p.w
	}
	grob := max(1, len(proben)/6000)

	anteil := func(x [3]float64, senk, waag bool) float64 {
		R := Drehmatrix(x[0], x[1], x[2])
		var s float64
		for _, p := range proben {
			a, b, ok := winkelFehler(p, R)
			if !ok {
				continue
			}
			if (senk && a < 1) || (waag && b < 1) {
				s += p.w
			}
		}
		return s / summe
	}
	// Spitze statt Zufall: Dichte innerhalb 1° gegenüber der Dichte 2–8° daneben
	spitze := func(x [3]float64, senk bool) float64 {
		R := Drehmatrix(x[0], x[1], x[2])
		var innen, ring float64
		for _, p := range proben {
			a, b, ok := winkelFehler(p, R)
			if !ok {
				continue
			}
			if !senk {
				a = b
			}
			if a < 1 {
				innen += p.w
			} else if a >= 2 && a < 8 {
				ring += p.w
			}
		}
		return (innen / 1) / (ring/6 + 1e-9)
	}

	// 1. Suche Roll/Pitch nur über senkrechte Anteile – zuerst bis ±45°,
	// nur wenn dort nichts Eindeutiges liegt, bis ±60° (steile Hochhausaufnahmen)
	suche := func(pmax float64) ([3]float64, float64, float64, bool) {
		grenzen[1] = pmax
		m := &orientModell{proben: proben, senk: 1, sigma: 2.5, summe: summe, schrittProben: grob, vorliebe: 1e-5}
		bestV, bx := -math.MaxFloat64, [3]float64{}
		for r := -15.0; r <= 15; r += 1 {
			for p := -pmax; p <= pmax; p += 1 {
				if v := m.wert(r, p, 0); v > bestV {
					bestV, bx = v, [3]float64{r, p, 0}
				}
			}
		}
		m.schrittProben, m.vorliebe = 1, 0
		m.sigma = 1.5
		bx = m.verfeinern(bx, [3]bool{true, true, false}, 0.5, 0.02)
		m.sigma = 0.8
		bx = m.verfeinern(bx, [3]bool{true, true, false}, 0.2, 0.01)
		m.sigma = 0.5
		bx = m.verfeinern(bx, [3]bool{true, true, false}, 0.1, 0.005)
		g, sp := anteil(bx, true, false), spitze(bx, true)
		amRand := math.Abs(bx[1]) > pmax-1 || math.Abs(bx[0]) > 14.5
		return bx, g, sp, !amRand && g >= 0.015 && sp >= 2.5
	}
	bx, g, _, gut := suche(45)
	if !gut {
		if bx2, g2, _, gut2 := suche(60); gut2 {
			bx, g, gut = bx2, g2, true
		}
	}
	grenzen[1] = 60
	e := OrientierungsErgebnis{Roll: bx[0], Pitch: bx[1], Guete: g}
	keineSenkrechten := !gut

	if keineSenkrechten && !frontal {
		// nur nach waagerechten Strukturen (Horizont) drehen
		mh := &orientModell{proben: proben, waag: 1, sigma: 1.5, summe: summe}
		bestV, br := -math.MaxFloat64, 0.0
		for r := -15.0; r <= 15; r += 0.5 {
			if v := mh.wert(r, 0, 0); v > bestV {
				bestV, br = v, r
			}
		}
		x := mh.verfeinern([3]float64{br, 0, 0}, [3]bool{true, false, false}, 0.25, 0.01)
		g := anteil(x, false, true)
		sp := spitze(x, false)
		if g < 0.05 || sp < 2 {
			return OrientierungsErgebnis{}, FehlerKeineRichtung
		}
		e = OrientierungsErgebnis{Roll: x[0], Guete: g, NurRoll: true, Unsicher: g < 0.1}
	} else if keineSenkrechten && frontal {
		return OrientierungsErgebnis{}, FehlerKeineRichtung
	}

	mitWaag := false
	if !frontal && !keineSenkrechten && waagerechtGewicht > 0 && spitze(bx, false) >= waagSpitzeMin {
		// 1b. Ohne Yaw: waagerechte Kanten helfen mit, Roll und Pitch zu verfeinern.
		// Nur Kanten nahe der Waagerechten zählen (Abstand < 4σ); schräg
		// zulaufende Kanten einer Ansicht von der Seite bleiben ohne Einfluss.
		mw := &orientModell{proben: proben, senk: 1, waag: waagerechtGewicht, sigma: 1.5, summe: summe}
		x := mw.verfeinern(bx, [3]bool{true, true, false}, 0.5, 0.02)
		mw.sigma = 0.8
		x = mw.verfeinern(x, [3]bool{true, true, false}, 0.2, 0.01)
		// Die senkrechte Ausrichtung darf dadurch nur wenig leiden
		if anteil(x, true, false) >= 0.9*anteil(bx, true, false) && math.Abs(x[0]-bx[0]) <= 3 && math.Abs(x[1]-bx[1]) <= 3 {
			e.Roll, e.Pitch = x[0], x[1]
			bx = x
			mitWaag = anteil(x, false, true) > 0.02
		}
	}

	if frontal {
		// 2. Yaw: senkrechte und waagerechte Anteile gemeinsam
		mf := &orientModell{proben: proben, senk: 1, waag: 1, sigma: 2, summe: summe, schrittProben: grob, vorliebe: 1e-5}
		bestV, by := -math.MaxFloat64, 0.0
		for y := -40.0; y <= 40; y += 1 {
			if v := mf.wert(bx[0], bx[1], y); v > bestV {
				bestV, by = v, y
			}
		}
		mf.schrittProben, mf.vorliebe = 1, 0
		mf.sigma = 1.2
		x := mf.verfeinern([3]float64{bx[0], bx[1], by}, [3]bool{true, true, true}, 0.5, 0.02)
		mf.sigma = 0.8
		x = mf.verfeinern(x, [3]bool{true, true, true}, 0.2, 0.01)
		e.Roll, e.Pitch, e.Yaw = x[0], x[1], x[2]
		e.Guete = anteil(x, true, true)
		bx = x
	}
	e.Unsicher = e.Unsicher || e.Guete < 0.04 || math.Abs(e.Pitch) > 50 || math.Abs(e.Roll) > 14.5
	// Punkte, die nach der Korrektur senkrecht stehen (zur Anzeige, ausgedünnt)
	R := Drehmatrix(e.Roll, e.Pitch, e.Yaw)
	for i, p := range proben {
		if a, b, ok := winkelFehler(p, R); ok && (a < 0.7 || ((frontal || mitWaag) && b < 0.7)) && i%2 == 0 {
			e.Proben = append(e.Proben, orte[i])
		}
	}
	return e, nil
}

// graustufenGanzzahlig: Verkleinerung um einen ganzzahligen Faktor (n×n-Blöcke).
// Gleich große Blöcke verfälschen die Kantenrichtungen nicht – bei
// krummen Faktoren entstehen Treppen, die Kanten leicht zur Bildachse ziehen.
func graustufenGanzzahlig(b *Bild, ziel int) ([]float64, int, int, float64) {
	n := int(math.Ceil(float64(max(b.W, b.H)) / float64(ziel)))
	if n < 1 {
		n = 1
	}
	w, h := b.W/n, b.H/n
	g := make([]float64, w*h)
	parallel(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				var s float64
				for yy := y * n; yy < y*n+n; yy++ {
					row := b.Pix[(yy*b.W+x*n)*3:]
					for xx := 0; xx < n; xx++ {
						s += 0.299*float64(row[xx*3]) + 0.587*float64(row[xx*3+1]) + 0.114*float64(row[xx*3+2])
					}
				}
				g[y*w+x] = s / float64(n*n)
			}
		}
	})
	return g, w, h, float64(n)
}
