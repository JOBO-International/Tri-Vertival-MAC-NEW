// Package ki füllt die leeren Ränder eines entzerrten Bildes mit KI auf –
// offline mit LaMa (ONNX) oder online über Stability AI.
package ki

import (
	"math"
	"runtime"
	"sort"
	"sync"

	"vertical3/kern"
)

// Fortschritt meldet den Stand (0…1) und einen kurzen Text.
type Fortschritt func(anteil float64, text string)

func melde(f Fortschritt, a float64, t string) {
	if f != nil {
		f(a, t)
	}
}

func parallel(n int, f func(a, b int)) {
	k := runtime.NumCPU()
	if k > n {
		k = n
	}
	if k < 1 {
		k = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		a, b := n*i/k, n*(i+1)/k
		wg.Add(1)
		go func() { defer wg.Done(); f(a, b) }()
	}
	wg.Wait()
}

// skaliere: bilineare Größenänderung (für Vergrößerung und mäßige Verkleinerung).
func skaliere(b *kern.Bild, w, h int) *kern.Bild {
	if b.W == w && b.H == h {
		return b
	}
	if w < b.W/2 && h < b.H/2 { // starke Verkleinerung: Flächenmittel
		k := b.Verkleinert(int(math.Max(float64(w), float64(h))))
		if k.W == w && k.H == h {
			return k
		}
		b = k
	}
	out := kern.NeuesBild(w, h)
	sx, sy := float64(b.W)/float64(w), float64(b.H)/float64(h)
	parallel(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				abtaste(b, (float64(x)+0.5)*sx-0.5, (float64(y)+0.5)*sy-0.5, out.Pix[(y*w+x)*3:])
			}
		}
	})
	return out
}

func abtaste(b *kern.Bild, x, y float64, ziel []uint8) {
	x = math.Max(0, math.Min(float64(b.W-1), x))
	y = math.Max(0, math.Min(float64(b.H-1), y))
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, b.W-1), min(y0+1, b.H-1)
	fx, fy := x-float64(x0), y-float64(y0)
	for c := 0; c < 3; c++ {
		p00 := float64(b.Pix[(y0*b.W+x0)*3+c])
		p10 := float64(b.Pix[(y0*b.W+x1)*3+c])
		p01 := float64(b.Pix[(y1*b.W+x0)*3+c])
		p11 := float64(b.Pix[(y1*b.W+x1)*3+c])
		ziel[c] = uint8((p00*(1-fx)+p10*fx)*(1-fy) + (p01*(1-fx)+p11*fx)*fy + 0.5)
	}
}

// maskeVerkleinert: ein Arbeitspixel gilt als leer, wenn irgendein
// Originalpixel darunter leer ist.
func maskeVerkleinert(m []uint8, w, h, nw, nh int) []uint8 {
	out := make([]uint8, nw*nh)
	sx, sy := float64(w)/float64(nw), float64(h)/float64(nh)
	parallel(nh, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			ya, yb := int(float64(y)*sy), int(math.Ceil(float64(y+1)*sy))
			for x := 0; x < nw; x++ {
				xa, xb := int(float64(x)*sx), int(math.Ceil(float64(x+1)*sx))
			such:
				for yy := ya; yy < min(yb, h); yy++ {
					for xx := xa; xx < min(xb, w); xx++ {
						if m[yy*w+xx] != 0 {
							out[y*nw+x] = 255
							break such
						}
					}
				}
			}
		}
	})
	return out
}

// weite: Maske um r Pixel vergrößern (quadratisch).
func weite(m []uint8, w, h, r int) []uint8 {
	if r <= 0 {
		return m
	}
	tmp := make([]uint8, len(m))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for d := -r; d <= r; d++ {
				xx := x + d
				if xx >= 0 && xx < w && m[y*w+xx] != 0 {
					tmp[y*w+x] = 255
					break
				}
			}
		}
	}
	out := make([]uint8, len(m))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for d := -r; d <= r; d++ {
				yy := y + d
				if yy >= 0 && yy < h && tmp[yy*w+x] != 0 {
					out[y*w+x] = 255
					break
				}
			}
		}
	}
	return out
}

// weich: Box-Unschärfe (2 Durchgänge) einer Maske, Ergebnis 0…1.
func weich(m []uint8, w, h, r int) []float32 {
	a := make([]float32, len(m))
	for i, v := range m {
		a[i] = float32(v) / 255
	}
	b := make([]float32, len(m))
	for durch := 0; durch < 2; durch++ {
		parallel(h, func(y0, y1 int) { // waagerecht
			for y := y0; y < y1; y++ {
				var s float32
				n := 0
				for x := -r; x < w+r; x++ {
					if x+r < w && x+r >= 0 {
						s += a[y*w+x+r]
						n++
					}
					if x-r-1 >= 0 && x-r-1 < w {
						s -= a[y*w+x-r-1]
						n--
					}
					if x >= 0 && x < w {
						b[y*w+x] = s / float32(n)
					}
				}
			}
		})
		parallel(w, func(x0, x1 int) { // senkrecht
			for x := x0; x < x1; x++ {
				var s float32
				n := 0
				for y := -r; y < h+r; y++ {
					if y+r < h && y+r >= 0 {
						s += b[(y+r)*w+x]
						n++
					}
					if y-r-1 >= 0 && y-r-1 < h {
						s -= b[(y-r-1)*w+x]
						n--
					}
					if y >= 0 && y < h {
						a[y*w+x] = s / float32(n)
					}
				}
			}
		})
	}
	return a
}

// einsetzen überträgt die KI-Füllung (beliebige Größe) in das Bild in voller
// Auflösung: innerhalb der Maske vollständig, an der Kante weich überblendet.
// Damit die Füllung nicht glatter wirkt als das Foto, bekommt sie ein
// feines Korn in der Stärke des Bildrauschens.
func einsetzen(orig *kern.Bild, maske []uint8, fuellung *kern.Bild) *kern.Bild {
	w, h := orig.W, orig.H
	out := &kern.Bild{W: w, H: h, Pix: append([]uint8(nil), orig.Pix...)}
	r := max(2, int(math.Round(float64(max(w, h))/1000)))
	alpha := weich(weite(maske, w, h, r), w, h, r)
	sx, sy := float64(fuellung.W)/float64(w), float64(fuellung.H)/float64(h)
	korn := 0.0
	if sx < 0.9 { // vergrößerte Füllung: Korn des Fotos nachbilden
		korn = rauschen(orig, maske) * math.Sqrt(1-sx*sx)
	}
	parallel(h, func(y0, y1 int) {
		var px [3]uint8
		zufall := uint32(y0*7919 + 12345)
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				a := float64(alpha[i])
				if maske[i] != 0 {
					a = 1
				}
				if a <= 0.002 {
					continue
				}
				abtaste(fuellung, (float64(x)+0.5)*sx-0.5, (float64(y)+0.5)*sy-0.5, px[:])
				n := 0.0
				if korn > 0 {
					// einfache Pseudozufallszahl, annähernd normalverteilt
					var s float64
					for k := 0; k < 4; k++ {
						zufall = zufall*1664525 + 1013904223
						s += float64(zufall>>8) / float64(1<<24)
					}
					n = (s - 2) * math.Sqrt(3) * korn * a
				}
				for c := 0; c < 3; c++ {
					v := a*(float64(px[c])+n) + (1-a)*float64(orig.Pix[i*3+c])
					out.Pix[i*3+c] = uint8(math.Max(0, math.Min(255, v+0.5)))
				}
			}
		}
	})
	return out
}

// rauschen schätzt das Bildrauschen (Standardabweichung der Helligkeit
// gegenüber dem Mittel der Nachbarn) in ruhigen Bildbereichen.
func rauschen(b *kern.Bild, maske []uint8) float64 {
	w, h := b.W, b.H
	lum := func(x, y int) float64 {
		o := (y*w + x) * 3
		return 0.299*float64(b.Pix[o]) + 0.587*float64(b.Pix[o+1]) + 0.114*float64(b.Pix[o+2])
	}
	var werte []float64
	schritt := max(3, min(w, h)/300)
	for y := 2; y < h-2; y += schritt {
		for x := 2; x < w-2; x += schritt {
			frei := true
			for dy := -2; dy <= 2 && frei; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if maske[(y+dy)*w+x+dx] != 0 {
						frei = false
						break
					}
				}
			}
			if !frei {
				continue
			}
			m := (lum(x-1, y) + lum(x+1, y) + lum(x, y-1) + lum(x, y+1)) / 4
			werte = append(werte, math.Abs(lum(x, y)-m))
		}
	}
	if len(werte) < 50 {
		return 0
	}
	// unteres Quartil = ruhige Flächen (Himmel, Wand), nicht Kanten
	sort.Float64s(werte)
	q := werte[len(werte)/4]
	return math.Min(6, q*1.25*0.9)
}
