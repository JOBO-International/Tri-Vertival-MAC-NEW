package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"vertical3/neigung"

	"image"
	_ "image/jpeg"

	"vertical3/kern"
)

func main() {
	fmt.Println("== Transformation (Größe)")
	for _, p := range []kern.Parameter{{-0.73, -5.61, 0, 14, true, false}, {2.5, 18, 0, 23, true, false}, {-4, -25, 10, 14, true, false}, {1, 10, -20, 24, true, false}, {2.5, 18, 0, 23, false, false}} {
		_, w, h, err := kern.Transformation(1662, 1247, p)
		fmt.Println(p, w, h, err)
	}
	fmt.Println("== Kanten")
	W, H, f35 := 4000, 3000, 24.0
	K := kern.Kameramatrix(f35, W, H)
	for _, rp := range [][2]float64{{3, 12}, {-5, -20}, {8, 25}, {-12, 35}} {
		R := kern.Drehmatrix(rp[0], rp[1], 0)
		var Rt kern.Mat3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				Rt[i][j] = R[j][i]
			}
		}
		proj := func(x, y, z float64) (float64, float64) {
			p := K.Mul(Rt).MulVec([3]float64{x, y, z})
			return p[0] / p[2], p[1] / p[2]
		}
		var ls []kern.Linie
		for _, x := range []float64{-4, 3} {
			a, b := proj(x, -6, 10)
			c, d := proj(x, 2, 10)
			ls = append(ls, kern.Linie{a, b, c, d})
		}
		r, p, _ := kern.WinkelAusSenkrechten(ls, W, H, f35)
		ha, hb := proj(-5, 0, 10)
		hc, hd := proj(5, 0, 10)
		fmt.Printf("echt %v -> %.3f %.3f | Horizont %.3f\n", rp, r, p, kern.RollAusHorizont(kern.Linie{ha, hb, hc, hd}))
	}
	fmt.Println("== Schwerkraft")
	for _, rp := range [][3]float64{{3, 12, 0}, {-5, -20, 0}, {4, 15, 1}, {-2, 8, 3}} {
		r, p := rp[0]*math.Pi/180, rp[1]*math.Pi/180
		d := [3]float64{math.Sin(r) * math.Cos(p), math.Cos(r) * math.Cos(p), -math.Sin(p)}
		for i := 0; i < int(rp[2]); i++ {
			d = [3]float64{-d[1], d[0], d[2]}
		}
		apple := [3]float64{d[0], -d[1], -d[2]}
		a, b, ok := neigung.NeigungAusSchwerkraft(apple, [3]float64{1, -1, -1})
		fmt.Printf("echt %v -> %.3f %.3f %v\n", rp, a, b, ok)
	}
	// Entzerrung eines Bildes für Pixelvergleich
	f, _ := os.Open(os.Args[1])
	img, _, _ := image.Decode(f)
	b := kern.AusImage(img)
	out, err := kern.Korrigiere(b, kern.Parameter{2.5, 18, 0, 23, false, false}, [3]uint8{255, 255, 255}, false)
	fmt.Println("warp", out.W, out.H, err)
	o, _ := os.Create(os.Args[2])
	png.Encode(o, out.RGBA())
	o.Close()
}
