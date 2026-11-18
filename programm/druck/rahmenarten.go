package druck

// Zusätzliche Rahmenarten (ab Rahmen 1.1): Bilderrahmen-Leisten, Diarahmen,
// Kleinbildfilm, Büttenrand, Schatten, abgerundete Ecken, unscharfer Hintergrund.

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"vertical3/kern"
)

// rauschen: einfache, wiederholbare Pseudo-Zufallszahl 0…1.
func rauschen(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	return float64((h^(h>>16))&0xffff) / 65535
}

// glatt: weiches Rauschen (bilinear zwischen Gitterpunkten).
func glatt(x, y, zelle float64) float64 {
	gx, gy := x/zelle, y/zelle
	x0, y0 := math.Floor(gx), math.Floor(gy)
	fx, fy := gx-x0, gy-y0
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a := rauschen(int(x0), int(y0))
	b := rauschen(int(x0)+1, int(y0))
	c := rauschen(int(x0), int(y0)+1)
	d := rauschen(int(x0)+1, int(y0)+1)
	return (a*(1-fx)+b*fx)*(1-fy) + (c*(1-fx)+d*fx)*fy
}

func setze(img *image.RGBA, x, y int, r, g, b float64) {
	o := y*img.Stride + x*4
	img.Pix[o] = uint8(math.Max(0, math.Min(255, r)))
	img.Pix[o+1] = uint8(math.Max(0, math.Min(255, g)))
	img.Pix[o+2] = uint8(math.Max(0, math.Min(255, b)))
	img.Pix[o+3] = 255
}

func mische(img *image.RGBA, x, y int, c [3]float64, a float64) {
	if a <= 0 {
		return
	}
	o := y*img.Stride + x*4
	for k := 0; k < 3; k++ {
		img.Pix[o+k] = uint8(math.Max(0, math.Min(255, float64(img.Pix[o+k])*(1-a)+c[k]*a)))
	}
}

// abstandRechteck: Abstand eines Punktes zum Rechteck (0 innen).
func abstandRechteck(x, y float64, r image.Rectangle) float64 {
	dx := math.Max(math.Max(float64(r.Min.X)-x, 0), x-float64(r.Max.X))
	dy := math.Max(math.Max(float64(r.Min.Y)-y, 0), y-float64(r.Max.Y))
	return math.Hypot(dx, dy)
}

// schatten: weicher Schlagschatten des Rechtecks r.
func schatten(img *image.RGBA, r image.Rectangle, versatz, unschaerfe int, staerke float64) {
	s := r.Add(image.Pt(versatz, versatz))
	ber := s.Inset(-3 * unschaerfe).Intersect(img.Bounds())
	parallel(ber.Dy(), func(y0, y1 int) {
		for y := ber.Min.Y + y0; y < ber.Min.Y+y1; y++ {
			for x := ber.Min.X; x < ber.Max.X; x++ {
				d := abstandRechteck(float64(x)+0.5, float64(y)+0.5, s)
				a := staerke * math.Exp(-(d*d)/(2*float64(unschaerfe*unschaerfe)))
				mische(img, x, y, [3]float64{0, 0, 0}, a)
			}
		}
	})
}

// fasenKante: geschnittene Passepartout-Kante um das Foto (oben/links hell, unten/rechts grau).
func fasenKante(img *image.RGBA, fr image.Rectangle, breite int) {
	for i := 1; i <= breite; i++ {
		r := fr.Inset(-i)
		fuelle(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), [3]uint8{255, 255, 255})
		fuelle(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), [3]uint8{252, 252, 250})
		fuelle(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), [3]uint8{214, 212, 206})
		fuelle(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), [3]uint8{226, 224, 218})
	}
}

type leistenArt struct {
	farbe  [3]float64
	maser  float64 // Holzmaserung 0…1
	glanz  float64
	profil func(t float64) float64 // Helligkeit über die Leistenbreite (0 = außen, 1 = innen)
}

var leisten = map[string]leistenArt{
	RahmenHolzHell: {farbe: [3]float64{196, 160, 112}, maser: 1, glanz: 0.1,
		profil: func(t float64) float64 { return 0.88 + 0.2*math.Sin(math.Pi*math.Min(1, t*1.15)) }},
	RahmenNussbaum: {farbe: [3]float64{98, 64, 40}, maser: 1, glanz: 0.12,
		profil: func(t float64) float64 { return 0.85 + 0.25*math.Sin(math.Pi*math.Min(1, t*1.15)) }},
	RahmenSchwarz: {farbe: [3]float64{28, 28, 30}, glanz: 0.25,
		profil: func(t float64) float64 { return 0.9 + 0.35*math.Pow(math.Sin(math.Pi*t), 3) }},
	RahmenWeiss: {farbe: [3]float64{238, 237, 232}, glanz: 0.05,
		profil: func(t float64) float64 { return 0.93 + 0.07*math.Sin(math.Pi*t) }},
	RahmenGold: {farbe: [3]float64{190, 150, 70}, glanz: 0.45,
		profil: func(t float64) float64 {
			return 0.72 + 0.3*math.Abs(math.Sin(3.2*math.Pi*t)) + 0.15*math.Sin(math.Pi*t)
		}},
	RahmenMuseum: {farbe: [3]float64{24, 24, 24}, glanz: 0.15,
		profil: func(t float64) float64 { return 0.95 + 0.2*math.Sin(math.Pi*t) }},
}

// zeichneLeiste: Bilderrahmen-Leiste mit Gehrung an den Ecken.
func zeichneLeiste(img *image.RGBA, art leistenArt, breite int) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	bw := float64(breite)
	parallel(H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < W; x++ {
				dl, dr, do, du := x, W-1-x, y, H-1-y
				d := min(dl, dr, do, du)
				if d >= breite {
					continue
				}
				t := (float64(d) + 0.5) / bw
				var seite float64 // Licht von oben links
				var laengs, quer float64
				switch d {
				case do:
					seite, laengs, quer = 1.12, float64(x), float64(y)
				case dl:
					seite, laengs, quer = 1.04, float64(y), float64(x)
				case dr:
					seite, laengs, quer = 0.88, float64(y), float64(x)
				default:
					seite, laengs, quer = 0.8, float64(x), float64(y)
				}
				h := art.profil(t) * seite
				if art.maser > 0 { // Maserung längs der Leiste
					m := math.Sin(quer*0.9/math.Max(1, bw/40)+6*glatt(laengs, quer*3, bw*1.5))*0.5 + 0.5
					h *= 1 - 0.13*art.maser*m - 0.05*art.maser*glatt(laengs*4, quer, bw*0.6)
				}
				g := art.glanz * math.Max(0, math.Sin(math.Pi*t)-0.6) * 2.5 * (seite - 0.75)
				// Kanten: außen und innen dunkle Fuge
				if d == 0 || d == breite-1 {
					h *= 0.55
				}
				c := art.farbe
				setze(img, x, y, c[0]*h+255*g, c[1]*h+255*g, c[2]*h+255*g)
			}
		}
	})
	// Gehrungsfugen (feine Diagonalen)
	for i := 0; i < breite; i++ {
		for _, p := range [][2]int{{i, i}, {W - 1 - i, i}, {i, H - 1 - i}, {W - 1 - i, H - 1 - i}} {
			mische(img, p[0], p[1], [3]float64{0, 0, 0}, 0.25)
		}
	}
}

// vorFoto zeichnet alles, was unter dem Foto liegt.
func vorFoto(img *image.RGBA, a Auftrag, fr image.Rectangle, ppm float64, foto *kern.Bild, e Ergebnis) {
	b := img.Bounds()
	sPx := math.Min(float64(b.Dx()), float64(b.Dy()))
	switch a.Rahmen {
	case RahmenHolzHell, RahmenNussbaum, RahmenSchwarz, RahmenWeiss, RahmenGold, RahmenMuseum:
		l, m := leiste(a.Rahmen)
		s := (e.Foto.X - e.Feld.X) / (l + m) // Bezugsmaß aus dem linken Rand
		bw := max(2, int(math.Round(l*s*ppm)))
		zeichneLeiste(img, leisten[a.Rahmen], bw)
		fasenKante(img, fr, max(1, int(math.Round(sPx*0.004))))
	case RahmenDoppellinie:
		c := a.LinieFarbe
		d := max(1, int(math.Round(sPx*0.0018)))
		umriss(img, fr.Inset(-int(sPx*0.012)), d, c)
		umriss(img, fr.Inset(-int(sPx*0.024)), d, c)
	case RahmenSchatten:
		schatten(img, fr, int(sPx*0.008), max(1, int(sPx*0.014)), 0.45)
	case RahmenUnscharf:
		unscharferHintergrund(img, foto)
		schatten(img, fr, int(sPx*0.006), max(1, int(sPx*0.012)), 0.55)
	case RahmenBuetten:
		buettenPapier(img, sPx)
	case RahmenDia:
		diaRahmen(img, fr, sPx)
	}
}

// nachFoto zeichnet Dinge über dem Foto (Ecken, Filmkante, Randbeschriftung).
func nachFoto(img *image.RGBA, a Auftrag, fr image.Rectangle, ppm float64, e Ergebnis, schrift *opentype.Font) {
	b := img.Bounds()
	sPx := math.Min(float64(b.Dx()), float64(b.Dy()))
	switch a.Rahmen {
	case RahmenAbgerundet:
		rundeEcken(img, fr, math.Min(float64(fr.Dx()), float64(fr.Dy()))*0.045, RandFarbe(a))
	case RahmenDia:
		rundeEcken(img, fr, sPx*0.012, [3]uint8{236, 234, 228})
		// Schatten der Fensterkante auf dem Bild
		for i := 0; i < max(1, int(sPx*0.004)); i++ {
			r := fr.Inset(i)
			al := 0.35 * (1 - float64(i)/math.Max(1, sPx*0.004))
			for x := r.Min.X; x < r.Max.X; x++ {
				mische(img, x, r.Min.Y, [3]float64{0, 0, 0}, al)
			}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				mische(img, r.Min.X, y, [3]float64{0, 0, 0}, al)
			}
		}
	case RahmenFilmstreifen:
		filmPerforation(img, fr, sPx, a.Text == "", schrift)
	}
}

// rundeEcken: Ecken des Fotos abrunden (kantengeglättet).
func rundeEcken(img *image.RGBA, fr image.Rectangle, radius float64, hg [3]uint8) {
	r := int(math.Ceil(radius))
	c := [3]float64{float64(hg[0]), float64(hg[1]), float64(hg[2])}
	for _, ecke := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		for dy := 0; dy < r; dy++ {
			for dx := 0; dx < r; dx++ {
				// Abstand vom Kreismittelpunkt
				d := math.Hypot(radius-(float64(dx)+0.5), radius-(float64(dy)+0.5))
				al := math.Max(0, math.Min(1, d-radius+0.5))
				if al <= 0 {
					continue
				}
				x, y := fr.Min.X+dx, fr.Min.Y+dy
				if ecke[0] == 1 {
					x = fr.Max.X - 1 - dx
				}
				if ecke[1] == 1 {
					y = fr.Max.Y - 1 - dy
				}
				if image.Pt(x, y).In(img.Bounds()) {
					mische(img, x, y, c, al)
				}
			}
		}
	}
}

// unscharferHintergrund: vergrößertes, stark weichgezeichnetes Foto als Fläche.
func unscharferHintergrund(img *image.RGBA, foto *kern.Bild) {
	if foto == nil {
		return
	}
	klein := foto.Verkleinert(24)
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	// „Füllen“: Foto so skalieren, dass es die Fläche ganz bedeckt
	s := math.Max(float64(W)/float64(klein.W), float64(H)/float64(klein.H))
	ox := (float64(W) - float64(klein.W)*s) / 2
	oy := (float64(H) - float64(klein.H)*s) / 2
	parallel(H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < W; x++ {
				qx := (float64(x)-ox)/s - 0.5
				qy := (float64(y)-oy)/s - 0.5
				x0, y0 := int(math.Floor(qx)), int(math.Floor(qy))
				fx, fy := qx-float64(x0), qy-float64(y0)
				fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
				var c [3]float64
				for k := 0; k < 3; k++ {
					p := func(xx, yy int) float64 {
						xx, yy = max(0, min(klein.W-1, xx)), max(0, min(klein.H-1, yy))
						return float64(klein.Pix[(yy*klein.W+xx)*3+k])
					}
					c[k] = (p(x0, y0)*(1-fx)+p(x0+1, y0)*fx)*(1-fy) + (p(x0, y0+1)*(1-fx)+p(x0+1, y0+1)*fx)*fy
				}
				setze(img, x, y, c[0]*0.72, c[1]*0.72, c[2]*0.72)
			}
		}
	})
}

// buettenPapier: cremefarbener Abzug mit gezacktem Büttenrand auf grauem Grund.
func buettenPapier(img *image.RGBA, sPx float64) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	rand := sPx * 0.03   // Abstand des Papiers zum Bildrand
	amp := sPx * 0.011   // Zackenhöhe
	welle := sPx * 0.016 // Zackenabstand
	papier := [3]float64{249, 246, 237}
	parallel(H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < W; x++ {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				// Abstand nach innen zu jeder Kante, mit Zacken entlang der Kante
				zack := func(entlang float64) float64 {
					t := math.Mod(entlang/welle, 1)
					return amp * (math.Abs(t-0.5)*2*0.8 + 0.4*glatt(entlang, 0, welle*0.7))
				}
				innen := math.Min(math.Min(fx-rand-zack(fy), float64(W)-fx-rand-zack(fy+7)),
					math.Min(fy-rand-zack(fx+3), float64(H)-fy-rand-zack(fx+11)))
				if innen >= 0.5 {
					n := 0.985 + 0.03*glatt(fx, fy, sPx*0.05)
					setze(img, x, y, papier[0]*n, papier[1]*n, papier[2]*n)
				} else if innen > -0.5 {
					al := innen + 0.5
					mische(img, x, y, papier, al)
				} else if innen > -sPx*0.012 { // weicher Schatten unten rechts
					sch := math.Exp(innen / (sPx * 0.005))
					if fx > float64(W)/2 || fy > float64(H)/2 {
						mische(img, x, y, [3]float64{0, 0, 0}, 0.18*sch)
					}
				}
			}
		}
	})
}

// diaRahmen: Kunststoff-Diarahmen mit leicht gewölbtem Rand.
func diaRahmen(img *image.RGBA, fr image.Rectangle, sPx float64) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	wulst := sPx * 0.03
	parallel(H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < W; x++ {
				d := math.Min(math.Min(float64(x), float64(W-1-x)), math.Min(float64(y), float64(H-1-y)))
				h := 1.0
				if d < wulst { // gewölbter Außenrand, Licht von oben links
					t := d / wulst
					licht := 1.0
					if x > W-1-int(wulst) || y > H-1-int(wulst) {
						licht = -1
					}
					h = 1 + 0.05*licht*math.Cos(t*math.Pi/2) - 0.06*(1-t)
				}
				h *= 0.99 + 0.02*rauschen(x/2, y/2)
				setze(img, x, y, 226*h, 223*h, 214*h)
			}
		}
	})
	// eingeprägte Rille um das Fenster
	g := fr.Inset(-int(sPx * 0.025))
	umriss(img, g, max(1, int(sPx*0.002)), [3]uint8{218, 216, 210})
	umriss(img, g.Inset(max(1, int(sPx*0.002))), max(1, int(sPx*0.0015)), [3]uint8{246, 245, 240})
}

// filmPerforation: Löcher und Randbeschriftung eines Kleinbildfilms.
func filmPerforation(img *image.RGBA, fr image.Rectangle, sPx float64, randText bool, schrift *opentype.Font) {
	b := img.Bounds()
	quer := fr.Dx() >= fr.Dy()
	// Maßstab: Bildhöhe (kurze Seite) entspricht 24 mm
	kurz := float64(min(fr.Dx(), fr.Dy()))
	k := kurz / 24
	lochL, lochQ, teilung := 2.8*k, 1.98*k, 4.75*k
	abstand := 2.0 * k // Loch vom Filmrand
	loch := [3]float64{255, 255, 255}
	zeichneLoch := func(cx, cy, w, h float64) {
		r := math.Min(w, h) * 0.22
		for y := int(cy - h/2 - 1); y <= int(cy+h/2+1); y++ {
			for x := int(cx - w/2 - 1); x <= int(cx+w/2+1); x++ {
				if !image.Pt(x, y).In(b) {
					continue
				}
				// abgerundetes Rechteck, kantengeglättet
				dx := math.Max(math.Abs(float64(x)+0.5-cx)-(w/2-r), 0)
				dy := math.Max(math.Abs(float64(y)+0.5-cy)-(h/2-r), 0)
				d := math.Hypot(dx, dy) - r
				mische(img, x, y, loch, math.Max(0, math.Min(1, 0.5-d)))
			}
		}
	}
	if quer {
		for cx := float64(b.Min.X) + teilung/2; cx < float64(b.Max.X); cx += teilung {
			zeichneLoch(cx, float64(b.Min.Y)+abstand+lochQ/2, lochL, lochQ)
			zeichneLoch(cx, float64(b.Max.Y)-abstand-lochQ/2, lochL, lochQ)
		}
	} else {
		for cy := float64(b.Min.Y) + teilung/2; cy < float64(b.Max.Y); cy += teilung {
			zeichneLoch(float64(b.Min.X)+abstand+lochQ/2, cy, lochQ, lochL)
			zeichneLoch(float64(b.Max.X)-abstand-lochQ/2, cy, lochQ, lochL)
		}
	}
	// Randbeschriftung (Bildnummern) – nur ohne eigene Beschriftung
	if !randText || schrift == nil {
		return
	}
	face, err := opentype.NewFace(schrift, &opentype.FaceOptions{Size: 1.6 * k, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return
	}
	defer face.Close()
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{235, 140, 40, 255}), Face: face}
	if quer {
		y := b.Max.Y - int(abstand*0.55)
		d.Dot = fixed.P(fr.Min.X+int(4*k), y)
		d.DrawString("24")
		d.Dot = fixed.P(fr.Min.X+fr.Dx()/2, y)
		d.DrawString("24A")
		y2 := b.Min.Y + int(abstand*0.55+1.2*k)
		d.Dot = fixed.P(fr.Min.X+fr.Dx()/3, y2)
		d.DrawString("VERTICAL3 400")
	} else {
		d.Dot = fixed.P(b.Max.X-int(abstand*0.9), fr.Min.Y+int(6*k))
		d.DrawString("24")
		d.Dot = fixed.P(b.Max.X-int(abstand*0.9), fr.Min.Y+fr.Dy()/2)
		d.DrawString("24A")
	}
}
