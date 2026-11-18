package main

import (
	"fmt"
	"math"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"vertical3/kern"
)

// ------------------------------------------------------------------ Zoom

// einzelansicht malt Vorher- oder Nachher-Bild in die Einzelansicht – ganz
// (aus dem Zwischenspeicher) oder vergrößert direkt aus der vollen Auflösung.
func (p *programm) einzelansicht(c *walk.Canvas, b walk.Rectangle, name string, bild *kern.Bild) (int, int, int, int) {
	bw, bh := float64(b.Width), float64(b.Height)
	f0 := math.Min((bw-20)/float64(bild.W), (bh-20)/float64(bild.H))
	if name == "nachher" && p.modus == "" && !p.original {
		// Platz für den Ausschnittrahmen lassen, der größer als das Foto sein darf
		if r, ok := p.ausschnittSeiten(p.ausgabe, bild.W, bild.H); ok {
			_, _, smax := ausschnittGrenzen(r, float64(bild.W), float64(bild.H))
			mw, _, _ := ausschnittGrenzen(r, float64(bild.W), float64(bild.H))
			ew := mw * smax
			f0 = math.Min((bw-30)/ew, (bh-30)/(ew/r))
		}
	}
	raster := p.raster && p.modus == ""
	nachher := name == "nachher"
	if p.zoom <= 1.0001 {
		p.zoom, p.zx, p.zy = 1, 0.5, 0.5
		dw, dh := int(float64(bild.W)*f0), int(float64(bild.H)*f0)
		ox, oy := (b.Width-dw)/2, (b.Height-dh)/2
		p.zeichneBild(c, name, bild, walk.Rectangle{X: ox, Y: oy, Width: dw, Height: dh}, raster)
		p.ansicht.ox, p.ansicht.oy, p.ansicht.dw, p.ansicht.dh = float64(ox), float64(oy), float64(dw), float64(dh)
		p.ansicht.ok, p.ansicht.nachher = true, nachher
		p.zoomAnzeigen()
		return ox, oy, dw, dh
	}

	dw, dh := float64(bild.W)*f0*p.zoom, float64(bild.H)*f0*p.zoom
	lage := func(z *float64, d, platz float64) float64 {
		if d <= platz {
			*z = 0.5
			return (platz - d) / 2
		}
		o := math.Max(platz-d, math.Min(0, platz/2-*z*d))
		*z = (platz/2 - o) / d
		return o
	}
	ox, oy := lage(&p.zx, dw, bw), lage(&p.zy, dh, bh)
	p.ansicht.ox, p.ansicht.oy, p.ansicht.dw, p.ansicht.dh = ox, oy, dw, dh
	p.ansicht.ok, p.ansicht.nachher = true, nachher
	p.zoomAnzeigen()

	x0, y0 := int(math.Max(0, math.Floor(ox))), int(math.Max(0, math.Floor(oy)))
	x1, y1 := int(math.Min(bw, math.Ceil(ox+dw))), int(math.Min(bh, math.Ceil(oy+dh)))
	w, h := x1-x0, y1-y0
	if w <= 0 || h <= 0 {
		return int(ox), int(oy), int(dw), int(dh)
	}
	par, rand := p.parameter(), p.randFarbe()
	key := fmt.Sprintf("%p|%s|%.5f|%.2f|%.2f|%d|%d|%d|%d|%v|%v|%v", p.geladen, name, p.zoom, ox, oy, x0, y0, w, h, raster, par, rand)
	if key != p.zoomKey || p.zoomBmp == nil {
		g := p.geladen.Bild
		var S kern.Mat3
		var skal float64
		if nachher {
			M, ow, oh, err := kern.Transformation(g.W, g.H, par)
			if err != nil {
				return int(ox), int(oy), int(dw), int(dh)
			}
			sx, sy := float64(ow)/dw, float64(oh)/dh
			A := kern.Mat3{{sx, 0, (float64(x0)+0.5-ox)*sx - 0.5}, {0, sy, (float64(y0)+0.5-oy)*sy - 0.5}, {0, 0, 1}}
			S, skal = M.Inv().Mul(A), sx
		} else {
			sx, sy := float64(g.W)/dw, float64(g.H)/dh
			S = kern.Mat3{{sx, 0, (float64(x0)+0.5-ox)*sx - 0.5}, {0, sy, (float64(y0)+0.5-oy)*sy - 0.5}, {0, 0, 1}}
			skal = sx
		}
		img := g.Ausschnitt(w, h, S, skal, rand)
		if raster {
			for i := 1; i < 8; i++ {
				if x := int(ox+dw*float64(i)/8) - x0; x >= 0 && x < w {
					for y := 0; y < h; y++ {
						o := y*img.Stride + x*4
						img.Pix[o], img.Pix[o+1], img.Pix[o+2] = rasterFarbe[0], rasterFarbe[1], rasterFarbe[2]
					}
				}
				if y := int(oy+dh*float64(i)/8) - y0; y >= 0 && y < h {
					for x := 0; x < w; x++ {
						o := y*img.Stride + x*4
						img.Pix[o], img.Pix[o+1], img.Pix[o+2] = rasterFarbe[0], rasterFarbe[1], rasterFarbe[2]
					}
				}
			}
		}
		bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
		if err != nil {
			return int(ox), int(oy), int(dw), int(dh)
		}
		if p.zoomBmp != nil {
			p.zoomBmp.Dispose()
		}
		p.zoomBmp, p.zoomKey = bmp, key
	}
	c.DrawImagePixels(p.zoomBmp, walk.Point{X: x0, Y: y0})
	return int(math.Round(ox)), int(math.Round(oy)), int(math.Round(dw)), int(math.Round(dh))
}

func (p *programm) zoomAnzeigen() {
	t := "Ganz"
	if p.zoom > 1.0001 && p.geladen != nil {
		t = fmt.Sprintf("%.0f %%", p.ansicht.dw/float64(p.geladen.Bild.W)*100)
	}
	if p.ui.zoomText != t {
		p.ui.zoomText = t
		if p.panel != nil {
			p.panel.Invalidate()
		}
	}
}

// zoomUm vergrößert um den Bildschirmpunkt (x, y); der Punkt bleibt stehen.
func (p *programm) zoomUm(x, y, faktor float64) {
	if p.geladen == nil {
		return
	}
	if !p.ansicht.ok {
		p.setzeStatus("Zoomen geht in der Einzelansicht – Vorher/Nachher nebeneinander ausschalten (V).")
		return
	}
	a := p.ansicht
	basisW, basisH := a.dw/p.zoom, a.dh/p.zoom
	zmax := math.Max(1, 4*float64(p.geladen.Bild.W)/basisW) // bis 400 % der Originalpixel
	neu := math.Max(1, math.Min(zmax, p.zoom*faktor))
	u, v := (x-a.ox)/a.dw, (y-a.oy)/a.dh
	dw, dh := basisW*neu, basisH*neu
	cb := p.bildBereich()
	p.zx = (float64(cb.Width)/2 - (x - u*dw)) / dw
	p.zy = (float64(cb.Height)/2 - (y - v*dh)) / dh
	p.zoom = neu
	p.cw.Invalidate()
}

func (p *programm) zoomSchritt(faktor float64) {
	cb := p.bildBereich()
	p.zoomUm(float64(cb.Width)/2, float64(cb.Height)/2, faktor)
}

func (p *programm) zoomGanz() {
	p.zoom, p.zx, p.zy = 1, 0.5, 0.5
	p.cw.Invalidate()
}

// zoom100: ein Bildschirmpixel = ein Pixel des Originalfotos.
func (p *programm) zoom100() {
	if p.geladen == nil || !p.ansicht.ok {
		return
	}
	ziel := float64(p.geladen.Bild.W) / (p.ansicht.dw / p.zoom)
	p.zoomSchritt(ziel / p.zoom)
}

func (p *programm) mausrad(x, y int, knopf walk.MouseButton) {
	delta := walk.MouseWheelEventDelta(knopf)
	pt := win.POINT{X: int32(x), Y: int32(y)}
	win.ScreenToClient(p.cw.Handle(), &pt)
	if cb := p.cw.ClientBoundsPixels(); int(pt.X) >= cb.Width { // Maus steht über dem Bedienfeld
		q := win.POINT{X: int32(x), Y: int32(y)}
		win.ScreenToClient(p.panel.Handle(), &q)
		p.panelRad(int(q.X), int(q.Y), delta, walk.MouseWheelEventKeyState(knopf)&0x4 != 0)
		return
	}
	p.zoomUm(float64(pt.X), float64(pt.Y), math.Pow(1.25, float64(delta)/120))
}

func (p *programm) verschiebe(ox, oy float64) {
	if !p.ansicht.ok {
		return
	}
	cb := p.bildBereich()
	p.zx = (float64(cb.Width)/2 - ox) / p.ansicht.dw
	p.zy = (float64(cb.Height)/2 - oy) / p.ansicht.dh
	p.cw.Invalidate()
}

func (p *programm) ziehenEnde() {
	if p.ziehen == nil {
		return
	}
	p.ziehen = nil
	if p.modus != "" {
		p.cw.SetCursor(walk.CursorCross())
	} else {
		p.cw.SetCursor(walk.CursorArrow())
	}
}

// ------------------------------------------------------------------ Bezugsmarken im Ergebnis

// maleMarkenErgebnis zeigt die Bezugsmarken auch im korrigierten Bild –
// dort sieht man, ob sie wirklich gerade bzw. rund geworden sind.
func (p *programm) maleMarkenErgebnis(c *walk.Canvas, nachher bool) {
	if len(p.marken) == 0 || !p.kantenZeigen || !p.ansicht.ok || p.modus != "" {
		return
	}
	g := p.geladen.Bild
	a := p.ansicht
	var M kern.Mat3
	var ow, oh int
	if nachher {
		var err error
		if M, ow, oh, err = kern.Transformation(g.W, g.H, p.parameter()); err != nil {
			return
		}
	}
	punkt := func(q [2]float64) walk.Point {
		x, y := q[0], q[1]
		if nachher {
			t := M.MulVec([3]float64{x, y, 1})
			return walk.Point{X: int(a.ox + (t[0]/t[2]+0.5)/float64(ow)*a.dw), Y: int(a.oy + (t[1]/t[2]+0.5)/float64(oh)*a.dh)}
		}
		return walk.Point{X: int(a.ox + (x+0.5)/float64(g.W)*a.dw), Y: int(a.oy + (y+0.5)/float64(g.H)*a.dh)}
	}
	k := float64(c.DPI()) / 96
	for i, m := range p.marken {
		abw := 0.0
		if i < len(p.markenLsg.Abweichung) {
			abw = p.markenLsg.Abweichung[i]
		}
		pinsel, _ := walk.NewSolidColorBrush(markenFarbe(abw))
		st, _ := walk.NewGeometricPen(walk.PenSolid, int(math.Max(2, 2*k)), pinsel)
		pts := m.P
		zu := m.Art == kern.MarkeRechteck
		if m.Art == kern.MarkeKreisBoden || m.Art == kern.MarkeKreisWand {
			if C, err := kern.KegelschnittAus(m.P); err == nil {
				pts = kern.EllipsenPunkte(C, 72)
			}
		}
		for j := 0; j+1 < len(pts); j++ {
			c.DrawLinePixels(st, punkt(pts[j]), punkt(pts[j+1]))
		}
		if zu && len(pts) > 2 {
			c.DrawLinePixels(st, punkt(pts[len(pts)-1]), punkt(pts[0]))
		}
		st.Dispose()
		pinsel.Dispose()
	}
}

// maleAutoPunkte: Kantenpunkte der Automatik, die nach der Korrektur gerade
// stehen – sichtbar, solange die Werte unverändert sind (Taste K blendet aus).
func (p *programm) maleAutoPunkte(c *walk.Canvas, nachher bool) {
	if len(p.autoPunkte) == 0 || !p.kantenZeigen || !p.ansicht.ok || p.modus != "" || p.parameter() != p.autoPar {
		return
	}
	g := p.geladen.Bild
	a := p.ansicht
	var M kern.Mat3
	var ow, oh int
	if nachher {
		var err error
		if M, ow, oh, err = kern.Transformation(g.W, g.H, p.parameter()); err != nil {
			return
		}
	}
	k := float64(c.DPI()) / 96
	r := int(math.Max(1, math.Round(1.5*k)))
	farbe := pinselVorrat.pinsel(farbeKante)
	schritt := max(1, len(p.autoPunkte)/4000)
	for i := 0; i < len(p.autoPunkte); i += schritt {
		q := p.autoPunkte[i]
		var x, y float64
		if nachher {
			t := M.MulVec([3]float64{q[0], q[1], 1})
			x, y = a.ox+(t[0]/t[2]+0.5)/float64(ow)*a.dw, a.oy+(t[1]/t[2]+0.5)/float64(oh)*a.dh
		} else {
			x, y = a.ox+(q[0]+0.5)/float64(g.W)*a.dw, a.oy+(q[1]+0.5)/float64(g.H)*a.dh
		}
		if x < a.ox || y < a.oy || x > a.ox+a.dw || y > a.oy+a.dh {
			continue // liegt nach dem Zuschneiden außerhalb des Bildes
		}
		c.FillRectanglePixels(farbe, walk.Rectangle{X: int(x) - r/2, Y: int(y) - r/2, Width: r, Height: r})
	}
}
