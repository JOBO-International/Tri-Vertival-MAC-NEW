package kern

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestOrientVarianten(t *testing.T) {
	l := os.Getenv("V3VAR")
	if l == "" {
		t.Skip()
	}
	type bildT struct {
		name string
		b    *Bild
		f    float64
		soll string
	}
	var bilder []bildT
	for _, e := range strings.Split(l, ";") {
		f := strings.Split(e, "|")
		fh, err := os.Open(f[0])
		if err != nil {
			continue
		}
		img, _, _ := image.Decode(fh)
		fh.Close()
		f35, _ := strconv.ParseFloat(f[1], 64)
		bilder = append(bilder, bildT{f[0][strings.LastIndex(f[0], "/")+1:], AusImage(img), f35, f[2] + "/" + f[3]})
	}
	for _, v := range []struct {
		tg  int
		koh float64
		sa  float64
	}{{1, 0.6, 0.75}, {2, 0.6, 0.75}, {3, 0.7, 0.75}, {2, 0.75, 0.6}} {
		tensorGlaettung, kohMin, schwellAnteil = v.tg, v.koh, v.sa
		fmt.Printf("== Glättung %d koh %.2f schwelle %.2f\n", v.tg, v.koh, v.sa)
		for _, b := range bilder {
			r, err := AutoOrientierung(b.b, b.f, false)
			fs := ""
			if err != nil {
				fs = "FEHLT"
			}
			fmt.Printf("  %-24.24s soll %-12s ist %+6.2f %+6.2f g %.3f %s\n", b.name, b.soll, r.Roll, r.Pitch, r.Guete, fs)
		}
	}
}
