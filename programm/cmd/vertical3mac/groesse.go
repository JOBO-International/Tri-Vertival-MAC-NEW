package main

// Ausgabegröße und Ausschnitt (Rahmen mit festem Seitenverhältnis).
// Die Größenliste entspricht der Windows-Version.

import (
	"fmt"
	"math"
	"strings"

	"vertical3/kern"
)

// Arten einer Größenangabe
const (
	gArtOriginal = "original"
	gArtLang     = "lang"    // längste Seite A px
	gArtBreite   = "breite"  // Breite A px
	gArtHoehe    = "hoehe"   // Höhe A px
	gArtBox      = "box"     // passt in A × B px
	gArtProzent  = "prozent" // A %
	gArtDruck    = "druck"   // passt in A × B cm bei 300 dpi
)

type groesse struct {
	Name string  `json:"name"`
	Art  string  `json:"art"`
	A    float64 `json:"a"`
	B    float64 `json:"b,omitempty"`
}

var standardGroessen = []groesse{
	{Name: "Original (volle Auflösung)", Art: gArtOriginal},
	{Name: "4K UHD – passt in 3840 × 2160", Art: gArtBox, A: 3840, B: 2160},
	{Name: "Full HD – passt in 1920 × 1080", Art: gArtBox, A: 1920, B: 1080},
	{Name: "Längste Seite 3000 px", Art: gArtLang, A: 3000},
	{Name: "Längste Seite 2048 px", Art: gArtLang, A: 2048},
	{Name: "Längste Seite 1600 px (Web)", Art: gArtLang, A: 1600},
	{Name: "Längste Seite 1080 px (soziale Netzwerke)", Art: gArtLang, A: 1080},
	{Name: "Längste Seite 800 px (E-Mail)", Art: gArtLang, A: 800},
	{Name: "Fotopapier 9 × 13 cm", Art: gArtDruck, A: 9, B: 13},
	{Name: "Fotopapier 10 × 15 cm", Art: gArtDruck, A: 10, B: 15},
	{Name: "Fotopapier 11 × 15 cm", Art: gArtDruck, A: 11, B: 15},
	{Name: "Fotopapier 13 × 18 cm", Art: gArtDruck, A: 13, B: 18},
	{Name: "Fotopapier 13 × 19 cm", Art: gArtDruck, A: 13, B: 19},
	{Name: "Fotopapier 15 × 20 cm", Art: gArtDruck, A: 15, B: 20},
	{Name: "Fotopapier 15 × 21 cm", Art: gArtDruck, A: 15, B: 21},
	{Name: "Fotopapier 18 × 24 cm", Art: gArtDruck, A: 18, B: 24},
	{Name: "Fotopapier 18 × 27 cm", Art: gArtDruck, A: 18, B: 27},
	{Name: "Fotopapier 20 × 25 cm", Art: gArtDruck, A: 20, B: 25},
	{Name: "Fotopapier 20 × 30 cm", Art: gArtDruck, A: 20, B: 30},
	{Name: "Fotopapier 24 × 30 cm", Art: gArtDruck, A: 24, B: 30},
	{Name: "Fotopapier 24 × 36 cm", Art: gArtDruck, A: 24, B: 36},
	{Name: "Fotopapier 30 × 40 cm", Art: gArtDruck, A: 30, B: 40},
	{Name: "Fotopapier 30 × 45 cm", Art: gArtDruck, A: 30, B: 45},
	{Name: "Fotopapier 40 × 50 cm", Art: gArtDruck, A: 40, B: 50},
	{Name: "Fotopapier 40 × 60 cm", Art: gArtDruck, A: 40, B: 60},
	{Name: "Fotopapier 50 × 60 cm", Art: gArtDruck, A: 50, B: 60},
	{Name: "Fotopapier 50 × 70 cm", Art: gArtDruck, A: 50, B: 70},
	{Name: "Fotopapier 50 × 75 cm", Art: gArtDruck, A: 50, B: 75},
	{Name: "Fotopapier 60 × 80 cm", Art: gArtDruck, A: 60, B: 80},
	{Name: "Fotopapier 60 × 90 cm", Art: gArtDruck, A: 60, B: 90},
	{Name: "Fotopapier 10 × 10 cm (quadratisch)", Art: gArtDruck, A: 10, B: 10},
	{Name: "Fotopapier 13 × 13 cm (quadratisch)", Art: gArtDruck, A: 13, B: 13},
	{Name: "Fotopapier 15 × 15 cm (quadratisch)", Art: gArtDruck, A: 15, B: 15},
	{Name: "Fotopapier 20 × 20 cm (quadratisch)", Art: gArtDruck, A: 20, B: 20},
	{Name: "Fotopapier 30 × 30 cm (quadratisch)", Art: gArtDruck, A: 30, B: 30},
	{Name: "Fotopapier 10 × 30 cm (Panorama)", Art: gArtDruck, A: 10, B: 30},
	{Name: "Fotopapier 20 × 60 cm (Panorama)", Art: gArtDruck, A: 20, B: 60},
	{Name: "Fotopapier DIN A6 – 10,5 × 14,8 cm", Art: gArtDruck, A: 10.5, B: 14.8},
	{Name: "Fotopapier DIN A5 – 14,8 × 21 cm", Art: gArtDruck, A: 14.8, B: 21},
	{Name: "Fotopapier DIN A4 – 21 × 29,7 cm", Art: gArtDruck, A: 21, B: 29.7},
	{Name: "Fotopapier DIN A3 – 29,7 × 42 cm", Art: gArtDruck, A: 29.7, B: 42},
	{Name: "Fotopapier DIN A3+ – 32,9 × 48,3 cm", Art: gArtDruck, A: 32.9, B: 48.3},
	{Name: "Fotopapier DIN A2 – 42 × 59,4 cm", Art: gArtDruck, A: 42, B: 59.4},
	{Name: "Fotopapier 4 × 6 Zoll (10,2 × 15,2 cm)", Art: gArtDruck, A: 10.16, B: 15.24},
	{Name: "Fotopapier 5 × 7 Zoll (12,7 × 17,8 cm)", Art: gArtDruck, A: 12.7, B: 17.78},
	{Name: "Fotopapier 8 × 10 Zoll (20,3 × 25,4 cm)", Art: gArtDruck, A: 20.32, B: 25.4},
	{Name: "Fotopapier Letter 8,5 × 11 Zoll (21,6 × 27,9 cm)", Art: gArtDruck, A: 21.59, B: 27.94},
	{Name: "Fotopapier 11 × 14 Zoll (27,9 × 35,6 cm)", Art: gArtDruck, A: 27.94, B: 35.56},
	{Name: "Fotopapier 13 × 19 Zoll (33 × 48,3 cm)", Art: gArtDruck, A: 33.02, B: 48.26},
	{Name: "50 %", Art: gArtProzent, A: 50},
	{Name: "25 %", Art: gArtProzent, A: 25},
}

var groessenArten = []struct{ art, text string }{
	{gArtLang, "Längste Seite (px)"},
	{gArtBreite, "Breite (px)"},
	{gArtHoehe, "Höhe (px)"},
	{gArtBox, "Passt in Breite × Höhe (px)"},
	{gArtDruck, "Druckformat cm × cm (300 dpi)"},
	{gArtProzent, "Prozent"},
}

// Zielmass: Ausgabegröße für ein Bild w×h (Seitenverhältnis bleibt erhalten).
func (g groesse) Zielmass(w, h int, nichtVergroessern bool) (int, int) {
	return kern.Ausgabemass(g.Art, g.A, g.B, w, h, nichtVergroessern)
}

func (g groesse) Beschreibung() string {
	z := func(v float64) string {
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", v), "0"), ".")
	}
	switch g.Art {
	case gArtLang:
		return "längste Seite " + z(g.A) + " px"
	case gArtBreite:
		return "Breite " + z(g.A) + " px"
	case gArtHoehe:
		return "Höhe " + z(g.A) + " px"
	case gArtBox:
		return "passt in " + z(g.A) + " × " + z(g.B) + " px"
	case gArtDruck:
		return "passt auf " + z(g.A) + " × " + z(g.B) + " cm bei 300 dpi"
	case gArtProzent:
		return z(g.A) + " %"
	}
	return "volle Auflösung"
}

// ausschnittZustand: Lage des Rahmens, unabhängig von der Bildgröße.
type ausschnittZustand struct {
	cx, cy  float64 // Mitte in Bildanteilen (0..1)
	s       float64 // Größe: Anteil am größtmöglichen Rahmen
	gedreht bool    // Hoch-/Querformat gegenüber dem Bild getauscht
}

func (a *ausschnittZustand) zuruecksetzen() { *a = ausschnittZustand{cx: 0.5, cy: 0.5, s: 1} }

// seiten: Seitenverhältnis (Breite/Höhe) der Größe g für ein Bild w×h;
// ok = false, wenn die Größe kein festes Seitenverhältnis hat.
func (a ausschnittZustand) seiten(g groesse, w, h int) (float64, bool) {
	if (g.Art != gArtDruck && g.Art != gArtBox) || g.A <= 0 || g.B <= 0 || w <= 0 || h <= 0 {
		return 0, false
	}
	lang, kurz := math.Max(g.A, g.B), math.Min(g.A, g.B)
	quer := w >= h
	if a.gedreht {
		quer = !quer
	}
	if quer {
		return lang / kurz, true
	}
	return kurz / lang, true
}

// rechteck: Rahmen in Pixeln eines Bildes w×h.
func (a ausschnittZustand) rechteck(g groesse, w, h int) (x, y, rw, rh float64, ok bool) {
	r, ok := a.seiten(g, w, h)
	if !ok {
		return 0, 0, float64(w), float64(h), false
	}
	W, H := float64(w), float64(h)
	mw, _, smax := ausschnittGrenzen(r, W, H)
	s := math.Max(0.1, math.Min(smax, a.s))
	rw, rh = mw*s, mw*s/r
	x = klemme(a.cx*W-rw/2, W-rw)
	y = klemme(a.cy*H-rh/2, H-rh)
	return x, y, rw, rh, true
}

// ausschnittGrenzen: größter Rahmen ganz im Bild und Faktor smax, bei dem der
// Rahmen das ganze Foto umschließt (darüber hinaus: Rand).
func ausschnittGrenzen(r, W, H float64) (mw, mh, smax float64) {
	mw, mh = W, W/r
	if mh > H {
		mw, mh = H*r, H
	}
	aw := W
	if W/r < H {
		aw = H * r
	}
	return mw, mh, aw / mw
}

func klemme(v, spiel float64) float64 {
	return math.Max(math.Min(0, spiel), math.Min(math.Max(0, spiel), v))
}

// zielFuerAusschnitt: genaue Zielgröße in Pixeln für einen Ausschnitt cw×ch.
func zielFuerAusschnitt(g groesse, cw, ch int, nichtGroesser bool) (int, int) {
	a, b := g.A, g.B
	if g.Art == gArtDruck {
		a, b = a/2.54*300, b/2.54*300
	}
	if (cw >= ch) != (a >= b) {
		a, b = b, a
	}
	tw, th := int(math.Round(a)), int(math.Round(b))
	if nichtGroesser && (cw < tw || ch < th) {
		return cw, ch
	}
	return tw, th
}

// anwenden: beim Speichern – erst ausschneiden, dann auf die Zielgröße skalieren.
func (a ausschnittZustand) anwenden(erg *kern.Bild, g groesse, nichtGroesser bool, rand [3]uint8) *kern.Bild {
	x, y, rw, rh, ok := a.rechteck(g, erg.W, erg.H)
	if !ok {
		return erg.Skaliert(g.Zielmass(erg.W, erg.H, nichtGroesser))
	}
	c := erg.AusschnittMitRand(int(math.Round(x)), int(math.Round(y)), int(math.Round(rw)), int(math.Round(rh)), rand)
	return c.Skaliert(zielFuerAusschnitt(g, c.W, c.H, nichtGroesser))
}

// ausgabeText: Größe des gespeicherten Bildes für ein korrigiertes Bild w×h.
func (a ausschnittZustand) ausgabeText(g groesse, w, h int, nichtGroesser bool) string {
	if w <= 0 || g.Art == gArtOriginal || g.Art == "" {
		return fmt.Sprintf("Ausgabe: %d × %d px (Original)", w, h)
	}
	var tw, th int
	if _, ok := a.seiten(g, w, h); ok {
		_, _, rw, rh, _ := a.rechteck(g, w, h)
		tw, th = zielFuerAusschnitt(g, max(1, int(math.Round(rw))), max(1, int(math.Round(rh))), nichtGroesser)
	} else {
		tw, th = g.Zielmass(w, h, nichtGroesser)
	}
	return fmt.Sprintf("Ausgabe: %d × %d px", tw, th)
}
