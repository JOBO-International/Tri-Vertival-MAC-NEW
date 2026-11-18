package main

// Zusatzprogramme direkt aus Vertical3 entfernen – unabhängig vom
// Windows-Deinstallationsprogramm des Zusatzes (falls dieses fehlt,
// blockiert wird oder aus einer alten Version stammt).

import (
	enc "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows/registry"
)

const deinstPfad = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

type installierterZusatz struct {
	schluessel string // z. B. "Vertical3-Drucken"
	anzeige    string // Anzeigename
	version    string
	deinst     string // Pfad des Deinstallationsprogramms
}

func (z installierterZusatz) kurz() string {
	// "Vertical3-Filter" (bisherige Setups) oder "Vertical3-Filter_is1" (Inno-Setup-Setups)
	return strings.TrimSuffix(strings.TrimPrefix(z.schluessel, "Vertical3-"), "_is1")
}

// installierteZusaetze: alle Zusatzprogramme, die in Windows eingetragen sind,
// dazu solche, die nur als zusatz\*.json vorhanden sind.
func (p *programm) installierteZusaetze() []installierterZusatz {
	var out []installierterZusatz
	gesehen := map[string]bool{}
	if k, err := registry.OpenKey(registry.CURRENT_USER, deinstPfad, registry.ENUMERATE_SUB_KEYS); err == nil {
		namen, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range namen {
			if !strings.HasPrefix(strings.ToLower(n), "vertical3-") {
				continue
			}
			z := installierterZusatz{schluessel: n}
			if gesehen[strings.ToLower(z.kurz())] {
				continue
			}
			if s, err := registry.OpenKey(registry.CURRENT_USER, deinstPfad+`\`+n, registry.QUERY_VALUE); err == nil {
				z.anzeige, _, _ = s.GetStringValue("DisplayName")
				z.version, _, _ = s.GetStringValue("DisplayVersion")
				z.deinst, _, _ = s.GetStringValue("UninstallString")
				s.Close()
			}
			z.deinst = strings.Trim(z.deinst, `" `)
			if z.anzeige == "" {
				z.anzeige = "Vertical3 " + z.kurz()
			}
			out = append(out, z)
			gesehen[strings.ToLower(z.kurz())] = true
		}
	}
	// nur als Beschreibung vorhanden (ohne Windows-Eintrag)
	dateien, _ := filepath.Glob(filepath.Join(p.appDir, "zusatz", "*.json"))
	for _, f := range dateien {
		basis := strings.TrimSuffix(filepath.Base(f), ".json")
		if gesehen[strings.ToLower(basis)] {
			continue
		}
		name := basis
		for _, z := range p.zusaetze {
			if strings.EqualFold(strings.TrimSuffix(filepath.Base(z.pfad), ".exe"), "Vertical3-"+basis) || strings.EqualFold(z.Name, "Vertical3 "+basis) {
				name = z.Name
			}
		}
		out = append(out, installierterZusatz{schluessel: "Vertical3-" + strings.ToUpper(basis[:1]) + basis[1:], anzeige: name + " (nur Dateien)"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].anzeige < out[j].anzeige })
	return out
}

// entferneZusatz löscht Dateien, Startmenü-Eintrag und Windows-Eintrag.
func (p *programm) entferneZusatz(z installierterZusatz) error {
	kurz := strings.ToLower(z.kurz()) // drucken, filter, ki, rahmen, neigung
	var fehler []string
	weg := func(pfad string) {
		if err := os.Remove(pfad); err != nil && !os.IsNotExist(err) {
			fehler = append(fehler, filepath.Base(pfad))
		}
	}
	// Programmdatei aus der Beschreibung
	json := filepath.Join(p.appDir, "zusatz", kurz+".json")
	exeNamen := map[string]string{"drucken": "Vertical3-Druck.exe", "filter": "Vertical3-Filter.exe", "ki": "Vertical3-KI.exe",
		"rahmen": "Vertical3-Rahmen.exe", "neigung": "Vertical3-Neigung.exe"}
	exe := exeNamen[kurz]
	if exe == "" { // unbekannter Zusatz: Programmname aus der Beschreibung
		if b, err := os.ReadFile(json); err == nil {
			var zz zusatz
			if jsonLesen(b, &zz) == nil && filepath.Base(zz.Exe) == zz.Exe {
				exe = zz.Exe
			}
		}
	}
	if exe != "" {
		weg(filepath.Join(p.appDir, exe))
	}
	weg(json)
	// Kurzanleitungen
	liesmich, _ := filepath.Glob(filepath.Join(p.appDir, "LIESMICH-*.txt"))
	for _, l := range liesmich {
		b := strings.ToLower(filepath.Base(l))
		if strings.HasPrefix(b, "liesmich-"+kurz) || (kurz == "neigung" && strings.HasPrefix(b, "liesmich-kameraneigung")) {
			weg(l)
		}
	}
	if kurz == "ki" {
		os.RemoveAll(filepath.Join(p.appDir, "ki"))
		if d, err := os.UserCacheDir(); err == nil { // %LOCALAPPDATA%\Vertical3\ki (heruntergeladenes Modell)
			os.RemoveAll(filepath.Join(d, "Vertical3", "ki"))
		}
	}
	// Startmenü
	if d, err := os.UserConfigDir(); err == nil {
		sm := filepath.Join(d, "Microsoft", "Windows", "Start Menu", "Programs", "Vertical3")
		links, _ := filepath.Glob(filepath.Join(sm, "*.lnk"))
		name := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(z.anzeige, "(Zusatzprogramm)")))
		for _, l := range links {
			ln := strings.ToLower(strings.TrimSuffix(filepath.Base(l), ".lnk"))
			if ln == name || ln == strings.Replace(name, "tri-vertical", "vertical3", 1) || (kurz == "drucken" && ln == "vertical3 drucken") || (kurz == "ki" && strings.HasPrefix(ln, "vertical3 ki")) {
				os.Remove(l)
			}
		}
	}
	// Deinstallationsprogramm
	if z.deinst != "" {
		weg(z.deinst)
	}
	weg(filepath.Join(p.appDir, "zusatz", "Vertical3-"+z.kurz()+"-deinstallieren.exe"))
	// Inno-Setup: eigener Deinstallationsordner zusatz\<kurz>-deinst
	inno := filepath.Join(p.appDir, "zusatz", kurz+"-deinst")
	if _, err := os.Stat(inno); err == nil {
		if err := os.RemoveAll(inno); err != nil {
			fehler = append(fehler, filepath.Base(inno))
		}
	}
	if len(fehler) > 0 {
		return fmt.Errorf("Diese Dateien sind noch in Benutzung und konnten nicht gelöscht werden:\n%s\n\nBitte das Zusatzprogramm schließen und noch einmal versuchen.",
			strings.Join(fehler, "\n"))
	}
	// Windows-Eintrag zuletzt – erst wenn alles weg ist
	for _, k := range []string{"Vertical3-" + z.kurz(), "Vertical3-" + z.kurz() + "_is1"} {
		if err := registry.DeleteKey(registry.CURRENT_USER, deinstPfad+`\`+k); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("Der Eintrag in den Windows-Einstellungen konnte nicht entfernt werden: %v", err)
		}
	}
	return nil
}

// zusaetzeAnzeigen: Übersicht unter Hilfe, mit Entfernen.
func (p *programm) zusaetzeAnzeigen() {
	var dlg *walk.Dialog
	var liste *walk.ListBox
	var entf *walk.PushButton
	var schliessen *walk.PushButton
	zs := p.installierteZusaetze()
	texte := func() []string {
		var t []string
		for _, z := range zs {
			s := z.anzeige
			if z.version != "" {
				s += "  –  Version " + z.version
			}
			t = append(t, s)
		}
		if len(t) == 0 {
			t = []string{"– keine Zusatzprogramme installiert –"}
		}
		return t
	}
	geaendert := false
	err := Dialog{
		AssignTo: &dlg, Title: "Zusatzprogramme", CancelButton: &schliessen, DefaultButton: &schliessen,
		MinSize: Size{Width: 520, Height: 340}, Layout: VBox{},
		Children: []Widget{
			Label{Text: "Installierte Zusatzprogramme. Zusatzprogramme werden mit ihrem eigenen Setup installiert\nund erscheinen danach im Menü Datei und im Bedienfeld."},
			ListBox{AssignTo: &liste, Model: texte(), OnCurrentIndexChanged: func() {
				if entf != nil {
					entf.SetEnabled(liste.CurrentIndex() >= 0 && len(zs) > 0)
				}
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &entf, Text: "Ausgewähltes Zusatzprogramm entfernen …", Enabled: false, OnClicked: func() {
					i := liste.CurrentIndex()
					if i < 0 || i >= len(zs) {
						return
					}
					z := zs[i]
					if walk.MsgBox(dlg, "Zusatzprogramm entfernen", z.anzeige+" wirklich entfernen?\n\nDas Hauptprogramm Vertical3 und deine Bilder bleiben unverändert.",
						walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != 6 { // IDYES
						return
					}
					if err := p.entferneZusatz(z); err != nil {
						walk.MsgBox(dlg, "Zusatzprogramm entfernen", err.Error(), walk.MsgBoxIconWarning)
						return
					}
					geaendert = true
					zs = p.installierteZusaetze()
					liste.SetModel(texte())
					entf.SetEnabled(false)
				}},
				HSpacer{},
				PushButton{AssignTo: &schliessen, Text: "Schließen", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(p.mw)
	if err != nil {
		return
	}
	dlg.Run()
	if geaendert {
		p.zusaetze = ladeZusaetze(p.appDir)
		p.panel.Invalidate()
		walk.MsgBox(p.mw, "Zusatzprogramme", "Entfernt. Bitte Vertical3 neu starten, damit auch die Menüeinträge verschwinden.", walk.MsgBoxIconInformation)
	}
	p.cw.SetFocus()
}

func jsonLesen(b []byte, v any) error { return enc.Unmarshal(entferneBOMText(b), v) }
