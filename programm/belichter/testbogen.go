package belichter

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Testbogen: Lineale, 10-cm-Quadrate und Eckmarken in echten Millimetern der
// Belichtungsfläche – nach dem Belichten nachmessen, ob alles stimmt.
func Testbogen(e Einrichtung) *image.RGBA {
	pw, ph := e.Pixel()
	img := image.NewRGBA(image.Rect(0, 0, pw, ph))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	fb, fh := e.FlaecheB*10, e.FlaecheH*10 // mm
	sx, sy := float64(pw)/fb, float64(ph)/fh
	schwarz := color.RGBA{0, 0, 0, 255}
	grau := color.RGBA{150, 150, 150, 255}
	rechteck := func(x0, y0, x1, y1 float64, c color.RGBA) { // mm
		for y := int(math.Round(y0 * sy)); y < int(math.Round(y1*sy)); y++ {
			for x := int(math.Round(x0 * sx)); x < int(math.Round(x1*sx)); x++ {
				if image.Pt(x, y).In(img.Rect) {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	waag := func(x0, x1, y, d float64, c color.RGBA) { rechteck(x0, y-d/2, x1, y+d/2, c) }
	senk := func(x, y0, y1, d float64, c color.RGBA) { rechteck(x-d/2, y0, x+d/2, y1, c) }

	ft, _ := opentype.Parse(goregular.TTF)
	schrift := func(mmHoehe float64) font.Face {
		f, _ := opentype.NewFace(ft, &opentype.FaceOptions{Size: mmHoehe * sy * 1.4, DPI: 72, Hinting: font.HintingNone})
		return f
	}
	text := func(s string, xmm, ymm, hmm float64) {
		f := schrift(hmm)
		d := font.Drawer{Dst: img, Src: image.NewUniform(schwarz), Face: f,
			Dot: fixed.P(int(xmm*sx), int(ymm*sy))}
		d.DrawString(s)
	}

	// Lineale oben und links (mm-Striche, cm-Striche mit Zahl)
	for x := 0.0; x <= fb+0.01; x++ {
		l := 2.0
		if math.Mod(x, 10) < 0.01 {
			l = 6
		} else if math.Mod(x, 5) < 0.01 {
			l = 4
		}
		senk(x+0.1, 0, l, 0.2, schwarz)
		senk(x+0.1, fh-l, fh, 0.2, schwarz)
		if math.Mod(x, 10) < 0.01 && x > 0 && x < fb-5 {
			text(fmt.Sprint(int(x/10)), x+0.8, 10, 3)
		}
	}
	for y := 0.0; y <= fh+0.01; y++ {
		l := 2.0
		if math.Mod(y, 10) < 0.01 {
			l = 6
		} else if math.Mod(y, 5) < 0.01 {
			l = 4
		}
		waag(0, l, y+0.1, 0.2, schwarz)
		waag(fb-l, fb, y+0.1, 0.2, schwarz)
		if math.Mod(y, 10) < 0.01 && y > 0 && y < fh-5 {
			text(fmt.Sprint(int(y/10)), 7.5, y+1.5, 3)
		}
	}
	// 10-cm-Quadrate in der Mitte (Rahmen 0,3 mm)
	cx, cy := fb/2, fh/2
	for _, q := range [][2]float64{{cx - 105, cy - 50}, {cx + 5, cy - 50}} {
		x, y := q[0], q[1]
		waag(x, x+100, y, 0.3, schwarz)
		waag(x, x+100, y+100, 0.3, schwarz)
		senk(x, y, y+100, 0.3, schwarz)
		senk(x+100, y, y+100, 0.3, schwarz)
	}
	text("10 × 10 cm", cx-80, cy+5, 5)
	text("10 × 10 cm", cx+30, cy+5, 5)
	// Graustufen-Keil unter den Quadraten
	for i := 0; i <= 10; i++ {
		v := uint8(255 - i*255/10)
		rechteck(cx-110+float64(i)*20, cy+60, cx-90+float64(i)*20, cy+75, color.RGBA{v, v, v, 255})
		text(fmt.Sprintf("%d%%", i*10), cx-108+float64(i)*20, cy+81, 2.5)
	}
	// Mittelkreuz und Diagonalen-Hilfslinien
	waag(cx-10, cx+10, cy, 0.2, grau)
	senk(cx, cy-10, cy+10, 0.2, grau)
	// Beschriftung
	komma := func(v float64) string { return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1) }
	text(fmt.Sprintf("Tri-Vertical Belichter – Testbogen  ·  Belichtungsfläche %s × %s cm  ·  Datei %s × %s cm, %.0f dpi",
		komma(e.FlaecheB), komma(e.FlaecheH), komma(e.DateiB), komma(e.DateiH), e.DPI), 15, 22, 4)
	text("Nachmessen: Die Quadrate müssen genau 10 × 10 cm groß sein, die Lineale am Rand ganz sichtbar.", 15, 30, 3.2)
	text("Abweichung? Gemessene Werte im Programm unter „Einrichtung“ eintragen.", 15, 36, 3.2)
	return img
}
