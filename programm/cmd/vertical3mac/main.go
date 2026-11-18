// Tri-Vertical für macOS – Perspektivkorrektur (Roll, Pitch, Yaw).
//
// Oberfläche mit Fyne; Kern (kern, bild, meta, profil) ist derselbe wie in der
// Windows-Version. Siehe MAC-LIESMICH.txt.
package main

import (
	"embed"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"vertical3/bild"
	"vertical3/kern"
	"vertical3/meta"
	"vertical3/profil"
)

// Programmversion (entspricht der Windows-Version).
const grundVersion = "3.9.1"

const vorschauMax = 1600

//go:embed standardprofile/*.json
var standardProfile embed.FS

var randFarben = []struct {
	Name  string
	Farbe [3]uint8
}{{"Weiß", [3]uint8{255, 255, 255}}, {"Schwarz", [3]uint8{0, 0, 0}}, {"Grau", [3]uint8{128, 128, 128}}}

// regler: Schieberegler mit Wertanzeige.
type regler struct {
	name          string
	von, bis      float64
	schritt       float64
	slider        *widget.Slider
	anzeige       *widget.Label
	wert          float64
	einheit       string
	nachAenderung func(*regler)
}

func neuerRegler(name string, von, bis, schritt, start float64, einheit string, f func(*regler)) *regler {
	r := &regler{name: name, von: von, bis: bis, schritt: schritt, wert: start, einheit: einheit, nachAenderung: f}
	r.slider = widget.NewSlider(von, bis)
	r.slider.Step = schritt
	r.slider.Value = start
	r.anzeige = widget.NewLabel("")
	r.zeigeWert()
	r.slider.OnChanged = func(v float64) {
		r.wert = math.Round(v/r.schritt) * r.schritt
		r.zeigeWert()
		if r.nachAenderung != nil {
			r.nachAenderung(r)
		}
	}
	return r
}

func (r *regler) zeigeWert() {
	r.anzeige.SetText(fmt.Sprintf("%+.1f%s", r.wert, r.einheit))
	if r.einheit == " mm" {
		r.anzeige.SetText(fmt.Sprintf("%.1f mm", r.wert))
	}
}

// setze stellt den Regler von außen ein (löst die Vorschau einmal aus).
func (r *regler) setze(v float64) {
	v = math.Max(r.von, math.Min(r.bis, math.Round(v/r.schritt)*r.schritt))
	r.slider.SetValue(v) // ruft OnChanged
	if r.wert != v {     // Wert unverändert: OnChanged läuft nicht
		r.wert = v
		r.zeigeWert()
	}
}

func (r *regler) zeile() fyne.CanvasObject {
	return container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel(r.name), r.anzeige),
		r.slider,
	)
}

type programm struct {
	fa                  fyne.App
	w                   fyne.Window
	appDir, profilDir   string
	exiftool, version   string
	profile             []*profil.Profil
	roll, pitch, yaw, f *regler

	pfad      string
	geladen   *bild.Geladen
	aktProfil *profil.Profil
	autoF35   float64
	vorschau  *kern.Bild
	nachher   *kern.Bild
	fehler    string

	zuschneiden bool
	original    bool
	yawAktiv    bool
	yawMitte    bool
	randIdx     int

	ansicht *ansicht
	status  *widget.Label
	info    *widget.Label
	chkYaw  *widget.Check
	chkOrig *widget.Check
	chkVgl  *widget.Check

	// Drehen/Spiegeln
	ausrichtung []string
	ausrichtQ   kern.Mat3

	// Ausgabegröße und Ausschnitt
	ausgabe           groesse
	nichtVergroessern bool
	aus               ausschnittZustand
	ausInfo           *widget.Label
	ausBox            *fyne.Container
	sGroesse, sX, sY  *widget.Slider
	vergleich         bool

	// Bezugsmarken
	marken         []kern.Marke
	markeNeu       string
	neuePunkte     [][2]float64
	markenLsg      kern.Loesung
	markenFehler   string
	brennweiteFrei bool
	markenInfo     *widget.Label
	markenAuswahl  *widget.Select

	mu           sync.Mutex
	timer        *time.Timer
	rechenNr     int
	beschaeftigt bool
}

func main() {
	exe, _ := os.Executable()
	p := &programm{version: grundVersion, zuschneiden: true, nichtVergroessern: true,
		ausgabe: standardGroessen[0], ausrichtQ: einheitsMatrix()}
	p.aus.zuruecksetzen()
	p.appDir = filepath.Dir(exe)
	cfg, err := os.UserConfigDir() // macOS: ~/Library/Application Support
	if err != nil || cfg == "" {
		cfg = p.appDir
	}
	datenDir := filepath.Join(cfg, "Tri-Vertical")
	p.profilDir = filepath.Join(datenDir, "kameraprofile")
	p.exiftool = findeExiftool(p.appDir)
	p.legeStandardprofileAn()
	p.profile = profil.Lade(p.profilDir)

	p.fa = fyneapp.NewWithID("de.tri-vertical.mac")
	p.w = p.fa.NewWindow("Tri-Vertical " + p.version + " – Perspektivkorrektur")
	p.w.Resize(fyne.NewSize(1300, 850))
	p.baueOberflaeche()
	p.baueMenue()
	p.w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		for _, u := range uris {
			if bild.IstBild(u.Path()) {
				p.lade(u.Path())
				return
			}
		}
	})
	p.setzeStatus("Bild öffnen mit ⌘O oder über den Knopf rechts (oder Datei auf das Fenster ziehen).")
	for _, a := range os.Args[1:] {
		if st, err := os.Stat(a); err == nil && !st.IsDir() && bild.IstBild(a) {
			p.lade(a)
			break
		}
	}
	p.w.ShowAndRun()
}

func findeExiftool(appDir string) string {
	if e := meta.FindeExiftool(appDir); e != "" {
		return e
	}
	// aus dem Finder gestartet hat die App nur einen kurzen Suchpfad
	for _, p := range []string{"/opt/homebrew/bin/exiftool", "/usr/local/bin/exiftool", "/usr/bin/exiftool"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (p *programm) legeStandardprofileAn() {
	os.MkdirAll(p.profilDir, 0o755)
	vorhanden, _ := filepath.Glob(filepath.Join(p.profilDir, "*.json"))
	if len(vorhanden) > 0 {
		return
	}
	eintraege, _ := standardProfile.ReadDir("standardprofile")
	for _, e := range eintraege {
		b, _ := standardProfile.ReadFile("standardprofile/" + e.Name())
		os.WriteFile(filepath.Join(p.profilDir, e.Name()), b, 0o644)
	}
}

// ------------------------------------------------------------------ Oberfläche

func (p *programm) baueOberflaeche() {
	aend := func(r *regler) {
		if r == p.yaw {
			p.yawMitte = false // von Hand: Yaw dreht um die Unterkante
		}
		if r == p.f {
			p.yawGrenzeAnpassen()
		}
		p.planeVorschau()
	}
	p.roll = neuerRegler("Roll (Horizont)", -45, 45, 0.1, 0, "°", aend)
	p.pitch = neuerRegler("Pitch (stürzende Linien)", -60, 60, 0.1, 0, "°", aend)
	p.yaw = neuerRegler("Yaw (Perspektivwechsel)", -45, 45, 0.1, 0, "°", aend)
	p.f = neuerRegler("Brennweite (KB-äquiv.)", 8, 300, 0.5, 28, " mm", aend)
	p.yaw.slider.Disable()

	p.ansicht = neueAnsicht(p)

	p.status = widget.NewLabel("")
	p.status.Wrapping = fyne.TextWrapWord
	p.info = widget.NewLabel("Kein Bild geöffnet.")
	p.info.Wrapping = fyne.TextWrapWord

	p.chkYaw = widget.NewCheck("Yaw-Achse verwenden", func(an bool) {
		p.yawAktiv = an
		if an {
			p.yaw.slider.Enable()
		} else {
			p.yaw.slider.Disable()
			p.yaw.setze(0)
			p.yawMitte = false
		}
		p.planeVorschau()
	})
	p.chkOrig = widget.NewCheck("Original zeigen (Leertaste)", func(an bool) { p.original = an; p.zeigeBild() })
	p.chkVgl = widget.NewCheck("Vorher / Nachher nebeneinander (V)", func(an bool) { p.vergleich = an; p.zeigeBild() })
	chkZu := widget.NewCheck("Zuschneiden", func(an bool) { p.zuschneiden = an; p.planeVorschau() })
	chkZu.SetChecked(true)
	namen := make([]string, len(randFarben))
	for i, r := range randFarben {
		namen[i] = r.Name
	}
	rand := widget.NewSelect(namen, func(s string) {
		for i, r := range randFarben {
			if r.Name == s {
				p.randIdx = i
			}
		}
		p.planeVorschau()
	})
	rand.SetSelected(randFarben[0].Name)

	bOeffnen := widget.NewButton("Bild öffnen …", p.oeffnenDialog)
	bAuto := widget.NewButton("Automatisch ausrichten", func() { p.automatik(false) })
	bFront := widget.NewButton("Automatisch frontal ausrichten", func() { p.automatik(true) })
	bReset := widget.NewButton("Zurücksetzen", p.zuruecksetzen)
	bSpeichern := widget.NewButtonWithIcon("Speichern unter …", nil, p.speichernDialog)
	bSpeichern.Importance = widget.HighImportance

	panel := container.NewVBox(
		bOeffnen,
		widget.NewSeparator(),
		p.roll.zeile(), p.pitch.zeile(), p.yaw.zeile(), p.f.zeile(),
		p.chkYaw,
		widget.NewSeparator(),
		bAuto, bFront, bReset,
		widget.NewSeparator(),
		chkZu,
		container.NewBorder(nil, nil, widget.NewLabel("Rand:"), nil, rand),
		p.chkOrig,
		p.chkVgl,
		container.NewGridWithColumns(3,
			widget.NewButton("＋", func() { p.zoomMitte(1.5) }),
			widget.NewButton("－", func() { p.zoomMitte(1 / 1.5) }),
			widget.NewButton("Ganzes Bild", func() { p.ansicht.ganz() })),
		widget.NewSeparator(),
		p.baueGroessenPanel(),
		widget.NewSeparator(),
		p.baueMarkenPanel(),
		widget.NewSeparator(),
		p.info,
		layoutAbstand(),
		bSpeichern,
	)
	rechts := container.NewVScroll(container.NewPadded(panel))
	rechts.SetMinSize(fyne.NewSize(330, 0))
	unten := container.NewVBox(widget.NewSeparator(), p.status)
	p.w.SetContent(container.NewBorder(nil, unten, nil, rechts, p.ansicht))

	// Tastenkürzel: ⌘O, ⌘S, A, ⇧A, Leertaste (Original halten)
	cv := p.w.Canvas()
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) { p.oeffnenDialog() })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) { p.speichernDialog() })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: 0}, func(fyne.Shortcut) { p.automatik(false) })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierShift}, func(fyne.Shortcut) { p.automatik(true) })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: 0}, func(fyne.Shortcut) { p.chkVgl.SetChecked(!p.chkVgl.Checked) })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySpace, Modifier: 0}, func(fyne.Shortcut) { p.chkOrig.SetChecked(!p.chkOrig.Checked) })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) { p.drehen(kern.DrehLinks) })
	cv.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyRight, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) { p.drehen(kern.DrehRechts) })
}

func layoutAbstand() fyne.CanvasObject { return widget.NewLabel("") }

func (p *programm) baueMenue() {
	oeffnen := fyne.NewMenuItem("Bild öffnen …", p.oeffnenDialog)
	oeffnen.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper}
	speichern := fyne.NewMenuItem("Speichern unter …", p.speichernDialog)
	speichern.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierSuper}
	datei := fyne.NewMenu("Datei", oeffnen, speichern)

	kameras := fyne.NewMenu("Kameras",
		fyne.NewMenuItem("Kameraprofil importieren …", p.profilImportDialog),
		fyne.NewMenuItem("Bekannte Kameras anzeigen", p.profileAnzeigen),
		fyne.NewMenuItem("Profilordner öffnen", func() { exec.Command("open", p.profilDir).Start() }),
	)
	bildMenue := fyne.NewMenu("Bild",
		fyne.NewMenuItem("Nach links drehen (90°)", func() { p.drehen(kern.DrehLinks) }),
		fyne.NewMenuItem("Nach rechts drehen (90°)", func() { p.drehen(kern.DrehRechts) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Waagerecht spiegeln", func() { p.drehen(kern.SpiegelWaag) }),
		fyne.NewMenuItem("Senkrecht spiegeln", func() { p.drehen(kern.SpiegelSenk) }),
	)
	ansichtMenue := fyne.NewMenu("Ansicht",
		fyne.NewMenuItem("Vergrößern", func() { p.zoomMitte(1.5) }),
		fyne.NewMenuItem("Verkleinern", func() { p.zoomMitte(1 / 1.5) }),
		fyne.NewMenuItem("Ganzes Bild", func() { p.ansicht.ganz() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Automatisch ausrichten", func() { p.automatik(false) }),
		fyne.NewMenuItem("Automatisch frontal ausrichten", func() { p.automatik(true) }),
	)
	hilfe := fyne.NewMenu("Hilfe",
		fyne.NewMenuItem("Über Tri-Vertical", func() {
			dialog.ShowInformation("Über Tri-Vertical", "Tri-Vertical "+p.version+" (macOS)\nPerspektivkorrektur in drei Achsen\n\nNur für den privaten Gebrauch.", p.w)
		}),
	)
	p.w.SetMainMenu(fyne.NewMainMenu(datei, bildMenue, kameras, ansichtMenue, hilfe))
}

func (p *programm) setzeStatus(t string) {
	fyne.Do(func() { p.status.SetText(t) })
}

func (p *programm) meldung(titel, text string) {
	fyne.Do(func() { dialog.ShowInformation(titel, text, p.w) })
}

func (p *programm) fehlerDialog(titel string, err error) {
	fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s: %w", titel, err), p.w) })
}

func (p *programm) parameter() kern.Parameter {
	yaw := p.yaw.wert
	if !p.yawAktiv {
		yaw = 0
	}
	return kern.Parameter{Roll: p.roll.wert, Pitch: p.pitch.wert, Yaw: yaw, YawMitte: p.yawMitte,
		F35: math.Max(5, p.f.wert), Zuschneiden: p.zuschneiden}
}

func (p *programm) randFarbe() [3]uint8 {
	i := p.randIdx
	if i < 0 || i >= len(randFarben) {
		i = 0
	}
	return randFarben[i].Farbe
}

// ------------------------------------------------------------------ Laden

func (p *programm) oeffnenDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			p.fehlerDialog("Bild öffnen", err)
			return
		}
		if r == nil {
			return
		}
		pfad := r.URI().Path()
		r.Close()
		p.lade(pfad)
	}, p.w)
	d.SetFilter(storage.NewExtensionFileFilter(bild.Endungen))
	if p.pfad != "" {
		if l, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(p.pfad))); err == nil {
			d.SetLocation(l)
		}
	}
	d.Resize(fyne.NewSize(900, 600))
	d.Show()
}

func (p *programm) lade(pfad string) {
	if p.beschaeftigt {
		return
	}
	p.beschaeftigt = true
	if e := strings.ToLower(filepath.Ext(pfad)); e == ".dng" || !strings.Contains(".jpg.jpeg.png.tif.tiff.heic.heif.webp", e) {
		p.setzeStatus("Entwickle RAW-Datei " + filepath.Base(pfad) + " …")
	} else {
		p.setzeStatus("Lade " + filepath.Base(pfad) + " …")
	}
	go func() {
		g, err := bild.Lade(pfad, p.exiftool)
		var klein *kern.Bild
		if err == nil {
			klein = g.Bild.Verkleinert(vorschauMax)
		}
		fyne.Do(func() {
			p.beschaeftigt = false
			if err != nil {
				dialog.ShowError(fmt.Errorf("Das Bild konnte nicht geöffnet werden:\n%w", err), p.w)
				p.status.SetText("Bild konnte nicht geöffnet werden.")
				return
			}
			p.uebernimm(pfad, g, klein)
		})
	}()
}

func (p *programm) uebernimm(pfad string, g *bild.Geladen, klein *kern.Bild) {
	p.pfad, p.geladen, p.vorschau, p.nachher = pfad, g, klein, nil
	p.aktProfil = profil.Passend(g.Meta, p.profile)
	p.autoF35 = profil.KBBrennweite(g.Meta, p.aktProfil)
	kamera := strings.TrimSpace(g.Meta["Make"] + " " + g.Meta["Model"])
	if kamera == "" {
		kamera = "unbekannt"
	}
	t := fmt.Sprintf("%s\n%d × %d\nKamera: %s\nProfil: %s", filepath.Base(pfad), g.Bild.W, g.Bild.H, kamera, p.aktProfil.Name())
	if g.Hinweis != "" {
		t += "\n" + g.Hinweis
	}
	p.info.SetText(t)
	p.yawMitte = false
	p.ausrichtung, p.ausrichtQ = nil, einheitsMatrix()
	p.aus.zuruecksetzen()
	p.syncAusschnitt()
	p.marken, p.neuePunkte, p.markeNeu, p.markenLsg, p.markenFehler = nil, nil, "", kern.Loesung{}, ""
	p.markenAuswahl.ClearSelected()
	p.markenInfo.SetText("Noch keine Marken gesetzt.")
	p.ansicht.ganz()
	p.roll.setze(0)
	p.pitch.setze(0)
	p.yaw.setze(0)
	p.f.setze(math.Round(p.autoF35*10) / 10)
	p.yawGrenzeAnpassen()
	p.w.SetTitle("Tri-Vertical " + p.version + " – " + filepath.Base(pfad))
	p.status.SetText("Bild geladen. Automatisch ausrichten (Taste A), Regler verschieben oder speichern.")
	p.zeigeBild()
	p.planeVorschau()
}

func (p *programm) zuruecksetzen() {
	p.yawMitte = false
	p.roll.setze(0)
	p.pitch.setze(0)
	p.yaw.setze(0)
	if p.geladen != nil {
		p.f.setze(math.Round(p.autoF35*10) / 10)
	}
}

// ------------------------------------------------------------------ Vorschau

func (p *programm) planeVorschau() {
	if p.vorschau == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.timer != nil {
		p.timer.Stop()
	}
	p.rechenNr++
	nr := p.rechenNr
	par, rand, quelle := p.parameter(), p.randFarbe(), p.vorschau
	p.timer = time.AfterFunc(30*time.Millisecond, func() {
		erg, err := kern.Korrigiere(quelle, par, rand, true)
		fyne.Do(func() {
			p.mu.Lock()
			aktuell := nr == p.rechenNr
			p.mu.Unlock()
			if !aktuell {
				return
			}
			if err != nil {
				p.fehler = err.Error()
				p.status.SetText(p.fehler)
			} else {
				p.fehler = ""
				p.nachher = erg
			}
			p.zeigeBild()
		})
	})
}

// zeigeBild zeigt Vorher, Nachher oder beide (nur im Oberflächen-Thread aufrufen).
func (p *programm) zeigeBild() {
	if p.vorschau == nil {
		p.ansicht.setzeBild(nil, false)
		return
	}
	original := p.original || p.markeNeu != ""
	nachherOK := p.nachher != nil && p.fehler == ""
	switch {
	case p.vergleich && !original && nachherOK:
		p.ansicht.setzeBild(vergleichsBild(p.vorschau, p.nachher), false)
	case original || !nachherOK:
		p.ansicht.setzeBild(p.vorschau.RGBA(), true)
	default:
		img := p.nachher.RGBA()
		p.schleierAusschnitt(img)
		p.ansicht.setzeBild(img, false)
	}
	p.ausInfo.SetText(p.ausgabeText())
}

// vergleichsBild: Vorher und Nachher in gleicher Höhe nebeneinander.
func vergleichsBild(a, b *kern.Bild) *image.RGBA {
	th := 900
	skal := func(x *kern.Bild) *kern.Bild {
		return x.Skaliert(max(1, int(math.Round(float64(x.W)*float64(th)/float64(x.H)))), th)
	}
	va, vb := skal(a).RGBA(), skal(b).RGBA()
	luecke := 24
	out := image.NewRGBA(image.Rect(0, 0, va.Rect.Dx()+luecke+vb.Rect.Dx(), th))
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = 0x2b, 0x2b, 0x2b, 255
	}
	draw.Draw(out, image.Rect(0, 0, va.Rect.Dx(), th), va, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(va.Rect.Dx()+luecke, 0, out.Rect.Dx(), th), vb, image.Point{}, draw.Src)
	return out
}

func (p *programm) zoomMitte(f float64) {
	if a := p.ansicht; a.hatAbbildung {
		a.zoomSchritt(f, float64(a.pw)/2, float64(a.ph)/2)
	}
}

func einheitsMatrix() kern.Mat3 { return kern.Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} }

// yawGrenzeAnpassen: Yaw-Bereich nach Bildbreite und Brennweite begrenzen.
func (p *programm) yawGrenzeAnpassen() {
	if p.yaw == nil || p.f == nil {
		return
	}
	grenze := 45.0
	if p.geladen != nil {
		w, h := p.geladen.Bild.W, p.geladen.Bild.H
		f := kern.BrennweitePx(p.f.wert, w, h)
		halb := math.Atan(float64(w)/2/f) * 180 / math.Pi
		grenze = math.Max(3, math.Min(45, math.Floor(halb*0.75*2)/2))
	}
	p.yaw.von, p.yaw.bis = -grenze, grenze
	p.yaw.slider.Min, p.yaw.slider.Max = -grenze, grenze
	if math.Abs(p.yaw.wert) > grenze {
		p.yaw.setze(math.Copysign(grenze, p.yaw.wert))
	}
	p.yaw.slider.Refresh()
}

// ------------------------------------------------------------------ Automatik

func (p *programm) automatik(frontal bool) {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	if frontal && !p.yawAktiv {
		p.chkYaw.SetChecked(true) // frontale Ausrichtung braucht die Yaw-Achse
	}
	g, f35 := p.geladen, p.f.wert
	p.beschaeftigt = true
	p.status.SetText("Werte die Richtungen aller Kanten im Bild aus …")
	go func() {
		o, err := kern.AutoOrientierung(g.Bild, f35, frontal)
		var am kern.AutoErgebnisMarken
		var errM error
		if err != nil || (o.NurRoll && !frontal) {
			am, errM = kern.AutoMarken(g.Bild, f35, frontal)
		}
		fyne.Do(func() {
			p.beschaeftigt = false
			if p.geladen != g {
				return
			}
			titel := "Automatisch ausrichten"
			if frontal {
				titel = "Automatisch frontal ausrichten"
			}
			if errM == nil && len(am.Marken) > 0 && (err != nil || o.NurRoll) {
				l := am.Loesung
				p.marken, p.markenLsg = am.Marken, l
				p.roll.setze(math.Round(l.Roll*10) / 10)
				p.pitch.setze(math.Round(l.Pitch*10) / 10)
				if l.BrennweiteBestimmt {
					p.f.setze(math.Round(l.F35*10) / 10)
				}
				if frontal {
					p.setzeYawAuto(math.Round(l.Yaw*10) / 10)
				} else {
					p.yaw.setze(0)
				}
				p.status.SetText(fmt.Sprintf("Automatisch an Rechtecken/Kreisen ausgerichtet: Roll %+.1f°, Pitch %+.1f°.", p.roll.wert, p.pitch.wert))
				return
			}
			if err != nil {
				p.status.SetText(err.Error())
				dialog.ShowInformation(titel, err.Error(), p.w)
				return
			}
			voll := kern.Parameter{Roll: o.Roll, Pitch: o.Pitch, Yaw: o.Yaw, F35: f35, Zuschneiden: p.zuschneiden, YawMitte: true}
			begr, anteil := kern.BegrenzeKorrektur(g.Bild.W, g.Bild.H, voll, 1.6)
			p.roll.setze(math.Round(begr.Roll*10) / 10)
			p.pitch.setze(math.Round(begr.Pitch*10) / 10)
			p.setzeYawAuto(math.Round(begr.Yaw*10) / 10)
			var t string
			switch {
			case o.NurRoll:
				t = fmt.Sprintf("Keine senkrechten Strukturen gefunden – nur nach dem Horizont gedreht: Roll %+.1f°.", o.Roll)
			case frontal:
				t = fmt.Sprintf("Automatisch frontal ausgerichtet: Roll %+.1f°, Pitch %+.1f°, Yaw %+.1f°.", begr.Roll, begr.Pitch, begr.Yaw)
			default:
				t = fmt.Sprintf("Automatisch ausgerichtet: Roll %+.1f°, Pitch %+.1f°.", begr.Roll, begr.Pitch)
			}
			if anteil < 0.999 {
				t += fmt.Sprintf(" Die volle Korrektur würde das Bild extrem verzerren – zu %.0f %% angewendet.", anteil*100)
			}
			t += fmt.Sprintf(" Übereinstimmung %.0f %%", o.Guete*100)
			if o.Unsicher {
				t += " – unsicher, bitte prüfen (ggf. Brennweite kontrollieren)."
			} else {
				t += "."
			}
			p.status.SetText(t)
		})
	}()
}

// setzeYawAuto: Yaw aus einer automatischen frontalen Ausrichtung (Drehpunkt
// Bildmitte); ein von Hand geänderter Yaw wechselt danach zur Unterkante.
func (p *programm) setzeYawAuto(v float64) {
	if !p.yawAktiv {
		v = 0
	}
	p.yaw.setze(v)
	p.yawMitte = p.yaw.wert != 0
}

// ------------------------------------------------------------------ Speichern

func (p *programm) speichernDialog() {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	vorschlag := bild.Zielname(p.pfad, "")
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			p.fehlerDialog("Speichern", err)
			return
		}
		if w == nil {
			return
		}
		ziel := w.URI().Path()
		w.Close()
		if filepath.Ext(ziel) == "" {
			ziel += ".jpg"
		}
		p.speichere(ziel)
	}, p.w)
	d.SetFileName(filepath.Base(vorschlag))
	d.SetFilter(storage.NewExtensionFileFilter([]string{".jpg", ".jpeg", ".tif", ".tiff", ".png"}))
	if l, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(vorschlag))); err == nil {
		d.SetLocation(l)
	}
	d.Resize(fyne.NewSize(900, 600))
	d.Show()
}

func (p *programm) speichere(ziel string) {
	par, rand, g := p.parameter(), p.randFarbe(), p.geladen
	groe, nichtGroesser, aus := p.ausgabe, p.nichtVergroessern, p.aus
	p.beschaeftigt = true
	if groe.Art == gArtOriginal || groe.Art == "" {
		p.status.SetText("Speichere in voller Auflösung …")
	} else {
		p.status.SetText("Speichere (" + groe.Name + ") …")
	}
	go func() {
		erg, err := kern.Korrigiere(g.Bild, par, rand, false)
		if err == nil && groe.Art != gArtOriginal && groe.Art != "" {
			erg = aus.anwenden(erg, groe, nichtGroesser, rand) // erst Ausschnitt, dann Zielgröße
		}
		if err == nil {
			err = bild.Speichere(erg, ziel, g.Exif, g.ICC)
		}
		fyne.Do(func() {
			p.beschaeftigt = false
			if err != nil {
				dialog.ShowError(fmt.Errorf("Fehler beim Speichern:\n%w", err), p.w)
				p.status.SetText("Speichern fehlgeschlagen.")
				return
			}
			p.status.SetText(fmt.Sprintf("Gespeichert: %s  (%d × %d)", ziel, erg.W, erg.H))
		})
	}()
}

// ------------------------------------------------------------------ Kameraprofile

func (p *programm) profilImportDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			if err != nil {
				p.fehlerDialog("Profil importieren", err)
			}
			return
		}
		pfad := r.URI().Path()
		r.Close()
		status, name, err := profil.Importiere(pfad, p.profilDir, false)
		if err != nil {
			p.fehlerDialog("Profil importieren", err)
			return
		}
		if status == "vorhanden" {
			dialog.ShowConfirm("Kameraprofil vorhanden",
				fmt.Sprintf("Für „%s“ gibt es schon ein Profil.\n\nDurch das neue ersetzen?", name),
				func(ja bool) {
					if ja {
						if _, _, err := profil.Importiere(pfad, p.profilDir, true); err != nil {
							p.fehlerDialog("Profil importieren", err)
							return
						}
						p.profile = profil.Lade(p.profilDir)
						p.setzeStatus("Profil ersetzt: " + name)
					}
				}, p.w)
			return
		}
		p.profile = profil.Lade(p.profilDir)
		p.setzeStatus("Profil importiert: " + name)
	}, p.w)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".v3cam", ".json"}))
	d.Resize(fyne.NewSize(900, 600))
	d.Show()
}

func (p *programm) profileAnzeigen() {
	zeilen := profil.Uebersicht(p.profile)
	sort.SliceStable(zeilen, func(i, j int) bool { return zeilen[i].Hersteller < zeilen[j].Hersteller })
	var sb strings.Builder
	for _, z := range zeilen {
		fmt.Fprintf(&sb, "%s – %s\n", z.Hersteller, z.Modelle)
	}
	if sb.Len() == 0 {
		sb.WriteString("Keine Kameraprofile gefunden.")
	}
	t := widget.NewLabel(sb.String())
	s := container.NewVScroll(t)
	s.SetMinSize(fyne.NewSize(560, 380))
	dialog.ShowCustom("Bekannte Kameras", "Schließen", s, p.w)
}
