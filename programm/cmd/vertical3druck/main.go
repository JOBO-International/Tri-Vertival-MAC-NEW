// Vertical3 Drucken – Zusatzprogramm zu Vertical3.
//
// Aufruf durch Vertical3:  Vertical3-Druck.exe <auftrag.json>
//
//	{"bild": "...", "roll": 0, "pitch": 0, "yaw": 0, "f35": 28, "zuschneiden": true,
//	 "rand": [255,255,255], "exiftool": "..."}
//
// Ohne Auftrag fragt das Programm nach einem Bild (dann ohne Perspektivkorrektur).
package main

import (
	"encoding/json"
	"math"
	"os"
	"strings"

	"github.com/lxn/walk"

	"vertical3/bild"
	"vertical3/kern"
)

const version = "2.2"

// Auftrag vom Hauptprogramm
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

// programm hält das Foto und die Korrektur – dieselben Namen wie im Hauptprogramm.
type programm struct {
	pfad         string
	geladen      *bild.Geladen
	nachher      *kern.Bild
	par          kern.Parameter
	rand         [3]uint8
	mw           walk.Form
	beschaeftigt bool
}

func (p *programm) parameter() kern.Parameter { return p.par }
func (p *programm) randFarbe() [3]uint8       { return p.rand }
func (p *programm) setzeStatus(string)        {}

type farbkasten map[walk.Color]*walk.SolidColorBrush

var pinselVorrat = farbkasten{}

func (f farbkasten) pinsel(c walk.Color) *walk.SolidColorBrush {
	if b, ok := f[c]; ok {
		return b
	}
	b, _ := walk.NewSolidColorBrush(c)
	f[c] = b
	return b
}

func fehler(text string) {
	walk.MsgBox(nil, "Tri-Vertical Drucken", text, walk.MsgBoxIconError)
}

func main() {
	var a auftragDatei
	a.F35, a.Zuschneiden, a.Rand = 28, true, [3]uint8{255, 255, 255}
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if strings.EqualFold(strings.TrimSpace(arg[max(0, len(arg)-5):]), ".json") {
			b, err := os.ReadFile(arg)
			if err != nil || json.Unmarshal(b, &a) != nil {
				fehler("Der Druckauftrag von Tri-Vertical konnte nicht gelesen werden.")
				return
			}
			os.Remove(arg) // Auftragsdatei wird nicht mehr gebraucht
		} else {
			a.Bild = arg // direkt mit einem Bild gestartet
		}
	}
	if a.Bild == "" {
		dlg := walk.FileDialog{Title: "Tri-Vertical Drucken – Bild wählen",
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
	p := &programm{pfad: a.Bild, geladen: g, rand: a.Rand,
		par: kern.Parameter{Roll: a.Roll, Pitch: a.Pitch, Yaw: a.Yaw, YawMitte: a.YawMitte, F35: math.Max(5, a.F35), Zuschneiden: a.Zuschneiden}}
	klein := g.Bild.Verkleinert(1600)
	p.nachher, err = kern.Korrigiere(klein, p.par, p.rand, true)
	if err != nil { // Korrektur nicht möglich → unkorrigiert drucken
		p.par = kern.Parameter{F35: p.par.F35, Zuschneiden: p.par.Zuschneiden}
		p.nachher = klein
	}
	p.drucken()
}
