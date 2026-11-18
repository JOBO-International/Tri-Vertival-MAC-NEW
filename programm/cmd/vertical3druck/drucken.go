package main

// Fenster „Drucken“: Drucker, Papier, Bildgröße, Einpassen, Kopien und
// Vorschau. Rahmen und Beschriftung gibt es im Zusatz „Vertical3 Rahmen“.

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"vertical3/druck"
	"vertical3/kern"
)

var druckSchrift *opentype.Font

func ladeDruckSchrift() *opentype.Font {
	if druckSchrift != nil {
		return druckSchrift
	}
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\Windows`
	}
	for _, n := range []string{"segoeui.ttf", "arial.ttf"} {
		if b, err := os.ReadFile(filepath.Join(windir, "Fonts", n)); err == nil {
			if f, err := opentype.Parse(b); err == nil {
				druckSchrift = f
				return f
			}
		}
	}
	druckSchrift, _ = opentype.Parse(goregular.TTF)
	return druckSchrift
}

var (
	groessenNamen = func() []string {
		n := []string{"Maximal (druckbarer Bereich)", "Randlos (ganzes Blatt)"}
		for _, f := range druck.Formate {
			n = append(n, f.Name)
		}
		return append(n, "Eigene Größe")
	}()
)

type druckFenster struct {
	p       *programm
	dlg     *walk.Dialog
	bereit  bool
	vor     *walk.CustomWidget
	cbDr    *walk.ComboBox
	cbPap   *walk.ComboBox
	cbAus   *walk.ComboBox
	cbGr    *walk.ComboBox
	neW     *walk.NumberEdit
	neH     *walk.NumberEdit
	cbEin   *walk.ComboBox
	neKop   *walk.NumberEdit
	lblInfo *walk.Label
	btnDr   *walk.PushButton
	btnEin  *walk.PushButton

	namen    []string
	dr       *drucker
	ohne     drucker // Ersatz, wenn kein Drucker installiert ist
	auftrag  druck.Auftrag
	erg      druck.Ergebnis
	vbmp     *walk.Bitmap
	vkey     string
	ziehen   *walk.Point
	ziehVon  [2]float64
	seite    walk.Rectangle // Blatt in der Vorschau (Pixel)
	skala    float64        // Vorschau-Pixel je mm
	fotoQuer bool
}

// Ersatzpapiere ohne Drucker (für Vorschau und Export)
var ersatzPapiere = []papier{{9, "A4", 210, 297}, {8, "A3", 297, 420}, {11, "A5", 148, 210}, {0, "10 × 15 cm", 100, 150}, {0, "13 × 18 cm", 130, 180}, {1, "Letter", 215.9, 279.4}}

func (p *programm) drucken() {
	if p.geladen == nil || p.nachher == nil || p.beschaeftigt {
		return
	}
	d := &druckFenster{p: p}
	var std string
	d.namen, std = druckerListe()
	startIdx := 0
	for i, n := range d.namen {
		if n == std {
			startIdx = i
		}
	}
	druckerModell := d.namen
	if len(druckerModell) == 0 {
		druckerModell = []string{"(kein Drucker installiert – nur Vorschau und Speichern)"}
	}
	d.fotoQuer = p.nachher.W > p.nachher.H
	breite := 340

	var abbr *walk.PushButton
	err := Dialog{
		AssignTo: &d.dlg, Title: "Drucken – " + filepath.Base(p.pfad), CancelButton: &abbr,
		MinSize: Size{Width: 1150, Height: 740}, Size: Size{Width: 1200, Height: 780},
		Layout: HBox{},
		Children: []Widget{
			CustomWidget{AssignTo: &d.vor, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
				PaintMode: PaintBuffered, PaintPixels: d.male,
				OnMouseDown: d.mausDown, OnMouseMove: d.mausMove, OnMouseUp: d.mausUp},
			ScrollView{HorizontalFixed: true, MinSize: Size{Width: breite + 30}, MaxSize: Size{Width: breite + 30},
				Layout: VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{Title: "Drucker und Papier", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Drucker:"},
						ComboBox{AssignTo: &d.cbDr, Model: druckerModell, CurrentIndex: startIdx, OnCurrentIndexChanged: d.druckerGewechselt},
						Label{Text: "Papier:"},
						ComboBox{AssignTo: &d.cbPap, Model: []string{"…"}, CurrentIndex: 0, OnCurrentIndexChanged: d.papierGewechselt},
						Label{Text: "Ausrichtung:"},
						ComboBox{AssignTo: &d.cbAus, Model: []string{"Automatisch (wie das Foto)", "Hochformat", "Querformat"}, CurrentIndex: 0, OnCurrentIndexChanged: d.papierGewechselt},
						HSpacer{},
						PushButton{AssignTo: &d.btnEin, Text: "Druckereinstellungen …", OnClicked: d.treiberDialog},
					}},
					GroupBox{Title: "Bildgröße", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Größe:"},
						ComboBox{AssignTo: &d.cbGr, Model: groessenNamen, CurrentIndex: 0, OnCurrentIndexChanged: d.aktualisiere},
						Label{Text: "Eigene Größe:"},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							NumberEdit{AssignTo: &d.neW, Decimals: 1, MinValue: 2, MaxValue: 200, Value: 20.0, OnValueChanged: d.aktualisiere},
							Label{Text: "×"},
							NumberEdit{AssignTo: &d.neH, Decimals: 1, MinValue: 2, MaxValue: 200, Value: 30.0, OnValueChanged: d.aktualisiere},
							Label{Text: "cm"},
						}},
						Label{Text: "Einpassen:"},
						ComboBox{AssignTo: &d.cbEin, Model: []string{"Ganzes Foto zeigen", "Format füllen (beschneiden)"}, CurrentIndex: 0, OnCurrentIndexChanged: d.aktualisiere},
					}},
					GroupBox{Title: "Ausgabe", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Kopien:"},
						NumberEdit{AssignTo: &d.neKop, Decimals: 0, MinValue: 1, MaxValue: 99, Value: 1.0},
					}},
					Label{AssignTo: &d.lblInfo, Text: " ", MinSize: Size{Width: breite}, MaxSize: Size{Width: breite}},
					VSpacer{},
					PushButton{AssignTo: &d.btnDr, Text: "Drucken", OnClicked: d.druckeJetzt},
					PushButton{AssignTo: &abbr, Text: "Schließen", OnClicked: func() { d.dlg.Cancel() }},
				}},
		},
	}.Create(nil)
	if err != nil {
		walk.MsgBox(nil, "Drucken", err.Error(), walk.MsgBoxIconError)
		return
	}
	if ic, err := walk.NewIconFromResource("APPICON"); err == nil {
		d.dlg.SetIcon(ic)
	}
	d.dlg.SetTitle("Tri-Vertical Drucken – " + filepath.Base(p.pfad))
	// mittig auf dem Bildschirm (das Fenster hat kein Elternfenster)
	{
		var ar win.RECT
		win.SystemParametersInfo(0x0030 /* SPI_GETWORKAREA */, 0, unsafe.Pointer(&ar), 0)
		b := d.dlg.BoundsPixels()
		aw, ah := int(ar.Right-ar.Left), int(ar.Bottom-ar.Top)
		if b.Width > aw {
			b.Width = aw
		}
		if b.Height > ah {
			b.Height = ah
		}
		b.X, b.Y = int(ar.Left)+(aw-b.Width)/2, int(ar.Top)+(ah-b.Height)/2
		d.dlg.SetBoundsPixels(b)
	}
	d.bereit = true
	d.druckerGewechselt()
	d.dlg.Run()
	if d.vbmp != nil {
		d.vbmp.Dispose()
	}
}

func (d *druckFenster) druckerGewechselt() {
	if !d.bereit {
		return
	}
	d.dr = nil
	if i := d.cbDr.CurrentIndex(); i >= 0 && i < len(d.namen) {
		dr, err := neuerDrucker(d.dlg.Handle(), d.namen[i])
		if err != nil {
			walk.MsgBox(d.dlg, "Drucker", "Der Drucker „"+d.namen[i]+"“ meldet sich nicht:\n"+err.Error(), walk.MsgBoxIconWarning)
		} else {
			d.dr = dr
		}
	}
	var namen []string
	aktuell := 0
	if d.dr != nil && len(d.dr.papiere) > 0 {
		dm := d.dr.devmode()
		for i, pa := range d.dr.papiere {
			namen = append(namen, pa.name)
			if dm != nil && pa.id == dm.DmPaperSize {
				aktuell = i
			}
		}
	} else {
		for _, pa := range ersatzPapiere {
			namen = append(namen, pa.name)
		}
	}
	d.bereit = false
	d.cbPap.SetModel(namen)
	d.cbPap.SetCurrentIndex(aktuell)
	d.bereit = true
	d.btnDr.SetEnabled(d.dr != nil)
	d.btnEin.SetEnabled(d.dr != nil)
	d.papierGewechselt()
}

func (d *druckFenster) ausrichtung() int16 {
	switch d.cbAus.CurrentIndex() {
	case 1:
		return 1
	case 2:
		return 2
	}
	if d.fotoQuer {
		return 2
	}
	return 1
}

func (d *druckFenster) papierGewechselt() {
	if !d.bereit {
		return
	}
	i := d.cbPap.CurrentIndex()
	if d.dr != nil {
		var id int16
		if i >= 0 && i < len(d.dr.papiere) {
			id = d.dr.papiere[i].id
		}
		d.dr.setze(d.dlg.Handle(), id, d.ausrichtung())
	} else {
		pa := ersatzPapiere[max(0, i)]
		w, h := pa.w, pa.h
		if d.ausrichtung() == 2 {
			w, h = h, w
		}
		d.ohne = drucker{papierW: w, papierH: h, druckbar: druck.Rechteck{X: 4, Y: 4, W: w - 8, H: h - 8}, dpiX: 300, dpiY: 300}
	}
	d.aktualisiere()
}

func (d *druckFenster) treiberDialog() {
	if d.dr == nil {
		return
	}
	if d.dr.dialog(d.dlg.Handle()) {
		// Papier aus dem Treiberdialog übernehmen
		dm := d.dr.devmode()
		d.bereit = false
		for i, pa := range d.dr.papiere {
			if pa.id == dm.DmPaperSize {
				d.cbPap.SetCurrentIndex(i)
			}
		}
		if dm.DmOrientation == 2 {
			d.cbAus.SetCurrentIndex(2)
		} else {
			d.cbAus.SetCurrentIndex(1)
		}
		d.bereit = true
		d.aktualisiere()
	}
}

func (d *druckFenster) geraet() *drucker {
	if d.dr != nil {
		return d.dr
	}
	return &d.ohne
}

func (d *druckFenster) leseAuftrag() druck.Auftrag {
	g := d.geraet()
	a := druck.Auftrag{PapierW: g.papierW, PapierH: g.papierH, Druckbar: g.druckbar,
		Fuellen: d.cbEin.CurrentIndex() == 1, VerschiebX: d.auftrag.VerschiebX, VerschiebY: d.auftrag.VerschiebY,
		Rahmen: druck.RahmenKein}
	switch i := d.cbGr.CurrentIndex(); {
	case i == 0:
		a.Groesse = druck.GroesseMax
	case i == 1:
		a.Groesse = druck.GroesseRandlos
	case i >= 2 && i < 2+len(druck.Formate):
		a.Groesse = druck.GroesseFest
		a.FestW, a.FestH = druck.Formate[i-2].W, druck.Formate[i-2].H
	default:
		a.Groesse = druck.GroesseFest
		a.FestW, a.FestH = d.neW.Value()*10, d.neH.Value()*10
	}
	return a
}

func (d *druckFenster) aktualisiere() {
	if !d.bereit {
		return
	}
	eigene := d.cbGr.CurrentIndex() == len(groessenNamen)-1
	d.neW.SetEnabled(eigene)
	d.neH.SetEnabled(eigene)

	d.auftrag = d.leseAuftrag()
	q := d.p.geladen.Bild
	par := d.p.parameter()
	// Größe des korrigierten Vollbilds (für die Auflösungsangabe)
	_, ow, oh, err := kern.Transformation(q.W, q.H, par)
	if err != nil {
		ow, oh = d.p.nachher.W, d.p.nachher.H
	}
	d.erg = druck.Berechne(d.auftrag, ow, oh)
	b := d.erg.Beschnitt
	dpi := float64(ow) * (b[2] - b[0]) / (d.erg.Foto.W / 25.4)
	g := d.geraet()
	info := fmt.Sprintf("Blatt: %.1f × %.1f cm\nFoto: %.1f × %.1f cm, ca. %.0f dpi", g.papierW/10, g.papierH/10, d.erg.Foto.W/10, d.erg.Foto.H/10, dpi)
	if dpi < 150 {
		info += "\nHinweis: geringe Auflösung – der Druck kann unscharf wirken."
	}
	if d.erg.Verkleinert {
		info += "\nDie gewählte Größe passt nicht auf das Blatt – sie wurde verkleinert."
	}
	if d.auftrag.Groesse == druck.GroesseRandlos && d.dr != nil &&
		(g.druckbar.X > 0.5 || g.druckbar.Y > 0.5) {
		info += "\nFür echten Randlosdruck in den Druckereinstellungen ein randloses Papier wählen – sonst bleibt ein schmaler Rand."
	}
	if d.auftrag.Fuellen {
		info += "\nAusschnitt in der Vorschau mit der Maus verschieben."
	}
	d.lblInfo.SetText(info)
	d.vor.Invalidate()
}

// ------------------------------------------------------------------ Vorschau

func (d *druckFenster) male(c *walk.Canvas, _ walk.Rectangle) error {
	b := d.vor.ClientBoundsPixels()
	k := float64(c.DPI()) / 96
	c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(225, 225, 225)), b)
	g := d.geraet()
	if g.papierW <= 0 {
		return nil
	}
	rand := 30 * k
	s := math.Min((float64(b.Width)-2*rand)/g.papierW, (float64(b.Height)-2*rand)/g.papierH)
	pw, ph := int(g.papierW*s), int(g.papierH*s)
	px, py := (b.Width-pw)/2, (b.Height-ph)/2
	d.seite, d.skala = walk.Rectangle{X: px, Y: py, Width: pw, Height: ph}, s
	// Schatten und Blatt
	sch := int(4 * k)
	c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(180, 180, 180)), walk.Rectangle{X: px + sch, Y: py + sch, Width: pw, Height: ph})
	c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(255, 255, 255)), d.seite)
	// druckbarer Bereich (gestrichelt)
	if d.auftrag.Groesse != druck.GroesseRandlos {
		dr := g.druckbar
		r := walk.Rectangle{X: px + int(dr.X*s), Y: py + int(dr.Y*s), Width: int(dr.W * s), Height: int(dr.H * s)}
		strich, _ := walk.NewCosmeticPen(walk.PenDot, walk.RGB(190, 190, 190))
		c.DrawRectanglePixels(strich, r)
		strich.Dispose()
	}
	// Feld
	key := fmt.Sprintf("%v|%v|%.4f|%p", d.auftrag, d.erg, s, d.p.nachher)
	if key != d.vkey || d.vbmp == nil {
		img := druck.Komponiere(d.p.nachher, d.auftrag, d.erg, s, ladeDruckSchrift())
		if bmp, err := walk.NewBitmapFromImageForDPI(img, 96); err == nil {
			if d.vbmp != nil {
				d.vbmp.Dispose()
			}
			d.vbmp, d.vkey = bmp, key
		}
	}
	if d.vbmp != nil {
		c.DrawImagePixels(d.vbmp, walk.Point{X: px + int(math.Round(d.erg.Feld.X*s)), Y: py + int(math.Round(d.erg.Feld.Y*s))})
	}
	// Maße
	schrift, _ := walk.NewFont("Segoe UI", 9, 0)
	defer schrift.Dispose()
	c.DrawTextPixels(fmt.Sprintf("%.1f × %.1f cm", g.papierW/10, g.papierH/10), schrift, walk.RGB(90, 90, 90),
		walk.Rectangle{X: px, Y: py + ph + int(6*k), Width: pw, Height: int(20 * k)}, walk.TextCenter|walk.TextSingleLine)
	return nil
}

func (d *druckFenster) imFoto(x, y int) bool {
	f := d.erg.Foto
	fx := float64(d.seite.X) + f.X*d.skala
	fy := float64(d.seite.Y) + f.Y*d.skala
	return float64(x) >= fx && float64(x) <= fx+f.W*d.skala && float64(y) >= fy && float64(y) <= fy+f.H*d.skala
}

func (d *druckFenster) mausDown(x, y int, knopf walk.MouseButton) {
	if knopf == walk.LeftButton && d.auftrag.Fuellen && d.imFoto(x, y) {
		d.ziehen = &walk.Point{X: x, Y: y}
		d.ziehVon = [2]float64{d.auftrag.VerschiebX, d.auftrag.VerschiebY}
		d.vor.SetCursor(walk.CursorSizeAll())
		win.SetCapture(d.vor.Handle())
	}
}

func (d *druckFenster) mausMove(x, y int, knopf walk.MouseButton) {
	if d.ziehen == nil {
		if d.auftrag.Fuellen && d.imFoto(x, y) {
			d.vor.SetCursor(walk.CursorSizeAll())
		} else {
			d.vor.SetCursor(walk.CursorArrow())
		}
		return
	}
	if knopf&walk.LeftButton == 0 {
		d.mausUp(x, y, knopf)
		return
	}
	b := d.erg.Beschnitt
	fw, fh := d.erg.Foto.W*d.skala, d.erg.Foto.H*d.skala
	if a := b[2] - b[0]; a < 0.999 && fw > 0 { // waagerecht beschnitten
		d.auftrag.VerschiebX = clamp1(d.ziehVon[0] - float64(x-d.ziehen.X)*a/fw/((1-a)/2))
	}
	if a := b[3] - b[1]; a < 0.999 && fh > 0 {
		d.auftrag.VerschiebY = clamp1(d.ziehVon[1] - float64(y-d.ziehen.Y)*a/fh/((1-a)/2))
	}
	d.aktualisiere()
}

func (d *druckFenster) mausUp(x, y int, _ walk.MouseButton) {
	if d.ziehen != nil {
		d.ziehen = nil
		win.ReleaseCapture()
		d.vor.SetCursor(walk.CursorArrow())
	}
}

func clamp1(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// ------------------------------------------------------------------ Ausgabe

// vollbild: korrigiertes Foto in voller Auflösung (wie beim Speichern).
func (d *druckFenster) vollbild() (*kern.Bild, error) {
	return kern.Korrigiere(d.p.geladen.Bild, d.p.parameter(), d.p.randFarbe(), false)
}

func (d *druckFenster) arbeite(text string, f func() error, fertig string) {
	d.dlg.SetEnabled(false)
	d.dlg.SetCursor(walk.CursorWait())
	d.lblInfo.SetText(text)
	go func() {
		err := f()
		d.dlg.Synchronize(func() {
			d.dlg.SetEnabled(true)
			d.dlg.SetCursor(nil)
			d.aktualisiere()
			if err != nil {
				walk.MsgBox(d.dlg, "Drucken", err.Error(), walk.MsgBoxIconError)
				return
			}
			d.p.setzeStatus(fertig)
			walk.MsgBox(d.dlg, "Drucken", fertig, walk.MsgBoxIconInformation)
		})
	}()
}

func (d *druckFenster) druckeJetzt() {
	if d.dr == nil {
		return
	}
	a, e, dr := d.auftrag, d.erg, d.dr
	kopien := int(d.neKop.Value())
	titel := "Tri-Vertical – " + filepath.Base(d.p.pfad)
	d.arbeite("Bereite den Druck in voller Auflösung vor …", func() error {
		voll, err := d.vollbild()
		if err != nil {
			return err
		}
		// Druckauflösung: Druckerauflösung, höchstens 360 dpi
		ppm := math.Min(float64(dr.dpiX), 360) / 25.4
		img := druck.Komponiere(voll, a, e, ppm, ladeDruckSchrift())
		return dr.drucke(img, e.Feld, kopien, titel)
	}, fmt.Sprintf("Druckauftrag an „%s“ gesendet (%d %s).", dr.name, kopien, map[bool]string{true: "Kopie", false: "Kopien"}[kopien == 1]))
}

var _ = image.Rect
