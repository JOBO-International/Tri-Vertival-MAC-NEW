package main

// Bezugsmarken: Der Nutzer legt fest, was in Wirklichkeit senkrecht,
// waagerecht, rechtwinklig, rechteckig oder kreisrund ist. Vertical3 rechnet
// daraus Roll, Pitch und Yaw (auf Wunsch auch die Brennweite) aus.

import (
	"fmt"
	"math"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"vertical3/kern"
)

var markenNamen = map[string]string{
	kern.MarkeSenkrecht:  "Senkrechte",
	kern.MarkeWaagerecht: "Waagerechte",
	kern.MarkeEcke:       "Ecke",
	kern.MarkeRechteck:   "Rechteck",
	kern.MarkeKreisBoden: "Kreis am Boden",
	kern.MarkeKreisWand:  "Kreis an der Wand",
}

var markenKnopf = map[string]string{
	kern.MarkeSenkrecht:  "Senkrechte",
	kern.MarkeWaagerecht: "Waagerechte",
	kern.MarkeEcke:       "Ecke / Dreieck",
	kern.MarkeRechteck:   "Rechteck",
	kern.MarkeKreisBoden: "Kreis am Boden",
	kern.MarkeKreisWand:  "Kreis an Wand",
}

var markenHilfe = map[string]string{
	kern.MarkeSenkrecht:  "Klicke Anfang und Ende einer Kante, die in Wirklichkeit SENKRECHT ist (Hauskante, Fallrohr, Türrahmen).",
	kern.MarkeWaagerecht: "Klicke Anfang und Ende einer Kante, die WAAGERECHT stehen soll (Horizont oder – bei frontaler Ansicht – eine Fassadenkante).",
	kern.MarkeEcke:       "Klicke 3 Punkte: oberes Ende der senkrechten Kante, die rechtwinklige Ecke, Ende der waagerechten Kante (z. B. Fensterecke).",
	kern.MarkeRechteck:   "Klicke die 4 Ecken einer rechteckigen Fläche der Reihe nach – je größer, desto genauer (ganze Fassade, große Tür, Fensterreihe).",
	kern.MarkeKreisBoden: "Klicke 5 Punkte gleichmäßig verteilt auf den Rand eines runden Gegenstands, der WAAGERECHT liegt (Kanaldeckel, Tisch, Brunnen).",
	kern.MarkeKreisWand:  "Klicke 5 Punkte gleichmäßig verteilt auf den Rand eines Kreises an einer SENKRECHTEN Wand (Rundfenster, Uhr, Schild).",
}

const fangRadius = 9 // Bildschirmpixel zum Greifen eines Punktes

// ------------------------------------------------------------------ Modus

// markenStarten: Bezugsmarken anzeigen und bearbeiten (Taste A).
func (p *programm) markenStarten() {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	p.modus, p.markeNeu, p.punkte, p.ziehMarke = "marken", "", nil, [2]int{-1, -1}
	p.cw.SetCursor(walk.CursorArrow())
	p.knoepfeAnpassen()
	if len(p.marken) == 0 {
		p.setzeStatus("Bezugsmarken: im Bedienfeld eine Marke wählen (Ecke, Rechteck, Kreis) und im Bild setzen. Enter = fertig.")
	} else {
		p.markenStatus()
	}
	p.cw.Invalidate()
}

// markenBeenden: zurück zur Ergebnisansicht (Marken bleiben erhalten).
func (p *programm) markenBeenden(still bool) {
	if p.modus == "" {
		return
	}
	p.modus, p.markeNeu, p.punkte, p.maus, p.ziehMarke = "", "", nil, nil, [2]int{-1, -1}
	p.cw.SetCursor(walk.CursorArrow())
	p.knoepfeAnpassen()
	if !still {
		if len(p.marken) > 0 {
			p.markenStatus()
		} else {
			p.setzeStatus("Bezugsmarken beendet.")
		}
	}
	p.cw.Invalidate()
}

// markenZuruecksetzen: neues Bild – alle Marken weg.
func (p *programm) markenZuruecksetzen() {
	p.marken, p.markenLsg, p.markenFehler = nil, kern.Loesung{}, ""
	p.markenBeenden(true)
}

func (p *programm) knoepfeAnpassen() {
	if p.panel != nil {
		p.panel.Invalidate()
	}
}

// markeBeginnen: neue Marke der Art art setzen.
func (p *programm) markeBeginnen(art string) {
	if p.geladen == nil {
		return
	}
	if p.modus != "marken" {
		p.markenStarten()
	}
	p.markeNeu, p.punkte = art, nil
	p.cw.SetCursor(walk.CursorCross())
	p.setzeStatus(markenHilfe[art] + "  Esc = abbrechen, Rücktaste = letzten Punkt entfernen.")
	p.knoepfeAnpassen()
	p.cw.Invalidate()
}

func (p *programm) markeAbbrechen() {
	p.markeNeu, p.punkte, p.maus = "", nil, nil
	p.cw.SetCursor(walk.CursorArrow())
	p.knoepfeAnpassen()
	p.markenStatus()
	p.cw.Invalidate()
}

func (p *programm) alleMarkenLoeschen() {
	if len(p.marken) == 0 {
		return
	}
	p.marken, p.markenLsg, p.markenFehler = nil, kern.Loesung{}, ""
	p.setzeStatus("Alle Bezugsmarken gelöscht.")
	p.knoepfeAnpassen()
	p.cw.Invalidate()
}

// ------------------------------------------------------------------ Rechnen

// markenLoesen: Ausgleichsrechnung über alle Marken, Werte in die Regler.
func (p *programm) markenLoesen() {
	if p.geladen == nil || len(p.marken) == 0 {
		p.markenLsg, p.markenFehler = kern.Loesung{}, ""
		p.knoepfeAnpassen()
		return
	}
	g := p.geladen.Bild
	start := [3]float64{p.roll.wert, p.pitch.wert, p.yaw.wert}
	l, err := kern.LoeseMarken(p.marken, g.W, g.H, p.f35.wert, start, p.brennweiteFrei)
	if err == nil && !l.BrennweiteBestimmt {
		// automatisch gefundene, gleich große Fenster mit auswerten (wie bei der Automatik)
		auto := false
		for _, m := range p.marken {
			auto = auto || (m.Auto && m.Art == kern.MarkeRechteck && !m.NurSenkrecht)
		}
		if auto {
			if x, ok := kern.VerfeinereGleichGross(p.marken, g.W, g.H, p.f35.wert, [3]float64{l.Roll, l.Pitch, l.Yaw}); ok {
				l.Roll, l.Pitch, l.Yaw = x[0], x[1], x[2]
				l.Abweichung = kern.MarkenAbweichungen(p.marken, g.W, g.H, p.f35.wert, l.Roll, l.Pitch, l.Yaw)
			}
		}
	}
	p.markenLsg, p.markenFehler = l, ""
	if err != nil {
		p.markenFehler = err.Error()
		p.markenStatus()
		p.knoepfeAnpassen()
		p.cw.Invalidate()
		return
	}
	p.setzeWert(p.roll, math.Round(l.Roll*10)/10, nil)
	p.setzeWert(p.pitch, math.Round(l.Pitch*10)/10, nil)
	p.setzeYawAuto(math.Round(l.Yaw*10) / 10)
	if l.BrennweiteBestimmt {
		p.setzeWert(p.f35, math.Round(l.F35*10)/10, nil)
	}
	p.markenStatus()
	p.knoepfeAnpassen()
	p.cw.Invalidate()
}

func komma1(v float64) string { return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) }

// markenInfo: Kurztext für Bedienfeld und Statuszeile.
func (p *programm) markenInfo() string {
	if len(p.marken) == 0 {
		return "Noch keine Marken gesetzt."
	}
	if p.markenFehler != "" {
		return p.markenFehler
	}
	maxA := 0.0
	for _, a := range p.markenLsg.Abweichung {
		maxA = math.Max(maxA, a)
	}
	t := fmt.Sprintf("%d %s · größte Abweichung %s°", len(p.marken), map[bool]string{true: "Marke", false: "Marken"}[len(p.marken) == 1], komma1(maxA))
	return t
}

func (p *programm) markenStatus() {
	if len(p.marken) == 0 {
		p.setzeStatus("Keine Bezugsmarken. Im Bedienfeld eine Marke wählen und im Bild setzen.")
		return
	}
	if p.markenFehler != "" {
		p.setzeStatus(p.markenFehler)
		return
	}
	l := p.markenLsg
	t := fmt.Sprintf("Aus %s berechnet: Roll %+.1f°, Pitch %+.1f°", p.markenInfo(), l.Roll, l.Pitch)
	if l.YawBestimmt {
		t += fmt.Sprintf(", Yaw %+.1f°", l.Yaw)
	}
	if l.BrennweiteBestimmt {
		t += ", Brennweite " + komma1(l.F35) + " mm"
	}
	t += "."
	if l.Mehrdeutig {
		t += " Achtung: mehrere Lösungen möglich – bitte eine weitere Marke setzen (z. B. ein Rechteck)."
	} else if !l.YawBestimmt {
		t += " Yaw bleibt unverändert (für eine frontale Ausrichtung: „Automatisch frontal“, ein von Hand gesetztes Rechteck oder einen Kreis an der Wand)."
	}
	if p.modus == "marken" {
		t += " Punkte lassen sich mit der Maus verschieben; Rechtsklick auf einen Punkt löscht die Marke. Enter = fertig."
	}
	p.setzeStatus(t)
}

// verhaeltnisFragen: Seitenverhältnis einer Ecke (0 = unbekannt).
func (p *programm) verhaeltnisFragen() (float64, bool) {
	var dlg *walk.Dialog
	var cb *walk.ComboBox
	var neS, neW *walk.NumberEdit
	var ok, abbr *walk.PushButton
	wahl := []string{"unbekannt", "quadratisch (1 : 1)", "eigenes Verhältnis"}
	ergebnis, bestaetigt := 0.0, false
	err := Dialog{
		AssignTo: &dlg, Title: "Ecke – Seitenverhältnis", DefaultButton: &ok, CancelButton: &abbr,
		MinSize: Size{Width: 380}, Layout: VBox{},
		Children: []Widget{
			Label{Text: "Wie lang sind die beiden Seiten der Ecke in Wirklichkeit?\nMit Verhältnis bestimmt die Ecke allein alle drei Achsen;\nohne Verhältnis richtet sie eine senkrechte und eine waagerechte Kante aus."},
			ComboBox{AssignTo: &cb, Model: wahl, CurrentIndex: 0, OnCurrentIndexChanged: func() {
				neS.SetEnabled(cb.CurrentIndex() == 2)
				neW.SetEnabled(cb.CurrentIndex() == 2)
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "senkrecht"},
				NumberEdit{AssignTo: &neS, Decimals: 2, MinValue: 0.01, MaxValue: 1000, Value: 1.0, Enabled: false},
				Label{Text: ":  waagerecht"},
				NumberEdit{AssignTo: &neW, Decimals: 2, MinValue: 0.01, MaxValue: 1000, Value: 1.0, Enabled: false},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &ok, Text: "OK", OnClicked: func() {
					switch cb.CurrentIndex() {
					case 1:
						ergebnis = 1
					case 2:
						ergebnis = neS.Value() / neW.Value()
					}
					bestaetigt = true
					dlg.Accept()
				}},
				PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(p.mw)
	if err != nil {
		return 0, true
	}
	dlg.Run()
	return ergebnis, bestaetigt
}

// ------------------------------------------------------------------ Maus und Tasten

func (p *programm) zuBildschirm(pt [2]float64) walk.Point {
	return walk.Point{X: int(p.abb.ox + pt[0]*p.abb.f), Y: int(p.abb.oy + pt[1]*p.abb.f)}
}

func (p *programm) ausBildschirm(x, y int) [2]float64 {
	return [2]float64{(float64(x) - p.abb.ox) / p.abb.f, (float64(y) - p.abb.oy) / p.abb.f}
}

// punktBei: Marke/Punkt unter der Maus.
func (p *programm) punktBei(x, y int) (int, int, bool) {
	k := float64(p.cw.DPI()) / 96
	beste, bi, bj := fangRadius*k, -1, -1
	for i, m := range p.marken {
		for j, pt := range m.P {
			s := p.zuBildschirm(pt)
			if d := math.Hypot(float64(s.X-x), float64(s.Y-y)); d <= beste {
				beste, bi, bj = d, i, j
			}
		}
	}
	return bi, bj, bi >= 0
}

func (p *programm) taste(k walk.Key) {
	switch k {
	case walk.KeyAdd, walk.KeyOEMPlus:
		p.zoomSchritt(1.5)
		return
	case walk.KeySubtract, walk.KeyOEMMinus:
		p.zoomSchritt(1 / 1.5)
		return
	case walk.Key0, walk.KeyNumpad0:
		p.zoomGanz()
		return
	case walk.Key1, walk.KeyNumpad1:
		p.zoom100()
		return
	}
	if p.modus == "" {
		return
	}
	switch k {
	case walk.KeyEscape:
		if p.markeNeu != "" {
			p.markeAbbrechen()
		} else {
			p.markenBeenden(false)
		}
	case walk.KeyReturn:
		if p.markeNeu == "" {
			p.markenBeenden(false)
		}
	case walk.KeyBack:
		if len(p.punkte) > 0 {
			p.punkte = p.punkte[:len(p.punkte)-1]
			p.setzeStatus(fmt.Sprintf("Letzten Punkt entfernt (%d gesetzt).", len(p.punkte)))
			p.cw.Invalidate()
		}
	}
}

func (p *programm) mausBewegt(x, y int, knopf walk.MouseButton) {
	p.panelHoverAus()
	if p.aus.zieh != 0 {
		if knopf&walk.LeftButton == 0 {
			p.ausschnittEnde()
		} else {
			p.ausschnittZiehen(x, y)
		}
		return
	}
	if p.ziehen != nil {
		if knopf&(walk.LeftButton|walk.MiddleButton) == 0 {
			p.ziehenEnde()
		} else {
			p.verschiebe(p.ziehStart[0]+float64(x-p.ziehen.X), p.ziehStart[1]+float64(y-p.ziehen.Y))
			return
		}
	}
	if p.modus == "" && p.ziehen == nil {
		if t := p.ausschnittTreffer(x, y); t > 0 {
			p.ausschnittCursor(t)
		} else {
			p.cw.SetCursor(walk.CursorArrow())
		}
	}
	if p.modus == "" || !p.abb.ok {
		return
	}
	// Punkt ziehen
	if p.ziehMarke[0] >= 0 {
		if knopf&walk.LeftButton == 0 {
			p.mausLos(x, y, walk.LeftButton)
			return
		}
		i, j := p.ziehMarke[0], p.ziehMarke[1]
		if i < len(p.marken) && j < len(p.marken[i].P) {
			p.marken[i].P[j] = p.ausBildschirm(x, y)
			p.markenLoesen()
		}
		return
	}
	if p.markeNeu != "" {
		p.maus = &walk.Point{X: x, Y: y}
		p.cw.Invalidate()
		return
	}
	if _, _, ok := p.punktBei(x, y); ok {
		p.cw.SetCursor(walk.CursorSizeAll())
	} else {
		p.cw.SetCursor(walk.CursorArrow())
	}
}

func (p *programm) mausLos(x, y int, knopf walk.MouseButton) {
	p.ausschnittEnde()
	if p.ziehMarke[0] >= 0 {
		p.ziehMarke = [2]int{-1, -1}
		p.markenLoesen()
	}
	if p.ziehen != nil {
		p.ziehenEnde()
	}
}

func (p *programm) klick(x, y int, knopf walk.MouseButton) {
	p.cw.SetFocus()
	if knopf == walk.LeftButton && p.modus == "" && p.ausschnittStart(x, y) {
		return
	}
	// Verschieben: mittlere Maustaste immer, linke Taste außerhalb des Markenmodus
	if p.ansicht.ok && p.zoom > 1.0001 && (knopf == walk.MiddleButton || (knopf == walk.LeftButton && p.modus == "")) {
		p.ziehen = &walk.Point{X: x, Y: y}
		p.ziehStart = [2]float64{p.ansicht.ox, p.ansicht.oy}
		p.cw.SetCursor(walk.CursorSizeAll())
		return
	}
	if p.modus == "" || !p.abb.ok {
		return
	}
	if knopf == walk.RightButton {
		if p.markeNeu != "" {
			p.markeAbbrechen()
			return
		}
		if i, _, ok := p.punktBei(x, y); ok { // Marke löschen
			name := markenNamen[p.marken[i].Art]
			p.marken = append(p.marken[:i], p.marken[i+1:]...)
			p.markenLoesen()
			if len(p.marken) == 0 {
				p.setzeStatus(name + " gelöscht. Keine Marken mehr gesetzt.")
			}
			return
		}
		p.markenBeenden(false)
		return
	}
	if knopf != walk.LeftButton {
		return
	}
	if p.markeNeu == "" {
		if i, j, ok := p.punktBei(x, y); ok {
			p.ziehMarke = [2]int{i, j}
		}
		return
	}
	// neuen Punkt setzen
	p.punkte = append(p.punkte, p.ausBildschirm(x, y))
	p.cw.Invalidate()
	if len(p.punkte) < kern.PunkteFuer(p.markeNeu) {
		p.setzeStatus(fmt.Sprintf("%s – Punkt %d von %d gesetzt. %s", markenNamen[p.markeNeu], len(p.punkte), kern.PunkteFuer(p.markeNeu), markenHilfe[p.markeNeu]))
		return
	}
	m := kern.Marke{Art: p.markeNeu, P: append([][2]float64(nil), p.punkte...)}
	if m.Art == kern.MarkeEcke {
		v, ok := p.verhaeltnisFragen()
		if !ok {
			p.markeAbbrechen()
			return
		}
		m.Verhaeltnis = v
	}
	if m.Art == kern.MarkeKreisBoden || m.Art == kern.MarkeKreisWand {
		if _, err := kern.KegelschnittAus(m.P); err != nil {
			walk.MsgBox(p.mw, "Bezugsmarke", err.Error(), walk.MsgBoxIconWarning)
			p.punkte = nil
			return
		}
	}
	p.marken = append(p.marken, m)
	p.markeNeu, p.punkte, p.maus = "", nil, nil
	p.cw.SetCursor(walk.CursorArrow())
	p.markenLoesen()
}

// ------------------------------------------------------------------ Zeichnen

func markenFarbe(abw float64) walk.Color {
	switch {
	case abw <= 0.5:
		return walk.RGB(46, 204, 64) // passt
	case abw <= 1.5:
		return walk.RGB(255, 190, 0) // leichte Abweichung
	}
	return walk.RGB(255, 59, 59) // passt nicht zu den anderen
}

func (p *programm) maleMarken(c *walk.Canvas) {
	k := float64(c.DPI()) / 96
	dicke := int(math.Max(2, 2*k))
	linie := func(farbe walk.Color, pts [][2]float64, geschlossen bool, gestrichelt bool) {
		pinsel, _ := walk.NewSolidColorBrush(farbe)
		defer pinsel.Dispose()
		stil := walk.PenSolid
		if gestrichelt {
			stil = walk.PenDash
		}
		st, _ := walk.NewGeometricPen(stil, dicke, pinsel)
		defer st.Dispose()
		for i := 0; i+1 < len(pts); i++ {
			c.DrawLinePixels(st, p.zuBildschirm(pts[i]), p.zuBildschirm(pts[i+1]))
		}
		if geschlossen && len(pts) > 2 {
			c.DrawLinePixels(st, p.zuBildschirm(pts[len(pts)-1]), p.zuBildschirm(pts[0]))
		}
	}
	griff := func(farbe walk.Color, pt [2]float64) {
		s := p.zuBildschirm(pt)
		r := int(5 * k)
		c.FillEllipsePixels(pinselVorrat.pinsel(walk.RGB(255, 255, 255)), walk.Rectangle{X: s.X - r - 1, Y: s.Y - r - 1, Width: 2*r + 2, Height: 2*r + 2})
		c.FillEllipsePixels(pinselVorrat.pinsel(farbe), walk.Rectangle{X: s.X - r + 1, Y: s.Y - r + 1, Width: 2*r - 2, Height: 2*r - 2})
	}
	form := func(m kern.Marke, farbe walk.Color) {
		switch m.Art {
		case kern.MarkeSenkrecht, kern.MarkeWaagerecht, kern.MarkeEcke:
			linie(farbe, m.P, false, false)
		case kern.MarkeRechteck:
			linie(farbe, m.P, len(m.P) == 4, false)
		case kern.MarkeKreisBoden, kern.MarkeKreisWand:
			if C, err := kern.KegelschnittAus(m.P); err == nil {
				linie(farbe, kern.EllipsenPunkte(C, 72), false, false)
			}
		}
	}
	for i, m := range p.marken {
		abw := 0.0
		if i < len(p.markenLsg.Abweichung) {
			abw = p.markenLsg.Abweichung[i]
		}
		farbe := markenFarbe(abw)
		if p.markenFehler != "" {
			farbe = walk.RGB(255, 59, 59)
		}
		form(m, farbe)
		for _, pt := range m.P {
			griff(farbe, pt)
		}
		// Beschriftung mit Abweichung
		if len(m.P) > 0 {
			s := p.zuBildschirm(m.P[0])
			t := markenNamen[m.Art] + "  " + komma1(abw) + "°"
			if m.Auto {
				t = komma1(abw) + "°"
			}
			r := walk.Rectangle{X: s.X + int(10*k), Y: s.Y - int(22*k), Width: int(170 * k), Height: int(18 * k)}
			c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(30, 30, 30)), walk.Rectangle{X: r.X - 3, Y: r.Y, Width: p.textBreite(c, "klein", t) + 8, Height: r.Height})
			c.DrawTextPixels(t, p.schrift("klein"), walk.RGB(255, 255, 255), r, walk.TextVCenter|walk.TextSingleLine)
		}
	}
	// Marke im Entstehen
	if p.markeNeu != "" {
		blau := walk.RGB(40, 150, 255)
		pts := append([][2]float64(nil), p.punkte...)
		if p.maus != nil && len(pts) > 0 {
			pts = append(pts, p.ausBildschirm(p.maus.X, p.maus.Y))
		}
		if (p.markeNeu == kern.MarkeKreisBoden || p.markeNeu == kern.MarkeKreisWand) && len(pts) >= 5 {
			if C, err := kern.KegelschnittAus(pts); err == nil {
				linie(blau, kern.EllipsenPunkte(C, 72), false, true)
			}
		} else if p.markeNeu != kern.MarkeKreisBoden && p.markeNeu != kern.MarkeKreisWand {
			linie(blau, pts, false, true)
		}
		for _, pt := range p.punkte {
			griff(blau, pt)
		}
	}
}

// ------------------------------------------------------------------ Automatik

// automatik sucht Rechtecke und runde Formen im Bild, setzt sie als
// Bezugsmarken und richtet das Bild danach aus.
// frontal: Formen vollständig auswerten (Fläche wird frontal, mit Yaw).
func (p *programm) automatik(frontal bool) {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	if frontal && !p.yawAktiv {
		frontal = false // ohne Yaw-Achse gibt es keine frontale Ausrichtung
	}
	if p.modus != "" {
		p.markenBeenden(true)
	}
	g, f35 := p.geladen, p.f35.wert
	p.beschaeftigt = true
	p.mw.SetCursor(walk.CursorWait())
	p.setzeStatus("Werte die Richtungen aller Kanten im Bild aus …")
	p.panel.Invalidate()
	go func() {
		o, err := kern.AutoOrientierung(g.Bild, f35, frontal)
		// Rückfall: Rechtecke und Kreise (falls die Richtungsstatistik nichts findet)
		var am kern.AutoErgebnisMarken
		var errM error
		if err != nil || (o.NurRoll && !frontal) {
			am, errM = kern.AutoMarken(g.Bild, f35, frontal)
		}
		p.mw.Synchronize(func() {
			p.beschaeftigt = false
			p.mw.SetCursor(nil)
			p.panel.Invalidate()
			if p.geladen != g {
				return
			}
			titel := map[bool]string{true: "Automatisch frontal ausrichten", false: "Automatisch ausrichten"}[frontal]
			if errM == nil && len(am.Marken) > 0 && (err != nil || o.NurRoll) {
				p.automatikMarkenUebernehmen(am, frontal)
				return
			}
			if err != nil {
				p.setzeStatus(err.Error())
				walk.MsgBox(p.mw, titel, err.Error(), walk.MsgBoxIconInformation)
				return
			}
			p.marken, p.markenLsg, p.markenFehler = nil, kern.Loesung{}, ""
			// sehr starke Korrekturen nur so weit, dass das Bild nicht extrem verzerrt
			voll := kern.Parameter{Roll: o.Roll, Pitch: o.Pitch, Yaw: o.Yaw, F35: f35, Zuschneiden: p.zuschneiden, YawMitte: true}
			begr, anteil := kern.BegrenzeKorrektur(g.Bild.W, g.Bild.H, voll, 1.6)
			p.setzeWert(p.roll, math.Round(begr.Roll*10)/10, nil)
			p.setzeWert(p.pitch, math.Round(begr.Pitch*10)/10, nil)
			p.setzeYawAuto(math.Round(begr.Yaw*10) / 10)
			p.autoPunkte, p.autoPar = o.Proben, p.parameter()
			if anteil < 0.999 {
				p.autoPunkte = nil // Punkte gelten nur für die volle Korrektur
			}
			var t string
			switch {
			case o.NurRoll:
				t = fmt.Sprintf("Keine senkrechten Strukturen gefunden – nur nach waagerechten Strukturen (Horizont) gedreht: Roll %+.1f°.", o.Roll)
			case frontal:
				t = fmt.Sprintf("Automatisch frontal ausgerichtet: Roll %+.1f°, Pitch %+.1f°, Yaw %+.1f°.", begr.Roll, begr.Pitch, begr.Yaw)
			default:
				t = fmt.Sprintf("Automatisch ausgerichtet: Roll %+.1f°, Pitch %+.1f°.", begr.Roll, begr.Pitch)
			}
			if anteil < 0.999 {
				t += fmt.Sprintf(" Die volle Korrektur (Pitch %+.1f°) würde das Bild extrem verzerren – daher zu %.0f %% angewendet (Pitch %+.1f°); bei Bedarf am Regler nachstellen.", o.Pitch, anteil*100, begr.Pitch)
			}
			t += fmt.Sprintf(" Übereinstimmung %.0f %%", o.Guete*100)
			if o.Unsicher {
				t += " – unsicher, bitte prüfen (ggf. Brennweite kontrollieren oder Bezugsmarken setzen)."
			} else {
				t += ". Grüne Punkte = Kanten, die jetzt gerade stehen (Taste K blendet sie aus)."
			}
			p.setzeStatus(t)
			p.panel.Invalidate()
			p.cw.Invalidate()
		})
	}()
}

// automatikMarkenUebernehmen: Ergebnis der Formen-Automatik (Rechtecke, Kreise).
func (p *programm) automatikMarkenUebernehmen(e kern.AutoErgebnisMarken, frontal bool) {
	p.autoPunkte = nil
	l := e.Loesung
	p.marken = e.Marken
	p.setzeWert(p.roll, math.Round(l.Roll*10)/10, nil)
	p.setzeWert(p.pitch, math.Round(l.Pitch*10)/10, nil)
	if l.BrennweiteBestimmt {
		p.setzeWert(p.f35, math.Round(l.F35*10)/10, nil)
	}
	if frontal {
		p.setzeYawAuto(math.Round(l.Yaw*10) / 10)
	} else {
		p.setzeWert(p.yaw, 0, nil)
	}
	p.markenLsg, p.markenFehler = l, ""
	rechtecke, kreise := 0, 0
	for _, m := range e.Marken {
		if m.Art == kern.MarkeRechteck {
			rechtecke++
		} else {
			kreise++
		}
	}
	was := fmt.Sprintf("%d Rechtecken", rechtecke)
	if rechtecke == 1 {
		was = "1 Rechteck"
	}
	if kreise > 0 {
		was += fmt.Sprintf(" und %d runden Formen", kreise)
	}
	t := fmt.Sprintf("Automatisch ausgerichtet an %s: Roll %+.1f°, Pitch %+.1f°", was, p.roll.wert, p.pitch.wert)
	if frontal {
		t += fmt.Sprintf(", Yaw %+.1f°", p.yaw.wert)
	}
	if l.BrennweiteBestimmt {
		t += ", Brennweite " + komma1(l.F35) + " mm"
	}
	t += ". Die verwendeten Formen sind markiert (Taste K blendet sie aus); mit „Bearbeiten“ (M) lassen sie sich verschieben oder löschen."
	p.setzeStatus(t)
	p.panel.Invalidate()
	p.cw.Invalidate()
}
