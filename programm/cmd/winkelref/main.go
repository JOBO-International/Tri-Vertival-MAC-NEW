// winkelref: Referenzwerte der Go-Winkelschätzung für den Vergleich mit der C++-Version.
//
//	winkelref bild.jpg|png f35 ausgabe.xrgb
//
// Schreibt das Bild als XRGB (Zeilenabstand = Breite·4 + 16, Füllbytes 0xEE) und gibt
// "Breite Höhe Zeilenabstand Pitch Roll Güte NurRoll Unsicher" aus ("FEHLER" wenn keine Richtung).
package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"vertical3/kern"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		panic(err)
	}
	f35, _ := strconv.ParseFloat(os.Args[2], 64)
	b := kern.AusImage(img)
	// Optional: Bild vorher mit bekannten Winkeln verzerren ("pitch,roll", Kamera-Neigung)
	if w := os.Getenv("WARP"); w != "" {
		var wp, wr float64
		fmt.Sscanf(w, "%f,%f", &wp, &wr)
		k, err := kern.Korrigiere(b, kern.Parameter{Roll: wr, Pitch: wp, F35: f35}, [3]uint8{128, 128, 128}, false)
		if err != nil {
			panic(err)
		}
		b = k
	}
	stride := b.W*4 + 16
	out := make([]byte, stride*b.H)
	for i := range out {
		out[i] = 0xEE
	}
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			o := y*stride + x*4
			out[o] = 0
			copy(out[o+1:o+4], b.Pix[(y*b.W+x)*3:(y*b.W+x)*3+3])
		}
	}
	if err := os.WriteFile(os.Args[3], out, 0o644); err != nil {
		panic(err)
	}
	e, err := kern.AutoOrientierung(b, f35, false)
	if err != nil {
		fmt.Println(b.W, b.H, stride, "FEHLER")
		return
	}
	fmt.Println(b.W, b.H, stride, e.Pitch, e.Roll, e.Guete, e.NurRoll, e.Unsicher)
}
