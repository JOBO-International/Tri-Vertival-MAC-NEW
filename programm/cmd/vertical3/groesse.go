package main

// Ausgabegröße: Standardgrößen und eigene, gespeicherte Größen. Die Wahl
// gilt für alle weiteren Bilder (auch nach einem Neustart).

import (
	"fmt"
	"math"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"vertical3/kern"
)

// Arten einer Größenangabe
const (
	gArtOriginal = "original"
	gArtLang     = "lang"    // längste Seite A px
	gArtBreite   = "breite"  // Breite A px
	gArtHoehe    = "hoehe"   // Höhe A px
	gArtBox      = "box"     // passt in A × B px
	gArtProzent  = "prozent" // A %
	gArtDruck    = "druck"   // passt in A × B cm bei 300 dpi
)

type groesse struct {
	Name  string  `json:"name"`
	Art   string  `json:"art"`
	A     float64 `json:"a"`
	B     float64 `json:"b,omitempty"`
	eigen bool
}

var standardGroessen = []groesse{
	{Name: "Original (volle Auflösung)", Art: gArtOriginal},
	{Name: "4K UHD – passt in 3840 × 2160", Art: gArtBox, A: 3840, B: 2160},
	{Name: "Full HD – passt in 1920 × 1080", Art: gArtBox, A: 1920, B: 1080},
	{Name: "Längste Seite 3000 px", Art: gArtLang, A: 3000},
	{Name: "Längste Seite 2048 px", Art: gArtLang, A: 2048},
	{Name: "Längste Seite 1600 px (Web)", Art: gArtLang, A: 1600},
	{Name: "Längste Seite 1080 px (soziale Netzwerke)", Art: gArtLang, A: 1080},
	{Name: "Längste Seite 800 px (E-Mail)", Art: gArtLang, A: 800},
	{Name: "Fotopapier 9 × 13 cm", Art: gArtDruck, A: 9, B: 13},
	{Name: "Fotopapier 10 × 15 cm", Art: gArtDruck, A: 10, B: 15},
	{Name: "Fotopapier 11 × 15 cm", Art: gArtDruck, A: 11, B: 15},
	{Name: "Fotopapier 13 × 18 cm", Art: gArtDruck, A: 13, B: 18},
	{Name: "Fotopapier 13 × 19 cm", Art: gArtDruck, A: 13, B: 19},
	{Name: "Fotopapier 15 × 20 cm", Art: gArtDruck, A: 15, B: 20},
	{Name: "Fotopapier 15 × 21 cm", Art: gArtDruck, A: 15, B: 21},
	{Name: "Fotopapier 18 × 24 cm", Art: gArtDruck, A: 18, B: 24},
	{Name: "Fotopapier 18 × 27 cm", Art: gArtDruck, A: 18, B: 27},
	{Name: "Fotopapier 20 × 25 cm", Art: gArtDruck, A: 20, B: 25},
	{Name: "Fotopapier 20 × 30 cm", Art: gArtDruck, A: 20, B: 30},
	{Name: "Fotopapier 24 × 30 cm", Art: gArtDruck, A: 24, B: 30},
	{Name: "Fotopapier 24 × 36 cm", Art: gArtDruck, A: 24, B: 36},
	{Name: "Fotopapier 30 × 40 cm", Art: gArtDruck, A: 30, B: 40},
	{Name: "Fotopapier 30 × 45 cm", Art: gArtDruck, A: 30, B: 45},
	{Name: "Fotopapier 40 × 50 cm", Art: gArtDruck, A: 40, B: 50},
	{Name: "Fotopapier 40 × 60 cm", Art: gArtDruck, A: 40, B: 60},
	{Name: "Fotopapier 50 × 60 cm", Art: gArtDruck, A: 50, B: 60},
	{Name: "Fotopapier 50 × 70 cm", Art: gArtDruck, A: 50, B: 70},
	{Name: "Fotopapier 50 × 75 cm", Art: gArtDruck, A: 50, B: 75},
	{Name: "Fotopapier 60 × 80 cm", Art: gArtDruck, A: 60, B: 80},
	{Name: "Fotopapier 60 × 90 cm", Art: gArtDruck, A: 60, B: 90},
	{Name: "Fotopapier 10 × 10 cm (quadratisch)", Art: gArtDruck, A: 10, B: 10},
	{Name: "Fotopapier 13 × 13 cm (quadratisch)", Art: gArtDruck, A: 13, B: 13},
	{Name: "Fotopapier 15 × 15 cm (quadratisch)", Art: gArtDruck, A: 15, B: 15},
	{Name: "Fotopapier 20 × 20 cm (quadratisch)", Art: gArtDruck, A: 20, B: 20},
	{Name: "Fotopapier 30 × 30 cm (quadratisch)", Art: gArtDruck, A: 30, B: 30},
	{Name: "Fotopapier 10 × 30 cm (Panorama)", Art: gArtDruck, A: 10, B: 30},
	{Name: "Fotopapier 20 × 60 cm (Panorama)", Art: gArtDruck, A: 20, B: 60},
	{Name: "Fotopapier DIN A6 – 10,5 × 14,8 cm", Art: gArtDruck, A: 10.5, B: 14.8},
	{Name: "Fotopapier DIN A5 – 14,8 × 21 cm", Art: gArtDruck, A: 14.8, B: 21},
	{Name: "Fotopapier DIN A4 – 21 × 29,7 cm", Art: gArtDruck, A: 21, B: 29.7},
	{Name: "Fotopapier DIN A3 – 29,7 × 42 cm", Art: gArtDruck, A: 29.7, B: 42},
	{Name: "Fotopapier DIN A3+ – 32,9 × 48,3 cm", Art: gArtDruck, A: 32.9, B: 48.3},
	{Name: "Fotopapier DIN A2 – 42 × 59,4 cm", Art: gArtDruck, A: 42, B: 59.4},
	{Name: "Fotopapier 4 × 6 Zoll (10,2 × 15,2 cm)", Art: gArtDruck, A: 10.16, B: 15.24},
	{Name: "Fotopapier 5 × 7 Zoll (12,7 × 17,8 cm)", Art: gArtDruck, A: 12.7, B: 17.78},
	{Name: "Fotopapier 8 × 10 Zoll (20,3 × 25,4 cm)", Art: gArtDruck, A: 20.32, B: 25.4},
	{Name: "Fotopapier Letter 8,5 × 11 Zoll (21,6 × 27,9 cm)", Art: gArtDruck, A: 21.59, B: 27.94},
	{Name: "Fotopapier 11 × 14 Zoll (27,9 × 35,6 cm)", Art: gArtDruck, A: 27.94, B: 35.56},
	{Name: "Fotopapier 13 × 19 Zoll (33 × 48,3 cm)", Art: gArtDruck, A: 33.02, B: 48.26},
	{Name: "50 %", Art: gArtProzent, A: 50},
	{Name: "25 %", Art: gArtProzent, A: 25},
}

var groessenArten = []struct{ art, text string }{
	{gArtLang, "Längste Seite (px)"},
	{gArtBreite, "Breite (px)"},
	{gArtHoehe, "Höhe (px)"},
	{gArtBox, "Passt in Breite × Höhe (px)"},
	{gArtDruck, "Druckformat cm × cm (300 dpi)"},
	{gArtProzent, "Prozent"},
}

// Zielmass: Ausgabegröße für ein Bild w×h (Seitenverhältnis bleibt immer erhalten).
func (g groesse) Zielmass(w, h int, nichtVergroessern bool) (int, int) {
	return kern.Ausgabemass(g.Art, g.A, g.B, w, h, nichtVergroessern)
}

func (g groesse) Beschreibung() string {
	z := func(v float64) string {
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", v), "0"), ".")
	}
	switch g.Art {
	case gArtLang:
		return "längste Seite " + z(g.A) + " px"
	case gArtBreite:
		return "Breite " + z(g.A) + " px"
	case gArtHoehe:
		return "Höhe " + z(g.A) + " px"
	case gArtBox:
		return "passt in " + z(g.A) + " × " + z(g.B) + " px"
	case gArtDruck:
		return "passt auf " + z(g.A) + " × " + z(g.B) + " cm bei 300 dpi"
	case gArtProzent:
		return z(g.A) + " %"
	}
	return "volle Auflösung"
}

// alleGroessen: Standard + eigene.
func (p *programm) alleGroessen() []groesse {
	out := append([]groesse(nil), standardGroessen...)
	for _, g := range p.eigeneGroessen {
		g.eigen = true
		out = append(out, g)
	}
	return out
}

func (p *programm) groesseLaden() {
	e := ladeEinstellungen()
	p.eigeneGroessen = e.EigeneGroessen
	p.nichtVergroessern = e.NichtVergroessern == nil || *e.NichtVergroessern
	p.ausgabe = standardGroessen[0]
	for _, g := range p.alleGroessen() {
		if g.Name == e.Groesse {
			p.ausgabe = g
		}
	}
}

func (p *programm) groesseSpeichern() {
	e := ladeEinstellungen()
	e.Groesse = p.ausgabe.Name
	nv := p.nichtVergroessern
	e.NichtVergroessern = &nv
	e.EigeneGroessen = p.eigeneGroessen
	speichereEinstellungen(e)
}

// groesseText: Kurztext fürs Bedienfeld.
func (p *programm) groesseText() string {
	if p.ausgabe.Art == gArtOriginal || p.ausgabe.Art == "" {
		return "Original"
	}
	return p.ausgabe.Name
}

// groesseWaehlen: Dialog mit Standardgrößen und eigenen Größen.
func (p *programm) groesseWaehlen() {
	var dlg *walk.Dialog
	var liste *walk.ListBox
	var name *walk.LineEdit
	var art *walk.ComboBox
	var neA, neB *walk.NumberEdit
	var lblB, info, qual *walk.Label
	var loeschen *walk.PushButton
	var ok, abbr *walk.PushButton
	var nichtGroesser *walk.CheckBox

	alle := p.alleGroessen()
	namen := func() []string {
		var l []string
		for _, g := range alle {
			t := g.Name
			if g.eigen {
				t += "   (eigene)"
			}
			l = append(l, t)
		}
		return l
	}
	gewaehlt, nvWahl := -1, p.nichtVergroessern
	auswahl := 0
	for i, g := range alle {
		if g.Name == p.ausgabe.Name {
			auswahl = i
		}
	}
	artTexte := []string{}
	for _, a := range groessenArten {
		artTexte = append(artTexte, a.text)
	}
	vorschau := func() {
		if info == nil || liste == nil || loeschen == nil || nichtGroesser == nil {
			return
		}
		i := liste.CurrentIndex()
		if i < 0 || i >= len(alle) {
			info.SetText("")
			return
		}
		t := alle[i].Beschreibung()
		if p.geladen != nil {
			b, h := p.ausgabeMass(alle[i], nichtGroesser.Checked())
			t += fmt.Sprintf("  –  dieses Bild: %d × %d px", b, h)
		}
		info.SetText(t)
		if qual != nil {
			if q, farbe, ok := p.druckInfo(alle[i], nichtGroesser.Checked()); ok {
				qual.SetText(q)
				qual.SetTextColor(farbe)
			} else {
				qual.SetText("")
			}
		}
		loeschen.SetEnabled(alle[i].eigen)
	}
	artGewechselt := func() {
		if art == nil || neB == nil || lblB == nil {
			return
		}
		i := art.CurrentIndex()
		if i < 0 {
			return
		}
		zwei := groessenArten[i].art == gArtBox || groessenArten[i].art == gArtDruck
		neB.SetEnabled(zwei)
		lblB.SetEnabled(zwei)
	}

	err := Dialog{
		AssignTo: &dlg, Title: "Ausgabegröße", DefaultButton: &ok, CancelButton: &abbr,
		MinSize: Size{Width: 520, Height: 560}, Layout: VBox{},
		Children: []Widget{
			Label{Text: "Größe des gespeicherten Bildes. Fotopapier und „passt in B × H“: Ausschnitt mit dem Rahmen im Bild wählen\n(verschieben, an den Ecken ziehen) – er wird ausgeschnitten und auf die Zielgröße gebracht, Fotopapier mit 300 dpi."},
			ListBox{AssignTo: &liste, Model: namen(), CurrentIndex: auswahl, MinSize: Size{Height: 240},
				OnCurrentIndexChanged: vorschau, OnItemActivated: func() {
					gewaehlt, nvWahl = liste.CurrentIndex(), nichtGroesser.Checked()
					dlg.Accept()
				}},
			Label{AssignTo: &info},
			Label{AssignTo: &qual, Font: Font{Family: "Segoe UI", PointSize: 9, Bold: true}},
			CheckBox{AssignTo: &nichtGroesser, Text: "Kleinere Bilder nicht vergrößern", Checked: p.nichtVergroessern, OnCheckedChanged: vorschau},
			GroupBox{Title: "Eigene Größe anlegen", Layout: Grid{Columns: 4}, Children: []Widget{
				Label{Text: "Name:"},
				LineEdit{AssignTo: &name, ColumnSpan: 3, CueBanner: "z. B. Fotobuch, Website, Kunde Müller"},
				Label{Text: "Art:"},
				ComboBox{AssignTo: &art, Model: artTexte, CurrentIndex: 0, ColumnSpan: 3, OnCurrentIndexChanged: artGewechselt},
				Label{Text: "Wert:"},
				NumberEdit{AssignTo: &neA, Decimals: 1, MinValue: 1, MaxValue: 50000},
				Label{AssignTo: &lblB, Text: "×", Enabled: false},
				NumberEdit{AssignTo: &neB, Decimals: 1, MinValue: 1, MaxValue: 50000, Enabled: false},
				Composite{ColumnSpan: 4, Layout: HBox{MarginsZero: true}, Children: []Widget{
					PushButton{Text: "Speichern", OnClicked: func() {
						n := strings.TrimSpace(name.Text())
						if n == "" {
							walk.MsgBox(dlg, "Eigene Größe", "Bitte einen Namen eingeben.", walk.MsgBoxIconInformation)
							name.SetFocus()
							return
						}
						g := groesse{Name: n, Art: groessenArten[max(0, art.CurrentIndex())].art, A: neA.Value(), B: neB.Value(), eigen: true}
						if g.Art != gArtBox && g.Art != gArtDruck {
							g.B = 0
						}
						// gleicher Name: ersetzen
						ersetzt := false
						for i := range p.eigeneGroessen {
							if strings.EqualFold(p.eigeneGroessen[i].Name, n) {
								p.eigeneGroessen[i], ersetzt = g, true
							}
						}
						if !ersetzt {
							for _, s := range standardGroessen {
								if strings.EqualFold(s.Name, n) {
									walk.MsgBox(dlg, "Eigene Größe", "Diesen Namen hat schon eine Standardgröße.", walk.MsgBoxIconInformation)
									return
								}
							}
							p.eigeneGroessen = append(p.eigeneGroessen, g)
						}
						p.groesseSpeichern()
						alle = p.alleGroessen()
						liste.SetModel(namen())
						for i, a := range alle {
							if a.Name == n {
								liste.SetCurrentIndex(i)
							}
						}
						vorschau()
					}},
					PushButton{AssignTo: &loeschen, Text: "Ausgewählte eigene Größe löschen", Enabled: false, OnClicked: func() {
						i := liste.CurrentIndex()
						if i < 0 || i >= len(alle) || !alle[i].eigen {
							return
						}
						n := alle[i].Name
						for j := range p.eigeneGroessen {
							if p.eigeneGroessen[j].Name == n {
								p.eigeneGroessen = append(p.eigeneGroessen[:j], p.eigeneGroessen[j+1:]...)
								break
							}
						}
						if p.ausgabe.Name == n {
							p.ausgabe = standardGroessen[0]
						}
						p.groesseSpeichern()
						alle = p.alleGroessen()
						liste.SetModel(namen())
						liste.SetCurrentIndex(0)
						vorschau()
					}},
					HSpacer{},
				}},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &ok, Text: "Übernehmen", OnClicked: func() {
					gewaehlt, nvWahl = liste.CurrentIndex(), nichtGroesser.Checked()
					dlg.Accept()
				}},
				PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(p.mw)
	if err != nil {
		walk.MsgBox(p.mw, "Ausgabegröße", "Der Dialog konnte nicht geöffnet werden:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	neA.SetValue(2000)
	neB.SetValue(1500)
	vorschau()
	if dlg.Run() == walk.DlgCmdOK {
		if i := gewaehlt; i >= 0 && i < len(alle) {
			p.ausgabe = alle[i]
		}
		p.nichtVergroessern = nvWahl
		p.groesseSpeichern()
		p.setzeStatus("Ausgabegröße: " + p.ausgabe.Name + " – gilt für alle weiteren Bilder.")
		p.panel.Invalidate()
	}
	p.cw.SetFocus()
}

// korrigiertMass: Pixel des korrigierten Bildes in voller Auflösung.
func (p *programm) korrigiertMass() (int, int) {
	if p.geladen == nil {
		return 0, 0
	}
	w, h := p.geladen.Bild.W, p.geladen.Bild.H
	if p.nachher != nil && p.vorschau != nil && p.vorschau.W > 0 {
		// Seitenverhältnis/Größe nach Korrektur aus der Vorschau hochrechnen
		s := float64(p.geladen.Bild.W) / float64(p.vorschau.W)
		w, h = int(math.Round(float64(p.nachher.W)*s)), int(math.Round(float64(p.nachher.H)*s))
	}
	return w, h
}

// ausgabeMass: Größe des gespeicherten Bildes für die aktuelle Korrektur.
func (p *programm) ausgabeMass(g groesse, nichtGroesser bool) (int, int) {
	w, h := p.korrigiertMass()
	if w == 0 {
		return 0, 0
	}
	if _, ok := p.ausschnittSeiten(g, w, h); ok {
		cw, ch := p.quellMass(g)
		return zielFuerAusschnitt(g, cw, ch, nichtGroesser)
	}
	return g.Zielmass(w, h, nichtGroesser)
}

// Druckqualität: feste Schwellen (gelten für jede Papiergröße).
func qualitaetsStufe(dpi float64) (string, walk.Color) {
	switch {
	case dpi >= 299.5:
		return "sehr gut", walk.RGB(30, 150, 60)
	case dpi >= 250:
		return "gut", walk.RGB(80, 160, 40)
	case dpi >= 200:
		return "befriedigend", walk.RGB(200, 140, 0)
	case dpi >= 150:
		return "ausreichend", walk.RGB(220, 100, 0)
	}
	return "mangelhaft", walk.RGB(210, 40, 40)
}

// druckInfo: Qualitätshinweis zur Größe g für das geöffnete Bild.
// Bei Fotopapier: echte Auflösung (dpi) des Bildes auf dem Papier.
// Sonst: bis zu welcher Papiergröße das gespeicherte Bild sehr gut reicht.
func (p *programm) druckInfo(g groesse, nichtGroesser bool) (string, walk.Color, bool) {
	w, h := p.quellMass(g) // bei festem Seitenverhältnis: Pixel des Ausschnitts
	if w == 0 {
		return "", 0, false
	}
	cm := func(v float64) string { return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) }
	if g.Art == gArtDruck {
		pa, pb := g.A/2.54, g.B/2.54 // Zoll
		if (w >= h) != (pa >= pb) {
			pa, pb = pb, pa
		}
		// Bild passt aufs Papier: die engere Seite bestimmt die Druckgröße
		dpi := math.Max(float64(w)/pa, float64(h)/pb)
		stufe, farbe := qualitaetsStufe(dpi)
		t := fmt.Sprintf("Druckqualität: %s – %.0f dpi", stufe, math.Min(dpi, 9999))
		if dpi > 300.5 {
			t = "Druckqualität: sehr gut – 300 dpi (Bild hat mehr als genug Pixel)"
		} else if dpi < 299.5 {
			wer := "das Bild hat"
			if _, ok := p.ausschnittSeiten(g, w, h); ok {
				wer = "der Ausschnitt hat"
			}
			t += fmt.Sprintf(" (für 300 dpi wären %d × %d px nötig,\n"+wer+" %d × %d px)",
				int(math.Round(float64(w)*300/dpi)), int(math.Round(float64(h)*300/dpi)), w, h)
		}
		return t, farbe, true
	}
	bw, bh := g.Zielmass(w, h, nichtGroesser)
	return fmt.Sprintf("Reicht für Drucke in sehr guter Qualität (300 dpi) bis ca. %s × %s cm,\nin guter Qualität (250 dpi) bis ca. %s × %s cm",
		cm(float64(bw)/300*2.54), cm(float64(bh)/300*2.54), cm(float64(bw)/250*2.54), cm(float64(bh)/250*2.54)), walk.RGB(110, 110, 110), true
}
