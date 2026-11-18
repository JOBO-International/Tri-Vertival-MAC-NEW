package main

// Drehen (90°-Schritte) und Spiegeln des geöffneten Bildes. Gesetzte
// Bezugsmarken und die Korrekturwerte werden passend umgerechnet.

import (
	"fmt"
	"math"
	"path/filepath"

	"vertical3/kern"
)

var drehNamen = map[string]string{
	kern.DrehLinks:   "Um 90° nach links gedreht",
	kern.DrehRechts:  "Um 90° nach rechts gedreht",
	kern.SpiegelWaag: "Waagerecht gespiegelt",
	kern.SpiegelSenk: "Senkrecht gespiegelt",
}

func einheitsMatrix() kern.Mat3 { return kern.Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} }

func (p *programm) drehen(schritt string) {
	if p.geladen == nil || p.beschaeftigt {
		return
	}
	g := p.geladen
	w, h := g.Bild.W, g.Bild.H
	g.Bild = kern.Ausrichten(g.Bild, []string{schritt})
	p.vorschau = kern.Ausrichten(p.vorschau, []string{schritt})
	p.yawGrenzeAnpassen() // Breite und Höhe haben sich ggf. vertauscht

	// Marken mitdrehen
	for i := range p.marken {
		for j := range p.marken[i].P {
			p.marken[i].P[j] = kern.AusrichtenPunkt(p.marken[i].P[j], w, h, schritt)
		}
	}
	for j := range p.punkte {
		p.punkte[j] = kern.AusrichtenPunkt(p.punkte[j], w, h, schritt)
	}
	p.autoPunkte = nil
	Q := kern.AusrichtungsMatrix(schritt)
	p.ausrichtung = append(p.ausrichtung, schritt)
	p.ausrichtQ = Q.Mul(p.ausrichtQ)

	// Korrektur umrechnen, damit sie dasselbe bewirkt
	r, pi, y := kern.WinkelNachAusrichtung(p.roll.wert, p.pitch.wert, p.yaw.wert, Q)
	p.nachher = nil
	p.leereCache("")
	p.zoom, p.zx, p.zy = 1, 0.5, 0.5
	p.aus.zuruecksetzen() // Ausschnittrahmen folgt dem neuen Format
	p.setzeWert(p.roll, math.Round(r*10)/10, nil)
	p.setzeWert(p.pitch, math.Round(pi*10)/10, nil)
	p.setzeWert(p.yaw, math.Round(y*10)/10, nil)
	if len(p.marken) > 0 {
		p.markenLoesen()
	}
	p.txtDatei = fmt.Sprintf("%s  ·  %d × %d", filepath.Base(p.pfad), g.Bild.W, g.Bild.H)
	p.setzeStatus(drehNamen[schritt] + ".")
	p.planeVorschau()
	p.panel.Invalidate()
	p.cw.Invalidate()
}

// ausrichtungZuruecksetzen: neues Bild.
func (p *programm) ausrichtungZuruecksetzen() {
	p.aus.zuruecksetzen()
	p.autoPunkte = nil
	p.ausrichtung = nil
	p.ausrichtQ = einheitsMatrix()
}
