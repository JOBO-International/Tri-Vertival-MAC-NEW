package main

// Bedienfelder für Ausgabegröße/Ausschnitt, Bezugsmarken sowie Drehen/Spiegeln.

import (
	"fmt"
	"image"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"vertical3/kern"
)

// ------------------------------------------------------------------ Ausgabegröße

func (p *programm) baueGroessenPanel() fyne.CanvasObject {
	namen := make([]string, len(standardGroessen))
	for i, g := range standardGroessen {
		namen[i] = g.Name
	}
	auswahl := widget.NewSelect(namen, func(n string) {
		for _, g := range standardGroessen {
			if g.Name == n {
				p.ausgabe = g
			}
		}
		p.aus.zuruecksetzen()
		p.syncAusschnitt()
		p.zeigeBild()
	})
	auswahl.SetSelected(standardGroessen[0].Name)
	nv := widget.NewCheck("Nicht vergrößern", func(an bool) { p.nichtVergroessern = an; p.zeigeBild() })
	nv.SetChecked(true)

	p.ausInfo = widget.NewLabel("")
	p.ausInfo.Wrapping = fyne.TextWrapWord
	mk := func(von, bis float64, f func(float64)) *widget.Slider {
		s := widget.NewSlider(von, bis)
		s.Step = 0.005
		s.OnChanged = func(v float64) { f(v); p.zeigeBild() }
		return s
	}
	p.sGroesse = mk(0.1, 1.5, func(v float64) { p.aus.s = v })
	p.sX = mk(0, 1, func(v float64) { p.aus.cx = v })
	p.sY = mk(0, 1, func(v float64) { p.aus.cy = v })
	tausch := widget.NewCheck("Hoch-/Querformat tauschen", func(an bool) { p.aus.gedreht = an; p.zeigeBild() })
	p.ausBox = container.NewVBox(
		widget.NewLabel("Ausschnitt: Größe"), p.sGroesse,
		widget.NewLabel("Ausschnitt: waagerecht"), p.sX,
		widget.NewLabel("Ausschnitt: senkrecht"), p.sY,
		tausch,
	)
	p.ausBox.Hide()
	return container.NewVBox(widget.NewLabelWithStyle("Ausgabegröße", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		auswahl, nv, p.ausBox, p.ausInfo)
}

// syncAusschnitt: Regler und Sichtbarkeit nach Größenwahl bzw. neuem Bild.
func (p *programm) syncAusschnitt() {
	if p.ausBox == nil {
		return
	}
	_, fest := p.aus.seiten(p.ausgabe, 4, 3)
	if fest {
		p.ausBox.Show()
	} else {
		p.ausBox.Hide()
	}
	p.sGroesse.Value, p.sX.Value, p.sY.Value = p.aus.s, p.aus.cx, p.aus.cy
	p.sGroesse.Refresh()
	p.sX.Refresh()
	p.sY.Refresh()
}

// korrigiertMass: Größe des korrigierten Bildes in Pixeln der vollen Auflösung.
func (p *programm) korrigiertMass() (int, int) {
	if p.geladen == nil {
		return 0, 0
	}
	w, h := p.geladen.Bild.W, p.geladen.Bild.H
	if p.nachher != nil && p.vorschau != nil && p.vorschau.W > 0 {
		s := float64(p.geladen.Bild.W) / float64(p.vorschau.W)
		w, h = int(math.Round(float64(p.nachher.W)*s)), int(math.Round(float64(p.nachher.H)*s))
	}
	return w, h
}

func (p *programm) ausgabeText() string {
	w, h := p.korrigiertMass()
	return p.aus.ausgabeText(p.ausgabe, w, h, p.nichtVergroessern)
}

// schleierAusschnitt dunkelt den Bereich außerhalb des Ausschnittsrahmens ab.
func (p *programm) schleierAusschnitt(img *image.RGBA) {
	W, H := img.Rect.Dx(), img.Rect.Dy()
	x, y, rw, rh, ok := p.aus.rechteck(p.ausgabe, W, H)
	if !ok {
		return
	}
	x0, y0, x1, y1 := int(math.Round(x)), int(math.Round(y)), int(math.Round(x+rw)), int(math.Round(y+rh))
	for j := 0; j < H; j++ {
		row := img.Pix[j*img.Stride:]
		for i := 0; i < W; i++ {
			if i >= x0 && i < x1 && j >= y0 && j < y1 {
				// Rahmenlinie
				if i-x0 < 2 || x1-1-i < 2 || j-y0 < 2 || y1-1-j < 2 {
					row[i*4], row[i*4+1], row[i*4+2] = 255, 255, 255
				}
				continue
			}
			row[i*4] /= 3
			row[i*4+1] /= 3
			row[i*4+2] /= 3
		}
	}
}

// ------------------------------------------------------------------ Bezugsmarken

func (p *programm) baueMarkenPanel() fyne.CanvasObject {
	texte := make([]string, len(markenArten))
	for i, m := range markenArten {
		texte[i] = m.text
	}
	p.markenAuswahl = widget.NewSelect(texte, func(t string) { p.markeStarten(markenArtAusText(t)) })
	p.markenAuswahl.PlaceHolder = "Bezugsmarke setzen …"
	p.markenInfo = widget.NewLabel("Noch keine Marken gesetzt.")
	p.markenInfo.Wrapping = fyne.TextWrapWord
	frei := widget.NewCheck("Brennweite aus Marken berechnen", func(an bool) { p.brennweiteFrei = an; p.markenLoesen() })
	return container.NewVBox(
		widget.NewLabelWithStyle("Bezugsmarken", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		p.markenAuswahl,
		container.NewGridWithColumns(2,
			widget.NewButton("Marke fertig", p.markeFertig),
			widget.NewButton("Abbrechen", p.markeAbbrechen)),
		container.NewGridWithColumns(2,
			widget.NewButton("Letzte löschen", p.markeLetzteLoeschen),
			widget.NewButton("Alle löschen", p.markenAlleLoeschen)),
		frei,
		p.markenInfo,
	)
}

// ------------------------------------------------------------------ Drehen / Spiegeln

var drehNamen = map[string]string{
	kern.DrehLinks:   "Um 90° nach links gedreht",
	kern.DrehRechts:  "Um 90° nach rechts gedreht",
	kern.SpiegelWaag: "Waagerecht gespiegelt",
	kern.SpiegelSenk: "Senkrecht gespiegelt",
}

func (p *programm) drehen(schritt string) {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	g := p.geladen
	w, h := g.Bild.W, g.Bild.H
	g.Bild = kern.Ausrichten(g.Bild, []string{schritt})
	p.vorschau = kern.Ausrichten(p.vorschau, []string{schritt})
	for i := range p.marken {
		for j := range p.marken[i].P {
			p.marken[i].P[j] = kern.AusrichtenPunkt(p.marken[i].P[j], w, h, schritt)
		}
	}
	for j := range p.neuePunkte {
		p.neuePunkte[j] = kern.AusrichtenPunkt(p.neuePunkte[j], w, h, schritt)
	}
	Q := kern.AusrichtungsMatrix(schritt)
	p.ausrichtung = append(p.ausrichtung, schritt)
	p.ausrichtQ = Q.Mul(p.ausrichtQ)

	// Korrektur umrechnen, damit sie dasselbe bewirkt
	r, pi, y := kern.WinkelNachAusrichtung(p.roll.wert, p.pitch.wert, p.yaw.wert, Q)
	p.nachher = nil
	p.ansicht.ganz()
	p.aus.zuruecksetzen()
	p.syncAusschnitt()
	p.yawGrenzeAnpassen()
	p.roll.setze(math.Round(r*10) / 10)
	p.pitch.setze(math.Round(pi*10) / 10)
	p.yaw.setze(math.Round(y*10) / 10)
	if len(p.marken) > 0 {
		p.markenLoesen()
	}
	p.info.SetText(fmt.Sprintf("%s\n%d × %d", p.pfad, g.Bild.W, g.Bild.H))
	p.setzeStatus(drehNamen[schritt] + ".")
	p.zeigeBild()
	p.planeVorschau()
}
