// dngtest: DNG entwickeln (PNG) bzw. Rohdaten ausgeben – nur für Tests.
package main

import (
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"time"

	"vertical3/dng"
)

func main() {
	modus, ein, aus := os.Args[1], os.Args[2], os.Args[3]
	b, err := os.ReadFile(ein)
	if err != nil {
		panic(err)
	}
	t0 := time.Now()
	switch modus {
	case "roh":
		d, w, h, spp, err := dng.RohDaten(b)
		if err != nil {
			fmt.Println("FEHLER:", err)
			os.Exit(1)
		}
		f, _ := os.Create(aus)
		binary.Write(f, binary.LittleEndian, d)
		f.Close()
		fmt.Printf("roh %dx%dx%d in %v\n", w, h, spp, time.Since(t0))
	default:
		dng.NurGamma = modus == "linear"
		img, info, err := dng.Entwickle(b)
		if err != nil {
			fmt.Println("FEHLER:", err)
			os.Exit(1)
		}
		f, _ := os.Create(aus)
		png.Encode(f, img.RGBA())
		f.Close()
		fmt.Printf("entwickelt %dx%d (%s, Kompression %d, %d Bit) in %v\n", img.W, img.H, info.Typ, info.Kompression, info.Bits, time.Since(t0))
	}
}
