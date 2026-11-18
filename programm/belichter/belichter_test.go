package belichter

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"testing"

	"vertical3/filter"
	"vertical3/kern"
)

func TestAnordnung(t *testing.T) {
	for _, f := range Formate {
		p := Anordnung(f, 488, 386, 3)
		fmt.Printf("%-14s %2d Plätze\n", f.Name, len(p))
	}
}

func TestBogenRendern(t *testing.T) {
	d := os.Getenv("BELBILD")
	if d == "" {
		t.Skip()
	}
	fh, _ := os.Open(d)
	img, _, _ := image.Decode(fh)
	b := kern.AusImage(img)
	var sw filter.Einstellung
	for _, l := range filter.Vordefiniert {
		if l.Name == "S/W Rotfilter" {
			sw = l.Werte
		}
	}
	b = Grau(filter.Anwenden(b, sw))
	e := Standard()
	pw, ph := e.Pixel()
	f := Formate[5] // 13 × 18
	bg := Bogen{E: e, Plaetze: Anordnung(f, e.FlaecheB*10, e.FlaecheH*10, 3), Bilder: []*kern.Bild{b}, Schnittmarken: true, Abstand: 3}
	out := bg.Rendern(pw, ph)
	fmt.Println("Datei", pw, "x", ph, "Plätze", len(bg.Plaetze))
	for _, p := range bg.Plaetze[:2] {
		fmt.Printf("Platz %.1f,%.1f %.0fx%.0f mm -> %d x %d px\n", p.X, p.Y, p.B, p.H, int(p.B*float64(pw)/488+0.5), int(p.H*float64(ph)/386+0.5))
	}
	if err := SchreibeTIFF("/tmp/bel/bogen.tif", out, e.DPI); err != nil {
		t.Fatal(err)
	}
	o, _ := os.Create("/tmp/bel/bogen.png")
	png.Encode(o, out)
	o.Close()
	tb := Testbogen(e)
	SchreibeTIFF("/tmp/bel/testbogen.tif", tb, e.DPI)
	o, _ = os.Create("/tmp/bel/testbogen.png")
	png.Encode(o, tb)
	o.Close()
}
