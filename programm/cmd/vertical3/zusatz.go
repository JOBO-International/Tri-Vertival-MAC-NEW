package main

// Zusatzprogramme: werden mit eigenem Setup in den Programmordner installiert
// und melden sich über eine Beschreibung in "zusatz\<name>.json" an:
//
//	{"name": "Drucken", "exe": "Vertical3-Druck.exe", "version": "1.0",
//	 "menuetext": "Drucken …", "taste": "Strg+P", "knopf": "Drucken   ·   Strg+P",
//	 "beschreibung": "Drucken mit Formaten und Rahmen"}
//
// Vertical3 zeigt Menüeintrag und Knopf nur, wenn der Zusatz installiert ist,
// und übergibt beim Aufruf Foto und Korrektur als Auftragsdatei.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"vertical3/bild"
)

type zusatz struct {
	Name         string `json:"name"`
	Exe          string `json:"exe"`
	Version      string `json:"version"`
	Menuetext    string `json:"menuetext"`
	Taste        string `json:"taste"`
	Knopf        string `json:"knopf"`
	Beschreibung string `json:"beschreibung"`
	// Rolle "startwerte": liefert beim Öffnen eines Bildes Startwerte für die
	// Regler (Aufruf: exe --startwerte <eingabe.json> <ausgabe.json>).
	Rolle    string `json:"rolle"`
	OhneBild bool   `json:"ohne_bild"` // darf ohne geöffnetes Bild starten
	pfad     string
}

func ladeZusaetze(appDir string) []*zusatz {
	var out []*zusatz
	dateien, _ := filepath.Glob(filepath.Join(appDir, "zusatz", "*.json"))
	sort.Strings(dateien)
	for _, f := range dateien {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var z zusatz
		if json.Unmarshal(entferneBOMText(b), &z) != nil || z.Exe == "" || z.Name == "" {
			continue
		}
		// nur Programme im Programmordner, keine Pfadangaben
		if filepath.Base(z.Exe) != z.Exe || !strings.EqualFold(filepath.Ext(z.Exe), ".exe") {
			continue
		}
		z.pfad = filepath.Join(appDir, z.Exe)
		if _, err := os.Stat(z.pfad); err != nil {
			continue
		}
		if z.Menuetext == "" {
			z.Menuetext = z.Name + " …"
		}
		out = append(out, &z)
	}
	return out
}

func entferneBOMText(b []byte) []byte { return []byte(strings.TrimPrefix(string(b), "\ufeff")) }

// tastenkuerzel: "Strg+P" → Shortcut
func tastenkuerzel(t string) (Shortcut, bool) {
	t = strings.ToUpper(strings.ReplaceAll(t, " ", ""))
	var sc Shortcut
	for _, teil := range strings.Split(t, "+") {
		switch teil {
		case "STRG", "CTRL":
			sc.Modifiers |= walk.ModControl
		case "UMSCHALT", "SHIFT":
			sc.Modifiers |= walk.ModShift
		case "ALT":
			sc.Modifiers |= walk.ModAlt
		default:
			if len(teil) == 1 && teil[0] >= 'A' && teil[0] <= 'Z' {
				sc.Key = walk.Key(teil[0])
			} else if len(teil) == 1 && teil[0] >= '0' && teil[0] <= '9' {
				sc.Key = walk.Key(teil[0])
			} else {
				return sc, false
			}
		}
	}
	return sc, sc.Key != 0 && sc.Modifiers != 0
}

// zusatzMenue: Einträge für das Menü Datei.
func (p *programm) zusatzMenue() []MenuItem {
	var items []MenuItem
	for _, z := range p.zusaetze {
		zz := z
		a := Action{Text: z.Menuetext, OnTriggered: func() { p.starteZusatz(zz) }}
		if sc, ok := tastenkuerzel(z.Taste); ok {
			a.Shortcut = sc
		}
		items = append(items, a)
	}
	return items
}

// starteZusatz übergibt das aktuelle Foto samt Korrektur an das Zusatzprogramm.
func (p *programm) starteZusatz(z *zusatz) {
	if p.geladen == nil && !z.OhneBild {
		walk.MsgBox(p.mw, z.Name, "Bitte zuerst ein Bild öffnen.", walk.MsgBoxIconInformation)
		return
	}
	par := p.parameter()
	bildPfad := p.pfad
	if p.geladen == nil {
		bildPfad = ""
	}
	auftrag := map[string]any{
		"bild": bildPfad, "profile": p.profilDir, "roll": par.Roll, "pitch": par.Pitch, "yaw": par.Yaw, "yaw_mitte": par.YawMitte, "f35": par.F35,
		"zuschneiden": par.Zuschneiden, "rand": p.randFarbe(), "exiftool": p.exiftool,
		"design": designName, "vertical3": p.version,
		"ausrichtung": p.ausrichtung,
		"ausgabe":     map[string]any{"art": p.ausgabe.Art, "a": p.ausgabe.A, "b": p.ausgabe.B, "name": p.ausgabe.Name}, "nicht_vergroessern": p.nichtVergroessern,
	}
	b, _ := json.MarshalIndent(auftrag, "", "  ")
	datei := filepath.Join(os.TempDir(), fmt.Sprintf("vertical3_auftrag_%d.json", time.Now().UnixNano()))
	if err := os.WriteFile(datei, b, 0o644); err != nil {
		walk.MsgBox(p.mw, z.Name, "Auftrag konnte nicht übergeben werden:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	cmd := exec.Command(z.pfad, datei)
	cmd.Dir = p.appDir
	if err := cmd.Start(); err != nil {
		os.Remove(datei)
		walk.MsgBox(p.mw, z.Name, "Das Zusatzprogramm konnte nicht gestartet werden:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	p.setzeStatus(z.Name + " geöffnet.")
}

// neigungsZusatz: installiertes Zusatzprogramm, das Startwerte liefert.
func (p *programm) neigungsZusatz() *zusatz {
	for _, z := range p.zusaetze {
		if z.Rolle == "startwerte" {
			return z
		}
	}
	return nil
}

type startwerte struct {
	OK      bool    `json:"ok"`
	Roll    float64 `json:"roll"`
	Pitch   float64 `json:"pitch"`
	Hinweis string  `json:"hinweis"`
	Text    string  `json:"text"`
}

// neigungAbfragen fragt das Zusatzprogramm nach Startwerten für das Bild.
// Das Hauptprogramm selbst wertet keine Neigungsdaten aus.
func (p *programm) neigungAbfragen(pfad string, g *bild.Geladen) {
	z := p.neigungsZusatz()
	if z == nil {
		return
	}
	ein := map[string]any{"bild": pfad, "daten": g.Meta, "profile": p.profilDir, "exiftool": p.exiftool}
	b, _ := json.Marshal(ein)
	basis := filepath.Join(os.TempDir(), fmt.Sprintf("vertical3_start_%d", time.Now().UnixNano()))
	eingabe, ausgabe := basis+"_ein.json", basis+"_aus.json"
	if os.WriteFile(eingabe, b, 0o600) != nil {
		return
	}
	go func() {
		defer os.Remove(eingabe)
		defer os.Remove(ausgabe)
		cmd := exec.Command(z.pfad, "--startwerte", eingabe, ausgabe)
		cmd.Dir = p.appDir
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if cmd.Run() != nil {
			return
		}
		raus, err := os.ReadFile(ausgabe)
		var s startwerte
		if err != nil || json.Unmarshal(raus, &s) != nil {
			return
		}
		p.mw.Synchronize(func() {
			if p.pfad != pfad || p.geladen != g {
				return // inzwischen anderes Bild
			}
			p.hatDaten, p.autoRoll, p.autoPitch, p.kameraAktiv = s.OK, s.Roll, s.Pitch, s.OK
			if s.Text != "" {
				p.txtKamera = p.txtKameraBasis + "\n" + s.Text
			}
			// nur übernehmen, solange noch nichts von Hand eingestellt ist
			if s.OK && len(p.marken) == 0 && p.roll.wert == 0 && p.pitch.wert == 0 {
				p.kameradatenUebernehmen()
				p.setzeStatus(fmt.Sprintf("Kameraneigung übernommen: Roll %+.1f°, Pitch %+.1f°.", s.Roll, s.Pitch))
			}
			p.panel.Invalidate()
		})
	}()
}
