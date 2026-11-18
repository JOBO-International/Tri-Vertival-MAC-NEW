// Package druck berechnet den Seitenaufbau für den Fotodruck (Format,
// Einpassen, Rahmen, Beschriftung) und setzt das Bild in beliebiger
// Auflösung zusammen – für Vorschau, Drucker und Bildexport gleichermaßen.
package druck

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"vertical3/kern"
)

// Rechteck in Millimetern (Seitenkoordinaten, Ursprung oben links).
type Rechteck struct{ X, Y, W, H float64 }

// Rahmenarten
const (
	RahmenKein         = "kein"
	RahmenLinie        = "linie"
	RahmenPassepartout = "passepartout"
	RahmenGalerie      = "galerie"
	RahmenPolaroid     = "polaroid"
	RahmenFilm         = "film"
	// ab Rahmen 1.1
	RahmenDoppellinie  = "doppellinie"
	RahmenSchatten     = "schatten"
	RahmenAbgerundet   = "abgerundet"
	RahmenHolzHell     = "holz_hell"
	RahmenNussbaum     = "nussbaum"
	RahmenSchwarz      = "leiste_schwarz"
	RahmenWeiss        = "leiste_weiss"
	RahmenGold         = "gold"
	RahmenMuseum       = "museum"
	RahmenDia          = "dia"
	RahmenFilmstreifen = "filmstreifen"
	RahmenBuetten      = "buetten"
	RahmenUnscharf     = "unscharf"
	// ab Rahmen 1.4
	RahmenPassepartoutRein = "passepartout_rein" // weißer Rand ohne Linie ums Foto
)

// Größen
const (
	GroesseMax     = "max"     // druckbarer Bereich
	GroesseRandlos = "randlos" // ganzes Blatt
	GroesseFest    = "fest"    // FestW × FestH
)

// Auftrag beschreibt, wie gedruckt werden soll.
type Auftrag struct {
	PapierW, PapierH float64  // Blatt in mm, bereits gedreht
	Druckbar         Rechteck // bedruckbarer Bereich in mm
	Groesse          string
	FestW, FestH     float64 // mm; wird wie das Foto ausgerichtet
	Fuellen          bool    // true = Format füllen (beschneiden), false = ganzes Foto
	VerschiebX       float64 // Beschnitt-Lage −1 … +1 (0 = Mitte)
	VerschiebY       float64
	Rahmen           string
	LinieMM          float64
	LinieFarbe       [3]uint8
	Text             string
	TextPt           float64
}

// Ergebnis: Lage aller Teile auf dem Blatt.
type Ergebnis struct {
	Feld        Rechteck   // Foto samt Rahmen und Beschriftung
	Foto        Rechteck   // sichtbares Foto
	TextFeld    Rechteck   // Bereich der Beschriftung (W = 0: keine)
	Beschnitt   [4]float64 // Ausschnitt der Quelle als Anteile x0, y0, x1, y1
	Verkleinert bool       // Wunschgröße passte nicht auf das Blatt
}

type raender struct{ l, o, r, u float64 }

// RandFarbe: Farbe der Rahmenfläche.
func RandFarbe(a Auftrag) [3]uint8 {
	switch a.Rahmen {
	case RahmenPolaroid:
		return [3]uint8{249, 248, 244}
	case RahmenFilm:
		return [3]uint8{12, 12, 12}
	case RahmenLinie:
		return a.LinieFarbe
	case RahmenSchatten:
		return [3]uint8{238, 238, 236}
	case RahmenHolzHell, RahmenNussbaum, RahmenSchwarz, RahmenWeiss, RahmenGold, RahmenMuseum:
		return [3]uint8{250, 250, 247} // Passepartout
	case RahmenDia:
		return [3]uint8{236, 234, 228}
	case RahmenFilmstreifen:
		return [3]uint8{22, 18, 14}
	case RahmenBuetten:
		return [3]uint8{226, 224, 219}
	case RahmenUnscharf:
		return [3]uint8{60, 60, 60}
	}
	return [3]uint8{255, 255, 255}
}

// TextFarbe: Schriftfarbe passend zum Rahmen.
func TextFarbe(a Auftrag) [3]uint8 {
	switch a.Rahmen {
	case RahmenFilm, RahmenUnscharf:
		return [3]uint8{235, 235, 235}
	case RahmenFilmstreifen:
		return [3]uint8{235, 140, 40}
	}
	return [3]uint8{40, 40, 40}
}

func textHoeheMM(a Auftrag) float64 {
	if a.Text == "" {
		return 0
	}
	pt := a.TextPt
	if pt <= 0 {
		pt = 10
	}
	return pt * 25.4 / 72 * 2.2
}

// leiste: Breite der Rahmenleiste und des Passepartouts (Anteile von s).
func leiste(art string) (leisteA, mattA float64) {
	switch art {
	case RahmenGold:
		return 0.075, 0.07
	case RahmenMuseum:
		return 0.018, 0.13
	case RahmenHolzHell, RahmenNussbaum, RahmenSchwarz, RahmenWeiss:
		return 0.055, 0.07
	}
	return 0, 0
}

// textUnterFoto: Beschriftung direkt unter dem Foto (innerhalb des Rahmens).
func textUnterFoto(art string) bool {
	switch art {
	case RahmenDoppellinie, RahmenSchatten, RahmenAbgerundet, RahmenHolzHell, RahmenNussbaum, RahmenSchwarz,
		RahmenWeiss, RahmenGold, RahmenMuseum, RahmenDia, RahmenBuetten, RahmenUnscharf:
		return true
	}
	return false
}

func rahmenRaender(a Auftrag, s float64, quer bool) raender {
	var r raender
	if l, m := leiste(a.Rahmen); l > 0 {
		w := (l + m) * s
		r = raender{w, w, w, w + 0.01*s}
	}
	switch a.Rahmen {
	case RahmenDoppellinie:
		m := 0.065 * s
		r = raender{m, m, m, m}
	case RahmenSchatten, RahmenUnscharf:
		m := 0.075 * s
		r = raender{m, m, m, m}
	case RahmenAbgerundet:
		m := 0.04 * s
		r = raender{m, m, m, m}
	case RahmenDia:
		m := 0.15 * s
		r = raender{m, m, m, m}
	case RahmenBuetten:
		m := 0.085 * s
		r = raender{m, m, m, m}
	case RahmenFilmstreifen:
		lang, kurz := 0.17*s, 0.035*s
		if quer {
			r = raender{kurz, lang, kurz, lang} // Beschriftung steht im Filmrand unten
		} else {
			r = raender{lang, kurz, lang, kurz + textHoeheMM(a)}
		}
		return r
	}
	switch a.Rahmen {
	case RahmenLinie:
		m := a.LinieMM
		if m <= 0 {
			m = 1
		}
		r = raender{m, m, m, m}
	case RahmenPassepartout, RahmenPassepartoutRein:
		m := 0.08 * s
		r = raender{m, m, m, m * 1.15} // unten etwas breiter wirkt ausgewogen
	case RahmenGalerie:
		m := 0.07 * s
		r = raender{m, m, m, m}
	case RahmenPolaroid:
		m := 0.055 * s
		r = raender{m, m, m, 0.2 * s}
	case RahmenFilm:
		m := 0.045 * s
		r = raender{m, m, m, m}
	}
	if t := textHoeheMM(a); t > 0 && a.Rahmen != RahmenPolaroid {
		r.u += t
	}
	return r
}

// Berechne legt Foto und Rahmen auf dem Blatt fest.
func Berechne(a Auftrag, fotoW, fotoH int) Ergebnis {
	var e Ergebnis
	quer := fotoW > fotoH
	// verfügbarer Bereich
	var A Rechteck
	switch a.Groesse {
	case GroesseRandlos:
		A = Rechteck{0, 0, a.PapierW, a.PapierH}
	case GroesseFest:
		w, h := a.FestW, a.FestH
		if (w > h) != quer && w != h {
			w, h = h, w
		}
		d := a.Druckbar
		if w > d.W || h > d.H {
			f := math.Min(d.W/w, d.H/h)
			w, h = w*f, h*f
			e.Verkleinert = true
		}
		A = Rechteck{d.X + (d.W-w)/2, d.Y + (d.H-h)/2, w, h}
	default:
		A = a.Druckbar
	}
	s := math.Min(A.W, A.H)
	rd := rahmenRaender(a, s, quer)
	P := Rechteck{A.X + rd.l, A.Y + rd.o, A.W - rd.l - rd.r, A.H - rd.o - rd.u}
	if P.W < 1 || P.H < 1 {
		P = Rechteck{A.X, A.Y, math.Max(A.W, 1), math.Max(A.H, 1)}
	}
	ff := float64(fotoW) / float64(fotoH)
	e.Beschnitt = [4]float64{0, 0, 1, 1}
	if a.Fuellen {
		e.Foto = P
		pf := P.W / P.H
		if ff > pf { // Foto breiter → links/rechts beschneiden
			anteil := pf / ff
			x0 := (1 - anteil) / 2 * (1 + clamp(a.VerschiebX))
			e.Beschnitt = [4]float64{x0, 0, x0 + anteil, 1}
		} else {
			anteil := ff / pf
			y0 := (1 - anteil) / 2 * (1 + clamp(a.VerschiebY))
			e.Beschnitt = [4]float64{0, y0, 1, y0 + anteil}
		}
		e.Feld = A
	} else {
		w, h := P.W, P.W/ff
		if h > P.H {
			h, w = P.H, P.H*ff
		}
		fw, fh := w+rd.l+rd.r, h+rd.o+rd.u
		fx, fy := A.X+(A.W-fw)/2, A.Y+(A.H-fh)/2
		e.Feld = Rechteck{fx, fy, fw, fh}
		e.Foto = Rechteck{fx + rd.l, fy + rd.o, w, h}
	}
	if t := textHoeheMM(a); t > 0 {
		unten := e.Feld.Y + e.Feld.H
		if a.Rahmen == RahmenPolaroid {
			oben := e.Foto.Y + e.Foto.H
			e.TextFeld = Rechteck{e.Feld.X, oben, e.Feld.W, unten - oben}
		} else if a.Rahmen == RahmenFilmstreifen { // im unteren bzw. rechten Filmrand
			if quer { // zwischen Bild und Perforation
				e.TextFeld = Rechteck{e.Feld.X, e.Foto.Y + e.Foto.H, e.Feld.W, 0.055 * s}
			} else {
				e.TextFeld = Rechteck{e.Feld.X, unten - t, e.Feld.W, t}
			}
		} else if textUnterFoto(a.Rahmen) {
			e.TextFeld = Rechteck{e.Feld.X, e.Foto.Y + e.Foto.H + 0.012*s, e.Feld.W, t}
		} else {
			e.TextFeld = Rechteck{e.Feld.X, unten - t, e.Feld.W, t}
		}
	}
	return e
}

func clamp(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// Komponiere erzeugt das Feld (Foto + Rahmen + Text) mit pxProMM Bildpunkten je mm.
func Komponiere(foto *kern.Bild, a Auftrag, e Ergebnis, pxProMM float64, schrift *opentype.Font) *image.RGBA {
	W := max(1, int(math.Round(e.Feld.W*pxProMM)))
	H := max(1, int(math.Round(e.Feld.H*pxProMM)))
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	rf := RandFarbe(a)
	fuelle(img, image.Rect(0, 0, W, H), rf)
	px := func(r Rechteck) image.Rectangle { // Seiten-mm → Pixel im Feld
		x0 := int(math.Round((r.X - e.Feld.X) * pxProMM))
		y0 := int(math.Round((r.Y - e.Feld.Y) * pxProMM))
		x1 := int(math.Round((r.X + r.W - e.Feld.X) * pxProMM))
		y1 := int(math.Round((r.Y + r.H - e.Feld.Y) * pxProMM))
		return image.Rect(x0, y0, x1, y1)
	}
	fr := px(e.Foto)
	strich := func(mm float64) int { return max(1, int(math.Round(mm*pxProMM))) }

	// Rahmen-Details
	switch a.Rahmen {
	case RahmenGalerie: // feine schwarze Linie im weißen Rand, mit weißem Außenrand
		rand := fr.Min.X // linker Rand in Pixeln
		abstand := int(math.Round(float64(rand) * 0.4))
		umriss(img, img.Bounds().Inset(abstand), strich(0.8), [3]uint8{20, 20, 20})
	case RahmenPassepartout: // Schattenkante um das Foto
		umriss(img, fr.Inset(-strich(0.6)), strich(0.6), [3]uint8{205, 205, 200})
	}

	vorFoto(img, a, fr, pxProMM, foto, e)

	// Foto
	if foto != nil && !fr.Empty() {
		b := e.Beschnitt
		skaliere(foto, b[0]*float64(foto.W), b[1]*float64(foto.H), (b[2]-b[0])*float64(foto.W), (b[3]-b[1])*float64(foto.H), img, fr)
	}
	if a.Rahmen == RahmenFilm { // dünne helle Kante um das Bild
		umriss(img, fr.Inset(-strich(0.4)), strich(0.25), [3]uint8{90, 90, 90})
	}
	nachFoto(img, a, fr, pxProMM, e, schrift)

	// Beschriftung
	if a.Text != "" && e.TextFeld.W > 0 && schrift != nil {
		pt := a.TextPt
		if pt <= 0 {
			pt = 10
		}
		groessePx := pt * 25.4 / 72 * pxProMM
		face, err := opentype.NewFace(schrift, &opentype.FaceOptions{Size: groessePx, DPI: 72, Hinting: font.HintingNone})
		if err == nil {
			tr := px(e.TextFeld)
			tf := TextFarbe(a)
			d := &font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{tf[0], tf[1], tf[2], 255}), Face: face}
			text := a.Text
			// zu lang? kürzen
			for len([]rune(text)) > 3 && d.MeasureString(text).Ceil() > tr.Dx()-2*int(groessePx) {
				r := []rune(text)
				text = string(r[:len(r)-2]) + "…"
			}
			breite := d.MeasureString(text).Ceil()
			m := face.Metrics()
			hoehe := (m.Ascent + m.Descent).Ceil()
			x := tr.Min.X + (tr.Dx()-breite)/2
			y := tr.Min.Y + (tr.Dy()-hoehe)/2 + m.Ascent.Ceil()
			if a.Rahmen == RahmenPolaroid { // handschriftlich wirkende Lage: etwas über der Mitte
				y -= tr.Dy() / 10
			}
			d.Dot = fixed.P(x, y)
			d.DrawString(text)
			face.Close()
		}
	}
	return img
}

func fuelle(img *image.RGBA, r image.Rectangle, c [3]uint8) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		o := y*img.Stride + r.Min.X*4
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = c[0], c[1], c[2], 255
			o += 4
		}
	}
}

func umriss(img *image.RGBA, r image.Rectangle, d int, c [3]uint8) {
	fuelle(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+d), c)
	fuelle(img, image.Rect(r.Min.X, r.Max.Y-d, r.Max.X, r.Max.Y), c)
	fuelle(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+d, r.Max.Y), c)
	fuelle(img, image.Rect(r.Max.X-d, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// skaliere überträgt den Quellausschnitt (sx, sy, sw, sh in Quellpixeln) in das
// Zielrechteck – mit Flächenmittelung beim Verkleinern, bilinear beim Vergrößern.
func skaliere(q *kern.Bild, sx, sy, sw, sh float64, ziel *image.RGBA, zr image.Rectangle) {
	zw, zh := zr.Dx(), zr.Dy()
	if zw <= 0 || zh <= 0 {
		return
	}
	fx, fy := sw/float64(zw), sh/float64(zh)
	wert := func(x, y int) (int, int, int) {
		x = max(0, min(q.W-1, x))
		y = max(0, min(q.H-1, y))
		o := (y*q.W + x) * 3
		return int(q.Pix[o]), int(q.Pix[o+1]), int(q.Pix[o+2])
	}
	parallel(zh, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			ty := zr.Min.Y + y
			if ty < 0 || ty >= ziel.Bounds().Max.Y {
				continue
			}
			for x := 0; x < zw; x++ {
				tx := zr.Min.X + x
				if tx < 0 || tx >= ziel.Bounds().Max.X {
					continue
				}
				var r, g, b int
				if fx > 1.2 || fy > 1.2 { // Mittelwert über die Quellfläche
					qa, qb := sx+float64(x)*fx, sy+float64(y)*fy
					xa, xb := int(qa), max(int(qa)+1, int(qa+fx))
					ya, yb := int(qb), max(int(qb)+1, int(qb+fy))
					n := 0
					for yy := ya; yy < yb; yy++ {
						for xx := xa; xx < xb; xx++ {
							rr, gg, bb := wert(xx, yy)
							r, g, b, n = r+rr, g+gg, b+bb, n+1
						}
					}
					r, g, b = r/n, g/n, b/n
				} else { // bilinear
					px := sx + (float64(x)+0.5)*fx - 0.5
					py := sy + (float64(y)+0.5)*fy - 0.5
					x0, y0 := int(math.Floor(px)), int(math.Floor(py))
					ax, ay := px-float64(x0), py-float64(y0)
					r00, g00, b00 := wert(x0, y0)
					r10, g10, b10 := wert(x0+1, y0)
					r01, g01, b01 := wert(x0, y0+1)
					r11, g11, b11 := wert(x0+1, y0+1)
					mix := func(a, b, c, d int) int {
						return int(math.Round((float64(a)*(1-ax)+float64(b)*ax)*(1-ay) + (float64(c)*(1-ax)+float64(d)*ax)*ay))
					}
					r, g, b = mix(r00, r10, r01, r11), mix(g00, g10, g01, g11), mix(b00, b10, b01, b11)
				}
				o := ty*ziel.Stride + tx*4
				ziel.Pix[o], ziel.Pix[o+1], ziel.Pix[o+2], ziel.Pix[o+3] = uint8(r), uint8(g), uint8(b), 255
			}
		}
	})
}

func parallel(n int, f func(a, b int)) {
	k := runtime.NumCPU()
	if k > n {
		k = max(1, n)
	}
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		a, b := n*i/k, n*(i+1)/k
		wg.Add(1)
		go func() { defer wg.Done(); f(a, b) }()
	}
	wg.Wait()
}

// Formate für die Auswahl (cm, Breite × Höhe im Hochformat).
var Formate = []struct {
	Name string
	W, H float64 // mm
}{
	{"9 × 13 cm", 90, 130}, {"10 × 15 cm", 100, 150}, {"13 × 18 cm", 130, 180},
	{"15 × 20 cm", 150, 200}, {"18 × 24 cm", 180, 240}, {"20 × 30 cm", 200, 300},
	{"24 × 30 cm", 240, 300}, {"30 × 40 cm", 300, 400}, {"30 × 45 cm", 300, 450},
	{"A5 (14,8 × 21 cm)", 148, 210}, {"A4 (21 × 29,7 cm)", 210, 297}, {"A3 (29,7 × 42 cm)", 297, 420},
}
