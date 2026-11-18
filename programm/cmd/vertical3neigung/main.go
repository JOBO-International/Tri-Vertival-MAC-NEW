// Vertical3 Kameraneigung – Zusatzprogramm zu Vertical3.
//
// Liest die Neigungswinkel (Roll, Pitch), die Kameras wie iPhone, Leica
// M11/Q3/SL3/D-Lux 8 oder Lumix L10 in der Bilddatei hinterlegen, und gibt
// sie Vertical3 als Startwerte. Außerdem: Stapelverarbeitung eines Ordners.
//
// Aufrufe:
//
//	Vertical3-Neigung.exe --startwerte <eingabe.json> <ausgabe.json>   (durch Vertical3 beim Öffnen eines Bildes)
//	Vertical3-Neigung.exe <auftrag.json>                               (Stapelverarbeitung aus Vertical3)
//	Vertical3-Neigung.exe                                              (Stapelverarbeitung, eigenständig)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"vertical3/bild"
	"vertical3/kern"
	"vertical3/meta"
	"vertical3/neigung"
	"vertical3/profil"
)

const version = "1.3"
const titel = "Tri-Vertical Kameraneigung"

type eingabe struct {
	Bild     string            `json:"bild"`
	Daten    map[string]string `json:"daten"`
	Profile  string            `json:"profile"`
	Exiftool string            `json:"exiftool"`
}

type ausgabe struct {
	OK      bool    `json:"ok"`
	Roll    float64 `json:"roll"`
	Pitch   float64 `json:"pitch"`
	Hinweis string  `json:"hinweis"`
	Text    string  `json:"text"`
}

type auftragDatei struct {
	Bild        string   `json:"bild"`
	Yaw         float64  `json:"yaw"`
	YawMitte    bool     `json:"yaw_mitte"` // Yaw um die Bildmitte (sonst um die Unterkante)
	Zuschneiden bool     `json:"zuschneiden"`
	Rand        [3]uint8 `json:"rand"`
	Exiftool    string   `json:"exiftool"`
	Profile     string   `json:"profile"`
	Ausgabe     struct {
		Art  string  `json:"art"`
		A    float64 `json:"a"`
		B    float64 `json:"b"`
		Name string  `json:"name"`
	} `json:"ausgabe"`
	NichtVergroessern bool `json:"nicht_vergroessern"`
}

func programmOrdner() string {
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

func profilOrdner(vorgabe string) string {
	if vorgabe != "" {
		return vorgabe
	}
	return filepath.Join(programmOrdner(), "kameraprofile")
}

func main() {
	if len(os.Args) >= 4 && os.Args[1] == "--startwerte" {
		os.Exit(startwerte(os.Args[2], os.Args[3]))
	}
	a := auftragDatei{Zuschneiden: true, Rand: [3]uint8{255, 255, 255}}
	if len(os.Args) > 1 && strings.HasSuffix(strings.ToLower(os.Args[1]), ".json") {
		if b, err := os.ReadFile(os.Args[1]); err == nil {
			json.Unmarshal(b, &a)
			os.Remove(os.Args[1])
		}
	}
	if a.Exiftool == "" {
		a.Exiftool = meta.FindeExiftool(programmOrdner())
	}
	stapel(a)
}

// startwerte: Neigung eines Bildes für Vertical3.
func startwerte(ein, aus string) int {
	b, err := os.ReadFile(ein)
	if err != nil {
		return 1
	}
	var e eingabe
	if json.Unmarshal(b, &e) != nil {
		return 1
	}
	profile := profil.Lade(profilOrdner(e.Profile))
	if len(profile) == 0 {
		return 1
	}
	d := meta.Daten(e.Daten)
	pr := profil.Passend(d, profile)
	r, pi, ok, hinweis := neigung.Lies(d, pr)
	o := ausgabe{OK: ok, Roll: r, Pitch: pi, Hinweis: hinweis}
	o.Text = "Keine Neigungsdaten im Bild – automatisch ausrichten (Taste A)."
	if ok {
		o.Text = fmt.Sprintf("Neigungsdaten: Roll %+.1f°, Pitch %+.1f°", r, pi)
	}
	if !ok && e.Exiftool == "" {
		if h := pr.Text("hinweis_ohne_exiftool"); h != "" {
			hinweis = h
		}
	}
	if hinweis != "" {
		o.Text = hinweis + "\n" + o.Text
	}
	raus, _ := json.Marshal(o)
	if os.WriteFile(aus, raus, 0o600) != nil {
		return 1
	}
	return 0
}

func kuerze(l []string, n int) []string {
	if len(l) > n {
		return append(l[:n:n], "…")
	}
	return l
}

// stapel: alle Bilder eines Ordners mit ihren eigenen Neigungsdaten korrigieren.
func stapel(a auftragDatei) {
	var mw *walk.MainWindow
	var info *walk.Label
	var balken *walk.ProgressBar
	var knopf *walk.PushButton
	laeuft := false

	start := func() {
		if laeuft {
			return
		}
		dlg := walk.FileDialog{Title: "Ordner mit Bildern wählen"}
		if ok, _ := dlg.ShowBrowseFolder(mw); !ok {
			return
		}
		quelle := dlg.FilePath
		eintraege, _ := os.ReadDir(quelle)
		var dateien []string
		for _, e := range eintraege {
			if !e.IsDir() && bild.IstBild(e.Name()) {
				dateien = append(dateien, filepath.Join(quelle, e.Name()))
			}
		}
		sort.Strings(dateien)
		if len(dateien) == 0 {
			walk.MsgBox(mw, titel, "In diesem Ordner sind keine Bilder.", walk.MsgBoxIconInformation)
			return
		}
		ziel := filepath.Join(quelle, "korrigiert")
		ja := map[bool]string{true: "ja", false: "nein"}[a.Zuschneiden]
		gr := a.Ausgabe.Name
		if gr == "" || a.Ausgabe.Art == "original" {
			gr = "Original"
		}
		frage := fmt.Sprintf("%d Bilder gefunden.\n\nJedes Bild wird mit seinen eigenen Kamera-Neigungsdaten korrigiert.\n"+
			"Bilder ohne Neigungsdaten werden übersprungen.\n\nYaw %+.1f°, Zuschneiden: %s, Größe: %s.\n\nZiel: %s\n\nStarten?",
			len(dateien), a.Yaw, ja, gr, ziel)
		if walk.MsgBox(mw, titel, frage, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
			return
		}
		profile := profil.Lade(profilOrdner(a.Profile))
		if len(profile) == 0 {
			walk.MsgBox(mw, titel, "Keine Kameraprofile gefunden.", walk.MsgBoxIconError)
			return
		}
		os.MkdirAll(ziel, 0o755)
		laeuft = true
		knopf.SetEnabled(false)
		balken.SetRange(0, len(dateien))
		balken.SetValue(0)
		go func() {
			fertig := 0
			var uebersprungen, fehler []string
			for i, f := range dateien {
				name := filepath.Base(f)
				n := i
				mw.Synchronize(func() {
					info.SetText(fmt.Sprintf("%d von %d: %s", n+1, len(dateien), name))
					balken.SetValue(n)
				})
				g, err := bild.Lade(f, a.Exiftool)
				if err != nil {
					fehler = append(fehler, name+": "+err.Error())
					continue
				}
				pr := profil.Passend(g.Meta, profile)
				r, pi, ok, _ := neigung.Lies(g.Meta, pr)
				if !ok {
					uebersprungen = append(uebersprungen, name)
					continue
				}
				erg, err := kern.Korrigiere(g.Bild, kern.Parameter{Roll: r, Pitch: pi, Yaw: a.Yaw, YawMitte: a.YawMitte,
					F35: profil.KBBrennweite(g.Meta, pr), Zuschneiden: a.Zuschneiden}, a.Rand, false)
				if err == nil && a.Ausgabe.Art != "" && a.Ausgabe.Art != "original" {
					erg = erg.Skaliert(kern.Ausgabemass(a.Ausgabe.Art, a.Ausgabe.A, a.Ausgabe.B, erg.W, erg.H, a.NichtVergroessern))
				}
				if err == nil {
					err = bild.Speichere(erg, bild.Zielname(f, ziel), g.Exif, g.ICC)
				}
				if err != nil {
					fehler = append(fehler, name+": "+err.Error())
					continue
				}
				fertig++
			}
			mw.Synchronize(func() {
				laeuft = false
				knopf.SetEnabled(true)
				balken.SetValue(len(dateien))
				text := fmt.Sprintf("%d Bilder korrigiert und gespeichert in:\n%s", fertig, ziel)
				if len(uebersprungen) > 0 {
					text += fmt.Sprintf("\n\nOhne Neigungsdaten übersprungen (%d):\n%s", len(uebersprungen), strings.Join(kuerze(uebersprungen, 15), "\n"))
				}
				if len(fehler) > 0 {
					text += fmt.Sprintf("\n\nFehler (%d):\n%s", len(fehler), strings.Join(kuerze(fehler, 10), "\n"))
				}
				info.SetText(fmt.Sprintf("Fertig: %d Bilder.", fertig))
				walk.MsgBox(mw, titel, text, walk.MsgBoxIconInformation)
			})
		}()
	}

	if err := (MainWindow{
		AssignTo: &mw, Title: titel + " " + version + " – Stapelverarbeitung",
		Size: Size{Width: 520, Height: 230}, Layout: VBox{Margins: Margins{Left: 16, Top: 16, Right: 16, Bottom: 16}},
		Children: []Widget{
			Label{Text: "Korrigiert alle Bilder eines Ordners mit den Neigungsdaten, die die Kamera\nin der Datei hinterlegt hat (iPhone, Leica, Lumix …). Die Ergebnisse landen\nim Unterordner \"korrigiert\"; die Originale bleiben unverändert."},
			Label{AssignTo: &info, Text: "Bereit."},
			ProgressBar{AssignTo: &balken},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &knopf, Text: "Ordner wählen und starten …", OnClicked: start},
				PushButton{Text: "Schließen", OnClicked: func() {
					if !laeuft {
						mw.Close()
					}
				}},
			}},
		},
	}.Create()); err != nil {
		return
	}
	if ic, err := walk.NewIconFromResource("APPICON"); err == nil {
		mw.SetIcon(ic)
	}
	mw.Run()
}
