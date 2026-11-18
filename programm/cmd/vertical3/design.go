package main

// Drei Designs zum Umschalten (Menü Ansicht → Design):
//   "office" – hell, im Stil von Microsoft 365 (Fluent): weiß, Segoe UI, blauer Akzent, abgerundet
//   "kamera" – dunkel, im Stil eines Kameramenüs: schwarz, weiße Schrift, rote Akzente, kantig
//   "analog" – warm wie Fotopapier und Dunkelkammer: Elfenbein, Sepia-Schrift,
//              Rotlicht-Akzent, Werte in Schreibmaschinenschrift wie ein Filmrand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

var (
	designName         = "office"
	designGross        bool // Beschriftungen in Großbuchstaben
	designDunkel       bool
	designRund         int  // Eckenradius (96-dpi-Punkte), 0 = kantig
	designLinien       bool // Trennlinie neben Abschnittstiteln
	farbeAusGrund      walk.Color
	farbeAkzentFlaeche walk.Color
	farbeAkzentHover   walk.Color
	farbeRahmenText    walk.Color
	farbeKnopfText     walk.Color
	farbeZeileHover    walk.Color
	farbeHalo          walk.Color
	farbeReglerKnopf   walk.Color
	farbeReglerRand    walk.Color
	farbeSchalterAus   walk.Color
	farbeSchalterKnopf walk.Color
	farbeMarkeGrund    walk.Color
	farbeAbschnitt     walk.Color
	schriftFamilien    map[string]schriftSpez
)

type schriftSpez struct {
	familie string
	groesse int
	stil    walk.FontStyle
}

func setzeDesign(name string) {
	if name != "kamera" && name != "analog" {
		name = "office"
	}
	designName = name
	if name == "analog" {
		designGross, designDunkel, designRund, designLinien = false, false, 2, true
		farbeBild = walk.RGB(214, 205, 189)   // Leuchttisch-Rand
		farbePanel = walk.RGB(243, 236, 222)  // Elfenbein
		farbeStatus = walk.RGB(233, 224, 206) // Barytpapier
		farbeFlaeche = walk.RGB(251, 247, 239)
		farbeHover = walk.RGB(238, 228, 210)
		farbeLinie = walk.RGB(214, 201, 178)
		farbeSpur = walk.RGB(190, 175, 150)
		farbeText = walk.RGB(52, 42, 33) // Sepia
		farbeGrau = walk.RGB(122, 106, 88)
		farbeDunkelgrau = walk.RGB(170, 156, 134)
		farbeRot = walk.RGB(178, 62, 36) // Rotlicht der Dunkelkammer
		farbeRotDunkel = walk.RGB(140, 46, 26)
		farbeRotHell = walk.RGB(178, 62, 36)
		farbeAusGrund = walk.RGB(236, 228, 214)
		farbeAkzentFlaeche = walk.RGB(246, 230, 220)
		farbeAkzentHover = walk.RGB(240, 214, 200)
		farbeRahmenText = walk.RGB(150, 50, 28)
		farbeKnopfText = walk.RGB(255, 248, 238)
		farbeZeileHover = walk.RGB(238, 228, 210)
		farbeHalo = walk.RGB(238, 210, 196)
		farbeReglerKnopf = walk.RGB(178, 62, 36)
		farbeReglerRand = walk.RGB(251, 247, 239)
		farbeSchalterAus = walk.RGB(251, 247, 239)
		farbeSchalterKnopf = walk.RGB(255, 248, 238)
		farbeMarkeGrund = walk.RGB(60, 48, 38)
		farbeAbschnitt = walk.RGB(150, 50, 28)
		schriftFamilien = map[string]schriftSpez{
			"gross": {"Georgia", 24, walk.FontBold}, "kopf": {"Georgia", 15, walk.FontBold},
			"titel": {"Georgia", 11, walk.FontBold}, "abschnitt": {"Courier New", 10, walk.FontBold},
			"label": {"Segoe UI", 9, 0}, "wert": {"Consolas", 10, 0}, "knopf": {"Segoe UI Semibold", 9, 0},
			"text": {"Segoe UI", 9, 0}, "klein": {"Segoe UI", 8, 0},
		}
	} else if name == "kamera" {
		designGross, designDunkel, designRund, designLinien = true, true, 0, true
		farbeBild = walk.RGB(10, 10, 10)
		farbePanel = walk.RGB(20, 20, 20)
		farbeStatus = walk.RGB(16, 16, 16)
		farbeFlaeche = walk.RGB(34, 34, 34)
		farbeHover = walk.RGB(48, 48, 48)
		farbeLinie = walk.RGB(52, 52, 52)
		farbeSpur = walk.RGB(70, 70, 70)
		farbeText = walk.RGB(242, 242, 242)
		farbeGrau = walk.RGB(145, 145, 145)
		farbeDunkelgrau = walk.RGB(85, 85, 85)
		farbeRot = walk.RGB(226, 0, 26)
		farbeRotDunkel = walk.RGB(160, 0, 18)
		farbeRotHell = walk.RGB(255, 84, 96)
		farbeAusGrund = farbePanel
		farbeAkzentFlaeche = farbePanel
		farbeAkzentHover = walk.RGB(40, 10, 14)
		farbeRahmenText = farbeText
		farbeKnopfText = walk.RGB(255, 255, 255)
		farbeZeileHover = walk.RGB(28, 28, 28)
		farbeHalo = walk.RGB(70, 20, 24)
		farbeReglerKnopf = farbeText
		farbeReglerRand = farbeText
		farbeSchalterAus = farbeFlaeche
		farbeSchalterKnopf = farbeText
		farbeMarkeGrund = walk.RGB(30, 30, 30)
		farbeAbschnitt = farbeGrau
		schriftFamilien = map[string]schriftSpez{
			"gross": {"Bahnschrift", 22, walk.FontBold}, "kopf": {"Bahnschrift", 15, walk.FontBold},
			"titel": {"Bahnschrift", 11, walk.FontBold}, "abschnitt": {"Bahnschrift", 8, 0},
			"label": {"Bahnschrift", 9, 0}, "wert": {"Bahnschrift", 10, 0}, "knopf": {"Bahnschrift", 9, walk.FontBold},
			"text": {"Segoe UI", 9, 0}, "klein": {"Segoe UI", 8, 0},
		}
	} else {
		// Farben angelehnt an Microsoft Fluent (Office/Microsoft 365, hell)
		designGross, designDunkel, designRund, designLinien = false, false, 4, false
		farbeBild = walk.RGB(232, 232, 232)
		farbePanel = walk.RGB(255, 255, 255)
		farbeStatus = walk.RGB(243, 243, 243)
		farbeFlaeche = walk.RGB(255, 255, 255)
		farbeHover = walk.RGB(245, 245, 245)
		farbeLinie = walk.RGB(224, 224, 224)
		farbeSpur = walk.RGB(199, 199, 199)
		farbeText = walk.RGB(36, 36, 36)
		farbeGrau = walk.RGB(97, 97, 97)
		farbeDunkelgrau = walk.RGB(160, 160, 160)
		farbeRot = walk.RGB(15, 108, 189) // Akzent: Blau
		farbeRotDunkel = walk.RGB(17, 94, 163)
		farbeRotHell = walk.RGB(17, 94, 163)
		farbeAusGrund = walk.RGB(240, 240, 240)
		farbeAkzentFlaeche = walk.RGB(235, 243, 252)
		farbeAkzentHover = walk.RGB(207, 228, 250)
		farbeRahmenText = walk.RGB(15, 108, 189)
		farbeKnopfText = walk.RGB(255, 255, 255)
		farbeZeileHover = walk.RGB(245, 245, 245)
		farbeHalo = walk.RGB(207, 228, 250)
		farbeReglerKnopf = walk.RGB(15, 108, 189)
		farbeReglerRand = walk.RGB(255, 255, 255)
		farbeSchalterAus = walk.RGB(255, 255, 255)
		farbeSchalterKnopf = walk.RGB(255, 255, 255)
		farbeMarkeGrund = walk.RGB(66, 66, 66)
		farbeAbschnitt = farbeText
		schriftFamilien = map[string]schriftSpez{
			"gross": {"Segoe UI Semibold", 24, 0}, "kopf": {"Segoe UI Semibold", 14, 0},
			"titel": {"Segoe UI Semibold", 11, 0}, "abschnitt": {"Segoe UI Semibold", 10, 0},
			"label": {"Segoe UI", 9, 0}, "wert": {"Segoe UI Semibold", 9, 0}, "knopf": {"Segoe UI Semibold", 9, 0},
			"text": {"Segoe UI", 9, 0}, "klein": {"Segoe UI", 8, 0},
		}
	}
	// Schrift für den Logo-Schriftzug „-Vertical“
	logoFamilie := schriftFamilien["gross"].familie
	if designName == "office" {
		logoFamilie = "Segoe UI"
	}
	schriftFamilien["logo"] = schriftSpez{logoFamilie, 34, walk.FontBold}
	schriftFamilien["logoklein"] = schriftSpez{logoFamilie, 15, walk.FontBold}
	// Schriften neu anlegen
	for k, f := range schriftVorrat {
		f.Dispose()
		delete(schriftVorrat, k)
	}
	for k, h := range gdiSchriften {
		win.DeleteObject(win.HGDIOBJ(h))
		delete(gdiSchriften, k)
	}
	for k := range schriftArt {
		delete(schriftArt, k)
	}
}

// gt: Beschriftung je nach Design in Großbuchstaben.
func gt(s string) string {
	if designGross {
		return strings.ToUpper(s)
	}
	return s
}

// ------------------------------------------------------------------ Zeichenhilfen

func (p *programm) flaeche(c *walk.Canvas, r walk.Rectangle, grund walk.Color, k float64) {
	if designRund > 0 {
		d := int(float64(2*designRund)*k + 0.5)
		c.FillRoundedRectanglePixels(pinselVorrat.pinsel(grund), r, walk.Size{Width: d, Height: d})
		return
	}
	c.FillRectanglePixels(pinselVorrat.pinsel(grund), r)
}

func (p *programm) rahmen(c *walk.Canvas, r walk.Rectangle, farbe walk.Color, k float64) {
	if designRund > 0 {
		d := int(float64(2*designRund)*k + 0.5)
		c.DrawRoundedRectanglePixels(stift(farbe, 1), r, walk.Size{Width: d, Height: d})
		return
	}
	c.DrawRectanglePixels(stift(farbe, 1), r)
}

// ------------------------------------------------------------------ Einstellungen

func einstellungsDatei() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "Vertical3", "einstellungen.json")
}

type einstellungen struct {
	Design            string    `json:"design"`
	Groesse           string    `json:"ausgabegroesse,omitempty"`     // Name der gewählten Größe
	NichtVergroessern *bool     `json:"nicht_vergroessern,omitempty"` // Standard: ja
	EigeneGroessen    []groesse `json:"eigene_groessen,omitempty"`
	YawAchse          bool      `json:"yaw_achse,omitempty"` // Yaw-Regler und „Automatisch frontal“ (Standard: aus)
}

func ladeEinstellungen() einstellungen {
	e := einstellungen{Design: "office"}
	if f := einstellungsDatei(); f != "" {
		if b, err := os.ReadFile(f); err == nil {
			json.Unmarshal(b, &e)
		}
	}
	return e
}

func speichereEinstellungen(e einstellungen) {
	f := einstellungsDatei()
	if f == "" {
		return
	}
	os.MkdirAll(filepath.Dir(f), 0o755)
	b, _ := json.MarshalIndent(e, "", "  ")
	os.WriteFile(f, b, 0o644)
}

// waehleDesign schaltet um, speichert die Wahl und zeichnet alles neu.
func (p *programm) waehleDesign(name string) {
	setzeDesign(name)
	e := ladeEinstellungen()
	e.Design = designName
	speichereEinstellungen(e)
	p.designMenue()
	titelleiste(p.mw.Handle(), designDunkel)
	p.leereCache("")
	p.panel.Invalidate()
	p.cw.Invalidate()
}

func (p *programm) designMenue() {
	if p.actDesignOffice != nil {
		p.actDesignOffice.SetChecked(designName == "office")
		p.actDesignKamera.SetChecked(designName == "kamera")
		p.actDesignAnalog.SetChecked(designName == "analog")
	}
}

// titelleiste: dunkle oder helle Fenster-Titelleiste (Windows 10 ab 20H1 / 11).
func titelleiste(hwnd win.HWND, dunkel bool) {
	dwm := syscall.NewLazyDLL("dwmapi.dll")
	proc := dwm.NewProc("DwmSetWindowAttribute")
	if proc.Find() != nil {
		return
	}
	an := int32(0)
	if dunkel {
		an = 1
	}
	for _, attr := range []uintptr{20, 19} { // DWMWA_USE_IMMERSIVE_DARK_MODE (neu/alt)
		if r, _, _ := proc.Call(uintptr(hwnd), attr, uintptr(unsafe.Pointer(&an)), 4); r == 0 {
			break
		}
	}
	farbe := uint32(0xFFFFFFFF) // DWMWA_COLOR_DEFAULT
	if dunkel {
		farbe = 0x00141414
	}
	proc.Call(uintptr(hwnd), 35, uintptr(unsafe.Pointer(&farbe)), 4) // Titelfarbe (Windows 11)
	proc.Call(uintptr(hwnd), 34, uintptr(unsafe.Pointer(&farbe)), 4) // Rahmenfarbe (Windows 11)
}
