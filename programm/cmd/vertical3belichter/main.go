// Tri-Vertical Belichter – Zusatzprogramm: S/W-Bogen für den digitalen
// Schwarzweiß-Belichter (TIFF, RGB, 50 × 40 cm, 300 dpi).
//
// Aufruf durch Tri-Vertical:  Vertical3-Belichter.exe <auftrag.json>
// (gleicher Auftrag wie bei Rahmen/Filter). Ohne Auftrag wird nach Bildern gefragt.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/belichter"
	"vertical3/bild"
	"vertical3/filter"
	"vertical3/kern"
)

const version = "1.0"
const titel = "Tri-Vertical Belichter"

type auftragDatei struct {
	Bild        string   `json:"bild"`
	Roll        float64  `json:"roll"`
	Pitch       float64  `json:"pitch"`
	Yaw         float64  `json:"yaw"`
	YawMitte    bool     `json:"yaw_mitte"` // Yaw um die Bildmitte (sonst um die Unterkante)
	F35         float64  `json:"f35"`
	Zuschneiden bool     `json:"zuschneiden"`
	Rand        [3]uint8 `json:"rand"`
	Exiftool    string   `json:"exiftool"`
	Ausrichtung []string `json:"ausrichtung"`
}

// quelle: ein Bild für den Bogen.
type quelle struct {
	pfad     string
	klein    *kern.Bild                 // Vorschau (≤ 1600 px), schon korrigiert
	voll     func() (*kern.Bild, error) // volle Auflösung (korrigiert), beim Speichern
	exiftool string
}

// S/W-Looks: aus dem Filter-Paket plus klassische Farbfilter vor der Linse.
type swLook struct {
	name  string
	werte filter.Einstellung
}

func swLooks() []swLook {
	mix := func(r, g, b, kontrast float64) filter.Einstellung {
		e := filter.Neutral()
		e.SW, e.SWMix, e.Kontrast = 1, [3]float64{r, g, b}, kontrast
		return e
	}
	var l []swLook
	finde := func(name string) {
		for _, v := range filter.Vordefiniert {
			if v.Name == name {
				l = append(l, swLook{name, v.Werte})
			}
		}
	}
	finde("S/W klassisch")
	finde("S/W kontrastreich")
	finde("S/W weich")
	l = append(l,
		swLook{"Gelbfilter (Himmel etwas dunkler)", mix(0.45, 0.50, 0.05, 0.12)},
		swLook{"Orangefilter (Himmel dunkler, Haut heller)", mix(0.65, 0.40, -0.05, 0.18)})
	finde("S/W Rotfilter")
	l = append(l,
		swLook{"Grünfilter (Laub heller, Haut dunkler)", mix(0.15, 0.80, 0.05, 0.12)},
		swLook{"Blaufilter (dunstig, Haut dunkler)", mix(0.10, 0.30, 0.60, 0.05)})
	finde("S/W-Film 100 feinkörnig")
	finde("S/W-Film 400 körnig")
	finde("S/W-Film 400 weich")
	finde("S/W-Film gepusht 3200")
	finde("Infrarot S/W")
	finde("Orthochromatisch")
	finde("Sofortbild S/W")
	return l
}

func fehler(text string) { walk.MsgBox(nil, titel, text, walk.MsgBoxIconError) }

var bildFilter = bild.Filter()

// ladeQuelle: Bild ohne Perspektivkorrektur (weitere Bilder für den Bogen).
func ladeQuelle(pfad, exiftool string) (*quelle, error) {
	g, err := bild.Lade(pfad, exiftool)
	if err != nil {
		return nil, err
	}
	q := &quelle{pfad: pfad, klein: g.Bild.Verkleinert(1600), exiftool: exiftool}
	voll := g.Bild
	q.voll = func() (*kern.Bild, error) { return voll, nil }
	return q, nil
}

func main() {
	var a auftragDatei
	a.F35, a.Zuschneiden, a.Rand = 28, true, [3]uint8{255, 255, 255}
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if strings.EqualFold(filepath.Ext(arg), ".json") {
			b, err := os.ReadFile(arg)
			if err != nil || json.Unmarshal(b, &a) != nil {
				fehler("Der Auftrag von Tri-Vertical konnte nicht gelesen werden.")
				return
			}
			os.Remove(arg)
		} else {
			a.Bild = arg
		}
	}
	var quellen []*quelle
	if a.Bild == "" {
		dlg := walk.FileDialog{Title: titel + " – Bilder wählen", Filter: bildFilter}
		if ok, _ := dlg.ShowOpenMultiple(nil); !ok || len(dlg.FilePaths) == 0 {
			return
		}
		for _, p := range dlg.FilePaths {
			q, err := ladeQuelle(p, a.Exiftool)
			if err != nil {
				fehler(filepath.Base(p) + " konnte nicht geöffnet werden:\n" + err.Error())
				continue
			}
			quellen = append(quellen, q)
		}
		if len(quellen) == 0 {
			return
		}
	} else {
		g, err := bild.Lade(a.Bild, a.Exiftool)
		if err != nil {
			fehler("Das Bild konnte nicht geöffnet werden:\n" + err.Error())
			return
		}
		g.Bild = kern.Ausrichten(g.Bild, a.Ausrichtung)
		par := kern.Parameter{Roll: a.Roll, Pitch: a.Pitch, Yaw: a.Yaw, YawMitte: a.YawMitte, F35: math.Max(5, a.F35), Zuschneiden: a.Zuschneiden}
		klein := g.Bild.Verkleinert(1600)
		k, err := kern.Korrigiere(klein, par, a.Rand, true)
		if err != nil {
			k, par = klein, kern.Parameter{F35: par.F35}
		}
		voll := g.Bild
		rand := a.Rand
		quellen = []*quelle{{pfad: a.Bild, klein: k, exiftool: a.Exiftool, voll: func() (*kern.Bild, error) {
			if par.Roll == 0 && par.Pitch == 0 && par.Yaw == 0 {
				return voll, nil
			}
			return kern.Korrigiere(voll, par, rand, false)
		}}}
	}
	f := &fenster{quellen: quellen, looks: swLooks(), e: ladeEinrichtung(), exiftool: a.Exiftool}
	f.zeigen()
}

// ------------------------------------------------------------------ Einrichtung

func einrichtungsDatei() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "Vertical3", "belichter.json")
}

func ladeEinrichtung() belichter.Einrichtung {
	e := belichter.Standard()
	if b, err := os.ReadFile(einrichtungsDatei()); err == nil {
		json.Unmarshal(b, &e)
	}
	if e.DateiB <= 0 || e.DateiH <= 0 || e.FlaecheB <= 0 || e.FlaecheH <= 0 || e.DPI < 72 {
		e = belichter.Standard()
	}
	return e
}

func speichereEinrichtung(e belichter.Einrichtung) {
	f := einrichtungsDatei()
	os.MkdirAll(filepath.Dir(f), 0o755)
	b, _ := json.MarshalIndent(e, "", "  ")
	os.WriteFile(f, b, 0o644)
}

// ------------------------------------------------------------------ Fenster

type fenster struct {
	dlg      *walk.Dialog
	vor      *walk.CustomWidget
	cbLook   *walk.ComboBox
	slHell   *walk.Slider
	slKontr  *walk.Slider
	cbFormat *walk.ComboBox
	neAbst   *walk.NumberEdit
	chMarken *walk.CheckBox
	lblAnz   *walk.Label
	lblBild  *walk.Label
	lblEinr  *walk.Label
	lblInfo  *walk.Label
	btSpeich *walk.PushButton

	quellen  []*quelle
	looks    []swLook
	e        belichter.Einrichtung
	exiftool string
	bereit   bool
	arbeitet bool

	mu      sync.Mutex
	vbmp    *walk.Bitmap
	swKlein map[string]*kern.Bild // S/W-Vorschau je Quelle+Look
}

func (f *fenster) einstellung() filter.Einstellung {
	i := max(0, f.cbLook.CurrentIndex())
	e := f.looks[i].werte
	e.Belichtung += float64(f.slHell.Value()) / 100      // ±1 Blende
	e.Kontrast += float64(f.slKontr.Value()) / 100 * 0.5 // ±0,5
	return e
}

func (f *fenster) schluessel() string {
	return fmt.Sprintf("%d|%d|%d", f.cbLook.CurrentIndex(), f.slHell.Value(), f.slKontr.Value())
}

func (f *fenster) format() belichter.Format {
	return belichter.Formate[max(0, f.cbFormat.CurrentIndex())]
}

func (f *fenster) plaetze() []belichter.Platz {
	return belichter.Anordnung(f.format(), f.e.FlaecheB*10, f.e.FlaecheH*10, f.neAbst.Value())
}

func (f *fenster) zeigen() {
	var formNamen []string
	for _, x := range belichter.Formate {
		if x.Kurz == 0 {
			formNamen = append(formNamen, fmt.Sprintf("Ganzer Bogen (%s × %s cm)", komma(f.e.FlaecheB), komma(f.e.FlaecheH)))
		} else {
			formNamen = append(formNamen, x.Name)
		}
	}
	var lookNamen []string
	for _, l := range f.looks {
		lookNamen = append(lookNamen, l.name)
	}
	f.swKlein = map[string]*kern.Bild{}
	breite := 330
	var abbr *walk.PushButton
	grau := walk.RGB(100, 100, 100)
	err := Dialog{
		AssignTo: &f.dlg, Title: titel + " " + version, CancelButton: &abbr,
		MinSize: Size{Width: 1100, Height: 720}, Size: Size{Width: 1240, Height: 800},
		Layout: HBox{},
		Children: []Widget{
			CustomWidget{AssignTo: &f.vor, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
				PaintMode: PaintBuffered, PaintPixels: f.male},
			ScrollView{HorizontalFixed: true, MinSize: Size{Width: breite + 30}, MaxSize: Size{Width: breite + 30},
				Layout: VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{Title: "Schwarzweiß", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Look:"},
						ComboBox{AssignTo: &f.cbLook, Model: lookNamen, CurrentIndex: 0, OnCurrentIndexChanged: f.aktualisiere},
						Label{Text: "Helligkeit:"},
						Slider{AssignTo: &f.slHell, MinValue: -100, MaxValue: 100, Value: 0, OnValueChanged: f.aktualisiere},
						Label{Text: "Kontrast:"},
						Slider{AssignTo: &f.slKontr, MinValue: -100, MaxValue: 100, Value: 0, OnValueChanged: f.aktualisiere},
					}},
					GroupBox{Title: "Bogen", Layout: Grid{Columns: 2}, Children: []Widget{
						Label{Text: "Format:"},
						ComboBox{AssignTo: &f.cbFormat, Model: formNamen, CurrentIndex: 5, OnCurrentIndexChanged: f.aktualisiere},
						Label{Text: "Abstand:"},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							NumberEdit{AssignTo: &f.neAbst, Decimals: 1, OnValueChanged: f.aktualisiere},
							Label{Text: "mm zwischen den Bildern"},
						}},
						CheckBox{AssignTo: &f.chMarken, ColumnSpan: 2, Text: "Schnittmarken", Checked: true, OnCheckedChanged: f.aktualisiere},
						Label{AssignTo: &f.lblAnz, ColumnSpan: 2, Text: " ", Font: Font{Bold: true}},
					}},
					GroupBox{Title: "Bilder", Layout: VBox{}, Children: []Widget{
						Label{AssignTo: &f.lblBild, Text: " ", MinSize: Size{Width: breite - 20}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{Text: "Bilder hinzufügen …", OnClicked: f.hinzufuegen},
							PushButton{Text: "Nur das erste", OnClicked: f.nurErstes},
						}},
					}},
					GroupBox{Title: "Belichter", Layout: VBox{}, Children: []Widget{
						Label{AssignTo: &f.lblEinr, Text: " ", TextColor: grau},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{Text: "Einrichtung …", OnClicked: f.einrichtung},
							PushButton{Text: "Testbogen speichern …", OnClicked: f.testbogen},
						}},
					}},
					Label{AssignTo: &f.lblInfo, Text: " \n \n ", TextColor: grau, MinSize: Size{Width: breite}, MaxSize: Size{Width: breite}},
					VSpacer{},
					PushButton{AssignTo: &f.btSpeich, Text: "Für den Belichter speichern (TIFF) …", OnClicked: f.speichern},
					PushButton{AssignTo: &abbr, Text: "Schließen", OnClicked: func() { f.dlg.Cancel() }},
				}},
		},
	}.Create(nil)
	if err != nil {
		fehler(err.Error())
		return
	}
	if ic, err := walk.NewIconFromResource("APPICON"); err == nil {
		f.dlg.SetIcon(ic)
	}
	f.neAbst.SetRange(0, 20) // Bereich und Wert erst nach dem Anlegen setzen
	f.neAbst.SetValue(3)
	{ // mittig auf dem Bildschirm
		var ar win.RECT
		win.SystemParametersInfo(0x0030, 0, unsafe.Pointer(&ar), 0)
		b := f.dlg.BoundsPixels()
		aw, ah := int(ar.Right-ar.Left), int(ar.Bottom-ar.Top)
		b.Width, b.Height = min(b.Width, aw), min(b.Height, ah)
		b.X, b.Y = int(ar.Left)+(aw-b.Width)/2, int(ar.Top)+(ah-b.Height)/2
		f.dlg.SetBoundsPixels(b)
	}
	f.bereit = true
	f.aktualisiere()
	f.dlg.Run()
}

func komma(v float64) string {
	s := strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", v), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}

// swBild: S/W-Vorschau einer Quelle mit dem aktuellen Look.
func (f *fenster) swBild(i int) *kern.Bild {
	k := fmt.Sprint(i, "|", f.schluessel())
	if b, ok := f.swKlein[k]; ok {
		return b
	}
	b := belichter.Grau(filter.Anwenden(f.quellen[i].klein, f.einstellung()))
	f.swKlein[k] = b
	return b
}

func (f *fenster) aktualisiere() {
	if !f.bereit {
		return
	}
	pl := f.plaetze()
	fm := f.format()
	switch {
	case len(pl) == 0:
		f.lblAnz.SetText("Dieses Format passt mit diesem Abstand nicht auf den Bogen.")
	case fm.Kurz == 0:
		f.lblAnz.SetText("1 Bild über den ganzen Bogen")
	default:
		f.lblAnz.SetText(fmt.Sprintf("%d Bilder %s pro Bogen", len(pl), fm.Name))
	}
	n := len(f.quellen)
	if n == 1 {
		f.lblBild.SetText("1 Bild – füllt alle Plätze.\n" + filepath.Base(f.quellen[0].pfad))
	} else {
		f.lblBild.SetText(fmt.Sprintf("%d Bilder – werden der Reihe nach auf die Plätze verteilt.", n))
	}
	pw, ph := f.e.Pixel()
	f.lblEinr.SetText(fmt.Sprintf("Datei %s × %s cm, %.0f dpi (%d × %d Pixel)\nBelichtungsfläche %s × %s cm",
		komma(f.e.DateiB), komma(f.e.DateiH), f.e.DPI, pw, ph, komma(f.e.FlaecheB), komma(f.e.FlaecheH)))
	f.lblInfo.SetText("Jedes Bild wird mittig auf seinen Platz zugeschnitten\n(bei Bedarf um 90° gedreht). Die Vorschau zeigt\nden Bogen in echten Maßen der Belichtungsfläche.")
	// Vorschau neu
	f.erzeugeVorschau()
	f.vor.Invalidate()
}

func (f *fenster) bogen(bilder []*kern.Bild) belichter.Bogen {
	return belichter.Bogen{E: f.e, Plaetze: f.plaetze(), Bilder: bilder, Schnittmarken: f.chMarken.Checked(), Abstand: f.neAbst.Value()}
}

// erzeugeVorschau: Bogen in echten Proportionen der Belichtungsfläche.
func (f *fenster) erzeugeVorschau() {
	var bilder []*kern.Bild
	for i := range f.quellen {
		bilder = append(bilder, f.swBild(i))
	}
	bg := f.bogen(bilder)
	// Vorschau: Datei = Fläche (ohne Streckung), 2 px je mm
	vw, vh := int(f.e.FlaecheB*20), int(f.e.FlaecheH*20)
	img := bg.Rendern(vw, vh)
	bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
	if err != nil {
		return
	}
	if f.vbmp != nil {
		f.vbmp.Dispose()
	}
	f.vbmp = bmp
}

func (f *fenster) male(c *walk.Canvas, _ walk.Rectangle) error {
	b := f.vor.ClientBoundsPixels()
	hg, _ := walk.NewSolidColorBrush(walk.RGB(120, 120, 120))
	defer hg.Dispose()
	c.FillRectanglePixels(hg, b)
	if f.vbmp == nil {
		return nil
	}
	s := f.vbmp.Size()
	k := math.Min(float64(b.Width-40)/float64(s.Width), float64(b.Height-40)/float64(s.Height))
	w, h := int(float64(s.Width)*k), int(float64(s.Height)*k)
	r := walk.Rectangle{X: (b.Width - w) / 2, Y: (b.Height - h) / 2, Width: w, Height: h}
	sch, _ := walk.NewSolidColorBrush(walk.RGB(80, 80, 80))
	defer sch.Dispose()
	c.FillRectanglePixels(sch, walk.Rectangle{X: r.X + 5, Y: r.Y + 5, Width: w, Height: h})
	return c.DrawImageStretchedPixels(f.vbmp, r)
}

// ------------------------------------------------------------------ Bilder

func (f *fenster) hinzufuegen() {
	dlg := walk.FileDialog{Title: "Weitere Bilder für den Bogen", Filter: bildFilter}
	if ok, _ := dlg.ShowOpenMultiple(f.dlg); !ok {
		return
	}
	for _, p := range dlg.FilePaths {
		q, err := ladeQuelle(p, f.exiftool)
		if err != nil {
			walk.MsgBox(f.dlg, titel, filepath.Base(p)+" konnte nicht geöffnet werden:\n"+err.Error(), walk.MsgBoxIconError)
			continue
		}
		f.quellen = append(f.quellen, q)
	}
	f.aktualisiere()
}

func (f *fenster) nurErstes() {
	if len(f.quellen) > 1 {
		f.quellen = f.quellen[:1]
		f.swKlein = map[string]*kern.Bild{}
		f.aktualisiere()
	}
}

// ------------------------------------------------------------------ Einrichtung

func (f *fenster) einrichtung() {
	var dlg *walk.Dialog
	var db, dh, fb, fh, dpi, mb, mh *walk.NumberEdit
	var ok, abbr *walk.PushButton
	e := f.e
	err := (Dialog{
		AssignTo: &dlg, Title: "Belichter einrichten", DefaultButton: &ok, CancelButton: &abbr,
		Layout: Grid{Columns: 4},
		Children: []Widget{
			Label{ColumnSpan: 4, Text: "Die Belichter-Software erwartet eine Datei im Format „Datei“ und skaliert sie auf die\n" +
				"tatsächliche Belichtungsfläche. Tri-Vertical gleicht das aus, damit die Bilder in echter Größe\n" +
				"belichtet werden."},
			Label{Text: "Datei (cm):"},
			NumberEdit{AssignTo: &db, Decimals: 1},
			Label{Text: "×"},
			NumberEdit{AssignTo: &dh, Decimals: 1},
			Label{Text: "Belichtungsfläche (cm):"},
			NumberEdit{AssignTo: &fb, Decimals: 2},
			Label{Text: "×"},
			NumberEdit{AssignTo: &fh, Decimals: 2},
			Label{Text: "Auflösung (dpi):"},
			NumberEdit{AssignTo: &dpi, Decimals: 0},
			HSpacer{ColumnSpan: 2},
			Label{ColumnSpan: 4, Text: "\nTestbogen nachgemessen? Gemessene Größe der 10-cm-Quadrate eintragen –\n" +
				"die Belichtungsfläche wird dann automatisch korrigiert:"},
			Label{Text: "Quadrat gemessen (cm):"},
			NumberEdit{AssignTo: &mb, Decimals: 2},
			Label{Text: "×"},
			NumberEdit{AssignTo: &mh, Decimals: 2},
			PushButton{ColumnSpan: 2, Text: "Standard (50 × 40 / 48,8 × 38,6 / 300)", OnClicked: func() {
				s := belichter.Standard()
				db.SetValue(s.DateiB)
				dh.SetValue(s.DateiH)
				fb.SetValue(s.FlaecheB)
				fh.SetValue(s.FlaecheH)
				dpi.SetValue(s.DPI)
			}},
			PushButton{AssignTo: &ok, Text: "OK", OnClicked: func() {
				e = belichter.Einrichtung{DateiB: db.Value(), DateiH: dh.Value(), FlaecheB: fb.Value(), FlaecheH: fh.Value(), DPI: dpi.Value()}
				// Messung: Quadrat kam als mb × mh statt 10 × 10 cm heraus →
				// die Fläche ist im selben Verhältnis anders als angenommen
				e.FlaecheB *= mb.Value() / 10
				e.FlaecheH *= mh.Value() / 10
				dlg.Accept()
			}},
			PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
		},
	}).Create(f.dlg)
	if err != nil {
		walk.MsgBox(f.dlg, titel, err.Error(), walk.MsgBoxIconError)
		return
	}
	// Bereiche und Werte erst nach dem Anlegen setzen
	for _, x := range []struct {
		ne     *walk.NumberEdit
		lo, hi float64
		v      float64
	}{{db, 5, 200, f.e.DateiB}, {dh, 5, 200, f.e.DateiH}, {fb, 5, 200, f.e.FlaecheB}, {fh, 5, 200, f.e.FlaecheH},
		{dpi, 100, 1200, f.e.DPI}, {mb, 5, 15, 10}, {mh, 5, 15, 10}} {
		x.ne.SetRange(x.lo, x.hi)
		x.ne.SetValue(x.v)
	}
	if dlg.Run() != walk.DlgCmdOK {
		return
	}
	if e != f.e {
		f.e = e
		speichereEinrichtung(e)
		f.aktualisiere()
	}
}

// ------------------------------------------------------------------ Speichern

func (f *fenster) arbeite(text string, job func() error, fertig func()) {
	if f.arbeitet {
		return
	}
	f.arbeitet = true
	f.btSpeich.SetEnabled(false)
	f.lblInfo.SetText(text)
	f.dlg.SetCursor(walk.CursorWait())
	go func() {
		err := job()
		f.dlg.Synchronize(func() {
			f.arbeitet = false
			f.btSpeich.SetEnabled(true)
			f.dlg.SetCursor(nil)
			if err != nil {
				f.lblInfo.SetText("Fehler beim Speichern.")
				walk.MsgBox(f.dlg, titel, err.Error(), walk.MsgBoxIconError)
				return
			}
			fertig()
		})
	}()
}

func (f *fenster) zielDialog(titelText, vorschlag string) (string, bool) {
	dlg := walk.FileDialog{Title: titelText, FilePath: vorschlag, InitialDirPath: filepath.Dir(vorschlag),
		Filter: "TIFF für den Belichter (*.tif)|*.tif"}
	if ok, _ := dlg.ShowSave(f.dlg); !ok {
		return "", false
	}
	z := dlg.FilePath
	if !strings.EqualFold(filepath.Ext(z), ".tif") && !strings.EqualFold(filepath.Ext(z), ".tiff") {
		z += ".tif"
	}
	return z, true
}

func (f *fenster) speichern() {
	pl := f.plaetze()
	if len(pl) == 0 {
		walk.MsgBox(f.dlg, titel, "Dieses Format passt mit diesem Abstand nicht auf den Bogen.", walk.MsgBoxIconInformation)
		return
	}
	stamm := strings.TrimSuffix(f.quellen[0].pfad, filepath.Ext(f.quellen[0].pfad))
	fm := f.format()
	teil := "bogen"
	if fm.Kurz > 0 {
		teil = strings.NewReplacer(" ", "", "×", "x", "cm", "").Replace(fm.Name) + "-x" + fmt.Sprint(len(pl))
	}
	ziel, ok := f.zielDialog("Für den Belichter speichern", stamm+"_belichter_"+teil+".tif")
	if !ok {
		return
	}
	e := f.einstellung()
	quellen := append([]*quelle(nil), f.quellen...)
	bg := f.bogen(nil)
	pw, ph := f.e.Pixel()
	dpi := f.e.DPI
	f.arbeite("Rechne den Bogen in voller Auflösung …", func() error {
		for _, q := range quellen {
			v, err := q.voll()
			if err != nil {
				return fmt.Errorf("%s: %v", filepath.Base(q.pfad), err)
			}
			bg.Bilder = append(bg.Bilder, belichter.Grau(filter.Anwenden(v, e)))
		}
		return belichter.SchreibeTIFF(ziel, bg.Rendern(pw, ph), dpi)
	}, func() {
		f.lblInfo.SetText("Gespeichert: " + filepath.Base(ziel))
		walk.MsgBox(f.dlg, titel, fmt.Sprintf("Gespeichert für den Belichter:\n%s\n\n%d × %d Pixel, %.0f dpi, RGB, unkomprimiert", ziel, pw, ph, dpi),
			walk.MsgBoxIconInformation)
	})
}

func (f *fenster) testbogen() {
	stamm := filepath.Join(filepath.Dir(f.quellen[0].pfad), "Belichter-Testbogen")
	ziel, ok := f.zielDialog("Testbogen speichern", stamm+".tif")
	if !ok {
		return
	}
	e := f.e
	f.arbeite("Erzeuge den Testbogen …", func() error {
		return belichter.SchreibeTIFF(ziel, belichter.Testbogen(e), e.DPI)
	}, func() {
		f.lblInfo.SetText("Testbogen gespeichert: " + filepath.Base(ziel))
		walk.MsgBox(f.dlg, titel, "Testbogen gespeichert:\n"+ziel+"\n\nBitte belichten und nachmessen: Die beiden Quadrate müssen genau 10 × 10 cm groß sein, "+
			"die Lineale an allen vier Rändern vollständig sichtbar.\nWeicht etwas ab, unter „Einrichtung …“ die Belichtungsfläche anpassen.", walk.MsgBoxIconInformation)
	})
}
