// Vertical3 KI-Füllen – Zusatzprogramm zu Vertical3.
//
// Das ausgerichtete Bild wird nicht beschnitten: Es behält das Format des
// Originals, die dabei entstehenden leeren Ränder füllt eine KI auf –
// offline (LaMa, kostenlos, auf diesem PC) oder online (Stability AI).
//
// Aufruf durch Vertical3:  Vertical3-KI.exe <auftrag.json>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/bild"
	"vertical3/kern"
	"vertical3/ki"
)

const (
	version = "1.2"
	titelKI = "Tri-Vertical KI-Füllen"
)

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

type fenster struct {
	dlg       *walk.Dialog
	vor       *walk.CustomWidget
	cbAns     *walk.ComboBox
	lblInfo   *walk.Label
	lblFormat *walk.Label
	rbOff     *walk.RadioButton
	rbOn      *walk.RadioButton
	leMotiv   *walk.LineEdit
	lblSchl   *walk.Label
	btnSchl   *walk.PushButton
	btnFuell  *walk.PushButton
	btnSpeich *walk.PushButton
	btnDruck  *walk.PushButton
	btnFilter *walk.PushButton
	balken    *walk.ProgressBar
	lblStatus *walk.Label
	onlineBox *walk.Composite

	pfad   string
	g      *bild.Geladen
	par    kern.Parameter
	voll   *kern.Bild // ausgerichtet, Originalformat, mit leeren Rändern
	maske  []uint8
	anteil float64
	fertig *kern.Bild // gefüllt (volle Auflösung)
	wie    string     // "offline" / "online"

	pvVor, pvNach *kern.Bild // Vorschaubilder
	vbmp          *walk.Bitmap
	vkey          string
	teiler        float64
	ziehen        bool
	bildRect      walk.Rectangle
	beschaeftigt  bool
}

func fehler(text string) { walk.MsgBox(nil, titelKI, text, walk.MsgBoxIconError) }

func main() {
	exe, _ := os.Executable()
	ki.KIOrdner = filepath.Join(filepath.Dir(exe), "ki")

	var a auftragDatei
	a.F35 = 28
	auftrag := false
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if strings.EqualFold(filepath.Ext(arg), ".json") {
			b, err := os.ReadFile(arg)
			if err != nil || json.Unmarshal(b, &a) != nil {
				fehler("Der Auftrag von Tri-Vertical konnte nicht gelesen werden.")
				return
			}
			os.Remove(arg)
			auftrag = true
		} else {
			a.Bild = arg
		}
	}
	if !auftrag {
		walk.MsgBox(nil, titelKI, "KI-Füllen wird aus Tri-Vertical heraus gestartet:\n\n"+
			"1. Bild in Tri-Vertical öffnen und ausrichten (Neigungsdaten oder Taste A).\n"+
			"2. Im Menü Datei „Ränder mit KI füllen …“ wählen (oder Strg+K).\n\n"+
			"Das Bild wird dann ohne Beschnitt im Originalformat übernommen und die leeren Ränder werden gefüllt.",
			walk.MsgBoxIconInformation)
		return
	}
	g, err := bild.Lade(a.Bild, a.Exiftool)
	if err != nil {
		fehler("Das Bild konnte nicht geöffnet werden:\n" + err.Error())
		return
	}
	g.Bild = kern.Ausrichten(g.Bild, a.Ausrichtung)
	f := &fenster{pfad: a.Bild, g: g, teiler: 0.5,
		par: kern.Parameter{Roll: a.Roll, Pitch: a.Pitch, Yaw: a.Yaw, YawMitte: a.YawMitte, F35: math.Max(5, a.F35)}}
	f.zeige()
}

// ------------------------------------------------------------------ Fenster

func (f *fenster) zeige() {
	online, motiv := ki.Wahl()
	_, druckDa := zusatz("Vertical3-Druck.exe")
	_, filterDa := zusatz("Vertical3-Filter.exe")
	grau := walk.RGB(100, 100, 100)
	var abbr *walk.PushButton
	breite := 380
	err := Dialog{
		AssignTo: &f.dlg, Title: titelKI + " – " + filepath.Base(f.pfad), CancelButton: &abbr,
		MinSize: Size{Width: 1100, Height: 720}, Size: Size{Width: 1250, Height: 820},
		Layout: HBox{},
		Children: []Widget{
			Composite{StretchFactor: 1, Layout: VBox{MarginsZero: true}, Children: []Widget{
				CustomWidget{AssignTo: &f.vor, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
					PaintMode: PaintBuffered, PaintPixels: f.maleVorschau,
					OnMouseDown: f.vorDown, OnMouseMove: f.vorMove, OnMouseUp: f.vorUp},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					Label{Text: "Ansicht:"},
					ComboBox{AssignTo: &f.cbAns, Model: []string{"Ergebnis", "Vorher (leere Ränder)", "Vergleich (Trennlinie ziehen)"}, CurrentIndex: 0,
						OnCurrentIndexChanged: func() { f.vkey = ""; f.vor.Invalidate() }},
					Label{AssignTo: &f.lblInfo, Text: "", TextColor: grau},
					HSpacer{},
					PushButton{AssignTo: &f.btnSpeich, Text: "Bild speichern …", Enabled: false, MinSize: Size{Width: 130}, OnClicked: f.speichern},
					PushButton{AssignTo: &f.btnFilter, Text: "Filter …", Visible: filterDa, Enabled: false, MinSize: Size{Width: 90}, OnClicked: func() { f.weitergeben("Vertical3-Filter.exe") }},
					PushButton{AssignTo: &f.btnDruck, Text: "Drucken …", Visible: druckDa, Enabled: false, MinSize: Size{Width: 90}, OnClicked: func() { f.weitergeben("Vertical3-Druck.exe") }},
					PushButton{AssignTo: &abbr, Text: "Schließen", MinSize: Size{Width: 100}, OnClicked: func() { f.dlg.Cancel() }},
				}},
			}},
			ScrollView{HorizontalFixed: true, MinSize: Size{Width: breite + 24}, MaxSize: Size{Width: breite + 24}, Layout: VBox{MarginsZero: true}, Children: []Widget{
				GroupBox{Title: "Format", Layout: HBox{}, Children: []Widget{
					Label{AssignTo: &f.lblFormat, Text: "Bild wird vorbereitet …\n "},
					HSpacer{},
				}},
				GroupBox{Title: "KI", Layout: VBox{}, Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						RadioButton{AssignTo: &f.rbOff, Text: "Offline – kostenlos, bleibt auf diesem PC", OnClicked: func() { f.rbOn.SetChecked(false); f.rbOff.SetChecked(true); f.wahlGeaendert() }}, HSpacer{}}},
					Label{Text: "     Gut für Himmel, Fassaden, Wiesen und Pflaster.", TextColor: grau},
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						RadioButton{AssignTo: &f.rbOn, Text: "Online – Stability AI (ca. 5 Cent pro Bild)", OnClicked: func() { f.rbOff.SetChecked(false); f.rbOn.SetChecked(true); f.wahlGeaendert() }}, HSpacer{}}},
					Label{Text: "     Besser bei angeschnittenen Gegenständen am Rand\n     (z. B. Autos). Das Bild wird hochgeladen.", TextColor: grau},
					Composite{AssignTo: &f.onlineBox, Layout: VBox{MarginsZero: true}, Children: []Widget{
						Label{Text: "Motiv (optional, am besten auf Englisch):"},
						LineEdit{AssignTo: &f.leMotiv, Text: motiv, CueBanner: "z. B. modern apartment building, blue sky"},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							Label{AssignTo: &f.lblSchl, Text: ""},
							HSpacer{},
							PushButton{AssignTo: &f.btnSchl, Text: "API-Schlüssel …", OnClicked: f.schluesselDialog},
						}},
					}},
				}},
				PushButton{AssignTo: &f.btnFuell, Text: "Ränder mit KI füllen", Enabled: false, MinSize: Size{Height: 40}, OnClicked: f.fuellen},
				ProgressBar{AssignTo: &f.balken, MinValue: 0, MaxValue: 100, MaxSize: Size{Height: 12}},
				Label{AssignTo: &f.lblStatus, Text: "", TextColor: grau},
				VSpacer{},
				Label{Text: "Tipp: Angeschnittene Gegenstände am Bildrand kann\nkeine KI fehlerfrei ergänzen – das Ergebnis vor dem\nWeitergeben bitte in der Ansicht „Vergleich“ prüfen.", TextColor: grau},
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
	f.rbOn.SetChecked(online)
	f.rbOff.SetChecked(!online)
	f.zeigeSchluessel()
	f.wahlGeaendert()
	mittig(f.dlg)
	f.dlg.Closing().Attach(func(abbrechen *bool, _ walk.CloseReason) {
		ki.SetzeWahl(f.rbOn.Checked(), strings.TrimSpace(f.leMotiv.Text()))
	})
	f.vorbereiten()
	f.dlg.Run()
}

func mittig(d *walk.Dialog) {
	var ar win.RECT
	win.SystemParametersInfo(0x0030 /* SPI_GETWORKAREA */, 0, unsafe.Pointer(&ar), 0)
	b := d.BoundsPixels()
	k := float64(d.DPI()) / 96
	b.Width, b.Height = int(1250*k), int(840*k)
	aw, ah := int(ar.Right-ar.Left), int(ar.Bottom-ar.Top)
	b.Width, b.Height = min(b.Width, aw), min(b.Height, ah)
	b.X, b.Y = int(ar.Left)+(aw-b.Width)/2, int(ar.Top)+(ah-b.Height)/2
	d.SetBoundsPixels(b)
}

func (f *fenster) wahlGeaendert() {
	f.onlineBox.SetEnabled(f.rbOn.Checked())
}

func (f *fenster) zeigeSchluessel() {
	if ki.Schluessel() != "" {
		f.lblSchl.SetText("API-Schlüssel ist hinterlegt.")
		f.lblSchl.SetTextColor(walk.RGB(16, 124, 16))
	} else {
		f.lblSchl.SetText("Noch kein API-Schlüssel.")
		f.lblSchl.SetTextColor(walk.RGB(160, 60, 30))
	}
}

// vorbereiten: Bild in voller Auflösung ausrichten (ohne Beschnitt).
func (f *fenster) vorbereiten() {
	f.beschaeftige(true, "Bild wird ausgerichtet …")
	go func() {
		voll, maske, anteil, err := kern.KorrigiereImFormat(f.g.Bild, f.par)
		var pv *kern.Bild
		if err == nil {
			pv = vorschauMitLuecken(voll, maske)
		}
		f.dlg.Synchronize(func() {
			f.beschaeftige(false, "")
			f.lblStatus.SetText("")
			if err != nil {
				walk.MsgBox(f.dlg, titelKI, "Die Ausrichtung konnte nicht berechnet werden:\n"+err.Error(), walk.MsgBoxIconError)
				f.dlg.Cancel()
				return
			}
			f.voll, f.maske, f.anteil, f.pvVor, f.pvNach = voll, maske, anteil, pv, pv
			ggt := func(a, b int) int {
				for b != 0 {
					a, b = b, a%b
				}
				return a
			}
			t := ggt(voll.W, voll.H)
			seite := fmt.Sprintf("%d:%d", voll.W/t, voll.H/t)
			if voll.W/t > 50 {
				seite = strings.Replace(fmt.Sprintf("%.2f : 1", float64(voll.W)/float64(voll.H)), ".", ",", 1)
			}
			f.lblFormat.SetText(fmt.Sprintf("Originalformat %s  ·  %d × %d Pixel\nLeere Ränder: %s %% der Bildfläche",
				seite, voll.W, voll.H, strings.Replace(fmt.Sprintf("%.1f", anteil*100), ".", ",", 1)))
			if anteil < 0.0005 {
				f.lblStatus.SetText("Keine leeren Ränder – nichts zu füllen.")
				f.fertig = voll
				f.freigeben(true)
			} else {
				f.btnFuell.SetEnabled(true)
				f.lblInfo.SetText("Leere Ränder sind kariert dargestellt.")
			}
			f.vkey = ""
			f.vor.Invalidate()
		})
	}()
}

func (f *fenster) beschaeftige(an bool, text string) {
	f.beschaeftigt = an
	f.btnFuell.SetEnabled(!an && f.voll != nil && f.anteil >= 0.0005)
	f.rbOff.SetEnabled(!an)
	f.rbOn.SetEnabled(!an)
	f.onlineBox.SetEnabled(!an && f.rbOn.Checked())
	f.btnSpeich.SetEnabled(!an && f.fertig != nil)
	f.btnDruck.SetEnabled(!an && f.fertig != nil)
	f.btnFilter.SetEnabled(!an && f.fertig != nil)
	if an {
		f.dlg.SetCursor(walk.CursorWait())
		f.lblStatus.SetText(text)
	} else {
		f.dlg.SetCursor(nil)
	}
}

func (f *fenster) freigeben(ja bool) {
	f.btnSpeich.SetEnabled(ja)
	f.btnDruck.SetEnabled(ja)
	f.btnFilter.SetEnabled(ja)
}

// ------------------------------------------------------------------ Füllen

func (f *fenster) fuellen() {
	if f.voll == nil || f.beschaeftigt {
		return
	}
	online := f.rbOn.Checked()
	schluessel, motiv := "", strings.TrimSpace(f.leMotiv.Text())
	if online {
		schluessel = ki.Schluessel()
		if schluessel == "" {
			f.schluesselDialog()
			if schluessel = ki.Schluessel(); schluessel == "" {
				return
			}
		}
	} else if err := ki.Bereit(); err != nil {
		walk.MsgBox(f.dlg, titelKI, err.Error(), walk.MsgBoxIconError)
		return
	} else if !ki.ModellDa() {
		if walk.MsgBox(f.dlg, titelKI, fmt.Sprintf("Für die Offline-KI wird einmalig das KI-Modell LaMa (%d MB) aus dem Internet geladen "+
			"(OpenCV-Projekt auf GitHub). Danach arbeitet die Offline-KI ohne Internet.\n\nJetzt laden?", ki.ModellMB),
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
	}
	ki.SetzeWahl(online, motiv)
	f.beschaeftige(true, "KI wird gestartet …")
	f.balken.SetValue(0)
	start := time.Now()
	melde := func(a float64, t string) {
		f.dlg.Synchronize(func() {
			f.balken.SetValue(int(a * 100))
			f.lblStatus.SetText(t)
		})
	}
	go func() {
		var erg *kern.Bild
		var err error
		if online {
			erg, err = ki.Online(context.Background(), f.voll, f.maske, schluessel, motiv, melde)
		} else {
			erg, err = ki.Offline(f.voll, f.maske, melde)
		}
		var pv *kern.Bild
		if err == nil {
			pv = vorschau(erg)
		}
		f.dlg.Synchronize(func() {
			f.beschaeftige(false, "")
			if err != nil {
				f.balken.SetValue(0)
				f.lblStatus.SetText("Füllen fehlgeschlagen.")
				walk.MsgBox(f.dlg, titelKI, err.Error(), walk.MsgBoxIconError)
				return
			}
			f.fertig, f.pvNach = erg, pv
			f.wie = map[bool]string{true: "online", false: "offline"}[online]
			f.balken.SetValue(100)
			f.lblStatus.SetText(fmt.Sprintf("Fertig nach %.0f s – bitte prüfen und speichern.", time.Since(start).Seconds()))
			f.lblInfo.SetText("")
			f.cbAns.SetCurrentIndex(0)
			f.freigeben(true)
			f.vkey = ""
			f.vor.Invalidate()
		})
	}()
}

// ------------------------------------------------------------------ API-Schlüssel

func (f *fenster) schluesselDialog() {
	var dlg *walk.Dialog
	var le *walk.LineEdit
	var ok, abbr *walk.PushButton
	url := "https://platform.stability.ai/account/keys"
	err := Dialog{
		AssignTo: &dlg, Title: "API-Schlüssel für Stability AI", DefaultButton: &ok, CancelButton: &abbr,
		MinSize: Size{Width: 520, Height: 260}, MaxSize: Size{Width: 520, Height: 260}, Layout: VBox{},
		Children: []Widget{
			Label{Text: "Für die Online-KI wird ein Konto bei Stability AI mit Guthaben benötigt\n" +
				"(Abrechnung direkt bei Stability AI, etwa 5 Cent pro gefülltem Bild).\n\n" +
				"Den Schlüssel gibt es nach der Anmeldung unter:"},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "platform.stability.ai/account/keys"},
				PushButton{Text: "Seite öffnen", OnClicked: func() { oeffneURL(url) }},
				HSpacer{},
			}},
			Label{Text: "API-Schlüssel (beginnt mit „sk-“):"},
			LineEdit{AssignTo: &le, PasswordMode: true},
			Label{Text: "Der Schlüssel wird verschlüsselt nur auf diesem PC gespeichert.", TextColor: walk.RGB(100, 100, 100)},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "Schlüssel entfernen", OnClicked: func() {
					ki.SetzeSchluessel("")
					dlg.Accept()
				}},
				HSpacer{},
				PushButton{AssignTo: &ok, Text: "Speichern", OnClicked: func() {
					s := strings.TrimSpace(le.Text())
					if s == "" {
						walk.MsgBox(dlg, titelKI, "Bitte den API-Schlüssel eingeben.", walk.MsgBoxIconWarning)
						return
					}
					if err := ki.SetzeSchluessel(s); err != nil {
						walk.MsgBox(dlg, titelKI, "Der Schlüssel konnte nicht gespeichert werden:\n"+err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
				}},
				PushButton{AssignTo: &abbr, Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(f.dlg)
	if err != nil {
		return
	}
	le.SetFocus()
	dlg.Run()
	f.zeigeSchluessel()
}

func oeffneURL(u string) {
	p, _ := syscall.UTF16PtrFromString(u)
	o, _ := syscall.UTF16PtrFromString("open")
	win.ShellExecute(0, o, p, nil, nil, win.SW_SHOWNORMAL)
}

// ------------------------------------------------------------------ Speichern / Weitergeben

func (f *fenster) vorschlag() string {
	stamm := strings.TrimSuffix(filepath.Base(f.pfad), filepath.Ext(f.pfad))
	return stamm + "_ki-gefuellt.jpg"
}

func (f *fenster) speichern() {
	if f.fertig == nil {
		return
	}
	vorschlag := filepath.Join(filepath.Dir(f.pfad), f.vorschlag())
	dlg := walk.FileDialog{Title: "Gefülltes Bild speichern", FilePath: vorschlag, InitialDirPath: filepath.Dir(vorschlag),
		Filter: "JPEG (*.jpg)|*.jpg|TIFF verlustfrei (*.tif)|*.tif|PNG (*.png)|*.png"}
	if ok, _ := dlg.ShowSave(f.dlg); !ok {
		return
	}
	ziel := dlg.FilePath
	if filepath.Ext(ziel) == "" {
		ziel += []string{".jpg", ".jpg", ".tif", ".png"}[dlg.FilterIndex]
	}
	f.beschaeftige(true, "Speichere in voller Auflösung …")
	go func() {
		err := bild.Speichere(f.fertig, ziel, f.g.Exif, f.g.ICC)
		f.dlg.Synchronize(func() {
			f.beschaeftige(false, "")
			if err != nil {
				f.lblStatus.SetText("")
				walk.MsgBox(f.dlg, titelKI, "Speichern fehlgeschlagen:\n"+err.Error(), walk.MsgBoxIconError)
				return
			}
			f.lblStatus.SetText("Gespeichert.")
			walk.MsgBox(f.dlg, titelKI, "Gespeichert:\n"+ziel, walk.MsgBoxIconInformation)
		})
	}()
}

// zusatz: Pfad eines anderen Zusatzprogramms, falls installiert.
func zusatz(exe string) (string, bool) {
	selbst, _ := os.Executable()
	p := filepath.Join(filepath.Dir(selbst), exe)
	_, err := os.Stat(p)
	return p, err == nil
}

// weitergeben: gefülltes Bild an „Drucken“ oder „Filter“ übergeben.
func (f *fenster) weitergeben(exe string) {
	prog, ok := zusatz(exe)
	if !ok || f.fertig == nil {
		return
	}
	tmp := filepath.Join(os.TempDir(), "Vertical3", f.vorschlag())
	f.beschaeftige(true, "Bild wird übergeben …")
	go func() {
		os.MkdirAll(filepath.Dir(tmp), 0o755)
		err := bild.Speichere(f.fertig, tmp, f.g.Exif, f.g.ICC)
		if err == nil {
			err = exec.Command(prog, tmp).Start()
		}
		f.dlg.Synchronize(func() {
			f.beschaeftige(false, "")
			f.lblStatus.SetText("")
			if err != nil {
				walk.MsgBox(f.dlg, titelKI, err.Error(), walk.MsgBoxIconError)
			}
		})
	}()
}

// ------------------------------------------------------------------ Vorschau

const vorschauMax = 1600

func vorschau(b *kern.Bild) *kern.Bild { return b.Verkleinert(vorschauMax) }

// vorschauMitLuecken: leere Ränder als Schachbrett zeigen.
func vorschauMitLuecken(b *kern.Bild, maske []uint8) *kern.Bild {
	k := b.Verkleinert(vorschauMax)
	sx, sy := float64(b.W)/float64(k.W), float64(b.H)/float64(k.H)
	out := &kern.Bild{W: k.W, H: k.H, Pix: append([]uint8(nil), k.Pix...)}
	for y := 0; y < k.H; y++ {
		for x := 0; x < k.W; x++ {
			ox, oy := min(b.W-1, int((float64(x)+0.5)*sx)), min(b.H-1, int((float64(y)+0.5)*sy))
			if maske[oy*b.W+ox] == 0 {
				continue
			}
			v := uint8(205)
			if (x/10+y/10)%2 == 0 {
				v = 245
			}
			o := (y*k.W + x) * 3
			out.Pix[o], out.Pix[o+1], out.Pix[o+2] = v, v, v
		}
	}
	return out
}

func (f *fenster) maleVorschau(c *walk.Canvas, _ walk.Rectangle) error {
	b := f.vor.ClientBoundsPixels()
	c.FillRectanglePixels(pinsel(walk.RGB(232, 232, 232)), b)
	if f.pvVor == nil {
		return nil
	}
	modus := f.cbAns.CurrentIndex()
	basis := f.pvVor
	rand := 16
	s := math.Min(float64(b.Width-2*rand)/float64(basis.W), float64(b.Height-2*rand)/float64(basis.H))
	w, h := int(float64(basis.W)*s), int(float64(basis.H)*s)
	x, y := (b.Width-w)/2, (b.Height-h)/2
	f.bildRect = walk.Rectangle{X: x, Y: y, Width: w, Height: h}
	key := fmt.Sprintf("%p|%p|%d|%dx%d|%.4f", f.pvVor, f.pvNach, modus, w, h, f.teiler)
	if key != f.vkey || f.vbmp == nil {
		var img *kern.Bild
		switch modus {
		case 1:
			img = f.pvVor
		case 2:
			img = kern.NeuesBild(basis.W, basis.H)
			grenze := int(f.teiler * float64(basis.W))
			for yy := 0; yy < basis.H; yy++ {
				o := yy * basis.W * 3
				copy(img.Pix[o:o+grenze*3], f.pvVor.Pix[o:o+grenze*3])
				copy(img.Pix[o+grenze*3:o+basis.W*3], f.pvNach.Pix[o+grenze*3:o+basis.W*3])
			}
		default:
			img = f.pvNach
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
		r := walk.Rectangle{X: sx, Y: y + 10, Width: 90, Height: 22}
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
		for _, d := range []int{-1, 1} {
			c.DrawLinePixels(dunkel, walk.Point{X: lx + d*3, Y: my - 5}, walk.Point{X: lx + d*8, Y: my})
			c.DrawLinePixels(dunkel, walk.Point{X: lx + d*8, Y: my}, walk.Point{X: lx + d*3, Y: my + 5})
		}
		dunkel.Dispose()
		schild("Vorher", x+10, walk.RGB(66, 66, 66))
		schild("KI-gefüllt", x+w-100, walk.RGB(15, 108, 189))
	default:
		if f.fertig != nil && f.anteil >= 0.0005 {
			schild("KI-gefüllt", x+10, walk.RGB(15, 108, 189))
		}
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

var pinselVorrat = map[walk.Color]*walk.SolidColorBrush{}

func pinsel(c walk.Color) *walk.SolidColorBrush {
	if b, ok := pinselVorrat[c]; ok {
		return b
	}
	b, _ := walk.NewSolidColorBrush(c)
	pinselVorrat[c] = b
	return b
}
