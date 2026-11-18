package kern

import (
	"errors"
	"image"
	"math"
	"sort"
)

// Automatische Ausrichtung ohne Neigungsdaten: gerade Kanten im Bild finden,
// daraus den senkrechten Fluchtpunkt schätzen und Roll/Pitch berechnen.

// AutoErgebnis beschreibt das Ergebnis der automatischen Ausrichtung.
type AutoErgebnis struct {
	Roll, Pitch float64
	Yaw         float64 // nur bei VollstaendigAusrichten
	NurRoll     bool    // Pitch konnte nicht sicher bestimmt werden
	Unsicher    bool    // wenige Kanten – Ergebnis bitte prüfen
	Quelle      string  // "senkrecht", "kombiniert" (Horizont + Senkrechte), "waagerecht" oder "vollstaendig"
	Linien      []Linie // verwendete senkrechte Kanten, in Koordinaten des übergebenen Bildes
	LinienWaag  []Linie // verwendete waagerechte Kanten bzw. Horizont
	Gefunden    int     // Anzahl aller gefundenen geraden Kanten
}

var FehlerKeineKanten = errors.New("Keine eindeutigen geraden Kanten gefunden – bitte per Regler oder „Kanten markieren“ korrigieren.")

// segment ist eine gefundene gerade Kante.
type segment struct {
	Linie
	laenge     float64
	mx, my     float64 // Mittelpunkt
	tx, ty     float64 // Einheitsrichtung
	vonOben    float64 // Winkel zur Senkrechten (Grad, 0..90)
	vonWaagerr float64 // Winkel zur Waagerechten (Grad, 0..90)
}

const analyseMax = 1000

// AutoAusrichten analysiert b und liefert Roll/Pitch für die Korrektur.
func AutoAusrichten(b *Bild, f35 float64) (AutoErgebnis, error) {
	var erg AutoErgebnis
	k := b.Verkleinert(analyseMax)
	skal := float64(b.W) / float64(k.W)
	segs := FindeKanten(k)
	erg.Gefunden = len(segs)
	zuruek := func(ls []segment) []Linie {
		out := make([]Linie, len(ls))
		for i, s := range ls {
			out[i] = Linie{s.X1 * skal, s.Y1 * skal, s.X2 * skal, s.Y2 * skal}
		}
		return out
	}

	var senk, waag []segment
	for _, s := range segs {
		if s.vonOben <= 30 {
			senk = append(senk, s)
		} else if s.vonWaagerr <= 25 {
			waag = append(waag, s)
		}
	}

	// 1. Senkrechte Kanten → Fluchtpunkt → Roll und Pitch
	var inl []segment
	var vpOk, genug bool
	if inl, vpOk = fluchtpunktRansac(senk, k.W, k.H, f35); vpOk {
		var summe, xmin, xmax float64
		xmin, xmax = math.Inf(1), math.Inf(-1)
		for _, s := range inl {
			summe += s.laenge
			xmin, xmax = math.Min(xmin, s.mx), math.Max(xmax, s.mx)
		}
		genug = len(inl) >= 3 && summe >= 0.35*float64(k.H)
		if genug && xmax-xmin >= 0.2*float64(k.W) {
			linien := make([]Linie, len(inl))
			for i, s := range inl {
				linien[i] = s.Linie
			}
			r, p, err := WinkelAusSenkrechten(linien, k.W, k.H, f35)
			if err == nil && math.Abs(r) <= 30 && math.Abs(p) <= 40 {
				erg.Roll, erg.Pitch, erg.Quelle, erg.Linien = r, p, "senkrecht", zuruek(inl)
				erg.Unsicher = len(inl) < 6
				return erg, nil
			}
		}
	}

	// 2. Horizont + wenige Senkrechte: Roll aus dem Horizont, Pitch aus den Senkrechten
	if rh, inlH, ok := rollAusWaagerechten(waag, k.W); ok {
		if p, inlS, ok := pitchBeiRoll(senk, rh, k.W, k.H, f35); ok {
			erg.Roll, erg.Pitch, erg.Quelle = rh, p, "kombiniert"
			erg.Linien, erg.LinienWaag = zuruek(inlS), zuruek(inlH)
			erg.Unsicher = len(inlS) < 3
			return erg, nil
		}
	}

	// 3. Senkrechte liegen eng beieinander: Pitch unsicher, Roll aus der mittleren Richtung
	if vpOk && genug {
		var sx, sy float64
		for _, s := range inl {
			tx, ty := s.tx, s.ty
			if ty < 0 {
				tx, ty = -tx, -ty
			}
			sx += tx * s.laenge
			sy += ty * s.laenge
		}
		r := grad(math.Atan2(sx, sy))
		if math.Abs(r) <= 30 {
			erg.Roll, erg.NurRoll, erg.Unsicher, erg.Quelle, erg.Linien = r, true, true, "senkrecht", zuruek(inl)
			return erg, nil
		}
	}

	// 4. Nur waagerechte Kanten (z. B. Horizont) → Roll
	if r, inl, ok := rollAusWaagerechten(waag, k.W); ok {
		erg.Roll, erg.NurRoll, erg.Unsicher, erg.Quelle, erg.LinienWaag = r, true, true, "waagerecht", zuruek(inl)
		return erg, nil
	}
	return erg, FehlerKeineKanten
}

// FindeKanten sucht gerade Kanten (vereinfachtes LSD-Verfahren:
// Pixel mit gleicher Gradientenrichtung zu Regionen wachsen lassen).
func FindeKanten(b *Bild) []segment {
	w, h := b.W, b.H
	if w < 16 || h < 16 {
		return nil
	}
	// Grauwerte, leicht geglättet
	grau := make([]float64, w*h)
	for i := 0; i < w*h; i++ {
		grau[i] = 0.299*float64(b.Pix[3*i]) + 0.587*float64(b.Pix[3*i+1]) + 0.114*float64(b.Pix[3*i+2])
	}
	glatt := make([]float64, w*h)
	tmp := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a, c := max(x-1, 0), min(x+1, w-1)
			tmp[y*w+x] = (grau[y*w+a] + 2*grau[y*w+x] + grau[y*w+c]) / 4
		}
	}
	for y := 0; y < h; y++ {
		a, c := max(y-1, 0), min(y+1, h-1)
		for x := 0; x < w; x++ {
			glatt[y*w+x] = (tmp[a*w+x] + 2*tmp[y*w+x] + tmp[c*w+x]) / 4
		}
	}
	// Gradient (Sobel)
	mag := make([]float64, w*h)
	ang := make([]float64, w*h)
	const schwelle = 5.2
	parallel(h-2, func(y0, y1 int) {
		for y := y0 + 1; y < y1+1; y++ {
			for x := 1; x < w-1; x++ {
				g := func(dx, dy int) float64 { return glatt[(y+dy)*w+x+dx] }
				gx := (g(1, -1) + 2*g(1, 0) + g(1, 1) - g(-1, -1) - 2*g(-1, 0) - g(-1, 1)) / 8
				gy := (g(-1, 1) + 2*g(0, 1) + g(1, 1) - g(-1, -1) - 2*g(0, -1) - g(1, -1)) / 8
				m := math.Hypot(gx, gy)
				if m >= schwelle {
					mag[y*w+x] = m
					ang[y*w+x] = math.Atan2(gy, gx)
				}
			}
		}
	})
	// Startpunkte nach Gradientenstärke sortieren (Eimer-Sortierung)
	const eimer = 256
	var mmax float64
	for _, m := range mag {
		mmax = math.Max(mmax, m)
	}
	if mmax == 0 {
		return nil
	}
	faecher := make([][]int32, eimer)
	for i, m := range mag {
		if m > 0 {
			e := int(m / mmax * (eimer - 1))
			faecher[e] = append(faecher[e], int32(i))
		}
	}
	benutzt := make([]bool, w*h)
	toleranz := 22.5 * math.Pi / 180
	cosTol := math.Cos(toleranz)
	minLaenge := math.Max(15, 0.03*float64(max(w, h)))
	var segs []segment
	var region, stapel []int32
	for e := eimer - 1; e >= 0; e-- {
		for _, start := range faecher[e] {
			if benutzt[start] {
				continue
			}
			region, stapel = region[:0], append(stapel[:0], start)
			benutzt[start] = true
			sc, ss := math.Cos(ang[start]), math.Sin(ang[start])
			rc, rs := sc, ss
			for len(stapel) > 0 {
				i := stapel[len(stapel)-1]
				stapel = stapel[:len(stapel)-1]
				region = append(region, i)
				x, y := int(i)%w, int(i)/w
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := x+dx, y+dy
						if nx < 0 || ny < 0 || nx >= w || ny >= h {
							continue
						}
						j := int32(ny*w + nx)
						if benutzt[j] || mag[j] == 0 {
							continue
						}
						// gleiche Richtung wie die Region?
						if math.Cos(ang[j])*rc+math.Sin(ang[j])*rs < cosTol {
							continue
						}
						benutzt[j] = true
						stapel = append(stapel, j)
						sc += math.Cos(ang[j])
						ss += math.Sin(ang[j])
						n := math.Hypot(sc, ss)
						rc, rs = sc/n, ss/n
					}
				}
			}
			if float64(len(region)) < minLaenge {
				continue
			}
			if s, ok := passeGerade(region, w, mag, minLaenge); ok {
				segs = append(segs, s)
			}
		}
	}
	return segs
}

// passeGerade legt eine Gerade durch die Region und prüft, ob sie schmal genug ist.
func passeGerade(region []int32, w int, mag []float64, minLaenge float64) (segment, bool) {
	var sw, sx, sy float64
	for _, i := range region {
		m := mag[i]
		sw += m
		sx += m * float64(int(i)%w)
		sy += m * float64(int(i)/w)
	}
	cx, cy := sx/sw, sy/sw
	var xx, xy, yy float64
	for _, i := range region {
		m := mag[i]
		dx, dy := float64(int(i)%w)-cx, float64(int(i)/w)-cy
		xx += m * dx * dx
		xy += m * dx * dy
		yy += m * dy * dy
	}
	// Hauptrichtung der Region
	theta := 0.5 * math.Atan2(2*xy, xx-yy)
	tx, ty := math.Cos(theta), math.Sin(theta)
	lmin, lmax := math.Inf(1), math.Inf(-1)
	qmin, qmax := math.Inf(1), math.Inf(-1)
	for _, i := range region {
		dx, dy := float64(int(i)%w)-cx, float64(int(i)/w)-cy
		l := dx*tx + dy*ty
		q := -dx*ty + dy*tx
		lmin, lmax = math.Min(lmin, l), math.Max(lmax, l)
		qmin, qmax = math.Min(qmin, q), math.Max(qmax, q)
	}
	laenge, breite := lmax-lmin, qmax-qmin
	if laenge < minLaenge || breite > math.Max(4, 0.06*laenge) {
		return segment{}, false
	}
	if float64(len(region)) < 0.6*laenge { // zu löchrig
		return segment{}, false
	}
	s := segment{
		Linie:  Linie{cx + lmin*tx, cy + lmin*ty, cx + lmax*tx, cy + lmax*ty},
		laenge: laenge, mx: cx, my: cy, tx: tx, ty: ty,
	}
	s.vonOben = grad(math.Atan2(math.Abs(tx), math.Abs(ty)))
	s.vonWaagerr = 90 - s.vonOben
	return s, true
}

// fluchtpunktRansac sucht den Punkt, auf den die meisten (langen) senkrechten
// Kanten zulaufen, und liefert die passenden Kanten.
func fluchtpunktRansac(senk []segment, w, h int, f35 float64) ([]segment, bool) {
	if len(senk) < 2 {
		return nil, false
	}
	sort.Slice(senk, func(i, j int) bool { return senk[i].laenge > senk[j].laenge })
	if len(senk) > 150 {
		senk = senk[:150]
	}
	Ki := Kameramatrix(f35, w, h).Inv()
	homo := func(s segment) [3]float64 {
		return kreuz([3]float64{s.X1, s.Y1, 1}, [3]float64{s.X2, s.Y2, 1})
	}
	const tolGrad = 1.5
	cosTol := math.Cos(tolGrad * math.Pi / 180)
	passt := func(s segment, v [3]float64) bool {
		var qx, qy float64
		if math.Abs(v[2]) > 1e-12 {
			qx, qy = v[0]/v[2]-s.mx, v[1]/v[2]-s.my
		} else {
			qx, qy = v[0], v[1]
		}
		n := math.Hypot(qx, qy)
		if n < 1e-9 {
			return false
		}
		return math.Abs(qx*s.tx+qy*s.ty)/n >= cosTol
	}
	plausibel := func(v [3]float64) bool {
		d := Ki.MulVec(v)
		n := math.Sqrt(d[0]*d[0] + d[1]*d[1] + d[2]*d[2])
		if n < 1e-12 {
			return false
		}
		for i := range d {
			d[i] /= n
		}
		if d[1] < 0 {
			d = [3]float64{-d[0], -d[1], -d[2]}
		}
		p := grad(math.Asin(math.Max(-1, math.Min(1, -d[2]))))
		r := grad(math.Atan2(d[0], d[1]))
		return math.Abs(r) <= 30 && math.Abs(p) <= 40
	}
	var besteV [3]float64
	beste := -1.0
	for i := 0; i < len(senk); i++ {
		li := homo(senk[i])
		for j := i + 1; j < len(senk); j++ {
			v := kreuz(li, homo(senk[j]))
			if v == ([3]float64{}) || !plausibel(v) {
				continue
			}
			var punkte float64
			for _, s := range senk {
				if passt(s, v) {
					punkte += s.laenge
				}
			}
			if punkte > beste {
				beste, besteV = punkte, v
			}
		}
	}
	if beste < 0 {
		return nil, false
	}
	var inl []segment
	for _, s := range senk {
		if passt(s, besteV) {
			inl = append(inl, s)
		}
	}
	return inl, len(inl) >= 2
}

// rollAusWaagerechten: häufigster Winkel langer, nahezu waagerechter Kanten.
func rollAusWaagerechten(waag []segment, w int) (float64, []segment, bool) {
	var beste float64
	var besteInl []segment
	besteSumme := 0.0
	for _, k := range waag {
		r0 := RollAusHorizont(k.Linie)
		var summe, gew float64
		var inl []segment
		for _, s := range waag {
			r := RollAusHorizont(s.Linie)
			if math.Abs(r-r0) <= 1 {
				summe += s.laenge
				gew += r * s.laenge
				inl = append(inl, s)
			}
		}
		if summe > besteSumme {
			besteSumme, beste, besteInl = summe, gew/summe, inl
		}
	}
	if besteSumme < 0.5*float64(w) || math.Abs(beste) > 10 {
		return 0, nil, false
	}
	return beste, besteInl, true
}

// Ausschnitt rendert einen Bildschirmausschnitt w×h direkt aus b.
// S bildet Bildschirmpixel (x, y, 1) auf Quellkoordinaten ab (projektiv),
// skal = Quellpixel je Bildschirmpixel (zum Mitteln beim Verkleinern).
func (b *Bild) Ausschnitt(w, h int, S Mat3, skal float64, rand [3]uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	parallel(h, func(y0, y1 int) {
		var px [3]uint8
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				fx, fy := float64(x), float64(y)
				q2 := S[2][0]*fx + S[2][1]*fy + S[2][2]
				o := y*img.Stride + x*4
				sx := (S[0][0]*fx + S[0][1]*fy + S[0][2]) / q2
				sy := (S[1][0]*fx + S[1][1]*fy + S[1][2]) / q2
				if q2 <= 0 || sx < -0.5 || sy < -0.5 || sx > float64(b.W)-0.5 || sy > float64(b.H)-0.5 {
					px = rand
				} else if skal > 1.5 {
					mittel(b, sx, sy, skal, skal, px[:])
				} else {
					bilinear(b, sx, sy, px[:])
				}
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = px[0], px[1], px[2], 255
			}
		}
	})
	return img
}

// abbilden: Punkt mit der Drehung (roll, pitch, yaw) abbilden (ohne Verschiebung).
func abbilder(w, h int, f35, roll, pitch, yaw float64) func(x, y float64) (float64, float64, bool) {
	K := Kameramatrix(f35, w, h)
	H := K.Mul(Drehmatrix(roll, pitch, yaw)).Mul(K.Inv())
	return func(x, y float64) (float64, float64, bool) {
		t := H.MulVec([3]float64{x, y, 1})
		if t[2] <= 1e-9 {
			return 0, 0, false
		}
		return t[0] / t[2], t[1] / t[2], true
	}
}

// winkelNachKorrektur: Neigung eines Segments nach der Korrektur, in Grad.
// senkrecht = true: Abweichung von der Senkrechten, sonst von der Waagerechten.
func winkelNachKorrektur(f func(x, y float64) (float64, float64, bool), s segment, senkrecht bool) (float64, bool) {
	x1, y1, ok1 := f(s.X1, s.Y1)
	x2, y2, ok2 := f(s.X2, s.Y2)
	if !ok1 || !ok2 {
		return 0, false
	}
	dx, dy := x2-x1, y2-y1
	if senkrecht {
		if dy < 0 {
			dx, dy = -dx, -dy
		}
		return grad(math.Atan2(dx, dy)), true
	}
	if dx < 0 {
		dx, dy = -dx, -dy
	}
	return grad(math.Atan2(dy, dx)), true
}

// pitchBeiRoll: Bei bekanntem Roll (aus dem Horizont) genügt schon eine
// senkrechte Kante, um den Pitch zu bestimmen. Für jede Kante wird der Pitch
// gesucht, bei dem sie nach der Korrektur senkrecht steht; die größte
// übereinstimmende Gruppe (±1°) gewinnt.
func pitchBeiRoll(senk []segment, roll float64, w, h int, f35 float64) (float64, []segment, bool) {
	type kandidat struct {
		p float64
		s segment
	}
	var ks []kandidat
	for _, s := range senk {
		if s.laenge < 0.12*float64(h) {
			continue
		}
		winkel := func(p float64) float64 {
			v, ok := winkelNachKorrektur(abbilder(w, h, f35, roll, p, 0), s, true)
			if !ok {
				return math.NaN()
			}
			return v
		}
		a, b := -40.0, 40.0
		wa, wb := winkel(a), winkel(b)
		if math.IsNaN(wa) || math.IsNaN(wb) || wa*wb > 0 {
			continue
		}
		for i := 0; i < 40; i++ { // Bisektion
			m := (a + b) / 2
			wm := winkel(m)
			if math.IsNaN(wm) {
				break
			}
			if wa*wm <= 0 {
				b = m
			} else {
				a, wa = m, wm
			}
		}
		ks = append(ks, kandidat{(a + b) / 2, s})
	}
	if len(ks) == 0 {
		return 0, nil, false
	}
	beste, bestP := -1.0, 0.0
	for _, k := range ks {
		var summe float64
		for _, j := range ks {
			if math.Abs(j.p-k.p) <= 1 {
				summe += j.s.laenge
			}
		}
		if summe > beste {
			beste, bestP = summe, k.p
		}
	}
	if beste < 0.2*float64(h) {
		return 0, nil, false
	}
	var inl []segment
	var sp, sl float64
	for _, k := range ks {
		if math.Abs(k.p-bestP) <= 1 {
			inl = append(inl, k.s)
			sp += k.p * k.s.laenge
			sl += k.s.laenge
		}
	}
	p := sp / sl
	if math.Abs(p) > 40 {
		return 0, nil, false
	}
	return p, inl, true
}

var FehlerVollstaendig = errors.New("Für die vollständige Ausrichtung werden senkrechte UND waagerechte Kanten einer Fläche benötigt (z. B. eine frontal fotografierte Fassade). Es wurde nur senkrecht ausgerichtet.")

// VollstaendigAusrichten: wie AutoAusrichten, zusätzlich Yaw aus den
// waagerechten Kanten – die Fläche wird frontal ausgerichtet (Senkrechte
// senkrecht, Waagerechte waagerecht).
func VollstaendigAusrichten(b *Bild, f35 float64) (AutoErgebnis, error) {
	erg, err := AutoAusrichten(b, f35)
	if err != nil {
		return erg, err
	}
	if erg.NurRoll {
		return erg, FehlerVollstaendig
	}
	k := b.Verkleinert(analyseMax)
	skal := float64(b.W) / float64(k.W)
	segs := FindeKanten(k)
	minL := 0.05 * math.Min(float64(k.W), float64(k.H))
	var kand []segment
	for _, s := range segs {
		if s.vonOben > 30 && s.laenge >= minL {
			kand = append(kand, s)
		}
	}
	if len(kand) < 2 {
		return erg, FehlerVollstaendig
	}
	// Bewertung eines Yaw: Länge aller Kanten, die danach waagerecht (±1°) stehen
	bewerte := func(yaw float64) (float64, float64, []segment) {
		f := abbilder(k.W, k.H, f35, erg.Roll, erg.Pitch, yaw)
		var summe, fehler float64
		var inl []segment
		for _, s := range kand {
			a, ok := winkelNachKorrektur(f, s, false)
			if ok && math.Abs(a) <= 1 {
				summe += s.laenge
				fehler += s.laenge * a * a
				inl = append(inl, s)
			}
		}
		return summe, fehler, inl
	}
	besterY, besteS := 0.0, -1.0
	for y := -40.0; y <= 40.0; y += 0.5 {
		if sm, _, _ := bewerte(y); sm > besteS {
			besteS, besterY = sm, y
		}
	}
	// Feinsuche: bei gleicher Länge kleinster Restfehler
	grob := besterY
	besterF := math.Inf(1)
	for y := grob - 3; y <= grob+3; y += 0.02 {
		sm, fe, _ := bewerte(y)
		if sm >= besteS*0.9 && fe/math.Max(sm, 1) < besterF {
			besterF, besterY = fe/math.Max(sm, 1), y
		}
	}
	summe, _, inl := bewerte(besterY)
	if summe < 0.3*float64(k.W) || len(inl) < 2 || math.Abs(besterY) > 35 {
		return erg, FehlerVollstaendig
	}
	if _, _, _, err := Transformation(b.W, b.H, Parameter{Roll: erg.Roll, Pitch: erg.Pitch, Yaw: besterY, F35: f35}); err != nil {
		return erg, err
	}
	erg.Yaw, erg.Quelle = besterY, "vollstaendig"
	erg.LinienWaag = make([]Linie, len(inl))
	for i, s := range inl {
		erg.LinienWaag[i] = Linie{s.X1 * skal, s.Y1 * skal, s.X2 * skal, s.Y2 * skal}
	}
	return erg, nil
}
