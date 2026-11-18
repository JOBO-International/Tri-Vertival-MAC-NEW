package main

// Fenster „Rahmen“: Rahmenart, Beschriftung, Vorschau – und Speichern des
// gerahmten Bildes in voller Auflösung. Drucken übernimmt der Zusatz
// „Vertical3 Drucken“, falls installiert.

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"vertical3/bild"
	"vertical3/druck"
	"vertical3/kern"
)

var rahmenSchrift *opentype.Font

func ladeSchrift() *opentype.Font {
	if rahmenSchrift != nil {
		return rahmenSchrift
	}
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\Windows`
	}
	for _, n := range []string{"segoeui.ttf", "arial.ttf"} {
		if b, err := os.ReadFile(filepath.Join(windir, "Fonts", n)); err == nil {
			if f, err := opentype.Parse(b); err == nil {
				rahmenSchrift = f
				return f
			}
		}
	}
	rahmenSchrift, _ = opentype.Parse(goregular.TTF)
	return rahmenSchrift
}

var (
	rahmenListe = []struct{ Name, Wert string }{
		{"Linie", druck.RahmenLinie},
		{"Doppellinie", druck.RahmenDoppellinie},
		{"Passepartout (weißer Rand)", druck.RahmenPassepartout},
		{"Passepartout ohne Linie (nur weißer Rand)", druck.RahmenPassepartoutRein},
		{"Galerie (Rand mit Linie)", druck.RahmenGalerie},
		{"Museum (breites Passepartout, schmale Leiste)", druck.RahmenMuseum},
		{"Bilderrahmen Holz hell", druck.RahmenHolzHell},
		{"Bilderrahmen Nussbaum", druck.RahmenNussbaum},
		{"Bilderrahmen Schwarz", druck.RahmenSchwarz},
		{"Bilderrahmen Weiß", druck.RahmenWeiss},
		{"Bilderrahmen Gold", druck.RahmenGold},
		{"Schwebend mit Schatten", druck.RahmenSchatten},
		{"Abgerundete Ecken", druck.RahmenAbgerundet},
		{"Unscharfer Hintergrund", druck.RahmenUnscharf},
		{"Polaroid", druck.RahmenPolaroid},
		{"Diarahmen", druck.RahmenDia},
		{"Kleinbildfilm (Perforation)", druck.RahmenFilmstreifen},
		{"Film (schwarzer Rand)", druck.RahmenFilm},
		{"Büttenrand (Vintage-Abzug)", druck.RahmenBuetten},
		{"Kein Rahmen (nur Beschriftung)", druck.RahmenKein},
	}
	rahmenNamen = func() []string {
		var n []string
		for _, r := range rahmenListe {
			n = append(n, r.Name)
		}
		return n
	}()
	linienFarben = []struct {
		Name  string
		Farbe [3]uint8
	}{{"Schwarz", [3]uint8{0, 0, 0}}, {"Dunkelgrau", [3]uint8{64, 64, 64}}, {"Grau", [3]uint8{128, 128, 128}}, {"Weiß", [3]uint8{255, 255, 255}},
		{"Creme", [3]uint8{240, 234, 214}}, {"Dunkelblau", [3]uint8{24, 40, 72}}, {"Dunkelgrün", [3]uint8{30, 62, 42}}, {"Bordeaux", [3]uint8{102, 22, 34}}}
	textVorlagen = []string{"Keine Beschriftung", "Eigener Text", "Dateiname", "Aufnahmedatum", "Kamera und Objektiv", "Datum · Kamera"}
	// Bezugsgröße: Rahmen und Schrift werden so bemessen, als hätte das
	// gerahmte Bild diese Größe (lange × kurze Seite in mm).
	bezugNamen    = []string{"13 × 18 cm", "20 × 30 cm", "30 × 45 cm", "40 × 60 cm"}
	bezugWerte    = [][2]float64{{180, 130}, {300, 200}, {450, 300}, {600, 400}}
	bezugStart    = 1
	maxPixel      = 12000.0  // längste Kante des gespeicherten Bildes
	grosseFlaeche = 100000.0 // „Blatt“ in mm – beliebig groß, das Bild bestimmt die Größe
)

type rahmenFenster struct {
	p       *programm
	dlg     *walk.Dialog
	bereit  bool
	vor     *walk.CustomWidget
	cbRa    *walk.ComboBox
	neLin   *walk.NumberEdit
	cbLinF  *walk.ComboBox
	cbTxt   *walk.ComboBox
	leTxt   *walk.LineEdit
	nePt    *walk.NumberEdit
	cbBez   *walk.ComboBox
	lblInfo *walk.Label

	auftrag druck.Auftrag
	erg     druck.Ergebnis
	vbmp    *walk.Bitmap
	vkey    string
}

func (p *programm) rahmen() {
	if p.geladen == nil || p.nachher == nil {
		return
	}
	d := &rahmenFenster{p: p}
	_, druckDa := zusatz("Vertical3-Druck.exe")
	_, filterDa := zusatz("Vertical3-Filter.exe")
	breite := 340
	var abbr *walk.PushButton
	err := Dialog{
		AssignTo: &d.dlg, Title: "Tri-Vertical Rahmen – " + filepath.Base(p.pfad), CancelButton: &abbr,
		MinSize: Size{Width: 1100, Height: 720}, Size: Size{Width: 1200, Height: 780},
		Layout: HBox{},
		Children: []Widget{
			CustomWidget{AssignTo: &d.vor, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
				PaintMode: PaintBuffered, PaintPixels: d.male},
			ScrollView{HorizontalFixed: true, MinSize: Size{Width: breite + 30}, MaxSize: Size{Width: breite + 30},
				Layout: VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{Title: "Rahmen", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Art:"},
						ComboBox{AssignTo: &d.cbRa, Model: rahmenNamen, CurrentIndex: 3, OnCurrentIndexChanged: d.aktualisiere},
						Label{Text: "Linie / Farbe:"},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							NumberEdit{AssignTo: &d.neLin, Decimals: 1, MinValue: 0.2, MaxValue: 20, Value: 2.0, OnValueChanged: d.aktualisiere},
							Label{Text: "mm"},
							ComboBox{AssignTo: &d.cbLinF, Model: farbNamen(), CurrentIndex: 0, OnCurrentIndexChanged: d.aktualisiere},
						}},
					}},
					GroupBox{Title: "Beschriftung", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Vorlage:"},
						ComboBox{AssignTo: &d.cbTxt, Model: textVorlagen, CurrentIndex: 0, OnCurrentIndexChanged: d.vorlageGewaehlt},
						Label{Text: "Text:"},
						LineEdit{AssignTo: &d.leTxt, OnTextChanged: d.aktualisiere},
						Label{Text: "Schriftgröße (pt):"},
						NumberEdit{AssignTo: &d.nePt, Decimals: 0, MinValue: 6, MaxValue: 48, Value: 11.0, OnValueChanged: d.aktualisiere},
					}},
					GroupBox{Title: "Proportionen", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Wirkt wie:"},
						ComboBox{AssignTo: &d.cbBez, Model: bezugNamen, CurrentIndex: bezugStart, OnCurrentIndexChanged: d.aktualisiere},
						Label{ColumnSpan: 2, Text: "Rahmenbreite und Schrift werden so bemessen,\nals würde das Bild in dieser Größe gedruckt.", TextColor: walk.RGB(100, 100, 100)},
					}},
					Label{AssignTo: &d.lblInfo, Text: " ", MinSize: Size{Width: breite}, MaxSize: Size{Width: breite}},
					VSpacer{},
					PushButton{Text: "Bild speichern …", OnClicked: d.speichern},
					PushButton{Text: "Filter …", Visible: filterDa, OnClicked: d.filter},
					PushButton{Text: "Drucken …", Visible: druckDa, OnClicked: d.drucken},
					PushButton{AssignTo: &abbr, Text: "Schließen", OnClicked: func() { d.dlg.Cancel() }},
				}},
		},
	}.Create(nil)
	if err != nil {
		walk.MsgBox(nil, "Tri-Vertical Rahmen", err.Error(), walk.MsgBoxIconError)
		return
	}
	if ic, err := walk.NewIconFromResource("APPICON"); err == nil {
		d.dlg.SetIcon(ic)
	}
	{ // mittig auf dem Bildschirm
		var ar win.RECT
		win.SystemParametersInfo(0x0030 /* SPI_GETWORKAREA */, 0, unsafe.Pointer(&ar), 0)
		b := d.dlg.BoundsPixels()
		aw, ah := int(ar.Right-ar.Left), int(ar.Bottom-ar.Top)
		b.Width, b.Height = min(b.Width, aw), min(b.Height, ah)
		b.X, b.Y = int(ar.Left)+(aw-b.Width)/2, int(ar.Top)+(ah-b.Height)/2
		d.dlg.SetBoundsPixels(b)
	}
	d.bereit = true
	d.aktualisiere()
	d.dlg.Run()
	if d.vbmp != nil {
		d.vbmp.Dispose()
	}
}

func farbNamen() []string {
	var n []string
	for _, f := range linienFarben {
		n = append(n, f.Name)
	}
	return n
}

// zusatz: Pfad eines anderen Zusatzprogramms, falls installiert.
func zusatz(exe string) (string, bool) {
	selbst, _ := os.Executable()
	p := filepath.Join(filepath.Dir(selbst), exe)
	_, err := os.Stat(p)
	return p, err == nil
}

// vorlageGewaehlt füllt das Textfeld aus den Bilddaten.
func (d *rahmenFenster) vorlageGewaehlt() {
	if !d.bereit {
		return
	}
	m := d.p.geladen.Meta
	datum := ""
	if t := m["DateTimeOriginal"]; len(t) >= 10 {
		datum = t[8:10] + "." + t[5:7] + "." + t[0:4]
	}
	kamera := strings.TrimSpace(m["Model"])
	if mk := strings.TrimSpace(m["Make"]); mk != "" && !strings.HasPrefix(strings.ToLower(kamera), strings.ToLower(mk)) {
		kamera = mk + " " + kamera
	}
	if l := strings.TrimSpace(m["LensModel"]); l != "" {
		kamera += " · " + l
	}
	var t string
	switch d.cbTxt.CurrentIndex() {
	case 1:
		t = d.leTxt.Text()
		if t == "" {
			t = "Titel"
		}
	case 2:
		t = strings.TrimSuffix(filepath.Base(d.p.pfad), filepath.Ext(d.p.pfad))
	case 3:
		t = datum
	case 4:
		t = kamera
	case 5:
		t = strings.Trim(datum+" · "+kamera, " ·")
	}
	d.leTxt.SetText(t)
	d.aktualisiere()
}

// fotoGroesse: Pixel des korrigierten Fotos in voller Auflösung.
func (d *rahmenFenster) fotoGroesse() (int, int) {
	q := d.p.geladen.Bild
	_, ow, oh, err := kern.Transformation(q.W, q.H, d.p.parameter())
	if err != nil {
		return d.p.nachher.W, d.p.nachher.H
	}
	return ow, oh
}

func (d *rahmenFenster) leseAuftrag() druck.Auftrag {
	bz := bezugWerte[max(0, d.cbBez.CurrentIndex())]
	a := druck.Auftrag{PapierW: grosseFlaeche, PapierH: grosseFlaeche,
		Druckbar: druck.Rechteck{X: 0, Y: 0, W: grosseFlaeche, H: grosseFlaeche},
		Groesse:  druck.GroesseFest, FestW: bz[0], FestH: bz[1],
		LinieMM: d.neLin.Value(), TextPt: d.nePt.Value(), Text: strings.TrimSpace(d.leTxt.Text())}
	if d.cbTxt.CurrentIndex() == 0 {
		a.Text = ""
	}
	if i := d.cbRa.CurrentIndex(); i >= 0 && i < len(rahmenListe) {
		a.Rahmen = rahmenListe[i].Wert
	}
	if i := d.cbLinF.CurrentIndex(); i >= 0 {
		a.LinieFarbe = linienFarben[i].Farbe
	}
	return a
}

// pxProMM für das gespeicherte Bild: Foto behält seine vollen Pixel.
func (d *rahmenFenster) aufloesung(fotoW int) float64 {
	ppm := float64(fotoW) / d.erg.Foto.W
	if lang := math.Max(d.erg.Feld.W, d.erg.Feld.H) * ppm; lang > maxPixel {
		ppm *= maxPixel / lang
	}
	return ppm
}

func (d *rahmenFenster) aktualisiere() {
	if !d.bereit {
		return
	}
	art := rahmenListe[max(0, d.cbRa.CurrentIndex())].Wert
	d.neLin.SetEnabled(art == druck.RahmenLinie)
	d.cbLinF.SetEnabled(art == druck.RahmenLinie || art == druck.RahmenDoppellinie)
	d.leTxt.SetEnabled(d.cbTxt.CurrentIndex() != 0)
	d.nePt.SetEnabled(d.cbTxt.CurrentIndex() != 0)

	d.auftrag = d.leseAuftrag()
	ow, oh := d.fotoGroesse()
	d.erg = druck.Berechne(d.auftrag, ow, oh)
	ppm := d.aufloesung(ow)
	w, h := int(math.Round(d.erg.Feld.W*ppm)), int(math.Round(d.erg.Feld.H*ppm))
	d.lblInfo.SetText(fmt.Sprintf("Gespeichertes Bild: %d × %d Pixel\n(Foto in voller Auflösung, Rahmen ergänzt)", w, h))
	d.vor.Invalidate()
}

// ------------------------------------------------------------------ Vorschau

func (d *rahmenFenster) male(c *walk.Canvas, _ walk.Rectangle) error {
	b := d.vor.ClientBoundsPixels()
	k := float64(c.DPI()) / 96
	c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(225, 225, 225)), b)
	f := d.erg.Feld
	if f.W <= 0 || f.H <= 0 {
		return nil
	}
	rand := 30 * k
	s := math.Min((float64(b.Width)-2*rand)/f.W, (float64(b.Height)-2*rand)/f.H)
	pw, ph := int(f.W*s), int(f.H*s)
	px, py := (b.Width-pw)/2, (b.Height-ph)/2
	key := fmt.Sprintf("%v|%v|%.4f|%p", d.auftrag, d.erg, s, d.p.nachher)
	if key != d.vkey || d.vbmp == nil {
		img := druck.Komponiere(d.p.nachher, d.auftrag, d.erg, s, ladeSchrift())
		if bmp, err := walk.NewBitmapFromImageForDPI(img, 96); err == nil {
			if d.vbmp != nil {
				d.vbmp.Dispose()
			}
			d.vbmp, d.vkey = bmp, key
		}
	}
	sch := int(4 * k)
	c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(180, 180, 180)), walk.Rectangle{X: px + sch, Y: py + sch, Width: pw, Height: ph})
	if d.vbmp != nil {
		c.DrawImagePixels(d.vbmp, walk.Point{X: px, Y: py})
	}
	return nil
}

// ------------------------------------------------------------------ Ausgabe

// gerahmt: Bild mit Rahmen in voller Auflösung.
func (d *rahmenFenster) gerahmt() (*kern.Bild, error) {
	voll, err := kern.Korrigiere(d.p.geladen.Bild, d.p.parameter(), d.p.randFarbe(), false)
	if err != nil {
		return nil, err
	}
	ppm := d.aufloesung(voll.W)
	return kern.AusImage(druck.Komponiere(voll, d.auftrag, d.erg, ppm, ladeSchrift())), nil
}

func (d *rahmenFenster) arbeite(text string, f func() error, fertig string) {
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
				walk.MsgBox(d.dlg, "Tri-Vertical Rahmen", err.Error(), walk.MsgBoxIconError)
				return
			}
			if fertig != "" {
				walk.MsgBox(d.dlg, "Tri-Vertical Rahmen", fertig, walk.MsgBoxIconInformation)
			}
		})
	}()
}

func (d *rahmenFenster) stamm() string {
	return strings.TrimSuffix(filepath.Base(d.p.pfad), filepath.Ext(d.p.pfad)) + "_rahmen"
}

func (d *rahmenFenster) speichern() {
	vorschlag := filepath.Join(filepath.Dir(d.p.pfad), d.stamm()+".jpg")
	dlg := walk.FileDialog{Title: "Bild mit Rahmen speichern", FilePath: vorschlag, InitialDirPath: filepath.Dir(vorschlag),
		Filter: "JPEG (*.jpg)|*.jpg|TIFF verlustfrei (*.tif)|*.tif|PNG (*.png)|*.png"}
	if ok, _ := dlg.ShowSave(d.dlg); !ok {
		return
	}
	ziel := dlg.FilePath
	if filepath.Ext(ziel) == "" {
		ziel += []string{".jpg", ".jpg", ".tif", ".png"}[dlg.FilterIndex]
	}
	g := d.p.geladen
	d.arbeite("Speichere in voller Auflösung …", func() error {
		b, err := d.gerahmt()
		if err != nil {
			return err
		}
		return bild.Speichere(b, ziel, g.Exif, g.ICC)
	}, "Gespeichert:\n"+ziel)
}

// filter: das Foto (korrigiert, ohne Rahmen) an „Vertical3 Filter“ übergeben.
func (d *rahmenFenster) filter() {
	prog, ok := zusatz("Vertical3-Filter.exe")
	if !ok {
		return
	}
	tmp := filepath.Join(os.TempDir(), "Vertical3", d.stamm()+"_fuer_filter.jpg")
	g := d.p.geladen
	d.arbeite("Übergebe an Filter …", func() error {
		b, err := kern.Korrigiere(g.Bild, d.p.parameter(), d.p.randFarbe(), false)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(tmp), 0o755)
		if err := bild.Speichere(b, tmp, g.Exif, g.ICC); err != nil {
			return err
		}
		return exec.Command(prog, tmp).Start()
	}, "")
}

// drucken: gerahmtes Bild an „Vertical3 Drucken“ übergeben.
func (d *rahmenFenster) drucken() {
	prog, ok := zusatz("Vertical3-Druck.exe")
	if !ok {
		return
	}
	tmp := filepath.Join(os.TempDir(), "Vertical3", d.stamm()+".jpg")
	g := d.p.geladen
	d.arbeite("Bereite den Druck vor …", func() error {
		b, err := d.gerahmt()
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(tmp), 0o755)
		if err := bild.Speichere(b, tmp, g.Exif, g.ICC); err != nil {
			return err
		}
		return exec.Command(prog, tmp).Start()
	}, "")
}
