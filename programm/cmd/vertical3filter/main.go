// Vertical3 Filter – Zusatzprogramm zu Vertical3: Looks und Feinabstimmung.
//
// Aufruf durch Vertical3:  Vertical3-Filter.exe <auftrag.json>  (Foto + Perspektivkorrektur)
// Allein gestartet fragt das Programm nach einem Bild.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/bild"
	"vertical3/filter"
	"vertical3/kern"
)

const version = "1.5"

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
	Ausrichtung []string `json:"ausrichtung"` // Drehen/Spiegeln aus Vertical3 (ab 3.2)
}

// regler der Feinabstimmung
type fein struct {
	name      string
	von, bis  float64
	feld      func(e *filter.Einstellung) *float64
	slider    *walk.Slider
	wertLabel *walk.Label
	format    string
}

type fenster struct {
	dlg      *walk.Dialog
	vor      *walk.CustomWidget
	gal      *walk.CustomWidget
	cbAns    *walk.ComboBox
	slStark  *walk.Slider
	lblStark *walk.Label
	lblInfo  *walk.Label
	btnLoe   *walk.PushButton
	feine    []*fein
	setzt    bool

	pfad       string
	g          *bild.Geladen
	korrigiert bool
	par        kern.Parameter
	rand       [3]uint8
	basis      *kern.Bild // korrigierte Vorschau (unbearbeitet)
	ergebnis   *kern.Bild // Vorschau mit Filter

	looks    []filter.Look
	aktiv    int
	staerke  float64
	feinWert filter.Einstellung

	mini      map[int]*walk.Bitmap
	miniMu    sync.Mutex
	galZellen []galZelle
	galHover  int

	vbmp     *walk.Bitmap
	vkey     string
	teiler   float64 // Vergleichsansicht: Lage der Trennlinie 0…1
	ziehen   bool
	bildRect walk.Rectangle

	mu       sync.Mutex
	timer    *time.Timer
	rechenNr int
}

type galZelle struct {
	r   walk.Rectangle
	idx int
}

func komma(s string) string { return strings.Replace(s, ".", ",", 1) }

func fehler(text string) { walk.MsgBox(nil, "Tri-Vertical Filter", text, walk.MsgBoxIconError) }

func main() {
	var a auftragDatei
	a.F35, a.Zuschneiden, a.Rand = 28, true, [3]uint8{255, 255, 255}
	korrigiert := false
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if strings.EqualFold(filepath.Ext(arg), ".json") {
			b, err := os.ReadFile(arg)
			if err != nil || json.Unmarshal(b, &a) != nil {
				fehler("Der Auftrag von Tri-Vertical konnte nicht gelesen werden.")
				return
			}
			os.Remove(arg)
			korrigiert = true
		} else {
			a.Bild = arg
			// von einem anderen Zusatzprogramm übergeben – ist bereits korrigiert
			if strings.HasPrefix(strings.ToLower(arg), strings.ToLower(filepath.Join(os.TempDir(), "Vertical3"))) {
				korrigiert = true
			}
		}
	}
	if a.Bild == "" {
		dlg := walk.FileDialog{Title: "Tri-Vertical Filter – Bild wählen",
			Filter: bild.Filter()}
		if ok, _ := dlg.ShowOpen(nil); !ok {
			return
		}
		a.Bild = dlg.FilePath
	}
	g, err := bild.Lade(a.Bild, a.Exiftool)
	if err != nil {
		fehler("Das Bild konnte nicht geöffnet werden:\n" + err.Error())
		return
	}
	g.Bild = kern.Ausrichten(g.Bild, a.Ausrichtung)
	f := &fenster{pfad: a.Bild, g: g, rand: a.Rand, korrigiert: korrigiert, staerke: 1, teiler: 0.5,
		par:      kern.Parameter{Roll: a.Roll, Pitch: a.Pitch, Yaw: a.Yaw, YawMitte: a.YawMitte, F35: math.Max(5, a.F35), Zuschneiden: a.Zuschneiden},
		feinWert: filter.Einstellung{}, galHover: -1, mini: map[int]*walk.Bitmap{}}
	klein := g.Bild.Verkleinert(1600)
	if korrigiert {
		if k, err := kern.Korrigiere(klein, f.par, f.rand, true); err == nil {
			klein = k
		} else {
			f.korrigiert = false
		}
	}
	f.basis = klein
	f.ladeLooks()
	f.zeige()
}

// ------------------------------------------------------------------ eigene Filter

func eigenOrdner() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "Vertical3", "filter")
}

func (f *fenster) ladeLooks() {
	f.looks = append([]filter.Look(nil), filter.Vordefiniert...)
	dateien, _ := filepath.Glob(filepath.Join(eigenOrdner(), "*.v3look"))
	sort.Strings(dateien)
	for _, d := range dateien {
		b, err := os.ReadFile(d)
		if err != nil {
			continue
		}
		var l filter.Look
		if json.Unmarshal(b, &l) != nil || l.Name == "" {
			continue
		}
		l.Gruppe, l.Eigener, l.Dateipfad = "Eigene Filter", true, d
		f.looks = append(f.looks, l)
	}
	if f.aktiv >= len(f.looks) {
		f.aktiv = 0
	}
}

var unerlaubt = regexp.MustCompile(`[^\p{L}\p{N} _-]+`)

func (f *fenster) eigenenSpeichern() {
	var dlg *walk.Dialog
	var le *walk.LineEdit
	var ok, abbr *walk.PushButton
	var name string
	vorschlag := f.looks[f.aktiv].Name
	if !f.looks[f.aktiv].Eigener {
		vorschlag += " (angepasst)"
	}
	erg, err := Dialog{AssignTo: &dlg, Title: "Eigenen Filter speichern", DefaultButton: &ok, CancelButton: &abbr,
		MinSize: Size{Width: 340}, Layout: VBox{}, Children: []Widget{
			Label{Text: "Name des Filters:"},
			LineEdit{AssignTo: &le, Text: vorschlag},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{HSpacer{},
				PushButton{AssignTo: &ok, Text: "Speichern", OnClicked: func() { name = le.Text(); dlg.Accept() }},
				PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }}}},
		}}.Run(f.dlg)
	if err != nil || erg != walk.DlgCmdOK {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	l := filter.Look{Name: name, Gruppe: "Eigene Filter", Werte: f.einstellung()}
	b, _ := json.MarshalIndent(l, "", "  ")
	os.MkdirAll(eigenOrdner(), 0o755)
	datei := filepath.Join(eigenOrdner(), strings.TrimSpace(unerlaubt.ReplaceAllString(name, "_"))+".v3look")
	if err := os.WriteFile(datei, b, 0o644); err != nil {
		fehler("Der Filter konnte nicht gespeichert werden:\n" + err.Error())
		return
	}
	f.ladeLooks()
	for i, l := range f.looks {
		if l.Dateipfad == datei {
			f.aktiv = i
		}
	}
	// als eigener Filter gilt jetzt alles volle Stärke ohne Feinabstimmung
	f.setzeStaerke(1)
	f.feinZuruecksetzen()
	f.galerieGroesse()
	f.erzeugeMiniaturen()
}

func (f *fenster) eigenenLoeschen() {
	l := f.looks[f.aktiv]
	if !l.Eigener {
		return
	}
	if walk.MsgBox(f.dlg, "Filter löschen", "Den eigenen Filter „"+l.Name+"“ löschen?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
		return
	}
	os.Remove(l.Dateipfad)
	f.aktiv = 0
	f.ladeLooks()
	f.feinWert = filter.Einstellung{}
	f.zeigeRegler()
	f.galerieGroesse()
	f.erzeugeMiniaturen()
	f.neu()
}

// ------------------------------------------------------------------ Einstellung

func (f *fenster) einstellung() filter.Einstellung {
	e := filter.Mische(filter.Neutral(), f.looks[f.aktiv].Werte, f.staerke)
	return filter.Plus(e, f.feinWert)
}

// neu berechnet die Vorschau (verzögert, damit Regelbewegungen flüssig bleiben).
func (f *fenster) neu() {
	if f.dlg == nil {
		return
	}
	f.btnLoe.SetEnabled(f.looks[f.aktiv].Eigener)
	e := f.einstellung()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.timer != nil {
		f.timer.Stop()
	}
	f.rechenNr++
	nr := f.rechenNr
	quelle := f.basis
	f.timer = time.AfterFunc(40*time.Millisecond, func() {
		erg := filter.Anwenden(quelle, e)
		f.dlg.Synchronize(func() {
			f.mu.Lock()
			aktuell := nr == f.rechenNr
			f.mu.Unlock()
			if aktuell {
				f.ergebnis = erg
				f.vkey = ""
				f.vor.Invalidate()
			}
		})
	})
	f.gal.Invalidate()
}

func (f *fenster) setzeStaerke(s float64) {
	f.staerke = s
	f.setzt = true
	f.slStark.SetValue(int(math.Round(s * 100)))
	f.setzt = false
	f.lblStark.SetText(fmt.Sprintf("%.0f %%", s*100))
}

// feinZuruecksetzen: Regler zurück auf die Werte des gewählten Filters.
func (f *fenster) feinZuruecksetzen() {
	f.feinWert = filter.Einstellung{}
	f.zeigeRegler()
	f.neu()
}

// basisWert: Wert des Reglers, wie ihn der Filter bei der aktuellen Stärke vorgibt.
func (f *fenster) basisWert(r *fein) float64 {
	e := filter.Mische(filter.Neutral(), f.looks[f.aktiv].Werte, f.staerke)
	return *r.feld(&e)
}

// zeigeRegler: Die Regler zeigen Filterwert + eigene Änderung.
func (f *fenster) zeigeRegler() {
	f.setzt = true
	for _, r := range f.feine {
		v := f.basisWert(r) + *r.feld(&f.feinWert)
		v = math.Max(r.von, math.Min(r.bis, v))
		r.slider.SetValue(int(math.Round(v * 100)))
		anzeige := v * 100
		if r.name == "Belichtung" {
			anzeige = v
		}
		r.wertLabel.SetText(komma(fmt.Sprintf(r.format, anzeige)))
	}
	f.setzt = false
}

// ------------------------------------------------------------------ Fenster

func (f *fenster) zeige() {
	f.feine = []*fein{
		{name: "Belichtung", von: -2, bis: 2, format: "%+.2f EV", feld: func(e *filter.Einstellung) *float64 { return &e.Belichtung }},
		{name: "Kontrast", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Kontrast }},
		{name: "Lichter", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Lichter }},
		{name: "Tiefen", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Tiefen }},
		{name: "Sättigung", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Saettigung }},
		{name: "Wärme", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Temperatur }},
		{name: "Klarheit", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Klarheit }},
		{name: "Vignette", von: -1, bis: 1, format: "%+.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Vignette }},
		{name: "Körnung", von: 0, bis: 1, format: "%.0f", feld: func(e *filter.Einstellung) *float64 { return &e.Koernung }},
	}
	var feinWidgets []Widget
	for _, r := range f.feine {
		rr := r
		feinWidgets = append(feinWidgets,
			Label{Text: r.name + ":", MinSize: Size{Width: 75}, MaxSize: Size{Width: 75}},
			Slider{AssignTo: &rr.slider, MinValue: int(r.von * 100), MaxValue: int(r.bis * 100), Value: 0, MinSize: Size{Width: 220},
				OnValueChanged: func() {
					if f.setzt {
						return
					}
					v := float64(rr.slider.Value()) / 100
					*rr.feld(&f.feinWert) = v - f.basisWert(rr) // eigene Änderung gegenüber dem Filter
					anzeige := v * 100
					if rr.name == "Belichtung" {
						anzeige = v
					}
					rr.wertLabel.SetText(komma(fmt.Sprintf(rr.format, anzeige)))
					f.neu()
				}},
			Label{AssignTo: &rr.wertLabel, Text: komma(fmt.Sprintf(r.format, 0.0)), MinSize: Size{Width: 62}, MaxSize: Size{Width: 62}},
		)
	}
	_, druckDa := druckProgramm()
	selbst, _ := os.Executable()
	_, errR := os.Stat(filepath.Join(filepath.Dir(selbst), "Vertical3-Rahmen.exe"))
	rahmenDa := errR == nil
	titel := "Tri-Vertical Filter – " + filepath.Base(f.pfad)
	var abbr *walk.PushButton
	breite := 400
	err := Dialog{
		AssignTo: &f.dlg, Title: titel, CancelButton: &abbr,
		MinSize: Size{Width: 1150, Height: 760}, Size: Size{Width: 1250, Height: 820},
		Layout: HBox{},
		Children: []Widget{
			Composite{StretchFactor: 1, Layout: VBox{MarginsZero: true}, Children: []Widget{
				CustomWidget{AssignTo: &f.vor, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
					PaintMode: PaintBuffered, PaintPixels: f.maleVorschau,
					OnMouseDown: f.vorDown, OnMouseMove: f.vorMove, OnMouseUp: f.vorUp},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					Label{Text: "Ansicht:"},
					ComboBox{AssignTo: &f.cbAns, Model: []string{"Nachher", "Vorher", "Vergleich (Trennlinie ziehen)"}, CurrentIndex: 0,
						OnCurrentIndexChanged: func() { f.vkey = ""; f.vor.Invalidate() }},
					Label{AssignTo: &f.lblInfo, Text: map[bool]string{true: "Perspektivkorrektur aus Tri-Vertical übernommen", false: "ohne Perspektivkorrektur"}[f.korrigiert], TextColor: walk.RGB(110, 110, 110)},
					HSpacer{},
					PushButton{Text: "Bild speichern …", MinSize: Size{Width: 130}, OnClicked: f.speichern},
					PushButton{Text: "Rahmen …", Visible: rahmenDa, MinSize: Size{Width: 100}, OnClicked: f.rahmen},
					PushButton{Text: "Drucken …", Visible: druckDa, MinSize: Size{Width: 100}, OnClicked: f.drucken},
					PushButton{AssignTo: &abbr, Text: "Schließen", MinSize: Size{Width: 100}, OnClicked: func() { f.dlg.Cancel() }},
				}},
			}},
			ScrollView{HorizontalFixed: true, MinSize: Size{Width: breite + 30}, MaxSize: Size{Width: breite + 30},
				Layout: VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{Title: "Filter", Layout: VBox{MarginsZero: true}, Children: []Widget{
						ScrollView{HorizontalFixed: true, MinSize: Size{Height: 280}, MaxSize: Size{Height: 280}, Layout: VBox{MarginsZero: true},
							Children: []Widget{
								CustomWidget{AssignTo: &f.gal, ClearsBackground: false, PaintMode: PaintBuffered, PaintPixels: f.maleGalerie,
									MinSize: Size{Width: breite - 30, Height: 300}, MaxSize: Size{Width: breite - 30},
									OnMouseDown: f.galKlick, OnMouseMove: f.galMaus},
							}},
					}},
					GroupBox{Title: "Stärke", Layout: HBox{}, Children: []Widget{
						Slider{AssignTo: &f.slStark, MinValue: 0, MaxValue: 100, Value: 100, OnValueChanged: func() {
							if f.setzt {
								return
							}
							f.staerke = float64(f.slStark.Value()) / 100
							f.lblStark.SetText(fmt.Sprintf("%.0f %%", f.staerke*100))
							f.zeigeRegler()
							f.neu()
						}},
						Label{AssignTo: &f.lblStark, Text: "100 %", MinSize: Size{Width: 50}},
					}},
					GroupBox{Title: "Feinabstimmung", Layout: Grid{Columns: 3, Spacing: 2}, Children: append(feinWidgets,
						HSpacer{}, PushButton{Text: "Auf Filterwerte zurücksetzen", OnClicked: f.feinZuruecksetzen}, HSpacer{})},
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						PushButton{Text: "Als eigenen Filter speichern …", OnClicked: f.eigenenSpeichern},
						PushButton{AssignTo: &f.btnLoe, Text: "Löschen", Enabled: false, OnClicked: f.eigenenLoeschen},
					}},
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
	mittig(f.dlg)
	f.galerieGroesse()
	f.erzeugeMiniaturen()
	f.neu()
	f.dlg.Run()
}

func mittig(d *walk.Dialog) {
	var ar win.RECT
	win.SystemParametersInfo(0x0030 /* SPI_GETWORKAREA */, 0, unsafe.Pointer(&ar), 0)
	b := d.BoundsPixels()
	k := float64(d.DPI()) / 96
	b.Width, b.Height = int(1300*k), int(860*k) // gewünschte Größe, höchstens Bildschirm
	aw, ah := int(ar.Right-ar.Left), int(ar.Bottom-ar.Top)
	b.Width, b.Height = min(b.Width, aw), min(b.Height, ah)
	b.X, b.Y = int(ar.Left)+(aw-b.Width)/2, int(ar.Top)+(ah-b.Height)/2
	d.SetBoundsPixels(b)
}

// ------------------------------------------------------------------ Vorschau

func (f *fenster) maleVorschau(c *walk.Canvas, _ walk.Rectangle) error {
	b := f.vor.ClientBoundsPixels()
	c.FillRectanglePixels(pinsel(walk.RGB(232, 232, 232)), b)
	if f.basis == nil {
		return nil
	}
	nach := f.ergebnis
	if nach == nil {
		nach = f.basis
	}
	modus := f.cbAns.CurrentIndex()
	rand := 16
	s := math.Min(float64(b.Width-2*rand)/float64(f.basis.W), float64(b.Height-2*rand)/float64(f.basis.H))
	w, h := int(float64(f.basis.W)*s), int(float64(f.basis.H)*s)
	x, y := (b.Width-w)/2, (b.Height-h)/2
	f.bildRect = walk.Rectangle{X: x, Y: y, Width: w, Height: h}
	key := fmt.Sprintf("%p|%d|%dx%d|%.4f", nach, modus, w, h, f.teiler)
	if key != f.vkey || f.vbmp == nil {
		var img *kern.Bild
		switch modus {
		case 1:
			img = f.basis
		case 2:
			img = kern.NeuesBild(f.basis.W, f.basis.H)
			grenze := int(f.teiler * float64(f.basis.W))
			for yy := 0; yy < f.basis.H; yy++ {
				o := yy * f.basis.W * 3
				copy(img.Pix[o:o+grenze*3], f.basis.Pix[o:o+grenze*3])
				copy(img.Pix[o+grenze*3:o+f.basis.W*3], nach.Pix[o+grenze*3:o+f.basis.W*3])
			}
		default:
			img = nach
		}
		bmp, err := walk.NewBitmapFromImageForDPI(img.Anzeige(w, h, false, [3]uint8{}), 96)
		if err != nil {
			return nil
		}
		if f.vbmp != nil {
			f.vbmp.Dispose()
		}
		f.vbmp, f.vkey = bmp, key
	}
	c.DrawImagePixels(f.vbmp, walk.Point{X: x, Y: y})
	schrift, _ := walk.NewFont("Segoe UI", 9, walk.FontBold)
	defer schrift.Dispose()
	schild := func(t string, sx int, farbe walk.Color) {
		r := walk.Rectangle{X: sx, Y: y + 10, Width: 80, Height: 22}
		c.FillRectanglePixels(pinsel(farbe), r)
		c.DrawTextPixels(t, schrift, walk.RGB(255, 255, 255), r, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
	}
	switch modus {
	case 1:
		schild("Vorher", x+10, walk.RGB(66, 66, 66))
	case 2:
		lx := x + int(f.teiler*float64(w))
		weiss, _ := walk.NewGeometricPen(walk.PenSolid, 2, pinsel(walk.RGB(255, 255, 255)))
		c.DrawLinePixels(weiss, walk.Point{X: lx, Y: y}, walk.Point{X: lx, Y: y + h})
		weiss.Dispose()
		c.FillEllipsePixels(pinsel(walk.RGB(255, 255, 255)), walk.Rectangle{X: lx - 12, Y: y + h/2 - 12, Width: 24, Height: 24})
		dunkel, _ := walk.NewGeometricPen(walk.PenSolid, 2, pinsel(walk.RGB(60, 60, 60)))
		my := y + h/2
		for _, d := range []int{-1, 1} { // kleine Pfeile nach links und rechts
			c.DrawLinePixels(dunkel, walk.Point{X: lx + d*3, Y: my - 5}, walk.Point{X: lx + d*8, Y: my})
			c.DrawLinePixels(dunkel, walk.Point{X: lx + d*8, Y: my}, walk.Point{X: lx + d*3, Y: my + 5})
		}
		dunkel.Dispose()
		schild("Vorher", x+10, walk.RGB(66, 66, 66))
		schild("Nachher", x+w-90, walk.RGB(15, 108, 189))
	}
	return nil
}

func (f *fenster) vorDown(x, y int, k walk.MouseButton) {
	if f.cbAns.CurrentIndex() == 2 && k == walk.LeftButton {
		f.ziehen = true
		win.SetCapture(f.vor.Handle())
		f.vorMove(x, y, k)
	}
}

func (f *fenster) vorMove(x, y int, k walk.MouseButton) {
	if f.cbAns.CurrentIndex() == 2 {
		f.vor.SetCursor(walk.CursorSizeWE())
	} else {
		f.vor.SetCursor(walk.CursorArrow())
	}
	if !f.ziehen {
		return
	}
	if k&walk.LeftButton == 0 {
		f.vorUp(x, y, k)
		return
	}
	if f.bildRect.Width > 0 {
		f.teiler = math.Max(0, math.Min(1, float64(x-f.bildRect.X)/float64(f.bildRect.Width)))
		f.vor.Invalidate()
	}
}

func (f *fenster) vorUp(x, y int, _ walk.MouseButton) {
	if f.ziehen {
		f.ziehen = false
		win.ReleaseCapture()
	}
}

// ------------------------------------------------------------------ Galerie

const (
	zellW, zellH = 124, 112
	miniW, miniH = 112, 84
	kopfH        = 24
)

func (f *fenster) galerieGroesse() {
	// Höhe aus Anzahl der Filter und Gruppen
	spalten := 3
	h := 6
	gruppe := ""
	n := 0
	for _, l := range f.looks {
		if l.Gruppe != gruppe {
			if n > 0 {
				h += ((n + spalten - 1) / spalten) * zellH
			}
			h += kopfH
			gruppe, n = l.Gruppe, 0
		}
		n++
	}
	h += ((n+spalten-1)/spalten)*zellH + 6
	f.gal.SetMinMaxSize(walk.Size{Width: 370, Height: h}, walk.Size{Width: 370, Height: h})
	f.gal.Invalidate()
}

// erzeugeMiniaturen: jeder Filter als kleines Bild des aktuellen Fotos.
func (f *fenster) erzeugeMiniaturen() {
	klein := f.basis.Verkleinert(220)
	looks := append([]filter.Look(nil), f.looks...)
	go func() {
		for i, l := range looks {
			erg := filter.Anwenden(klein, l.Werte)
			s := math.Min(float64(miniW)/float64(erg.W), float64(miniH)/float64(erg.H))
			img := erg.Anzeige(max(1, int(float64(erg.W)*s)), max(1, int(float64(erg.H)*s)), false, [3]uint8{})
			idx := i
			f.dlg.Synchronize(func() {
				bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
				if err != nil {
					return
				}
				f.miniMu.Lock()
				if alt := f.mini[idx]; alt != nil {
					alt.Dispose()
				}
				f.mini[idx] = bmp
				f.miniMu.Unlock()
				f.gal.Invalidate()
			})
		}
	}()
}

func (f *fenster) maleGalerie(c *walk.Canvas, _ walk.Rectangle) error {
	b := f.gal.ClientBoundsPixels()
	k := float64(c.DPI()) / 96
	px := func(v int) int { return int(float64(v)*k + 0.5) }
	c.FillRectanglePixels(pinsel(walk.RGB(255, 255, 255)), b)
	kopf, _ := walk.NewFont("Segoe UI", 9, walk.FontBold)
	defer kopf.Dispose()
	text, _ := walk.NewFont("Segoe UI", 8, 0)
	defer text.Dispose()
	f.galZellen = f.galZellen[:0]
	y := px(6)
	gruppe := ""
	spalte := 0
	for i, l := range f.looks {
		if l.Gruppe != gruppe {
			if spalte != 0 {
				y += px(zellH)
				spalte = 0
			}
			gruppe = l.Gruppe
			c.DrawTextPixels(gruppe, kopf, walk.RGB(36, 36, 36), walk.Rectangle{X: px(8), Y: y, Width: b.Width, Height: px(kopfH)}, walk.TextVCenter|walk.TextSingleLine)
			y += px(kopfH)
		}
		r := walk.Rectangle{X: px(6) + spalte*px(zellW), Y: y, Width: px(zellW - 6), Height: px(zellH - 6)}
		if i == f.aktiv {
			c.FillRectanglePixels(pinsel(walk.RGB(15, 108, 189)), r)
			c.FillRectanglePixels(pinsel(walk.RGB(235, 243, 252)), walk.Rectangle{X: r.X + px(3), Y: r.Y + px(3), Width: r.Width - px(6), Height: r.Height - px(6)})
		} else if i == f.galHover {
			c.FillRectanglePixels(pinsel(walk.RGB(240, 240, 240)), r)
		}
		f.miniMu.Lock()
		bmp := f.mini[i]
		f.miniMu.Unlock()
		if bmp != nil {
			sz := bmp.Size()
			c.DrawImagePixels(bmp, walk.Point{X: r.X + (r.Width-sz.Width)/2, Y: r.Y + px(4) + (px(miniH)-sz.Height)/2})
		} else {
			c.FillRectanglePixels(pinsel(walk.RGB(225, 225, 225)), walk.Rectangle{X: r.X + px(6), Y: r.Y + px(4), Width: r.Width - px(12), Height: px(miniH)})
		}
		c.DrawTextPixels(l.Name, text, walk.RGB(36, 36, 36), walk.Rectangle{X: r.X, Y: r.Y + px(miniH+6), Width: r.Width, Height: px(18)},
			walk.TextCenter|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis|walk.TextNoPrefix)
		f.galZellen = append(f.galZellen, galZelle{r: r, idx: i})
		spalte++
		if spalte == 3 {
			spalte = 0
			y += px(zellH)
		}
	}
	return nil
}

func (f *fenster) zelleBei(x, y int) int {
	for _, z := range f.galZellen {
		if x >= z.r.X && x < z.r.X+z.r.Width && y >= z.r.Y && y < z.r.Y+z.r.Height {
			return z.idx
		}
	}
	return -1
}

func (f *fenster) galKlick(x, y int, k walk.MouseButton) {
	if i := f.zelleBei(x, y); i >= 0 && k == walk.LeftButton {
		f.aktiv = i
		if f.staerke == 0 {
			f.setzeStaerke(1)
		}
		f.feinWert = filter.Einstellung{} // neuer Filter: Regler zeigen seine Werte
		f.zeigeRegler()
		f.neu()
	}
}

func (f *fenster) galMaus(x, y int, _ walk.MouseButton) {
	i := f.zelleBei(x, y)
	if i >= 0 {
		f.gal.SetCursor(walk.CursorHand())
	} else {
		f.gal.SetCursor(walk.CursorArrow())
	}
	if i != f.galHover {
		f.galHover = i
		f.gal.Invalidate()
	}
}

// ------------------------------------------------------------------ Ausgabe

// vollbild: Foto in voller Auflösung, korrigiert und gefiltert.
func (f *fenster) vollbild() (*kern.Bild, error) {
	b := f.g.Bild
	if f.korrigiert {
		k, err := kern.Korrigiere(b, f.par, f.rand, false)
		if err != nil {
			return nil, err
		}
		b = k
	}
	return filter.Anwenden(b, f.einstellung()), nil
}

func (f *fenster) arbeite(text string, arbeit func() error, fertig func()) {
	f.dlg.SetEnabled(false)
	f.dlg.SetCursor(walk.CursorWait())
	alt := f.lblInfo.Text()
	f.lblInfo.SetText(text)
	go func() {
		err := arbeit()
		f.dlg.Synchronize(func() {
			f.dlg.SetEnabled(true)
			f.dlg.SetCursor(nil)
			f.lblInfo.SetText(alt)
			if err != nil {
				walk.MsgBox(f.dlg, "Tri-Vertical Filter", err.Error(), walk.MsgBoxIconError)
				return
			}
			fertig()
		})
	}()
}

func dateiName(s string) string {
	s = strings.ToLower(strings.TrimSpace(unerlaubt.ReplaceAllString(s, "")))
	return strings.NewReplacer(" ", "-", "/", "-", "&", "und").Replace(s)
}

func (f *fenster) speichern() {
	stamm := strings.TrimSuffix(filepath.Base(f.pfad), filepath.Ext(f.pfad))
	vorschlag := filepath.Join(filepath.Dir(f.pfad), stamm+"_"+dateiName(f.looks[f.aktiv].Name)+".jpg")
	dlg := walk.FileDialog{Title: "Bild mit Filter speichern", FilePath: vorschlag, InitialDirPath: filepath.Dir(vorschlag),
		Filter: "JPEG (*.jpg)|*.jpg|TIFF verlustfrei (*.tif)|*.tif|PNG (*.png)|*.png"}
	if ok, _ := dlg.ShowSave(f.dlg); !ok {
		return
	}
	ziel := dlg.FilePath
	if filepath.Ext(ziel) == "" {
		ziel += []string{".jpg", ".jpg", ".tif", ".png"}[dlg.FilterIndex]
	}
	f.arbeite("Speichere in voller Auflösung …", func() error {
		b, err := f.vollbild()
		if err != nil {
			return err
		}
		return bild.Speichere(b, ziel, f.g.Exif, f.g.ICC)
	}, func() {
		walk.MsgBox(f.dlg, "Tri-Vertical Filter", "Gespeichert:\n"+ziel, walk.MsgBoxIconInformation)
	})
}

// druckProgramm: Pfad des Zusatzes „Vertical3 Drucken“, falls installiert.
func druckProgramm() (string, bool) {
	exe, _ := os.Executable()
	p := filepath.Join(filepath.Dir(exe), "Vertical3-Druck.exe")
	_, err := os.Stat(p)
	return p, err == nil
}

// rahmen: gefiltertes Bild an „Vertical3 Rahmen“ übergeben.
func (f *fenster) rahmen() {
	exe, _ := os.Executable()
	prog := filepath.Join(filepath.Dir(exe), "Vertical3-Rahmen.exe")
	if _, err := os.Stat(prog); err != nil {
		return
	}
	stamm := strings.TrimSuffix(filepath.Base(f.pfad), filepath.Ext(f.pfad))
	tmp := filepath.Join(os.TempDir(), "Vertical3", stamm+"_"+dateiName(f.looks[f.aktiv].Name)+".jpg")
	f.arbeite("Übergebe an Rahmen …", func() error {
		b, err := f.vollbild()
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(tmp), 0o755)
		if err := bild.Speichere(b, tmp, f.g.Exif, f.g.ICC); err != nil {
			return err
		}
		return exec.Command(prog, tmp).Start()
	}, func() {})
}

func (f *fenster) drucken() {
	prog, ok := druckProgramm()
	if !ok {
		return
	}
	stamm := strings.TrimSuffix(filepath.Base(f.pfad), filepath.Ext(f.pfad))
	tmp := filepath.Join(os.TempDir(), "Vertical3", stamm+"_"+dateiName(f.looks[f.aktiv].Name)+".jpg")
	f.arbeite("Bereite den Druck vor …", func() error {
		b, err := f.vollbild()
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(tmp), 0o755)
		if err := bild.Speichere(b, tmp, f.g.Exif, f.g.ICC); err != nil {
			return err
		}
		return exec.Command(prog, tmp).Start()
	}, func() {})
}

// ------------------------------------------------------------------ Pinsel

var pinselVorrat = map[walk.Color]*walk.SolidColorBrush{}

func pinsel(c walk.Color) *walk.SolidColorBrush {
	if b, ok := pinselVorrat[c]; ok {
		return b
	}
	b, _ := walk.NewSolidColorBrush(c)
	pinselVorrat[c] = b
	return b
}
