package main

// Bildausschnitt für Ausgabegrößen mit festem Seitenverhältnis (Fotopapier,
// „passt in B × H px“): Ein Rahmen im Seitenverhältnis der Zielgröße liegt
// auf dem korrigierten Bild. Er lässt sich verschieben (innen ziehen) und an
// den Ecken größer/kleiner ziehen. Beim Speichern wird genau dieser Bereich
// ausgeschnitten und anschließend auf die Zielgröße skaliert.

import (
	"fmt"
	"math"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"vertical3/kern"
)

// ausschnittZustand: Lage des Rahmens, unabhängig von der Bildgröße.
type ausschnittZustand struct {
	cx, cy  float64 // Mitte in Bildanteilen (0..1)
	s       float64 // Größe: Anteil am größtmöglichen Rahmen (0..1)
	gedreht bool    // Hoch-/Querformat gegenüber dem Bild getauscht
	zieh    int     // 0 = nichts, 1 = verschieben, 2..5 = Ecke (lo, ro, ru, lu)
	start   [2]float64
	startZ  ausschnittZustand0
}

type ausschnittZustand0 struct{ cx, cy, s float64 }

func (a *ausschnittZustand) zuruecksetzen() {
	a.cx, a.cy, a.s, a.zieh = 0.5, 0.5, 1, 0
	a.gedreht = false // Format folgt wieder dem Bild (Hoch- oder Querformat)
}

// ausschnittSeiten: Seitenverhältnis der Größe g (Breite/Höhe) für ein Bild w×h.
// ok = false, wenn die Größe kein festes Seitenverhältnis hat.
func (p *programm) ausschnittSeiten(g groesse, w, h int) (float64, bool) {
	if (g.Art != gArtDruck && g.Art != gArtBox) || g.A <= 0 || g.B <= 0 || w <= 0 || h <= 0 {
		return 0, false
	}
	lang, kurz := math.Max(g.A, g.B), math.Min(g.A, g.B)
	quer := w >= h
	if p.aus.gedreht {
		quer = !quer
	}
	if quer {
		return lang / kurz, true
	}
	return kurz / lang, true
}

// ausschnittRechteck: Rahmen in Pixeln eines Bildes w×h für die Größe g.
func (p *programm) ausschnittRechteck(g groesse, w, h int) (x, y, rw, rh float64, ok bool) {
	r, ok := p.ausschnittSeiten(g, w, h)
	if !ok {
		return 0, 0, float64(w), float64(h), false
	}
	W, H := float64(w), float64(h)
	mw, _, smax := ausschnittGrenzen(r, W, H)
	s := math.Max(0.1, math.Min(smax, p.aus.s))
	rw, rh = mw*s, mw*s/r
	x = klemme(p.aus.cx*W-rw/2, W-rw)
	y = klemme(p.aus.cy*H-rh/2, H-rh)
	return x, y, rw, rh, true
}

// ausschnittGrenzen: größter Rahmen ganz im Bild (mw × mh) und Faktor smax,
// bei dem der Rahmen das ganze Foto umschließt (darüber hinaus: Rand).
func ausschnittGrenzen(r, W, H float64) (mw, mh, smax float64) {
	mw, mh = W, W/r
	if mh > H {
		mw, mh = H*r, H
	}
	aw := W // Rahmen, der das ganze Foto umschließt
	if W/r < H {
		aw = H * r
	}
	return mw, mh, aw / mw
}

// klemme: Lage einer Rahmenkante. Kleiner als das Bild → bleibt im Bild;
// größer → das Bild bleibt im Rahmen.
func klemme(v, spiel float64) float64 {
	return math.Max(math.Min(0, spiel), math.Min(math.Max(0, spiel), v))
}

// ausschnittAktiv: Rahmen wird angezeigt und beim Speichern verwendet.
func (p *programm) ausschnittAktiv() bool {
	if p.geladen == nil || p.nachher == nil {
		return false
	}
	_, ok := p.ausschnittSeiten(p.ausgabe, p.nachher.W, p.nachher.H)
	return ok
}

// quellMass: Pixel (volle Auflösung), die für die Größe g tatsächlich verwendet werden.
func (p *programm) quellMass(g groesse) (int, int) {
	w, h := p.korrigiertMass()
	if w == 0 {
		return 0, 0
	}
	if _, _, rw, rh, ok := p.ausschnittRechteck(g, w, h); ok {
		return max(1, int(math.Round(rw))), max(1, int(math.Round(rh)))
	}
	return w, h
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

// ausschnittAnwenden: beim Speichern – erst ausschneiden, dann skalieren.
func (p *programm) ausschnittAnwenden(erg *kern.Bild, g groesse, nichtGroesser bool, rand [3]uint8) *kern.Bild {
	x, y, rw, rh, ok := p.ausschnittRechteck(g, erg.W, erg.H)
	if !ok {
		return erg.Skaliert(g.Zielmass(erg.W, erg.H, nichtGroesser))
	}
	c := erg.AusschnittMitRand(int(math.Round(x)), int(math.Round(y)), int(math.Round(rw)), int(math.Round(rh)), rand)
	return c.Skaliert(zielFuerAusschnitt(g, c.W, c.H, nichtGroesser))
}

// ------------------------------------------------------------------ Anzeige

var schleierBmp win.HBITMAP

// schleierMalen dunkelt r halbtransparent ab (konstante Deckkraft).
func schleierMalen(c *walk.Canvas, r walk.Rectangle) {
	mem := win.CreateCompatibleDC(c.HDC())
	defer win.DeleteDC(mem)
	if schleierBmp == 0 {
		schleierBmp = win.CreateCompatibleBitmap(c.HDC(), 1, 1)
	}
	alt := win.SelectObject(mem, win.HGDIOBJ(schleierBmp))
	defer win.SelectObject(mem, alt)
	win.BitBlt(mem, 0, 0, 1, 1, 0, 0, 0, win.BLACKNESS)
	win.AlphaBlend(c.HDC(), int32(r.X), int32(r.Y), int32(r.Width), int32(r.Height), mem, 0, 0, 1, 1,
		win.BLENDFUNCTION{SourceConstantAlpha: 130})
}

// ausschnittBildschirm: Rahmen in Bildschirmkoordinaten der Nachher-Ansicht.
func (p *programm) ausschnittBildschirm() (walk.Rectangle, bool) {
	if !p.ansicht.ok || !p.ansicht.nachher || !p.ausschnittAktiv() {
		return walk.Rectangle{}, false
	}
	x, y, rw, rh, _ := p.ausschnittRechteck(p.ausgabe, p.nachher.W, p.nachher.H)
	a := p.ansicht
	fx, fy := a.dw/float64(p.nachher.W), a.dh/float64(p.nachher.H)
	return walk.Rectangle{X: int(math.Round(a.ox + x*fx)), Y: int(math.Round(a.oy + y*fy)),
		Width: int(math.Round(rw * fx)), Height: int(math.Round(rh * fy))}, true
}

func (p *programm) maleAusschnitt(c *walk.Canvas) {
	r, ok := p.ausschnittBildschirm()
	if !ok {
		return
	}
	a := p.ansicht
	bild := walk.Rectangle{X: int(math.Round(a.ox)), Y: int(math.Round(a.oy)), Width: int(math.Round(a.dw)), Height: int(math.Round(a.dh))}
	// abgedunkelter Bereich außerhalb des Rahmens
	{
		teile := []walk.Rectangle{
			{X: bild.X, Y: bild.Y, Width: bild.Width, Height: r.Y - bild.Y},
			{X: bild.X, Y: r.Y + r.Height, Width: bild.Width, Height: bild.Y + bild.Height - r.Y - r.Height},
			{X: bild.X, Y: r.Y, Width: r.X - bild.X, Height: r.Height},
			{X: r.X + r.Width, Y: r.Y, Width: bild.X + bild.Width - r.X - r.Width, Height: r.Height},
		}
		for _, t := range teile {
			t = schnitt(t, bild)
			if t.Width > 0 && t.Height > 0 {
				schleierMalen(c, t)
			}
		}
	}
	// Rahmen größer als das Foto: überstehende Flächen in der Randfarbe zeigen
	if r.X < bild.X || r.Y < bild.Y || r.X+r.Width > bild.X+bild.Width || r.Y+r.Height > bild.Y+bild.Height {
		rf := p.randFarbe()
		pin := pinselVorrat.pinsel(walk.RGB(rf[0], rf[1], rf[2]))
		innen := walk.Rectangle{X: max(r.X, bild.X), Y: max(r.Y, bild.Y)}
		innen.Width = min(r.X+r.Width, bild.X+bild.Width) - innen.X
		innen.Height = min(r.Y+r.Height, bild.Y+bild.Height) - innen.Y
		for _, t := range []walk.Rectangle{
			{X: r.X, Y: r.Y, Width: r.Width, Height: innen.Y - r.Y},
			{X: r.X, Y: innen.Y + innen.Height, Width: r.Width, Height: r.Y + r.Height - innen.Y - innen.Height},
			{X: r.X, Y: innen.Y, Width: innen.X - r.X, Height: innen.Height},
			{X: innen.X + innen.Width, Y: innen.Y, Width: r.X + r.Width - innen.X - innen.Width, Height: innen.Height},
		} {
			if t.Width > 0 && t.Height > 0 {
				c.FillRectanglePixels(pin, t)
			}
		}
	}
	k := float64(c.DPI()) / 96
	px := func(v float64) int { return max(1, int(v*k+0.5)) }
	c.DrawRectanglePixels(stift(walk.RGB(160, 160, 160), px(3)), r)
	c.DrawRectanglePixels(stift(walk.RGB(255, 255, 255), px(2)), r)
	// Drittel-Linien als Gestaltungshilfe
	for i := 1; i <= 2; i++ {
		x := r.X + r.Width*i/3
		y := r.Y + r.Height*i/3
		c.DrawLinePixels(stift(walk.RGB(220, 220, 220), 1), walk.Point{X: x, Y: r.Y}, walk.Point{X: x, Y: r.Y + r.Height})
		c.DrawLinePixels(stift(walk.RGB(220, 220, 220), 1), walk.Point{X: r.X, Y: y}, walk.Point{X: r.X + r.Width, Y: y})
	}
	// Eckgriffe
	g := px(14)
	for _, e := range ausschnittEcken(r) {
		c.FillRectanglePixels(pinselVorrat.pinsel(walk.RGB(255, 255, 255)), walk.Rectangle{X: e.X - g/2, Y: e.Y - g/2, Width: g, Height: g})
		c.DrawRectanglePixels(stift(farbeRot, 1), walk.Rectangle{X: e.X - g/2, Y: e.Y - g/2, Width: g, Height: g})
	}
	// Beschriftung
	cw, ch := p.quellMass(p.ausgabe)
	t := fmt.Sprintf("Ausschnitt %s  ·  %d × %d px", p.ausgabeKurz(), cw, ch)
	p.marke(c, t, r.X+px(8), r.Y+px(8), false)
}

func (p *programm) ausgabeKurz() string {
	z := func(v float64) string {
		return fmt.Sprint(math.Round(v*10) / 10)
	}
	r, _ := p.ausschnittSeiten(p.ausgabe, p.nachher.W, p.nachher.H)
	a, b := math.Max(p.ausgabe.A, p.ausgabe.B), math.Min(p.ausgabe.A, p.ausgabe.B)
	if r < 1 {
		a, b = b, a
	}
	e := " cm"
	if p.ausgabe.Art == gArtBox {
		e = " px"
	}
	return z(a) + " × " + z(b) + e
}

func ausschnittEcken(r walk.Rectangle) [4]walk.Point {
	return [4]walk.Point{{X: r.X, Y: r.Y}, {X: r.X + r.Width, Y: r.Y}, {X: r.X + r.Width, Y: r.Y + r.Height}, {X: r.X, Y: r.Y + r.Height}}
}

// ------------------------------------------------------------------ Maus

// ausschnittTreffer: 2..5 = Ecke, 1 = innen, 0 = nichts.
func (p *programm) ausschnittTreffer(x, y int) int {
	r, ok := p.ausschnittBildschirm()
	if !ok || p.modus != "" {
		return 0
	}
	g := int(12 * float64(p.cw.DPI()) / 96)
	for i, e := range ausschnittEcken(r) {
		if abs(x-e.X) <= g && abs(y-e.Y) <= g {
			return 2 + i
		}
	}
	if x > r.X && x < r.X+r.Width && y > r.Y && y < r.Y+r.Height {
		return 1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (p *programm) ausschnittCursor(t int) {
	switch t {
	case 1:
		p.cw.SetCursor(walk.CursorSizeAll())
	case 2, 4:
		p.cw.SetCursor(walk.CursorSizeNWSE())
	case 3, 5:
		p.cw.SetCursor(walk.CursorSizeNESW())
	}
}

func (p *programm) ausschnittStart(x, y int) bool {
	t := p.ausschnittTreffer(x, y)
	if t == 0 {
		return false
	}
	p.aus.zieh = t
	p.aus.start = [2]float64{float64(x), float64(y)}
	_, _, rw, rh, _ := p.ausschnittRechteck(p.ausgabe, p.nachher.W, p.nachher.H)
	X, Y, _, _, _ := p.ausschnittRechteck(p.ausgabe, p.nachher.W, p.nachher.H)
	// Mitte auf die tatsächliche (begrenzte) Lage setzen
	p.aus.cx, p.aus.cy = (X+rw/2)/float64(p.nachher.W), (Y+rh/2)/float64(p.nachher.H)
	p.aus.startZ = ausschnittZustand0{p.aus.cx, p.aus.cy, p.aus.s}
	p.ausschnittCursor(t)
	return true
}

func (p *programm) ausschnittZiehen(x, y int) {
	W, H := float64(p.nachher.W), float64(p.nachher.H)
	fx, fy := W/p.ansicht.dw, H/p.ansicht.dh // Bildpixel je Bildschirmpixel
	dx, dy := (float64(x)-p.aus.start[0])*fx, (float64(y)-p.aus.start[1])*fy
	z := p.aus.startZ
	if p.aus.zieh == 1 {
		p.aus.cx, p.aus.cy = z.cx+dx/W, z.cy+dy/H
	} else {
		// Ecke ziehen: gegenüberliegende Ecke bleibt stehen
		r, _ := p.ausschnittSeiten(p.ausgabe, p.nachher.W, p.nachher.H)
		mw, mh, smax := ausschnittGrenzen(r, W, H)
		w0, h0 := mw*z.s, mh*z.s
		sx, sy := []float64{-1, 1, 1, -1}[p.aus.zieh-2], []float64{-1, -1, 1, 1}[p.aus.zieh-2]
		fixX, fixY := z.cx*W-sx*w0/2, z.cy*H-sy*h0/2
		mx, my := z.cx*W+sx*w0/2+dx, z.cy*H+sy*h0/2+dy
		w := math.Max(math.Abs(mx-fixX), math.Abs(my-fixY)*r)
		w = math.Min(w, mw*smax) // höchstens so groß, dass das ganze Foto hineinpasst
		w = math.Max(w, mw*0.1)
		h := w / r
		p.aus.s = w / mw
		p.aus.cx, p.aus.cy = (fixX+sx*w/2)/W, (fixY+sy*h/2)/H
	}
	p.cw.Invalidate()
	p.panel.Invalidate()
}

func (p *programm) ausschnittEnde() {
	if p.aus.zieh == 0 {
		return
	}
	p.aus.zieh = 0
	// Mitte auf die begrenzte Lage festschreiben
	if p.nachher != nil {
		X, Y, rw, rh, ok := p.ausschnittRechteck(p.ausgabe, p.nachher.W, p.nachher.H)
		if ok {
			p.aus.cx, p.aus.cy = (X+rw/2)/float64(p.nachher.W), (Y+rh/2)/float64(p.nachher.H)
		}
	}
	p.panel.Invalidate()
}

// ausschnittDrehen tauscht Hoch- und Querformat des Rahmens.
func (p *programm) ausschnittDrehen() {
	if !p.ausschnittAktiv() {
		return
	}
	p.aus.gedreht = !p.aus.gedreht
	p.aus.s = 1
	p.cw.Invalidate()
	p.panel.Invalidate()
}

func (p *programm) ausschnittNeu() {
	p.aus.zuruecksetzen()
	p.cw.Invalidate()
	p.panel.Invalidate()
}

func schnitt(a, b walk.Rectangle) walk.Rectangle {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.Width, b.X+b.Width), min(a.Y+a.Height, b.Y+b.Height)
	return walk.Rectangle{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// ausschnittEinpassen: Rahmen so groß, dass das ganze Foto hineinpasst (Rest = Rand).
func (p *programm) ausschnittEinpassen() {
	if !p.ausschnittAktiv() {
		return
	}
	r, _ := p.ausschnittSeiten(p.ausgabe, p.nachher.W, p.nachher.H)
	_, _, smax := ausschnittGrenzen(r, float64(p.nachher.W), float64(p.nachher.H))
	p.aus.cx, p.aus.cy, p.aus.s = 0.5, 0.5, smax
	p.cw.Invalidate()
	p.panel.Invalidate()
}

// formatVorschlag: Fotopapier, dessen Seitenverhältnis am besten zum Bild passt.
// ok nur, wenn es deutlich weniger abschneidet als die gewählte Größe.
func (p *programm) formatVorschlag() (groesse, float64, bool) {
	if p.ausgabe.Art != gArtDruck || p.nachher == nil {
		return groesse{}, 0, false
	}
	ri := float64(p.nachher.W) / float64(p.nachher.H)
	verlust := func(g groesse) float64 {
		lang, kurz := math.Max(g.A, g.B), math.Min(g.A, g.B)
		rp := kurz / lang
		if ri >= 1 {
			rp = lang / kurz
		}
		return 1 - math.Min(ri, rp)/math.Max(ri, rp)
	}
	jetzt := verlust(p.ausgabe)
	flaeche := p.ausgabe.A * p.ausgabe.B
	var best groesse
	bestV, bestD := 2.0, math.MaxFloat64
	for _, g := range p.alleGroessen() {
		if g.Art != gArtDruck || g.Name == p.ausgabe.Name {
			continue
		}
		v := verlust(g)
		d := math.Abs(math.Log(g.A * g.B / flaeche)) // ähnliche Größe bevorzugen
		if v < bestV-0.01 || (math.Abs(v-bestV) <= 0.01 && d < bestD) {
			best, bestV, bestD = g, v, d
		}
	}
	if best.Name == "" || jetzt-bestV < 0.03 {
		return groesse{}, 0, false
	}
	return best, bestV, true
}

func (p *programm) formatUebernehmen(g groesse) {
	p.ausgabe = g
	p.groesseSpeichern()
	p.aus.zuruecksetzen()
	p.setzeStatus("Ausgabegröße: " + g.Name + " – gilt für alle weiteren Bilder.")
	p.cw.Invalidate()
	p.panel.Invalidate()
}
