package kern

import (
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
)

// Bild ist ein einfaches RGB-Bild (3 Byte pro Pixel), schnell zu verarbeiten.
type Bild struct {
	W, H int
	Pix  []uint8
}

func NeuesBild(w, h int) *Bild { return &Bild{W: w, H: h, Pix: make([]uint8, w*h*3)} }

// AusImage wandelt ein beliebiges image.Image in ein Bild um.
func AusImage(img image.Image) *Bild {
	b := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok || b.Min != (image.Point{}) {
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	}
	out := NeuesBild(b.Dx(), b.Dy())
	parallel(out.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			s := rgba.Pix[y*rgba.Stride:]
			d := out.Pix[y*out.W*3:]
			for x := 0; x < out.W; x++ {
				d[x*3], d[x*3+1], d[x*3+2] = s[x*4], s[x*4+1], s[x*4+2]
			}
		}
	})
	return out
}

// RGBA wandelt in ein image.RGBA (zum Speichern / Anzeigen).
func (b *Bild) RGBA() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, b.W, b.H))
	parallel(b.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			s := b.Pix[y*b.W*3:]
			d := img.Pix[y*img.Stride:]
			for x := 0; x < b.W; x++ {
				d[x*4], d[x*4+1], d[x*4+2], d[x*4+3] = s[x*3], s[x*3+1], s[x*3+2], 255
			}
		}
	})
	return img
}

// Verkleinert liefert eine verkleinerte Kopie (Flächenmittel), längste Kante max.
func (b *Bild) Verkleinert(max int) *Bild {
	f := float64(max) / math.Max(float64(b.W), float64(b.H))
	if f >= 1 {
		return b
	}
	w, h := int(math.Max(1, math.Round(float64(b.W)*f))), int(math.Max(1, math.Round(float64(b.H)*f)))
	out := NeuesBild(w, h)
	sx, sy := float64(b.W)/float64(w), float64(b.H)/float64(h)
	parallel(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			ya, yb := int(float64(y)*sy), int(math.Min(float64(b.H), float64(y+1)*sy))
			if yb <= ya {
				yb = ya + 1
			}
			for x := 0; x < w; x++ {
				xa, xb := int(float64(x)*sx), int(math.Min(float64(b.W), float64(x+1)*sx))
				if xb <= xa {
					xb = xa + 1
				}
				var r, g, bl, n int
				for yy := ya; yy < yb; yy++ {
					row := b.Pix[(yy*b.W+xa)*3:]
					for xx := 0; xx < xb-xa; xx++ {
						r += int(row[xx*3])
						g += int(row[xx*3+1])
						bl += int(row[xx*3+2])
						n++
					}
				}
				o := (y*w + x) * 3
				out.Pix[o], out.Pix[o+1], out.Pix[o+2] = uint8(r/n), uint8(g/n), uint8(bl/n)
			}
		}
	})
	return out
}

// Gedreht dreht/spiegelt nach EXIF-Ausrichtung (1–8) in die Normallage.
func (b *Bild) Gedreht(orient int) *Bild {
	if orient <= 1 || orient > 8 {
		return b
	}
	w, h := b.W, b.H
	ow, oh := w, h
	if orient >= 5 {
		ow, oh = h, w
	}
	out := NeuesBild(ow, oh)
	parallel(oh, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < ow; x++ {
				var sx, sy int
				switch orient {
				case 2:
					sx, sy = w-1-x, y
				case 3:
					sx, sy = w-1-x, h-1-y
				case 4:
					sx, sy = x, h-1-y
				case 5:
					sx, sy = y, x
				case 6:
					sx, sy = y, h-1-x
				case 7:
					sx, sy = w-1-y, h-1-x
				case 8:
					sx, sy = w-1-y, x
				}
				s, d := (sy*w+sx)*3, (y*ow+x)*3
				out.Pix[d], out.Pix[d+1], out.Pix[d+2] = b.Pix[s], b.Pix[s+1], b.Pix[s+2]
			}
		}
	})
	return out
}

// Korrigiere wendet die Perspektivkorrektur an. schnell = bilinear (Vorschau),
// sonst bikubisch (Speichern).
func Korrigiere(b *Bild, p Parameter, rand [3]uint8, schnell bool) (*Bild, error) {
	M, ow, oh, err := Transformation(b.W, b.H, p)
	if err != nil {
		return nil, err
	}
	Mi := M.Inv()
	out := NeuesBild(ow, oh)
	parallel(oh, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < ow; x++ {
				fx, fy := float64(x), float64(y)
				q0 := Mi[0][0]*fx + Mi[0][1]*fy + Mi[0][2]
				q1 := Mi[1][0]*fx + Mi[1][1]*fy + Mi[1][2]
				q2 := Mi[2][0]*fx + Mi[2][1]*fy + Mi[2][2]
				sx, sy := q0/q2, q1/q2
				d := (y*ow + x) * 3
				if sx < -0.5 || sy < -0.5 || sx > float64(b.W)-0.5 || sy > float64(b.H)-0.5 || q2 <= 0 {
					out.Pix[d], out.Pix[d+1], out.Pix[d+2] = rand[0], rand[1], rand[2]
					continue
				}
				if schnell {
					bilinear(b, sx, sy, out.Pix[d:d+3])
				} else {
					bikubisch(b, sx, sy, out.Pix[d:d+3])
				}
			}
		}
	})
	return out, nil
}

func klemm(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func bilinear(b *Bild, x, y float64, ziel []uint8) {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	xa, xb := klemm(x0, 0, b.W-1), klemm(x0+1, 0, b.W-1)
	ya, yb := klemm(y0, 0, b.H-1), klemm(y0+1, 0, b.H-1)
	for c := 0; c < 3; c++ {
		p00 := float64(b.Pix[(ya*b.W+xa)*3+c])
		p10 := float64(b.Pix[(ya*b.W+xb)*3+c])
		p01 := float64(b.Pix[(yb*b.W+xa)*3+c])
		p11 := float64(b.Pix[(yb*b.W+xb)*3+c])
		v := (p00*(1-fx)+p10*fx)*(1-fy) + (p01*(1-fx)+p11*fx)*fy
		ziel[c] = uint8(v + 0.5)
	}
}

func kubGewicht(t float64) float64 {
	const a = -0.75 // wie OpenCV
	t = math.Abs(t)
	if t <= 1 {
		return ((a+2)*t-(a+3))*t*t + 1
	}
	if t < 2 {
		return ((a*t-5*a)*t+8*a)*t - 4*a
	}
	return 0
}

func bikubisch(b *Bild, x, y float64, ziel []uint8) {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	var wx, wy [4]float64
	var ix, iy [4]int
	for i := 0; i < 4; i++ {
		wx[i] = kubGewicht(fx - float64(i-1))
		wy[i] = kubGewicht(fy - float64(i-1))
		ix[i] = klemm(x0+i-1, 0, b.W-1)
		iy[i] = klemm(y0+i-1, 0, b.H-1)
	}
	for c := 0; c < 3; c++ {
		var s float64
		for j := 0; j < 4; j++ {
			row := iy[j] * b.W
			var z float64
			for i := 0; i < 4; i++ {
				z += wx[i] * float64(b.Pix[(row+ix[i])*3+c])
			}
			s += wy[j] * z
		}
		ziel[c] = uint8(math.Max(0, math.Min(255, s+0.5)))
	}
}

func parallel(n int, f func(a, b int)) {
	k := runtime.NumCPU()
	if k > n {
		k = n
	}
	if k <= 1 {
		f(0, n)
		return
	}
	var wg sync.WaitGroup
	block := (n + k - 1) / k
	for a := 0; a < n; a += block {
		b := a + block
		if b > n {
			b = n
		}
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			f(a, b)
		}(a, b)
	}
	wg.Wait()
}

// Anzeige skaliert das Bild bilinear auf w×h und zeichnet optional ein
// 8×8-Hilfsraster in der Farbe raster ein (für die Bildschirmvorschau).
func (b *Bild) Anzeige(w, h int, raster bool, farbe [3]uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	sx, sy := float64(b.W)/float64(w), float64(b.H)/float64(h)
	parallel(h, func(y0, y1 int) {
		var px [3]uint8
		for y := y0; y < y1; y++ {
			fy := (float64(y)+0.5)*sy - 0.5
			for x := 0; x < w; x++ {
				fx := (float64(x)+0.5)*sx - 0.5
				if sx > 1.5 { // stark verkleinern: Mittelwert eines kleinen Blocks
					mittel(b, fx, fy, sx, sy, px[:])
				} else {
					bilinear(b, fx, fy, px[:])
				}
				o := y*img.Stride + x*4
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = px[0], px[1], px[2], 255
			}
		}
	})
	if raster {
		for i := 1; i < 8; i++ {
			x, y := w*i/8, h*i/8
			for yy := 0; yy < h; yy++ {
				o := yy*img.Stride + x*4
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = farbe[0], farbe[1], farbe[2]
			}
			for xx := 0; xx < w; xx++ {
				o := y*img.Stride + xx*4
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = farbe[0], farbe[1], farbe[2]
			}
		}
	}
	return img
}

func mittel(b *Bild, fx, fy, sx, sy float64, ziel []uint8) {
	xa, xb := klemm(int(fx-sx/2+0.5), 0, b.W-1), klemm(int(fx+sx/2+0.5), 0, b.W-1)
	ya, yb := klemm(int(fy-sy/2+0.5), 0, b.H-1), klemm(int(fy+sy/2+0.5), 0, b.H-1)
	var s [3]int
	n := 0
	for y := ya; y <= yb; y++ {
		for x := xa; x <= xb; x++ {
			o := (y*b.W + x) * 3
			s[0] += int(b.Pix[o])
			s[1] += int(b.Pix[o+1])
			s[2] += int(b.Pix[o+2])
			n++
		}
	}
	ziel[0], ziel[1], ziel[2] = uint8(s[0]/n), uint8(s[1]/n), uint8(s[2]/n)
}
