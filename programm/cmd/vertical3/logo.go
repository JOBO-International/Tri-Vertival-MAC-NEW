package main

// Tri-Vertical-Bildzeichen: Die Buchstaben T, r und i bilden zusammen eine 3.
// Das Zeichen wird zur Laufzeit kantengeglättet gerastert, damit es in jeder
// Größe und in den Farben des gewählten Designs scharf erscheint.

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/lxn/walk"
)

type logoStrecke struct{ x1, y1, x2, y2 float64 }

// Geometrie im Raster 120 × 160 (wie im Logo-Entwurf).
var logoStrecken = func() []logoStrecke {
	s := []logoStrecke{
		{100, 12, 100, 148}, // Rücken der 3
		{18, 12, 100, 12},   // T-Balken = oberer Bogen
		{40, 12, 40, 48},    // T-Stamm
		{18, 148, 100, 148}, // unterer Bogen
		{34, 124, 34, 140},  // i-Stamm
		{50, 100, 50, 76},   // r-Stamm
		{64, 64, 100, 64},   // r-Schulter = mittlerer Bogen
	}
	// Rundung der r-Schulter (quadratische Kurve 50,76 → 50,64 → 64,64)
	px, py := 50.0, 76.0
	for i := 1; i <= 8; i++ {
		t := float64(i) / 8
		x := (1-t)*(1-t)*50 + 2*(1-t)*t*50 + t*t*64
		y := (1-t)*(1-t)*76 + 2*(1-t)*t*64 + t*t*64
		s = append(s, logoStrecke{px, py, x, y})
		px, py = x, y
	}
	return s
}()

const logoPunktX, logoPunktY = 34.0, 107.0

func abstandStrecke(x, y float64, s logoStrecke) float64 {
	dx, dy := s.x2-s.x1, s.y2-s.y1
	t := ((x-s.x1)*dx + (y-s.y1)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	ex, ey := s.x1+t*dx-x, s.y1+t*dy-y
	return math.Hypot(ex, ey)
}

var logoVorrat = map[string]*walk.Bitmap{}

// logoBild liefert das Bildzeichen in der Höhe h (Pixel), Breite = h·0,75.
func logoBild(h int, strich, punkt walk.Color) *walk.Bitmap {
	key := fmt.Sprint(h, strich, punkt)
	if b, ok := logoVorrat[key]; ok {
		return b
	}
	w := int(math.Ceil(float64(h) * 0.75))
	s := float64(h) / 160 // Pixel je Rastereinheit
	// Bei kleinen Größen kräftiger, damit das Zeichen lesbar bleibt
	halb := 5.5
	pr := 7.0
	if h < 48 {
		halb, pr = 7.5, 9.5
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	sr, sg, sb := float64(strich.R()), float64(strich.G()), float64(strich.B())
	pr_, pg, pb := float64(punkt.R()), float64(punkt.G()), float64(punkt.B())
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			x, y := (float64(i)+0.5)/s, (float64(j)+0.5)/s
			d := 1e9
			for _, st := range logoStrecken {
				d = math.Min(d, abstandStrecke(x, y, st))
			}
			aS := math.Max(0, math.Min(1, 0.5-(d-halb)*s))
			dp := math.Hypot(x-logoPunktX, y-logoPunktY)
			aP := math.Max(0, math.Min(1, 0.5-(dp-pr)*s))
			if aS == 0 && aP == 0 {
				continue
			}
			// Punkt liegt über dem Strich
			a := aP + aS*(1-aP)
			r := (pr_*aP + sr*aS*(1-aP))
			g := (pg*aP + sg*aS*(1-aP))
			b := (pb*aP + sb*aS*(1-aP))
			img.SetRGBA(i, j, color.RGBA{uint8(r + 0.5), uint8(g + 0.5), uint8(b + 0.5), uint8(a*255 + 0.5)})
		}
	}
	bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
	if err != nil {
		return nil
	}
	logoVorrat[key] = bmp
	return bmp
}

// logoFarben: Strich in Textfarbe, i-Punkt in der Akzentfarbe des Designs.
func logoFarben() (walk.Color, walk.Color) { return farbeText, farbeRot }

// maleLogo zeichnet Bildzeichen + „-Vertical“ zentriert in r.
// Rückgabe: Unterkante des Logos.
func (p *programm) maleLogo(c *walk.Canvas, r walk.Rectangle, markHoehe int, schriftArt string, zentriert bool) {
	strich, punkt := logoFarben()
	bmp := logoBild(markHoehe, strich, punkt)
	wort := gt("-Vertical")
	tb := p.textBreite(c, schriftArt, wort)
	mw := int(math.Ceil(float64(markHoehe) * 0.75))
	luecke := markHoehe / 14
	gesamt := mw + luecke + tb
	x := r.X
	if zentriert {
		x = r.X + (r.Width-gesamt)/2
	}
	y := r.Y + (r.Height-markHoehe)/2
	if bmp != nil {
		c.DrawImagePixels(bmp, walk.Point{X: x, Y: y})
	}
	// Schrift so setzen, dass ihre Grundlinie auf dem unteren Bogen der 3 sitzt
	unten := y + int(float64(markHoehe)*152/160)
	c.DrawTextPixels(wort, p.schrift(schriftArt), farbeText,
		walk.Rectangle{X: x + mw + luecke, Y: y, Width: tb + 8, Height: unten - y + markHoehe/9},
		walk.TextLeft|walk.TextBottom|walk.TextSingleLine)
}

// logoLeeren gibt die gerasterten Zeichen frei (z. B. beim Designwechsel).
func logoLeeren() {
	for k, b := range logoVorrat {
		b.Dispose()
		delete(logoVorrat, k)
	}
}
