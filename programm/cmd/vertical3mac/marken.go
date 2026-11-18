package main

// Bezugsmarken: bekannte Senkrechte, Waagerechte, rechte Winkel, Rechtecke und
// Kreise im Bild anklicken; Roll, Pitch (und Yaw) werden daraus berechnet.
// Die Marken liegen in Pixelkoordinaten des Originalbildes.

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"vertical3/kern"
)

var markenArten = []struct{ art, text string }{
	{kern.MarkeSenkrecht, "Senkrechte (2 Punkte)"},
	{kern.MarkeWaagerecht, "Waagerechte (2 Punkte)"},
	{kern.MarkeEcke, "Rechter Winkel (3 Punkte: oben, Ecke, seitlich)"},
	{kern.MarkeRechteck, "Rechteck (4 Ecken im Umlauf)"},
	{kern.MarkeKreisBoden, "Kreis am Boden (mind. 5 Randpunkte)"},
	{kern.MarkeKreisWand, "Kreis an der Wand (mind. 5 Randpunkte)"},
}

func markenArtText(art string) string {
	for _, m := range markenArten {
		if m.art == art {
			return m.text
		}
	}
	return ""
}

func markenArtAusText(t string) string {
	for _, m := range markenArten {
		if m.text == t {
			return m.art
		}
	}
	return ""
}

// vorschauFaktor: Vollbild-Pixel je Vorschau-Pixel.
func (p *programm) vorschauFaktor() float64 {
	if p.geladen == nil || p.vorschau == nil || p.vorschau.W == 0 {
		return 1
	}
	return float64(p.geladen.Bild.W) / float64(p.vorschau.W)
}

// markeStarten: nächste Klicks gehören zu einer neuen Marke dieser Art.
func (p *programm) markeStarten(art string) {
	if p.geladen == nil || art == "" {
		return
	}
	p.markeNeu, p.neuePunkte = art, nil
	p.chkOrig.SetChecked(true) // Marken werden im Original gesetzt
	p.zeigeBild()
	p.markenStatus()
}

func (p *programm) markenKlick(vx, vy float64) {
	f := p.vorschauFaktor()
	p.neuePunkte = append(p.neuePunkte, [2]float64{vx * f, vy * f})
	if p.markeNeu != kern.MarkeKreisBoden && p.markeNeu != kern.MarkeKreisWand &&
		len(p.neuePunkte) >= kern.PunkteFuer(p.markeNeu) {
		p.markeFertig()
		return
	}
	p.ansicht.raster.Refresh()
	p.markenStatus()
}

// markeFertig übernimmt die angeklickten Punkte als Marke und rechnet neu.
func (p *programm) markeFertig() {
	if p.markeNeu == "" {
		return
	}
	if len(p.neuePunkte) < kern.PunkteFuer(p.markeNeu) {
		p.setzeStatus(fmt.Sprintf("Noch zu wenige Punkte (%d von mindestens %d).", len(p.neuePunkte), kern.PunkteFuer(p.markeNeu)))
		return
	}
	m := kern.Marke{Art: p.markeNeu, P: append([][2]float64(nil), p.neuePunkte...)}
	p.marken = append(p.marken, m)
	p.neuePunkte = nil
	p.markenLoesen()
	// gleiche Art gleich noch einmal bereithalten (mehrere Marken sind üblich)
	p.ansicht.raster.Refresh()
}

func (p *programm) markeAbbrechen() {
	p.markeNeu, p.neuePunkte = "", nil
	if p.markenAuswahl != nil {
		p.markenAuswahl.ClearSelected()
	}
	p.ansicht.raster.Refresh()
	p.markenStatus()
}

func (p *programm) markeLetzteLoeschen() {
	if len(p.neuePunkte) > 0 {
		p.neuePunkte = p.neuePunkte[:len(p.neuePunkte)-1]
	} else if len(p.marken) > 0 {
		p.marken = p.marken[:len(p.marken)-1]
		p.markenLoesen()
	}
	p.ansicht.raster.Refresh()
	p.markenStatus()
}

func (p *programm) markenAlleLoeschen() {
	p.marken, p.neuePunkte, p.markenLsg, p.markenFehler = nil, nil, kern.Loesung{}, ""
	p.ansicht.raster.Refresh()
	p.markenStatus()
}

// markenLoesen: Ausgleichsrechnung über alle Marken; übernimmt Roll/Pitch/Yaw.
func (p *programm) markenLoesen() {
	if p.geladen == nil || len(p.marken) == 0 {
		p.markenLsg, p.markenFehler = kern.Loesung{}, ""
		p.markenStatus()
		return
	}
	g := p.geladen.Bild
	start := [3]float64{p.roll.wert, p.pitch.wert, p.yaw.wert}
	l, err := kern.LoeseMarken(p.marken, g.W, g.H, p.f.wert, start, p.brennweiteFrei)
	if err == nil && !l.BrennweiteBestimmt {
		auto := false
		for _, m := range p.marken {
			auto = auto || (m.Auto && m.Art == kern.MarkeRechteck && !m.NurSenkrecht)
		}
		if auto {
			if x, ok := kern.VerfeinereGleichGross(p.marken, g.W, g.H, p.f.wert, [3]float64{l.Roll, l.Pitch, l.Yaw}); ok {
				l.Roll, l.Pitch, l.Yaw = x[0], x[1], x[2]
				l.Abweichung = kern.MarkenAbweichungen(p.marken, g.W, g.H, p.f.wert, l.Roll, l.Pitch, l.Yaw)
			}
		}
	}
	p.markenLsg, p.markenFehler = l, ""
	if err != nil {
		p.markenFehler = err.Error()
		p.markenStatus()
		return
	}
	p.roll.setze(math.Round(l.Roll*10) / 10)
	p.pitch.setze(math.Round(l.Pitch*10) / 10)
	p.setzeYawAuto(math.Round(l.Yaw*10) / 10)
	if l.BrennweiteBestimmt {
		p.f.setze(math.Round(l.F35*10) / 10)
	}
	p.markenStatus()
}

func (p *programm) markenStatus() {
	t := ""
	switch {
	case p.markeNeu != "":
		t = fmt.Sprintf("Marke „%s“: %d Punkt(e) gesetzt – ins Bild klicken.", markenArtText(p.markeNeu), len(p.neuePunkte))
		if p.markeNeu == kern.MarkeKreisBoden || p.markeNeu == kern.MarkeKreisWand {
			t += " Mit „Marke fertig“ abschließen (mind. 5 Punkte)."
		}
	case len(p.marken) == 0:
		t = "Noch keine Marken gesetzt."
	case p.markenFehler != "":
		t = p.markenFehler
	default:
		maxA := 0.0
		for _, a := range p.markenLsg.Abweichung {
			maxA = math.Max(maxA, a)
		}
		t = fmt.Sprintf("%d Marke(n) · größte Abweichung %.1f°", len(p.marken), maxA)
		if p.markenLsg.Mehrdeutig {
			t += " – mehrdeutig, bitte eine weitere Marke setzen."
		}
	}
	p.markenInfo.SetText(t)
	p.setzeStatus(t)
}

var (
	farbeMarke = color.RGBA{0xe0, 0x30, 0x30, 0xff}
	farbeNeu   = color.RGBA{0xff, 0xa0, 0x00, 0xff}
)

// zeichneMarken blendet Marken (und die gerade gesetzten Punkte) ein.
func (p *programm) zeichneMarken(out *image.RGBA, a *ansicht) {
	f := p.vorschauFaktor()
	abb := func(pt [2]float64) (float64, float64) {
		return a.ox + pt[0]/f*a.s, a.oy + pt[1]/f*a.s
	}
	zeichne := func(pts [][2]float64, art string, c color.RGBA) {
		geschlossen := art == kern.MarkeRechteck
		for i, pt := range pts {
			x, y := abb(pt)
			if i > 0 && art != kern.MarkeKreisBoden && art != kern.MarkeKreisWand {
				x0, y0 := abb(pts[i-1])
				linie(out, x0, y0, x, y, c)
			}
			punkt(out, x, y, 4, c)
		}
		if geschlossen && len(pts) == 4 {
			x0, y0 := abb(pts[3])
			x1, y1 := abb(pts[0])
			linie(out, x0, y0, x1, y1, c)
		}
	}
	for _, m := range p.marken {
		zeichne(m.P, m.Art, farbeMarke)
		if m.Art == kern.MarkeKreisBoden || m.Art == kern.MarkeKreisWand {
			if C, err := kern.KegelschnittAus(m.P); err == nil {
				ep := kern.EllipsenPunkte(C, 90)
				for i := range ep {
					x0, y0 := abb(ep[i])
					x1, y1 := abb(ep[(i+1)%len(ep)])
					linie(out, x0, y0, x1, y1, farbeMarke)
				}
			}
		}
	}
	if p.markeNeu != "" {
		zeichne(p.neuePunkte, p.markeNeu, farbeNeu)
	}
}
