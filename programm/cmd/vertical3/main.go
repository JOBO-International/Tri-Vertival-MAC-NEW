// Vertical3 – Perspektivkorrektur für Windows.
package main

import (
	"embed"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/bild"
	"vertical3/kern"
	"vertical3/meta"
	"vertical3/profil"
	rawlib "vertical3/roh"
	"vertical3/update"
)

// Programmversion (version.txt im Programmordner hat Vorrang, siehe Updates).
const grundVersion = "3.9.1"

const vorschauMax = 1600

//go:embed standardprofile/*.json
var standardProfile embed.FS

var randFarben = []struct {
	Name  string
	Farbe [3]uint8
}{{"Weiß", [3]uint8{255, 255, 255}}, {"Schwarz", [3]uint8{0, 0, 0}}, {"Grau", [3]uint8{128, 128, 128}}}

type regler struct {
	name     string
	von, bis float64
	schritt  float64
	slider   *walk.Slider
	feld     *walk.NumberEdit
	wert     float64
}

type programm struct {
	appDir, profilDir, version, exiftool string

	mw                                   *walk.MainWindow
	cw                                   *walk.CustomWidget
	panel                                *walk.CustomWidget
	actVergleich, actOriginal, actRaster *walk.Action
	actKanten                            *walk.Action
	actYaw, actFrontal                   *walk.Action
	yawAktiv                             bool // Yaw-Achse eingeschaltet (Einstellung)
	actDesignOffice, actDesignKamera     *walk.Action
	actDesignAnalog                      *walk.Action
	kantenZeigen                         bool
	// Zustand des selbst gezeichneten Bedienfelds
	zuschneiden, raster, vergleich, original bool
	randIdx                                  int
	txtDatei, txtKamera, statusText          string
	kameraAktiv                              bool
	ui                                       panelZustand
	roll, pitch, yaw, f35                    *regler
	setzt                                    bool

	profile  []*profil.Profil
	zusaetze []*zusatz

	// geladenes Bild
	pfad           string
	geladen        *bild.Geladen
	aktProfil      *profil.Profil
	hatDaten       bool
	txtKameraBasis string
	// Automatik: Kantenpunkte, die nach der Korrektur gerade stehen (Vollbild)
	autoPunkte [][2]float64
	autoPar    kern.Parameter
	yawMitte   bool // Yaw stammt aus einer automatischen frontalen Ausrichtung (Drehpunkt Bildmitte)
	// Drehen/Spiegeln und Ausgabegröße
	ausrichtung       []string
	ausrichtQ         kern.Mat3
	ausgabe           groesse
	eigeneGroessen    []groesse
	nichtVergroessern bool
	autoRoll          float64
	autoPitch         float64
	autoF35           float64
	vorschau          *kern.Bild
	nachher           *kern.Bild
	cache             map[string]*walk.Bitmap
	fehler            string

	// Kanten markieren
	modus  string // "" oder "marken" (Bezugsmarken bearbeiten)
	punkte [][2]float64
	maus   *walk.Point

	// Bezugsmarken
	marken         []kern.Marke
	markeNeu       string       // Art der Marke, die gerade gesetzt wird ("" = keine)
	ziehMarke      [2]int       // Marke/Punkt, der gezogen wird ({-1,-1} = keiner)
	markenLsg      kern.Loesung // letzte Ausgleichsrechnung
	markenFehler   string
	brennweiteFrei bool
	abb            struct { // Abbildung Vollbild → Bildschirm (nur Original-Ansicht)
		ox, oy, f float64
		ok        bool
	}

	// Zoom: 1 = ganzes Bild; zx/zy = Bildmitte der Ansicht in Bildanteilen
	zoom, zx, zy float64
	ansicht      struct {
		ox, oy, dw, dh float64
		ok, nachher    bool
	} // letzte Einzelansicht
	ziehen    *walk.Point
	aus       ausschnittZustand // Bildausschnitt für Ausgabegrößen mit festem Seitenverhältnis
	ziehStart [2]float64
	zoomBmp   *walk.Bitmap
	zoomKey   string

	mu           sync.Mutex
	timer        *time.Timer
	rechenNr     int
	beschaeftigt bool
}

func main() {
	if len(os.Args) == 4 && os.Args[1] == "--tausche" {
		pid, _ := strconv.Atoi(os.Args[2])
		tauscheNachEnde(pid, os.Args[3])
		return
	}
	exe, _ := os.Executable()
	p := &programm{appDir: filepath.Dir(exe), zoom: 1, zx: 0.5, zy: 0.5, zuschneiden: true, raster: true, kantenZeigen: true}
	p.profilDir = filepath.Join(p.appDir, "kameraprofile")
	p.version = update.InstallierteVersion(p.appDir, grundVersion)
	update.Aufraeumen(p.appDir)
	go func() { // nach einem Update läuft das alte Programm evtl. noch kurz
		for i := 0; i < 10; i++ {
			time.Sleep(2 * time.Second)
			update.Aufraeumen(p.appDir)
		}
	}()
	p.legeStandardprofileAn()
	p.profile = profil.Lade(p.profilDir)
	p.exiftool = meta.FindeExiftool(p.appDir)
	p.roll = &regler{name: "Roll  (Horizont) ", von: -45, bis: 45, schritt: 0.1}
	p.pitch = &regler{name: "Pitch  (stürzende Linien) ", von: -60, bis: 60, schritt: 0.1}
	p.yaw = &regler{name: "Yaw  (Perspektivwechsel) ", von: -45, bis: 45, schritt: 0.1}
	p.f35 = &regler{name: "Brennweite (KB-äquiv., mm)", von: 8, bis: 300, schritt: 0.5, wert: 28}

	// Erster Start nach einer Neuinstallation: immer im Design „Hell“
	if marke := filepath.Join(p.appDir, "erststart.txt"); dateiDa(marke) {
		e := ladeEinstellungen()
		e.Design = "office"
		speichereEinstellungen(e)
		os.Remove(marke)
	}
	setzeDesign(ladeEinstellungen().Design)
	p.groesseLaden()
	p.yawAktiv = ladeEinstellungen().YawAchse // Standard: aus
	p.ausrichtungZuruecksetzen()
	p.zusaetze = ladeZusaetze(p.appDir)
	if err := p.baueFenster(); err != nil {
		walk.MsgBox(nil, "Tri-Vertical", "Das Programm konnte nicht starten:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	if ic, err := walk.NewIconFromResource("APPICON"); err == nil {
		p.mw.SetIcon(ic)
	}
	p.yawMenue()
	alleTastenFuer(p.cw.Handle())
	p.cw.MouseWheel().Attach(p.mausrad)
	p.panel.MouseWheel().Attach(func(x, y int, knopf walk.MouseButton) {
		pt := win.POINT{X: int32(x), Y: int32(y)}
		win.ScreenToClient(p.panel.Handle(), &pt)
		p.panelRad(int(pt.X), int(pt.Y), walk.MouseWheelEventDelta(knopf), walk.MouseWheelEventKeyState(knopf)&0x4 != 0)
	})
	titelleiste(p.mw.Handle(), designDunkel)
	p.designMenue()
	p.cw.MouseUp().Attach(func(x, y int, _ walk.MouseButton) { p.ziehenEnde() })
	win.ShowWindow(p.mw.Handle(), win.SW_MAXIMIZE)
	p.cw.SetFocus()
	p.setzeStatus("Bild öffnen mit Strg+O oder über den Knopf rechts.")

	// Aufruf mit Datei (Doppelklick auf Bild, .v3cam oder .v3update)
	var updates, profile, bilder []string
	for _, a := range os.Args[1:] {
		if st, err := os.Stat(a); err != nil || st.IsDir() {
			continue
		}
		switch e := strings.ToLower(filepath.Ext(a)); {
		case e == update.Endung:
			updates = append(updates, a)
		case e == profil.Endung || e == ".json":
			profile = append(profile, a)
		case bild.IstBild(a):
			bilder = append(bilder, a)
		}
	}
	p.mw.Synchronize(func() {
		switch {
		case len(updates) > 0:
			p.updateInstallieren(updates[0])
		case len(profile) > 0:
			p.profileImportieren(profile)
		case len(bilder) > 0:
			p.lade(bilder[0])
		}
	})
	p.mw.Run()
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

func (p *programm) setzeWert(r *regler, v float64, _ any) {
	if r == p.yaw {
		p.yawMitte = false // von Hand: Yaw dreht um die Unterkante (Boden bleibt waagerecht)
	}
	if r == p.yaw && !p.yawAktiv {
		v = 0 // Yaw-Achse ausgeschaltet
	}
	v = math.Max(r.von, math.Min(r.bis, math.Round(v/r.schritt)*r.schritt))
	r.wert = v
	if r == p.f35 {
		p.yawGrenzeAnpassen()
	}
	if p.panel != nil {
		p.panel.Invalidate()
	}
	p.planeVorschau()
}

// setzeYawAuto setzt Yaw aus einer automatischen frontalen Ausrichtung. Diese
// rechnet mit dem Drehpunkt in der Bildmitte; ein von Hand geänderter Yaw
// wechselt danach zur Unterkante.
func (p *programm) setzeYawAuto(v float64) {
	p.setzeWert(p.yaw, v, nil)
	p.yawMitte = p.yaw.wert != 0
}

func (p *programm) baueFenster() error {
	umschalten := func(was *bool) func() {
		return func() { p.umschalten(was) }
	}
	return MainWindow{
		AssignTo: &p.mw,
		Title:    "Tri-Vertical " + p.version + " – Perspektivkorrektur",
		MinSize:  Size{Width: 900, Height: 600},
		Size:     Size{Width: 1400, Height: 900},
		Layout:   HBox{MarginsZero: true, SpacingZero: true},
		MenuItems: []MenuItem{
			Menu{Text: "&Datei", Items: append(append([]MenuItem{
				Action{Text: "Bild öffnen …", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyO}, OnTriggered: p.oeffnen},
				Action{Text: "Speichern unter …", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyS}, OnTriggered: p.speichern},
			}, p.zusatzMenue()...),
				Separator{},
				Action{Text: "Beenden", OnTriggered: func() { p.mw.Close() }},
			)},
			Menu{Text: "&Kameras", Items: []MenuItem{
				Action{Text: "Kameraprofil importieren …", OnTriggered: p.profilImportDialog},
				Action{Text: "Bekannte Kameras anzeigen", OnTriggered: p.profileAnzeigen},
				Action{Text: "Profilordner öffnen", OnTriggered: func() { oeffneExtern(p.profilDir) }},
			}},
			Menu{Text: "&Bild", Items: []MenuItem{
				Action{Text: "Nach links drehen (90°)", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyLeft}, OnTriggered: func() { p.drehen(kern.DrehLinks) }},
				Action{Text: "Nach rechts drehen (90°)", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyRight}, OnTriggered: func() { p.drehen(kern.DrehRechts) }},
				Separator{},
				Action{Text: "Waagerecht spiegeln", OnTriggered: func() { p.drehen(kern.SpiegelWaag) }},
				Action{Text: "Senkrecht spiegeln", OnTriggered: func() { p.drehen(kern.SpiegelSenk) }},
				Separator{},
				Action{Text: "Ausgabegröße …", OnTriggered: p.groesseWaehlen},
			}},
			Menu{Text: "&Ansicht", Items: []MenuItem{
				Action{AssignTo: &p.actVergleich, Text: "Vorher / Nachher nebeneinander", Checkable: true,
					Shortcut: Shortcut{Key: walk.KeyV}, OnTriggered: umschalten(&p.vergleich)},
				Action{AssignTo: &p.actOriginal, Text: "Original zeigen", Checkable: true,
					Shortcut: Shortcut{Key: walk.KeySpace}, OnTriggered: umschalten(&p.original)},
				Action{AssignTo: &p.actRaster, Text: "Hilfsraster", Checkable: true, Checked: true,
					OnTriggered: umschalten(&p.raster)},
				Action{AssignTo: &p.actKanten, Text: "Erkannte Kanten anzeigen", Checkable: true, Checked: true,
					Shortcut: Shortcut{Key: walk.KeyK}, OnTriggered: umschalten(&p.kantenZeigen)},
				Separator{},
				Menu{Text: "Design", Items: []MenuItem{
					Action{AssignTo: &p.actDesignOffice, Text: "Hell", Checkable: true, OnTriggered: func() { p.waehleDesign("office") }},
					Action{AssignTo: &p.actDesignKamera, Text: "Dunkel", Checkable: true, OnTriggered: func() { p.waehleDesign("kamera") }},
					Action{AssignTo: &p.actDesignAnalog, Text: "Analog", Checkable: true, OnTriggered: func() { p.waehleDesign("analog") }},
				}},
				Separator{},
				Action{Text: "Vergrößern  (+ / Mausrad)", OnTriggered: func() { p.zoomSchritt(1.5) }},
				Action{Text: "Verkleinern  (− / Mausrad)", OnTriggered: func() { p.zoomSchritt(1 / 1.5) }},
				Action{Text: "Ganzes Bild  (0)", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.Key0}, OnTriggered: p.zoomGanz},
				Action{Text: "100 % – Originalpixel  (1)", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.Key1}, OnTriggered: p.zoom100},
				Separator{},
				Action{Text: "Automatisch ausrichten", Shortcut: Shortcut{Key: walk.KeyA}, OnTriggered: func() { p.automatik(false) }},
				Action{AssignTo: &p.actFrontal, Text: "Automatisch frontal ausrichten", Shortcut: Shortcut{Modifiers: walk.ModShift, Key: walk.KeyA}, OnTriggered: func() { p.automatik(true) }},
				Separator{},
				Action{AssignTo: &p.actYaw, Text: "Yaw-Achse verwenden (Perspektivwechsel, frontal)", Checkable: true, OnTriggered: p.yawUmschalten},
			}},
			Menu{Text: "&Hilfe", Items: []MenuItem{
				Action{Text: "Anleitung", OnTriggered: func() { oeffneExtern(filepath.Join(p.appDir, "ANLEITUNG.txt")) }},
				Separator{},
				Action{Text: "Update installieren …", OnTriggered: p.updateDialog},
				Action{Text: "Zusatzprogramme …", OnTriggered: p.zusaetzeAnzeigen},
				Action{Text: "Letztes Update rückgängig machen", OnTriggered: p.updateRueckgaengig},
				Separator{},
				Action{Text: "Über Tri-Vertical", OnTriggered: func() {
					walk.MsgBox(p.mw, "Über Tri-Vertical", "Tri-Vertical "+p.version+"\nPerspektivkorrektur in drei Achsen für Windows\n\nNur für den privaten Gebrauch.", walk.MsgBoxIconInformation)
				}},
			}},
		},
		Children: []Widget{
			CustomWidget{
				AssignTo: &p.cw, StretchFactor: 1, ClearsBackground: false, InvalidatesOnResize: true,
				// gepuffert zeichnen: kein schwarzes Löschen vor jedem Neuzeichnen (kein Flackern)
				PaintMode: PaintBuffered, PaintPixels: p.male,
				OnMouseDown: p.klick, OnMouseMove: p.mausBewegt, OnMouseUp: p.mausLos, OnKeyDown: p.taste,
			},
			CustomWidget{
				AssignTo: &p.panel, ClearsBackground: false, InvalidatesOnResize: true,
				MinSize: Size{Width: panelBreite}, MaxSize: Size{Width: panelBreite},
				PaintMode: PaintBuffered, PaintPixels: p.malePanel,
				OnMouseDown: p.panelKlick, OnMouseMove: p.panelMaus, OnMouseUp: p.panelLos,
			},
		},
	}.Create()
}

// umschalten wechselt eine Ansichts-Einstellung (Bedienfeld, Menü oder Taste).
func (p *programm) umschalten(was *bool) {
	*was = !*was
	if p.actVergleich != nil {
		p.actVergleich.SetChecked(p.vergleich)
		p.actOriginal.SetChecked(p.original)
		p.actRaster.SetChecked(p.raster)
		p.actKanten.SetChecked(p.kantenZeigen)
	}
	if was == &p.zuschneiden {
		p.planeVorschau()
	}
	if was == &p.brennweiteFrei {
		p.markenLoesen()
	}
	p.panel.Invalidate()
	p.cw.Invalidate()
}

func (p *programm) setzeStatus(t string) {
	p.statusText = t
	if p.cw != nil {
		p.cw.Invalidate()
	}
}

// mitFokus: nach einem Klick bekommt der Bildbereich den Tastaturfokus –
// sonst würde die Leertaste zusätzlich den zuletzt geklickten Knopf auslösen.
func (p *programm) mitFokus(f func()) walk.EventHandler {
	return func() {
		f()
		if p.cw != nil {
			p.cw.SetFocus()
		}
	}
}

func oeffneExtern(pfad string) {
	win.ShellExecute(0, nil, syscallUTF16(pfad), nil, nil, win.SW_SHOWNORMAL)
}

func (p *programm) parameter() kern.Parameter {
	yaw := p.yaw.wert
	if !p.yawAktiv {
		yaw = 0
	}
	return kern.Parameter{Roll: p.roll.wert, Pitch: p.pitch.wert, Yaw: yaw, YawMitte: p.yawMitte,
		F35: math.Max(5, p.f35.wert), Zuschneiden: p.zuschneiden}
}

func (p *programm) randFarbe() [3]uint8 {
	i := p.randIdx
	if i < 0 || i >= len(randFarben) {
		i = 0
	}
	return randFarben[i].Farbe
}

// ------------------------------------------------------------------ Laden

func (p *programm) oeffnen() {
	dlg := walk.FileDialog{Title: "Bild öffnen",
		Filter: bild.Filter()}
	if ok, _ := dlg.ShowOpen(p.mw); ok {
		p.lade(dlg.FilePath)
	}
}

func (p *programm) lade(pfad string) {
	if p.beschaeftigt {
		return
	}
	p.beschaeftigt = true
	if e := filepath.Ext(pfad); strings.EqualFold(e, ".dng") || rawlib.IstRoh(e) {
		p.setzeStatus("Entwickle RAW-Datei " + filepath.Base(pfad) + " …")
	} else {
		p.setzeStatus("Lade " + filepath.Base(pfad) + " …")
	}
	p.mw.SetCursor(walk.CursorWait())
	go func() {
		g, err := bild.Lade(pfad, p.exiftool)
		var klein *kern.Bild
		if err == nil {
			klein = g.Bild.Verkleinert(vorschauMax)
		}
		p.mw.Synchronize(func() {
			p.beschaeftigt = false
			p.mw.SetCursor(nil)
			if err != nil {
				walk.MsgBox(p.mw, "Fehler", "Das Bild konnte nicht geöffnet werden:\n"+err.Error(), walk.MsgBoxIconError)
				p.setzeStatus("Bild konnte nicht geöffnet werden.")
				return
			}
			p.uebernimm(pfad, g, klein)
		})
	}()
}

func (p *programm) uebernimm(pfad string, g *bild.Geladen, klein *kern.Bild) {
	p.pfad, p.geladen, p.vorschau = pfad, g, klein
	p.ausrichtungZuruecksetzen()
	p.aktProfil = profil.Passend(g.Meta, p.profile)
	p.hatDaten, p.autoRoll, p.autoPitch = false, 0, 0
	p.autoF35 = profil.KBBrennweite(g.Meta, p.aktProfil)

	p.nachher = nil
	p.leereCache("")
	p.zoom, p.zx, p.zy = 1, 0.5, 0.5

	kamera := strings.TrimSpace(g.Meta["Make"] + " " + g.Meta["Model"])
	if kamera == "" {
		kamera = "unbekannt"
	}
	p.txtDatei = fmt.Sprintf("%s  ·  %d × %d", filepath.Base(pfad), g.Bild.W, g.Bild.H)
	p.txtKameraBasis = fmt.Sprintf("Kamera: %s\nProfil: %s", kamera, p.aktProfil.Name())
	if g.Hinweis != "" {
		p.txtKameraBasis += "\n" + g.Hinweis
	}
	p.txtKamera = p.txtKameraBasis
	p.kameraAktiv = false
	p.panel.Invalidate()
	p.markenZuruecksetzen()
	p.setzeWert(p.yaw, 0, nil)
	p.setzeWert(p.f35, math.Round(p.autoF35*10)/10, nil)
	p.setzeWert(p.roll, 0, nil)
	p.setzeWert(p.pitch, 0, nil)
	p.mw.SetTitle("Tri-Vertical " + p.version + " – " + filepath.Base(pfad))
	p.setzeStatus("Bild geladen. Automatisch ausrichten (Taste A), Regler verschieben oder speichern.")
	p.planeVorschau()
	p.cw.SetFocus()
	p.neigungAbfragen(pfad, g)
}

func (p *programm) kameradatenUebernehmen() {
	if p.hatDaten {
		// Kamerawerte gelten für das Bild in Aufnahmelage – Drehen/Spiegeln berücksichtigen
		r, pi, y := kern.WinkelNachAusrichtung(p.autoRoll, p.autoPitch, 0, p.ausrichtQ)
		p.setzeWert(p.roll, math.Round(r*10)/10, nil)
		p.setzeWert(p.pitch, math.Round(pi*10)/10, nil)
		p.setzeWert(p.yaw, math.Round(y*10)/10, nil)
	}
}

func (p *programm) zuruecksetzen() {
	p.setzeWert(p.roll, 0, nil)
	p.setzeWert(p.pitch, 0, nil)
	p.setzeWert(p.yaw, 0, nil)
	if p.geladen != nil {
		p.setzeWert(p.f35, math.Round(p.autoF35*10)/10, nil)
	}
}

// ------------------------------------------------------------------ Vorschau

func (p *programm) planeVorschau() {
	if p.vorschau == nil || p.mw == nil {
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
		p.mw.Synchronize(func() {
			p.mu.Lock()
			aktuell := nr == p.rechenNr
			p.mu.Unlock()
			if !aktuell {
				return
			}
			if err != nil {
				p.fehler = err.Error()
			} else {
				p.fehler = ""
				p.nachher = erg
				p.leereCache("nachher")
			}
			p.cw.Invalidate()
		})
	})
}

// leereCache gibt die zwischengespeicherten Bildschirmbilder frei.
// name = "" leert alles, sonst nur die Einträge dieses Bildes.
func (p *programm) leereCache(name string) {
	if p.cache == nil {
		p.cache = map[string]*walk.Bitmap{}
	}
	for k, b := range p.cache {
		if name == "" || strings.HasPrefix(k, name+"/") {
			b.Dispose()
			delete(p.cache, k)
		}
	}
}

// zeichneBild malt ein Bild in genau der Bildschirmgröße (selbst skaliert,
// mit Raster) – gleich scharf auf jedem System.
func (p *programm) zeichneBild(c *walk.Canvas, name string, b *kern.Bild, r walk.Rectangle, raster bool) {
	if r.Width <= 0 || r.Height <= 0 || b == nil {
		return
	}
	if p.cache == nil {
		p.cache = map[string]*walk.Bitmap{}
	}
	key := fmt.Sprintf("%s/%dx%d/%v", name, r.Width, r.Height, raster)
	bmp, ok := p.cache[key]
	if !ok {
		groesse := fmt.Sprintf("%s/%dx%d/", name, r.Width, r.Height)
		for k, alt := range p.cache { // nur andere Größen desselben Bildes verwerfen
			if strings.HasPrefix(k, name+"/") && !strings.HasPrefix(k, groesse) {
				alt.Dispose()
				delete(p.cache, k)
			}
		}
		var err error
		bmp, err = walk.NewBitmapFromImageForDPI(b.Anzeige(r.Width, r.Height, raster, rasterFarbe), 96)
		if err != nil {
			return
		}
		p.cache[key] = bmp
	}
	c.DrawImagePixels(bmp, walk.Point{X: r.X, Y: r.Y})
}

func (p *programm) male(c *walk.Canvas, _ walk.Rectangle) error {
	ganz := p.cw.ClientBoundsPixels()
	b := p.bildBereich()
	f := p.farben()
	c.FillRectanglePixels(f.pinsel(farbeBild), ganz)
	schrift := p.schrift("titel")
	p.abb.ok = false
	defer p.maleStatus(c, ganz)

	if p.vorschau == nil {
		k := float64(c.DPI()) / 96
		px := func(v float64) int { return int(v*k + 0.5) }
		// Höhe aus der tatsächlichen Schrift – nichts wird abgeschnitten
		p.maleLogo(c, walk.Rectangle{X: 0, Y: b.Height/2 - px(100), Width: b.Width, Height: px(84)}, px(84), "logo", true)
		c.FillRectanglePixels(f.pinsel(farbeRot), walk.Rectangle{X: b.Width/2 - px(20), Y: b.Height / 2, Width: px(40), Height: px(3)})
		c.DrawTextPixels("Bild öffnen  ·  Strg+O", p.schrift("text"), farbeGrau,
			walk.Rectangle{X: 0, Y: b.Height/2 + px(14), Width: b.Width, Height: px(30)}, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
		return nil
	}
	original := p.original || p.modus != ""
	p.ansicht.ok = false
	if p.vergleich && !original && p.nachher != nil {
		p.maleVergleich(c, b, schrift)
	} else {
		bild, name := p.nachher, "nachher"
		if original || bild == nil || p.fehler != "" {
			bild, name = p.vorschau, "vorher"
		}
		ox, oy, _, _ := p.einzelansicht(c, b, name, bild)
		if original {
			p.abb.ox, p.abb.oy, p.abb.f, p.abb.ok = p.ansicht.ox, p.ansicht.oy, p.ansicht.dw/float64(p.geladen.Bild.W), true
		}
		p.maleMarkenErgebnis(c, name == "nachher")
		p.maleAutoPunkte(c, name == "nachher")
		if name == "nachher" && !original && p.modus == "" {
			p.maleAusschnitt(c)
		}
		if original && p.modus == "" {
			p.marke(c, gt("Original"), max(ox, 0)+12, max(oy, 0)+12, true)
		}
		if p.modus != "" {
			p.maleMarken(c)
		}
	}
	if p.fehler != "" {
		c.DrawTextPixels(p.fehler, schrift, farbeRotHell,
			walk.Rectangle{X: 0, Y: 10, Width: b.Width, Height: int(40 * float64(c.DPI()) / 96)}, walk.TextCenter|walk.TextSingleLine)
	}
	return nil
}

// maleVergleich: Vorher und Nachher im gleichen Maßstab, neben- oder untereinander.
func (p *programm) maleVergleich(c *walk.Canvas, b walk.Rectangle, schrift *walk.Font) {
	k := float64(c.DPI()) / 96
	luecke, textH := int(16*k), int(30*k)
	lF, rF, tF := float64(luecke), 10*k, float64(textH)
	w1, h1 := float64(p.vorschau.W), float64(p.vorschau.H)
	w2, h2 := float64(p.nachher.W), float64(p.nachher.H)
	cw, ch := float64(b.Width), float64(b.Height)
	fNeben := math.Min((cw-2*rF-lF)/(w1+w2), (ch-2*rF-tF)/math.Max(h1, h2))
	fUnter := math.Min((cw-2*rF)/math.Max(w1, w2), (ch-2*rF-lF-2*tF)/(h1+h2))
	neben := fNeben >= fUnter
	f := math.Max(0.01, math.Max(fNeben, fUnter))
	g1 := [2]int{int(w1 * f), int(h1 * f)}
	g2 := [2]int{int(w2 * f), int(h2 * f)}
	var o1, o2 [2]int
	if neben {
		gesamt := g1[0] + luecke + g2[0]
		x0 := (b.Width - gesamt) / 2
		hoehe := max(g1[1], g2[1])
		y0 := (b.Height-hoehe-textH)/2 + textH
		o1 = [2]int{x0, y0 + (hoehe-g1[1])/2}
		o2 = [2]int{x0 + g1[0] + luecke, y0 + (hoehe-g2[1])/2}
	} else {
		gesamt := g1[1] + g2[1] + luecke + 2*textH
		y0 := (b.Height - gesamt) / 2
		o1 = [2]int{(b.Width - g1[0]) / 2, y0 + textH}
		o2 = [2]int{(b.Width - g2[0]) / 2, y0 + 2*textH + g1[1] + luecke}
	}
	p.zeichneBild(c, "vorher", p.vorschau, walk.Rectangle{X: o1[0], Y: o1[1], Width: g1[0], Height: g1[1]}, false)
	p.zeichneBild(c, "nachher", p.nachher, walk.Rectangle{X: o2[0], Y: o2[1], Width: g2[0], Height: g2[1]}, p.raster)
	p.marke(c, gt("Vorher"), o1[0], o1[1]-textH+2, false)
	p.marke(c, gt("Nachher"), o2[0], o2[1]-textH+2, true)
}

// ------------------------------------------------------------------ Speichern

func (p *programm) speichern() { p.speichernUnd(false) }

// ausschnittWeiter speichert den Ausschnitt in der Zielgröße und öffnet das
// Ergebnis gleich wieder – zum Weiterbearbeiten (Filter, Rahmen, Drucken …).
func (p *programm) ausschnittWeiter() { p.speichernUnd(true) }

func (p *programm) speichernUnd(weiter bool) {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	vorschlag := bild.Zielname(p.pfad, "")
	titel := "Korrigiertes Bild speichern"
	if weiter {
		stamm := strings.TrimSuffix(vorschlag, filepath.Ext(vorschlag))
		vorschlag = stamm + "_" + strings.NewReplacer(" ", "", "×", "x", ",", "-").Replace(p.ausgabeKurz()) + filepath.Ext(vorschlag)
		titel = "Ausschnitt speichern und weiterbearbeiten"
	}
	dlg := walk.FileDialog{Title: titel, FilePath: vorschlag,
		InitialDirPath: filepath.Dir(vorschlag),
		Filter:         "JPEG (*.jpg)|*.jpg|TIFF verlustfrei (*.tif)|*.tif|PNG (*.png)|*.png"}
	switch strings.ToLower(filepath.Ext(vorschlag)) {
	case ".tif", ".tiff":
		dlg.FilterIndex = 2
	case ".png":
		dlg.FilterIndex = 3
	default:
		dlg.FilterIndex = 1
	}
	ok, _ := dlg.ShowSave(p.mw)
	if !ok {
		return
	}
	ziel := dlg.FilePath
	if filepath.Ext(ziel) == "" {
		ziel += []string{".jpg", ".jpg", ".tif", ".png"}[dlg.FilterIndex]
	}
	par, rand, g := p.parameter(), p.randFarbe(), p.geladen
	groe, nichtGroesser := p.ausgabe, p.nichtVergroessern
	p.beschaeftigt = true
	p.mw.SetCursor(walk.CursorWait())
	if groe.Art == gArtOriginal || groe.Art == "" {
		p.setzeStatus("Speichere in voller Auflösung …")
	} else {
		p.setzeStatus("Speichere (" + groe.Name + ") …")
	}
	go func() {
		erg, err := kern.Korrigiere(g.Bild, par, rand, false)
		if err == nil && groe.Art != gArtOriginal && groe.Art != "" {
			erg = p.ausschnittAnwenden(erg, groe, nichtGroesser, rand) // erst Ausschnitt, dann Zielgröße
		}
		if err == nil {
			err = bild.Speichere(erg, ziel, g.Exif, g.ICC)
		}
		p.mw.Synchronize(func() {
			p.beschaeftigt = false
			p.mw.SetCursor(nil)
			if err != nil {
				walk.MsgBox(p.mw, "Fehler beim Speichern", err.Error(), walk.MsgBoxIconError)
				p.setzeStatus("Speichern fehlgeschlagen.")
				return
			}
			p.setzeStatus(fmt.Sprintf("Gespeichert: %s  (%d × %d)", ziel, erg.W, erg.H))
			if weiter {
				p.lade(ziel) // gespeichertes Bild zum Weiterbearbeiten öffnen
			}
		})
	}()
}

// ------------------------------------------------------------------ Kameraprofile

func (p *programm) profilImportDialog() {
	dlg := walk.FileDialog{Title: "Kameraprofil importieren",
		Filter: "Vertical3-Kameraprofil (*.v3cam;*.json)|*.v3cam;*.json|Alle Dateien (*.*)|*.*"}
	if ok, _ := dlg.ShowOpenMultiple(p.mw); ok {
		p.profileImportieren(dlg.FilePaths)
	}
}

func (p *programm) profileImportieren(dateien []string) {
	var meldungen []string
	for _, f := range dateien {
		status, name, err := profil.Importiere(f, p.profilDir, false)
		if err != nil {
			meldungen = append(meldungen, fmt.Sprintf("• %s: FEHLER – %s", filepath.Base(f), err))
			continue
		}
		if status == "vorhanden" {
			if walk.MsgBox(p.mw, "Kameraprofil vorhanden",
				fmt.Sprintf("Für „%s“ gibt es schon ein Profil.\n\nDurch das neue ersetzen?", name),
				walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
				meldungen = append(meldungen, "• "+name+": nicht geändert")
				continue
			}
			status, name, err = profil.Importiere(f, p.profilDir, true)
			if err != nil {
				meldungen = append(meldungen, fmt.Sprintf("• %s: FEHLER – %s", name, err))
				continue
			}
		}
		text := "hinzugefügt"
		if status == "ersetzt" {
			text = "aktualisiert"
		}
		meldungen = append(meldungen, "• "+name+": "+text)
	}
	p.profile = profil.Lade(p.profilDir)
	walk.MsgBox(p.mw, "Kameraprofile", strings.Join(meldungen, "\n"), walk.MsgBoxIconInformation)
	if p.pfad != "" {
		p.lade(p.pfad)
	}
}

// kameraModell speist die Tabelle der bekannten Kameras.
type kameraModell struct {
	walk.TableModelBase
	zeilen []profil.Zeile
	eigene map[string]bool
}

func (m *kameraModell) RowCount() int { return len(m.zeilen) }
func (m *kameraModell) Value(r, c int) interface{} {
	z := m.zeilen[r]
	switch c {
	case 0:
		return z.Hersteller
	case 1:
		return z.Modelle
	case 2:
		return z.Neigung
	case 3:
		return z.Formate
	case 4:
		if m.eigene[z.Datei] || z.Datei == "" {
			return "eingebaut"
		}
		return "importiert"
	}
	return ""
}

func (p *programm) profileAnzeigen() {
	eingebaut := map[string]bool{}
	if eintraege, err := standardProfile.ReadDir("standardprofile"); err == nil {
		for _, e := range eintraege {
			eingebaut[e.Name()] = true
		}
	}
	modell := &kameraModell{zeilen: profil.Uebersicht(p.profile), eigene: eingebaut}
	et := "exiftool: nicht vorhanden – nötig für die Neigungsdaten von Nikon, Canon und OM System (siehe Anleitung)."
	if p.exiftool != "" {
		et = "exiftool: gefunden – auch Nikon, Canon, OM System und weitere Kameras werden ausgelesen."
	}
	var dlg *walk.Dialog
	Dialog{
		AssignTo: &dlg,
		Title:    "Bekannte Kameras",
		MinSize:  Size{Width: 1120, Height: 420},
		Size:     Size{Width: 1120, Height: 460},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: "Vertical3 erkennt die Kamera am Hersteller in den Bilddaten. Die Modelle sind Beispiele – " +
				"andere Modelle desselben Herstellers werden ebenfalls erkannt."},
			TableView{
				Model:            modell,
				AlternatingRowBG: true,
				Columns: []TableViewColumn{
					{Title: "Hersteller", Width: 95},
					{Title: "Modelle", Width: 345},
					{Title: "Neigungsdaten", Width: 300},
					{Title: "Formate", Width: 235},
					{Title: "Profil", Width: 80},
				},
			},
			Label{Text: et},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "Kameraprofil importieren …", OnClicked: func() {
					dlg.Accept()
					p.profilImportDialog()
				}},
				PushButton{Text: "Profilordner öffnen", OnClicked: func() { oeffneExtern(p.profilDir) }},
				HSpacer{},
				PushButton{Text: "Schließen", OnClicked: func() { dlg.Accept() }},
			}},
		},
	}.Run(p.mw)
}

// ------------------------------------------------------------------ Updates

func (p *programm) updateDialog() {
	dlg := walk.FileDialog{Title: "Vertical3-Update installieren",
		Filter: "Vertical3-Update (*.v3update)|*.v3update|Alle Dateien (*.*)|*.*"}
	if ok, _ := dlg.ShowOpen(p.mw); ok {
		p.updateInstallieren(dlg.FilePath)
	}
}

func (p *programm) updateInstallieren(pfad string) {
	m, err := update.Lies(pfad)
	if err == nil {
		err = update.Passend(m, p.version)
	}
	if err != nil {
		walk.MsgBox(p.mw, "Update nicht möglich", err.Error(), walk.MsgBoxIconWarning)
		return
	}
	text := fmt.Sprintf("Installiert: Version %s\nUpdate auf: Version %s\n\n%s\n\n"+
		"Der bisherige Stand wird vorher gesichert und kann über\nHilfe → „Letztes Update rückgängig machen“ wiederhergestellt werden.\n\nUpdate jetzt installieren?",
		p.version, m.Version, strings.TrimSpace(m.Beschreibung))
	if walk.MsgBox(p.mw, "Vertical3-Update", text, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
		return
	}
	if _, err := update.Installiere(pfad, p.appDir, p.version); err != nil {
		walk.MsgBox(p.mw, "Update fehlgeschlagen", err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(p.mw, "Vertical3-Update", "Update auf Version "+m.Version+" installiert.\n\nVertical3 wird jetzt neu gestartet.", walk.MsgBoxIconInformation)
	p.neuStarten()
}

func (p *programm) updateRueckgaengig() {
	v, _ := update.LetzteSicherung(p.appDir)
	if v == "" {
		walk.MsgBox(p.mw, "Update rückgängig", "Es gibt keinen gesicherten früheren Stand.", walk.MsgBoxIconInformation)
		return
	}
	if walk.MsgBox(p.mw, "Update rückgängig", fmt.Sprintf("Zurück zu Version %s?\n(Installiert ist Version %s.)", v, p.version),
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
		return
	}
	v, err := update.Rueckgaengig(p.appDir)
	if err != nil {
		walk.MsgBox(p.mw, "Fehler", err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(p.mw, "Update rückgängig", "Version "+v+" wiederhergestellt.\nVertical3 wird neu gestartet.", walk.MsgBoxIconInformation)
	p.neuStarten()
}

func (p *programm) neuStarten() {
	exe := filepath.Join(p.appDir, "Vertical3.exe")
	if len(update.Ausstehend) > 0 {
		// Gesperrte Programmdatei: die neue Version wartet, bis diese beendet
		// ist, tauscht sich dann selbst ein und startet Vertical3 neu.
		a := update.Ausstehend[0]
		cmd := exec.Command(a[0], "--tausche", strconv.Itoa(os.Getpid()), a[1])
		cmd.Dir = p.appDir
		if cmd.Start() != nil {
			walk.MsgBox(p.mw, "Vertical3-Update", "Bitte Vertical3 schließen und neu starten – das Update wird dann abgeschlossen.", walk.MsgBoxIconInformation)
		}
		p.mw.Close()
		return
	}
	cmd := exec.Command(exe)
	cmd.Dir = p.appDir
	cmd.Start()
	p.mw.Close()
}

// tauscheNachEnde läuft in der neuen Programmversion (Datei *.ausstehend):
// warten, bis das alte Programm beendet ist, dann die Programmdatei ersetzen
// und neu starten.
func tauscheNachEnde(pid int, ziel string) {
	if h, err := syscall.OpenProcess(0x00100000 /* SYNCHRONIZE */, false, uint32(pid)); err == nil {
		syscall.WaitForSingleObject(h, 60000)
		syscall.CloseHandle(h)
	}
	ich, _ := os.Executable()
	daten, err := os.ReadFile(ich)
	if err == nil {
		for i := 0; i < 60; i++ {
			if update.Ueberschreibe(ziel, daten) == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	cmd := exec.Command(ziel)
	cmd.Dir = filepath.Dir(ziel)
	cmd.Start()
}

func dateiDa(p string) bool { _, err := os.Stat(p); return err == nil }

// yawGrenzeAnpassen: Der sinnvolle Yaw hängt vom waagerechten Bildwinkel ab.
// Bei einer Drehung um den halben Bildwinkel würde eine Bildseite ins
// Unendliche gezogen; schon deutlich davor wird das Bild extrem verzerrt.
// Grenze: drei Viertel des halben waagerechten Bildwinkels (auf 0,5° gerundet).
func (p *programm) yawGrenzeAnpassen() {
	if p.yaw == nil || p.f35 == nil {
		return
	}
	grenze := 45.0
	if p.geladen != nil {
		w, h := p.geladen.Bild.W, p.geladen.Bild.H
		f := kern.BrennweitePx(p.f35.wert, w, h)
		halb := math.Atan(float64(w)/2/f) * 180 / math.Pi
		grenze = math.Max(3, math.Min(45, math.Floor(halb*0.75*2)/2))
	}
	p.yaw.von, p.yaw.bis = -grenze, grenze
	if math.Abs(p.yaw.wert) > grenze {
		p.yaw.wert = math.Copysign(grenze, p.yaw.wert)
		p.planeVorschau()
	}
	if p.panel != nil {
		p.panel.Invalidate()
	}
}

// yawUmschalten: Yaw-Achse ein/aus (Menü Ansicht), wird gespeichert.
func (p *programm) yawUmschalten() {
	p.yawAktiv = !p.yawAktiv
	e := ladeEinstellungen()
	e.YawAchse = p.yawAktiv
	speichereEinstellungen(e)
	p.yawMenue()
	if !p.yawAktiv && p.yaw.wert != 0 {
		p.setzeWert(p.yaw, 0, nil)
	}
	if p.yawAktiv {
		p.setzeStatus("Yaw-Achse eingeschaltet: Yaw-Regler und „Automatisch frontal“ (Umschalt+A) sind verfügbar.")
	} else {
		p.setzeStatus("Yaw-Achse ausgeschaltet: korrigiert wird nur Roll und Pitch.")
	}
	p.panel.Invalidate()
	p.cw.Invalidate()
}

func (p *programm) yawMenue() {
	if p.actYaw != nil {
		p.actYaw.SetChecked(p.yawAktiv)
	}
	if p.actFrontal != nil {
		p.actFrontal.SetVisible(p.yawAktiv)
	}
}
