package main

// Selbst gezeichnetes Bedienfeld. Farben und Schriften kommen aus dem
// gewählten Design (design.go): „Hell“ oder „Dunkel“.

import (
	"fmt"
	"math"
	"strings"
	"syscall"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/kern"
)

const panelBreite = 340 // in 96-dpi-Punkten

var (
	farbeBild       = walk.RGB(10, 10, 10)
	farbePanel      = walk.RGB(20, 20, 20)
	farbeStatus     = walk.RGB(16, 16, 16)
	farbeFlaeche    = walk.RGB(34, 34, 34)
	farbeHover      = walk.RGB(48, 48, 48)
	farbeLinie      = walk.RGB(52, 52, 52)
	farbeSpur       = walk.RGB(70, 70, 70)
	farbeText       = walk.RGB(242, 242, 242)
	farbeGrau       = walk.RGB(145, 145, 145)
	farbeDunkelgrau = walk.RGB(85, 85, 85)
	farbeRot        = walk.RGB(226, 0, 26)
	farbeRotDunkel  = walk.RGB(160, 0, 18)
	farbeRotHell    = walk.RGB(255, 84, 96)
	farbeMarke      = walk.RGB(255, 59, 59)
	farbeKante      = walk.RGB(70, 220, 110)
	farbeKanteWaag  = walk.RGB(40, 150, 255)
	rasterFarbe     = [3]uint8{150, 150, 150}
)

// ------------------------------------------------------------------ Pinsel & Schriften

type farbkasten map[walk.Color]*walk.SolidColorBrush

var pinselVorrat = farbkasten{}

func (p *programm) farben() farbkasten { return pinselVorrat }

func (f farbkasten) pinsel(c walk.Color) *walk.SolidColorBrush {
	if b, ok := f[c]; ok {
		return b
	}
	b, _ := walk.NewSolidColorBrush(c)
	f[c] = b
	return b
}

var stiftVorrat = map[string]walk.Pen{}

func stift(c walk.Color, breite int) walk.Pen {
	k := fmt.Sprint(c, breite)
	if s, ok := stiftVorrat[k]; ok {
		return s
	}
	s, _ := walk.NewGeometricPen(walk.PenSolid|walk.PenInsideFrame|walk.PenCapFlat, breite, pinselVorrat.pinsel(c))
	stiftVorrat[k] = s
	return s
}

var schriftVorrat = map[string]*walk.Font{}
var schriftArt = map[string][3]any{}
var gdiSchriften = map[string]win.HFONT{}

// textBreite misst die Breite einer einzeiligen Beschriftung in Pixeln.
func (p *programm) textBreite(c *walk.Canvas, art, text string) int {
	p.schrift(art)
	sp := schriftArt[art]
	dpi := c.DPI()
	key := fmt.Sprint(art, dpi)
	hf, ok := gdiSchriften[key]
	if !ok {
		lf := win.LOGFONT{LfHeight: -int32(win.MulDiv(int32(sp[1].(int)), int32(dpi), 72)), LfWeight: 400, LfCharSet: 1}
		if sp[2].(walk.FontStyle)&walk.FontBold != 0 {
			lf.LfWeight = 700
		}
		name, _ := syscall.UTF16FromString(sp[0].(string))
		copy(lf.LfFaceName[:], name)
		hf = win.CreateFontIndirect(&lf)
		gdiSchriften[key] = hf
	}
	alt := win.SelectObject(c.HDC(), win.HGDIOBJ(hf))
	defer win.SelectObject(c.HDC(), alt)
	u, _ := syscall.UTF16FromString(text)
	var g win.SIZE
	win.GetTextExtentPoint32(c.HDC(), &u[0], int32(len(u)-1), &g)
	return int(g.CX)
}

// schrift: technische Schrift (Bahnschrift, ab Windows 10 vorhanden) für
// Beschriftungen und Werte, Segoe UI für Fließtext.
func (p *programm) schrift(art string) *walk.Font {
	if f, ok := schriftVorrat[art]; ok {
		return f
	}
	sp, ok := schriftFamilien[art]
	if !ok {
		sp = schriftSpez{"Segoe UI", 9, 0}
	}
	schriftArt[art] = [3]any{sp.familie, sp.groesse, sp.stil}
	f, err := walk.NewFont(sp.familie, sp.groesse, sp.stil)
	if err != nil {
		f, _ = walk.NewFont("Segoe UI", sp.groesse, sp.stil)
	}
	schriftVorrat[art] = f
	return f
}

// ------------------------------------------------------------------ Bildfläche: Status & Marken

func (p *programm) statusHoehe() int { return int(30*float64(p.cw.DPI())/96 + 0.5) }

// bildBereich: Bildfläche ohne die Statuszeile unten.
func (p *programm) bildBereich() walk.Rectangle {
	b := p.cw.ClientBoundsPixels()
	b.Height -= p.statusHoehe()
	return b
}

func (p *programm) maleStatus(c *walk.Canvas, ganz walk.Rectangle) {
	h := p.statusHoehe()
	r := walk.Rectangle{X: 0, Y: ganz.Height - h, Width: ganz.Width, Height: h}
	c.FillRectanglePixels(pinselVorrat.pinsel(farbeStatus), r)
	c.FillRectanglePixels(pinselVorrat.pinsel(farbeLinie), walk.Rectangle{X: 0, Y: r.Y, Width: r.Width, Height: 1})
	k := float64(c.DPI()) / 96
	rechts := ""
	if p.geladen != nil {
		rechts = gt("Zoom") + "  " + gt(p.ui.zoomText)
	}
	rw := int(110 * k)
	c.DrawTextPixels(rechts, p.schrift("abschnitt"), farbeGrau,
		walk.Rectangle{X: r.Width - rw - int(14*k), Y: r.Y, Width: rw, Height: h}, walk.TextRight|walk.TextVCenter|walk.TextSingleLine)
	farbe := farbeGrau
	if p.modus != "" {
		farbe = farbeText
		c.FillRectanglePixels(pinselVorrat.pinsel(farbeRot), walk.Rectangle{X: 0, Y: r.Y + 1, Width: int(4 * k), Height: h - 1})
	}
	c.DrawTextPixels(p.statusText, p.schrift("klein"), farbe,
		walk.Rectangle{X: int(14 * k), Y: r.Y, Width: r.Width - rw - int(40*k), Height: h},
		walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
}

// marke: kleines Schild (z. B. ORIGINAL, VORHER, NACHHER) auf dem Bild.
func (p *programm) marke(c *walk.Canvas, text string, x, y int, rot bool) {
	k := float64(c.DPI()) / 96
	f := p.schrift("knopf")
	r := walk.Rectangle{X: x, Y: y, Width: p.textBreite(c, "knopf", text) + int(18*k), Height: int(22 * k)}
	grund := farbeMarkeGrund
	if rot {
		grund = farbeRot
	}
	p.flaeche(c, r, grund, k)
	c.DrawTextPixels(text, f, farbeKnopfText, r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
}

// ------------------------------------------------------------------ Bedienfeld

type element struct {
	r      walk.Rectangle
	id     string
	aus    bool // ausgegraut
	regler *regler
	aktion func()
}

type panelZustand struct {
	elemente []element
	hover    string
	zieht    *regler
	scroll   int
	hoehe    int
	zoomText string
}

func (p *programm) alleRegler() []*regler {
	if !p.yawAktiv {
		return []*regler{p.roll, p.pitch, p.f35}
	}
	return []*regler{p.roll, p.pitch, p.yaw, p.f35}
}

func reglerName(r *regler) string {
	n := strings.TrimSpace(r.name)
	if i := strings.Index(n, "("); i > 0 {
		n = strings.TrimSpace(n[:i])
	}
	if strings.HasPrefix(n, "Brennweite") {
		n = "Brennweite"
	}
	return gt(n)
}

func reglerZusatz(r *regler) string {
	n := strings.TrimSpace(r.name)
	if i := strings.Index(n, "("); i > 0 {
		z := strings.Trim(n[i:], "() ")
		if strings.HasPrefix(n, "Brennweite") {
			return "Kleinbild-äquivalent"
		}
		if strings.HasPrefix(n, "Yaw") && r.bis < 44.9 {
			return z + "  ·  max ±" + strings.Replace(strings.TrimSuffix(fmt.Sprintf("%.1f", r.bis), ".0"), ".", ",", 1) + "°"
		}
		return z
	}
	return ""
}

func reglerText(r *regler) string {
	t := strings.Replace(fmt.Sprintf("%.1f", r.wert), ".", ",", 1)
	if strings.HasPrefix(r.name, "Brennweite") {
		return t + " mm"
	}
	if r.wert > 0.04 {
		t = "+" + t
	} else if r.wert < -0.04 {
		t = strings.Replace(t, "-", "−", 1)
	} else {
		t = "0,0"
	}
	return t + "°"
}

func (p *programm) malePanel(c *walk.Canvas, _ walk.Rectangle) error {
	b := p.panel.ClientBoundsPixels()
	k := float64(c.DPI()) / 96
	px := func(v float64) int { return int(v*k + 0.5) }
	fk := pinselVorrat
	c.FillRectanglePixels(fk.pinsel(farbePanel), b)
	c.FillRectanglePixels(fk.pinsel(farbeLinie), walk.Rectangle{X: 0, Y: 0, Width: 1, Height: b.Height})

	p.ui.elemente = p.ui.elemente[:0]
	rand := px(20)
	breite := b.Width - 2*rand
	y := px(20) - p.ui.scroll

	text := func(t string, f *walk.Font, farbe walk.Color, r walk.Rectangle, format walk.DrawTextFormat) {
		c.DrawTextPixels(t, f, farbe, r, format)
	}
	abschnitt := func(titel string) {
		y += px(10)
		titel = gt(titel)
		f := p.schrift("abschnitt")
		tw := p.textBreite(c, "abschnitt", titel)
		text(titel, f, farbeAbschnitt, walk.Rectangle{X: rand, Y: y, Width: tw + 4, Height: px(18)}, walk.TextVCenter|walk.TextSingleLine)
		if designLinien {
			c.FillRectanglePixels(fk.pinsel(farbeLinie), walk.Rectangle{X: rand + tw + px(10), Y: y + px(9), Width: breite - tw - px(10), Height: 1})
		}
		y += px(26)
	}
	// knopf: art "rahmen" (Umriss), "rot" (rot gefüllt), "rotrahmen" (rot umrandet), "flaeche"
	knopf := func(id, beschriftung, art string, r walk.Rectangle, aus bool, aktion func()) {
		hover := p.ui.hover == id && !aus
		grund, rahmen, schrift := farbeFlaeche, farbeLinie, farbeText
		if designName == "office" {
			rahmen = walk.RGB(209, 209, 209)
		}
		switch art {
		case "rot":
			grund, rahmen, schrift = farbeRot, farbeRot, farbeKnopfText
			if hover {
				grund, rahmen = farbeRotDunkel, farbeRotDunkel
			}
		case "rotrahmen":
			grund, rahmen, schrift = farbeAkzentFlaeche, farbeRot, farbeRahmenText
			if hover {
				grund = farbeAkzentHover
			}
		default:
			if hover {
				grund, rahmen = farbeHover, farbeSpur
			}
		}
		if aus {
			grund, rahmen, schrift = farbeAusGrund, farbeLinie, farbeDunkelgrau
		}
		p.flaeche(c, r, grund, k)
		p.rahmen(c, r, rahmen, k)
		text(gt(beschriftung), p.schrift("knopf"), schrift, r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
		p.ui.elemente = append(p.ui.elemente, element{r: r, id: id, aus: aus, aktion: aktion})
	}
	zeile := func(h float64) walk.Rectangle {
		r := walk.Rectangle{X: rand, Y: y, Width: breite, Height: px(h)}
		return r
	}

	// Kopf
	p.maleLogo(c, walk.Rectangle{X: rand, Y: y, Width: breite, Height: px(32)}, px(32), "logoklein", false)
	text(gt("Perspektivkorrektur")+"  ·  "+p.version, p.schrift("klein"), farbeGrau,
		walk.Rectangle{X: rand, Y: y + px(36), Width: breite, Height: px(16)}, walk.TextVCenter|walk.TextSingleLine)
	y += px(60)

	knopf("oeffnen", "Bild öffnen   ·   Strg+O", "rahmen", zeile(32), false, p.oeffnen)
	y += px(32 + 12)

	// Datei- und Kamerainfo
	if p.txtDatei == "" {
		text("Kein Bild geladen", p.schrift("text"), farbeGrau, zeile(18), walk.TextVCenter|walk.TextSingleLine)
		y += px(22)
	} else {
		text(p.txtDatei, p.schrift("text"), farbeText, zeile(18), walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		y += px(22)
		f := p.schrift("klein")
		m, _, _ := c.MeasureTextPixels(p.txtKamera, f, walk.Rectangle{Width: breite, Height: 2000}, walk.TextWordbreak)
		text(p.txtKamera, f, farbeGrau, walk.Rectangle{X: rand, Y: y, Width: breite, Height: m.Height + 2}, walk.TextWordbreak)
		y += m.Height + px(6)
	}

	// Drehen und Spiegeln
	abschnitt("Drehen und Spiegeln")
	{
		ohne := p.geladen == nil
		hb := (breite - px(8)) / 2
		knoepfe := []struct{ id, t, s string }{
			{"dl", "Links drehen 90°", kern.DrehLinks}, {"dr", "Rechts drehen 90°", kern.DrehRechts},
			{"sw", "Spiegeln  ↔", kern.SpiegelWaag}, {"ss", "Spiegeln  ↕", kern.SpiegelSenk},
		}
		for i, kn := range knoepfe {
			sch := kn.s
			x := rand
			if i%2 == 1 {
				x = rand + breite - hb
			}
			knopf("dreh"+kn.id, kn.t, "rahmen", walk.Rectangle{X: x, Y: y, Width: hb, Height: px(30)}, ohne, func() { p.drehen(sch) })
			if i%2 == 1 {
				y += px(30 + 6)
			}
		}
		y += px(6)
	}

	// Regler
	abschnitt("Korrektur")
	for _, r := range p.alleRegler() {
		name := reglerName(r)
		f := p.schrift("label")
		tw := p.textBreite(c, "label", name)
		text(name, f, farbeText, walk.Rectangle{X: rand, Y: y, Width: tw + 4, Height: px(20)}, walk.TextVCenter|walk.TextSingleLine)
		text(reglerZusatz(r), p.schrift("klein"), farbeDunkelgrau,
			walk.Rectangle{X: rand + tw + px(10), Y: y, Width: breite - tw - px(90), Height: px(20)}, walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
		wr := walk.Rectangle{X: rand + breite - px(84), Y: y, Width: px(82), Height: px(20)}
		wf := farbeText
		if p.ui.hover == "wert"+r.name {
			wf = farbeRotHell
		}
		text(reglerText(r), p.schrift("wert"), wf, wr, walk.TextRight|walk.TextVCenter|walk.TextSingleLine|walk.TextNoClip)
		rr := r
		p.ui.elemente = append(p.ui.elemente, element{r: wr, id: "wert" + r.name, aktion: func() { p.wertEingeben(rr) }})
		y += px(22)
		// Spur
		spur := walk.Rectangle{X: rand + px(6), Y: y + px(9), Width: breite - px(12), Height: px(2)}
		c.FillRectanglePixels(fk.pinsel(farbeSpur), spur)
		pos := func(v float64) int {
			return spur.X + int(float64(spur.Width)*(v-r.von)/(r.bis-r.von)+0.5)
		}
		basis := 0.0
		if r.von >= 0 {
			basis = r.von
		}
		a, e := pos(basis), pos(r.wert)
		if a > e {
			a, e = e, a
		}
		c.FillRectanglePixels(fk.pinsel(farbeRot), walk.Rectangle{X: a, Y: spur.Y - px(1), Width: e - a + 1, Height: px(4)})
		if r.von < 0 { // Nullmarke
			c.FillRectanglePixels(fk.pinsel(farbeGrau), walk.Rectangle{X: pos(0), Y: spur.Y - px(4), Width: 1, Height: px(10)})
		}
		kx := pos(r.wert)
		kr := px(7)
		if p.ui.zieht == r || p.ui.hover == "regler"+r.name {
			c.FillEllipsePixels(fk.pinsel(farbeHalo), walk.Rectangle{X: kx - kr - px(4), Y: spur.Y + 1 - kr - px(4), Width: 2*kr + px(8), Height: 2*kr + px(8)})
		}
		c.FillEllipsePixels(fk.pinsel(farbeReglerRand), walk.Rectangle{X: kx - kr, Y: spur.Y + 1 - kr, Width: 2 * kr, Height: 2 * kr})
		if !designDunkel { // Fluent: weißer Ring mit grauem Rand, blauer Kern
			c.DrawEllipsePixels(stift(farbeSpur, 1), walk.Rectangle{X: kx - kr, Y: spur.Y + 1 - kr, Width: 2 * kr, Height: 2 * kr})
			ki := kr - px(3)
			c.FillEllipsePixels(fk.pinsel(farbeReglerKnopf), walk.Rectangle{X: kx - ki, Y: spur.Y + 1 - ki, Width: 2 * ki, Height: 2 * ki})
		}
		p.ui.elemente = append(p.ui.elemente, element{r: walk.Rectangle{X: rand - px(4), Y: y - px(2), Width: breite + px(8), Height: px(24)}, id: "regler" + r.name, regler: r})
		y += px(26)
	}
	halb := (breite - px(10)) / 2
	if p.neigungsZusatz() != nil { // nur mit Zusatzprogramm Kameraneigung
		knopf("kamera", "Kameraneigung", "rahmen", walk.Rectangle{X: rand, Y: y, Width: halb, Height: px(32)}, !p.kameraAktiv, p.kameradatenUebernehmen)
		knopf("zurueck", "Zurücksetzen", "rahmen", walk.Rectangle{X: rand + breite - halb, Y: y, Width: halb, Height: px(32)}, p.geladen == nil, p.zuruecksetzen)
	} else {
		knopf("zurueck", "Zurücksetzen", "rahmen", zeile(32), p.geladen == nil, p.zuruecksetzen)
	}
	y += px(32 + 12)

	schalter := func(id, beschriftung, taste string, wert *bool) {
		r := zeile(28)
		if p.ui.hover == id {
			p.flaeche(c, walk.Rectangle{X: r.X - px(6), Y: r.Y, Width: r.Width + px(12), Height: r.Height}, farbeZeileHover, k)
		}
		text(beschriftung, p.schrift("text"), farbeText, walk.Rectangle{X: r.X, Y: r.Y, Width: r.Width - px(90), Height: r.Height}, walk.TextVCenter|walk.TextSingleLine)
		if taste != "" {
			text(gt(taste), p.schrift("klein"), farbeDunkelgrau, walk.Rectangle{X: r.X + r.Width - px(130), Y: r.Y, Width: px(80), Height: r.Height}, walk.TextRight|walk.TextVCenter|walk.TextSingleLine)
		}
		// Schalter
		sw := walk.Rectangle{X: r.X + r.Width - px(38), Y: r.Y + (r.Height-px(18))/2, Width: px(38), Height: px(18)}
		if !designDunkel { // Fluent-Schalter: Pille mit rundem Knopf
			pille := walk.Size{Width: sw.Height, Height: sw.Height}
			kd := sw.Height - px(8)
			if *wert {
				c.FillRoundedRectanglePixels(fk.pinsel(farbeRot), sw, pille)
				c.FillEllipsePixels(fk.pinsel(farbeSchalterKnopf), walk.Rectangle{X: sw.X + sw.Width - px(4) - kd, Y: sw.Y + px(4), Width: kd, Height: kd})
			} else {
				c.FillRoundedRectanglePixels(fk.pinsel(farbeSchalterAus), sw, pille)
				c.DrawRoundedRectanglePixels(stift(farbeGrau, 1), sw, pille)
				c.FillEllipsePixels(fk.pinsel(farbeGrau), walk.Rectangle{X: sw.X + px(4), Y: sw.Y + px(4), Width: kd, Height: kd})
			}
		} else if *wert {
			c.FillRectanglePixels(fk.pinsel(farbeRot), sw)
			c.FillRectanglePixels(fk.pinsel(farbeText), walk.Rectangle{X: sw.X + sw.Width - px(16), Y: sw.Y + px(2), Width: px(14), Height: sw.Height - px(4)})
		} else {
			c.FillRectanglePixels(fk.pinsel(farbeFlaeche), sw)
			c.DrawRectanglePixels(stift(farbeSpur, 1), sw)
			c.FillRectanglePixels(fk.pinsel(farbeGrau), walk.Rectangle{X: sw.X + px(2), Y: sw.Y + px(2), Width: px(14), Height: sw.Height - px(4)})
		}
		p.ui.elemente = append(p.ui.elemente, element{r: r, id: id, aktion: func() { p.umschalten(wert) }})
		y += px(28)
	}
	// Ausrichten: Automatik
	abschnitt("Ausrichten")
	ohneBild := p.geladen == nil
	knopf("auto", "Automatisch ausrichten   ·   A", "rotrahmen", zeile(36), ohneBild || p.modus != "" || p.beschaeftigt, func() { p.automatik(false) })
	y += px(36 + 6)
	if p.yawAktiv {
		knopf("autofr", "Automatisch frontal   ·   Umschalt+A", "rahmen", zeile(30), ohneBild || p.modus != "" || p.beschaeftigt, func() { p.automatik(true) })
		y += px(30 + 6)
	}
	if p.ui.hover == "auto" || p.ui.hover == "autofr" {
		hilfe := "Senkrechte gerade – auch für Gebäude über Eck"
		if p.ui.hover == "autofr" {
			hilfe = "Senkrechte und Waagerechte gerade – Fläche wird frontal"
		}
		text(gt(hilfe), p.schrift("klein"), farbeGrau, zeile(16), walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
	}
	y += px(26)

	// Ansicht
	abschnitt("Ansicht")
	schalter("zuschn", "Auf Bildinhalt zuschneiden", "", &p.zuschneiden)
	schalter("raster", "Hilfsraster", "", &p.raster)
	schalter("kanten", "Erkannte Kanten anzeigen", "K", &p.kantenZeigen)
	schalter("vergl", "Vorher / Nachher", "V", &p.vergleich)
	schalter("orig", "Original zeigen", "Leertaste", &p.original)
	y += px(6)

	// Zoom
	text(gt("Zoom"), p.schrift("label"), farbeText, walk.Rectangle{X: rand, Y: y, Width: px(60), Height: px(28)}, walk.TextVCenter|walk.TextSingleLine)
	zt := p.ui.zoomText
	if zt == "" {
		zt = "Ganz"
	}
	text(gt(zt), p.schrift("wert"), farbeGrau, walk.Rectangle{X: rand + px(52), Y: y, Width: px(70), Height: px(28)}, walk.TextVCenter|walk.TextSingleLine)
	xr := rand + breite
	knopf("zplus", "+", "rahmen", walk.Rectangle{X: xr - px(30), Y: y, Width: px(30), Height: px(28)}, ohneBild, func() { p.zoomSchritt(1.5) })
	knopf("zganz", "Ganz", "rahmen", walk.Rectangle{X: xr - px(30+6+60), Y: y, Width: px(60), Height: px(28)}, ohneBild, p.zoomGanz)
	knopf("zminus", "−", "rahmen", walk.Rectangle{X: xr - px(30+6+60+6+30), Y: y, Width: px(30), Height: px(28)}, ohneBild, func() { p.zoomSchritt(1 / 1.5) })
	y += px(28 + 12)

	// Randfarbe
	text(gt("Randfarbe"), p.schrift("label"), farbeText, walk.Rectangle{X: rand, Y: y, Width: px(100), Height: px(26)}, walk.TextVCenter|walk.TextSingleLine)
	for i := len(randFarben) - 1; i >= 0; i-- {
		rf := randFarben[i]
		n := len(randFarben) - 1 - i
		r := walk.Rectangle{X: xr - px(26) - n*px(34), Y: y + px(2), Width: px(22), Height: px(22)}
		if i == p.randIdx {
			p.flaeche(c, walk.Rectangle{X: r.X - px(3), Y: r.Y - px(3), Width: r.Width + px(6), Height: r.Height + px(6)}, farbeRot, k)
		} else if p.ui.hover == "rand"+rf.Name {
			p.flaeche(c, walk.Rectangle{X: r.X - px(2), Y: r.Y - px(2), Width: r.Width + px(4), Height: r.Height + px(4)}, farbeGrau, k)
		}
		p.flaeche(c, r, walk.RGB(rf.Farbe[0], rf.Farbe[1], rf.Farbe[2]), k)
		p.rahmen(c, r, farbeSpur, k)
		idx := i
		p.ui.elemente = append(p.ui.elemente, element{r: r, id: "rand" + rf.Name, aktion: func() {
			p.randIdx = idx
			p.planeVorschau()
			p.panel.Invalidate()
		}})
	}
	if p.ui.hover != "" && strings.HasPrefix(p.ui.hover, "rand") {
		text(strings.TrimPrefix(p.ui.hover, "rand"), p.schrift("klein"), farbeGrau,
			walk.Rectangle{X: rand + px(82), Y: y, Width: px(70), Height: px(26)}, walk.TextVCenter|walk.TextSingleLine)
	}
	y += px(26 + 22)

	// Ausgabe
	text(gt("Größe"), p.schrift("label"), farbeText, walk.Rectangle{X: rand, Y: y, Width: px(60), Height: px(30)}, walk.TextVCenter|walk.TextSingleLine)
	gtxt := p.groesseText()
	if p.geladen != nil && p.ausgabe.Art != gArtOriginal && p.ausgabe.Art != "" {
		if b, h := p.ausgabeMass(p.ausgabe, p.nichtVergroessern); b > 0 {
			gtxt += fmt.Sprintf("  ·  %d × %d", b, h)
		}
	}
	knopf("groesse", gtxt, "rahmen", walk.Rectangle{X: rand + px(60), Y: y, Width: breite - px(60), Height: px(30)}, false, p.groesseWaehlen)
	y += px(30 + 4)
	if p.geladen != nil && p.ausgabe.Art == gArtDruck {
		if q, farbe, ok := p.druckInfo(p.ausgabe, p.nichtVergroessern); ok {
			if i := strings.Index(q, " ("); i > 0 {
				q = q[:i] // Kurzform im Bedienfeld
			}
			c.FillEllipsePixels(fk.pinsel(farbe), walk.Rectangle{X: rand + px(62), Y: y + px(5), Width: px(8), Height: px(8)})
			text(q, p.schrift("klein"), farbe, walk.Rectangle{X: rand + px(76), Y: y, Width: breite - px(76), Height: px(18)}, walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
			y += px(18)
		}
	}
	if p.ausschnittAktiv() {
		text("Ausschnitt: Rahmen im Bild verschieben, an den Ecken größer/kleiner ziehen",
			p.schrift("klein"), farbeGrau, walk.Rectangle{X: rand, Y: y + px(2), Width: breite, Height: px(30)}, walk.TextWordbreak)
		y += px(34)
		halb := (breite - px(8)) / 2
		knopf("ausdrehen", "Hoch / Quer tauschen", "rahmen", walk.Rectangle{X: rand, Y: y, Width: halb, Height: px(28)}, false, p.ausschnittDrehen)
		knopf("ausneu", "Größter Ausschnitt", "rahmen", walk.Rectangle{X: rand + halb + px(8), Y: y, Width: halb, Height: px(28)}, false, p.ausschnittNeu)
		y += px(28 + 6)
		knopf("auseinpassen", "Ganzes Foto einpassen (mit Rand)", "rahmen", zeile(28), false, p.ausschnittEinpassen)
		y += px(28 + 6)
		if g, v, ok := p.formatVorschlag(); ok {
			kurz := strings.TrimPrefix(g.Name, "Fotopapier ")
			t := fmt.Sprintf("Passt besser: %s", kurz)
			if v < 0.005 {
				t += " (ohne Beschnitt)"
			} else {
				t += fmt.Sprintf(" (nur %.0f %% Beschnitt)", v*100)
			}
			gg := g
			knopf("ausformat", t, "rahmen", zeile(28), false, func() { p.formatUebernehmen(gg) })
			y += px(28 + 6)
		}
		y += px(2)
		knopf("ausweiter", "Ausschnitt speichern und weiterbearbeiten", "rahmen", zeile(32), false, p.ausschnittWeiter)
		y += px(32 + 4)
	}
	y += px(6)
	knopf("speichern", "Speichern   ·   Strg+S", "rot", zeile(40), ohneBild, p.speichern)
	y += px(40 + 8)
	for _, z := range p.zusaetze { // installierte Zusatzprogramme
		if z.Knopf == "" {
			continue
		}
		zz := z
		knopf("zusatz"+z.Exe, z.Knopf, "rahmen", zeile(32), ohneBild && !z.OhneBild, func() { p.starteZusatz(zz) })
		y += px(32 + 8)
	}
	y += px(12)

	p.ui.hoehe = y + p.ui.scroll
	if p.ui.hoehe > b.Height { // Bildlaufleiste
		anteil := float64(b.Height) / float64(p.ui.hoehe)
		h := int(float64(b.Height) * anteil)
		oben := int(float64(p.ui.scroll) * anteil)
		c.FillRectanglePixels(fk.pinsel(farbeSpur), walk.Rectangle{X: b.Width - px(4), Y: oben, Width: px(3), Height: h})
	}
	return nil
}

func (p *programm) panelElement(x, y int) *element {
	for i := len(p.ui.elemente) - 1; i >= 0; i-- {
		e := &p.ui.elemente[i]
		if x >= e.r.X && x < e.r.X+e.r.Width && y >= e.r.Y && y < e.r.Y+e.r.Height {
			return e
		}
	}
	return nil
}

func (p *programm) setzeReglerAusX(r *regler, x int) {
	b := p.panel.ClientBoundsPixels()
	k := float64(p.panel.DPI()) / 96
	rand := int(20*k + 0.5)
	x0 := rand + int(6*k+0.5)
	w := b.Width - 2*rand - int(12*k+0.5)
	t := math.Max(0, math.Min(1, float64(x-x0)/float64(w)))
	v := r.von + t*(r.bis-r.von)
	if r.von < 0 && math.Abs(v) < (r.bis-r.von)*0.006 { // rastet bei 0 ein
		v = 0
	}
	if v != r.wert {
		p.setzeWert(r, v, nil)
	}
}

func (p *programm) panelKlick(x, y int, knopf walk.MouseButton) {
	if knopf != walk.LeftButton {
		return
	}
	e := p.panelElement(x, y)
	if e == nil || e.aus {
		return
	}
	if e.regler != nil {
		if p.geladen == nil {
			return
		}
		p.ui.zieht = e.regler
		win.SetCapture(p.panel.Handle())
		p.setzeReglerAusX(e.regler, x)
		p.panel.Invalidate()
		return
	}
	if e.aktion != nil {
		e.aktion()
	}
	if p.cw != nil && !strings.HasPrefix(e.id, "wert") {
		p.cw.SetFocus() // Tastenkürzel wirken wieder auf das Bild
	}
	p.panel.Invalidate()
}

func (p *programm) panelMaus(x, y int, knopf walk.MouseButton) {
	if p.ui.zieht != nil {
		if knopf&walk.LeftButton == 0 {
			p.panelLos(x, y, knopf)
			return
		}
		p.setzeReglerAusX(p.ui.zieht, x)
		return
	}
	id := ""
	if e := p.panelElement(x, y); e != nil && !e.aus {
		id = e.id
	}
	if id != "" {
		p.panel.SetCursor(walk.CursorHand())
	} else {
		p.panel.SetCursor(walk.CursorArrow())
	}
	if id != p.ui.hover {
		p.ui.hover = id
		p.panel.Invalidate()
	}
}

func (p *programm) panelLos(x, y int, _ walk.MouseButton) {
	if p.ui.zieht != nil {
		p.ui.zieht = nil
		win.ReleaseCapture()
		p.panel.Invalidate()
		p.cw.SetFocus()
	}
}

// panelRad: Mausrad über einem Regler verstellt ihn (Umschalt = 1°-Schritte),
// sonst wird das Bedienfeld gescrollt.
func (p *programm) panelRad(x, y, delta int, umschalt bool) {
	if e := p.panelElement(x, y); e != nil && e.regler != nil && p.geladen != nil {
		schritt := e.regler.schritt
		if umschalt {
			schritt *= 10
		}
		p.setzeWert(e.regler, e.regler.wert+float64(delta)/120*schritt, nil)
		return
	}
	b := p.panel.ClientBoundsPixels()
	neu := p.ui.scroll - delta/120*int(48*float64(p.panel.DPI())/96)
	neu = max(0, min(neu, p.ui.hoehe-b.Height))
	if neu != p.ui.scroll {
		p.ui.scroll = neu
		p.panel.Invalidate()
	}
}

func (p *programm) panelHoverAus() {
	if p.ui.hover != "" {
		p.ui.hover = ""
		p.panel.Invalidate()
	}
}

// wertEingeben: genauen Wert eintippen (Klick auf die Zahl).
func (p *programm) wertEingeben(r *regler) {
	if p.geladen == nil {
		return
	}
	var dlg *walk.Dialog
	var feld *walk.NumberEdit
	var ok, abbr *walk.PushButton
	einheit := "Grad"
	if strings.HasPrefix(r.name, "Brennweite") {
		einheit = "mm"
	}
	erg, err := Dialog{
		AssignTo: &dlg, Title: reglerName(r) + " eingeben", DefaultButton: &ok, CancelButton: &abbr,
		MinSize: Size{Width: 280}, Layout: VBox{},
		Children: []Widget{
			Label{Text: fmt.Sprintf("%s (%s, %g bis %g):", strings.TrimSpace(r.name), einheit, r.von, r.bis)},
			NumberEdit{AssignTo: &feld, Decimals: 1, Increment: r.schritt, MinValue: r.von, MaxValue: r.bis, Value: r.wert, SpinButtonsVisible: true},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &ok, Text: "OK", OnClicked: func() { dlg.Accept() }},
				PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Run(p.mw)
	if err == nil && erg == walk.DlgCmdOK {
		p.setzeWert(r, feld.Value(), nil)
	}
	p.cw.SetFocus()
}
