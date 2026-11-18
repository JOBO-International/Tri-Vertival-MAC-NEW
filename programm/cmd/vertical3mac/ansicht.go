package main

// Bildansicht mit Zoom (Mausrad / Trackpad), Verschieben (Ziehen) und
// Klicks zum Setzen von Bezugsmarken.

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type ansicht struct {
	widget.BaseWidget
	p      *programm
	raster *canvas.Raster

	img          *image.RGBA // anzuzeigendes Bild
	markenZeigen bool        // img ist die Original-Vorschau: Marken einblenden
	zoom, zx, zy float64     // zoom 1 = ganzes Bild; zx, zy = Bildmitte in Bildanteilen

	// Abbildung Bild → Raster (aus dem letzten Zeichnen)
	pw, ph       int
	ox, oy, s    float64
	hatAbbildung bool
}

func neueAnsicht(p *programm) *ansicht {
	a := &ansicht{p: p, zoom: 1, zx: 0.5, zy: 0.5}
	a.raster = canvas.NewRaster(a.zeichne)
	a.raster.ScaleMode = canvas.ImageScaleSmooth
	a.ExtendBaseWidget(a)
	return a
}

func (a *ansicht) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(a.raster) }

func (a *ansicht) MinSize() fyne.Size { return fyne.NewSize(400, 300) }

func (a *ansicht) setzeBild(img *image.RGBA, marken bool) {
	a.img, a.markenZeigen = img, marken
	a.raster.Refresh()
}

func (a *ansicht) ganz() { a.zoom, a.zx, a.zy = 1, 0.5, 0.5; a.raster.Refresh() }

func (a *ansicht) zoomSchritt(f float64, mx, my float64) {
	if a.img == nil || !a.hatAbbildung {
		return
	}
	// Bildpunkt unter dem Zeiger soll stehen bleiben
	iw, ih := float64(a.img.Rect.Dx()), float64(a.img.Rect.Dy())
	bx, by := (mx-a.ox)/a.s, (my-a.oy)/a.s
	neu := math.Max(1, math.Min(16, a.zoom*f))
	if neu == a.zoom {
		return
	}
	faktor := neu / a.zoom
	f0 := a.s / a.zoom
	s2 := f0 * neu
	// neue Bildmitte so, dass bx,by am Zeiger bleibt
	cx := bx - (mx-float64(a.pw)/2)/s2
	cy := by - (my-float64(a.ph)/2)/s2
	_ = faktor
	a.zoom = neu
	if neu == 1 {
		a.zx, a.zy = 0.5, 0.5
	} else {
		a.zx, a.zy = cx/iw, cy/ih
		a.begrenze()
	}
	a.raster.Refresh()
}

func (a *ansicht) begrenze() {
	a.zx = math.Max(0, math.Min(1, a.zx))
	a.zy = math.Max(0, math.Min(1, a.zy))
}

// Zeiger (dp, Widget) → Raster-Pixel
func (a *ansicht) rasterPos(pos fyne.Position) (float64, float64) {
	sz := a.Size()
	if sz.Width <= 0 || sz.Height <= 0 || a.pw == 0 {
		return 0, 0
	}
	return float64(pos.X) / float64(sz.Width) * float64(a.pw), float64(pos.Y) / float64(sz.Height) * float64(a.ph)
}

// Scrolled: Mausrad / Trackpad-Geste zoomt.
func (a *ansicht) Scrolled(ev *fyne.ScrollEvent) {
	mx, my := a.rasterPos(ev.Position)
	f := math.Pow(1.0015, float64(ev.Scrolled.DY))
	if f < 1.01 && f > 0.99 {
		return
	}
	a.zoomSchritt(f, mx, my)
}

func (a *ansicht) Dragged(ev *fyne.DragEvent) {
	if a.img == nil || a.zoom <= 1 || !a.hatAbbildung {
		return
	}
	sz := a.Size()
	if sz.Width <= 0 {
		return
	}
	k := float64(a.pw) / float64(sz.Width)
	iw, ih := float64(a.img.Rect.Dx()), float64(a.img.Rect.Dy())
	a.zx -= float64(ev.Dragged.DX) * k / (a.s * iw)
	a.zy -= float64(ev.Dragged.DY) * k / (a.s * ih)
	a.begrenze()
	a.raster.Refresh()
}

func (a *ansicht) DragEnd() {}

// Tapped: Klick – setzt einen Punkt der gerade angelegten Bezugsmarke.
func (a *ansicht) Tapped(ev *fyne.PointEvent) {
	if a.img == nil || !a.hatAbbildung || !a.markenZeigen || a.p.markeNeu == "" {
		return
	}
	mx, my := a.rasterPos(ev.Position)
	bx, by := (mx-a.ox)/a.s, (my-a.oy)/a.s
	if bx < 0 || by < 0 || bx > float64(a.img.Rect.Dx()) || by > float64(a.img.Rect.Dy()) {
		return
	}
	a.p.markenKlick(bx, by)
}

var ansichtHintergrund = color.RGBA{0x2b, 0x2b, 0x2b, 0xff}

// zeichne erzeugt das Rasterbild in genau der Widget-Größe in Pixeln.
func (a *ansicht) zeichne(w, h int) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = ansichtHintergrund.R, ansichtHintergrund.G, ansichtHintergrund.B, 255
	}
	a.pw, a.ph, a.hatAbbildung = w, h, false
	src := a.img
	if src == nil || w < 2 || h < 2 {
		return out
	}
	iw, ih := float64(src.Rect.Dx()), float64(src.Rect.Dy())
	f0 := math.Min(float64(w)/iw, float64(h)/ih)
	s := f0 * a.zoom
	ox := float64(w)/2 - a.zx*iw*s
	oy := float64(h)/2 - a.zy*ih*s
	if a.zoom <= 1 {
		ox, oy = (float64(w)-iw*s)/2, (float64(h)-ih*s)/2
	}
	a.ox, a.oy, a.s, a.hatAbbildung = ox, oy, s, true

	x0 := max(0, int(math.Ceil(ox)))
	x1 := min(w, int(math.Floor(ox+iw*s)))
	y0 := max(0, int(math.Ceil(oy)))
	y1 := min(h, int(math.Floor(oy+ih*s)))
	W, H := src.Rect.Dx(), src.Rect.Dy()
	for y := y0; y < y1; y++ {
		fy := (float64(y)+0.5-oy)/s - 0.5
		j := int(math.Floor(fy))
		ty := fy - float64(j)
		j0, j1 := min(max(j, 0), H-1), min(max(j+1, 0), H-1)
		d := out.Pix[y*out.Stride:]
		for x := x0; x < x1; x++ {
			if s < 1 { // verkleinert: einfaches Mittel aus vier Nachbarn (bilinear)
				fx := (float64(x)+0.5-ox)/s - 0.5
				i := int(math.Floor(fx))
				tx := fx - float64(i)
				i0, i1 := min(max(i, 0), W-1), min(max(i+1, 0), W-1)
				p00, p10 := src.Pix[j0*src.Stride+i0*4:], src.Pix[j0*src.Stride+i1*4:]
				p01, p11 := src.Pix[j1*src.Stride+i0*4:], src.Pix[j1*src.Stride+i1*4:]
				for c := 0; c < 3; c++ {
					v := (float64(p00[c])*(1-tx)+float64(p10[c])*tx)*(1-ty) + (float64(p01[c])*(1-tx)+float64(p11[c])*tx)*ty
					d[x*4+c] = uint8(v + 0.5)
				}
			} else { // vergrößert: Pixel sichtbar lassen
				i := min(max(int((float64(x)+0.5-ox)/s), 0), W-1)
				jj := min(max(int((float64(y)+0.5-oy)/s), 0), H-1)
				sp := src.Pix[jj*src.Stride+i*4:]
				d[x*4], d[x*4+1], d[x*4+2] = sp[0], sp[1], sp[2]
			}
			d[x*4+3] = 255
		}
	}
	if a.markenZeigen {
		a.p.zeichneMarken(out, a)
	}
	return out
}

// ---- kleine Zeichenhilfen

func punkt(img *image.RGBA, cx, cy float64, r int, c color.RGBA) {
	b := img.Bounds()
	for y := int(cy) - r; y <= int(cy)+r; y++ {
		for x := int(cx) - r; x <= int(cx)+r; x++ {
			if (image.Point{x, y}).In(b) && (x-int(cx))*(x-int(cx))+(y-int(cy))*(y-int(cy)) <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func linie(img *image.RGBA, x0, y0, x1, y1 float64, c color.RGBA) {
	n := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
	b := img.Bounds()
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		x, y := int(x0+(x1-x0)*t), int(y0+(y1-y0)*t)
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				if (image.Point{x + dx, y + dy}).In(b) {
					img.SetRGBA(x+dx, y+dy, c)
				}
			}
		}
	}
}
