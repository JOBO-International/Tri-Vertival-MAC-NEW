package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"vertical3/bild"
	"vertical3/druck"
	"vertical3/kern"
)

func main() {
	g, _ := bild.Lade(os.Args[1], "")
	k, _ := kern.Korrigiere(g.Bild, kern.Parameter{Roll: -0.8, Pitch: -5.9, F35: 14, Zuschneiden: true}, [3]uint8{}, true)
	foto := k
	f, _ := opentype.Parse(goregular.TTF)
	const s = 3.0 // px/mm
	type fall struct {
		pw, ph  float64
		gr      string
		fw, fh  float64
		fuellen bool
		rahmen  string
		text    string
	}
	faelle := []fall{
		{297, 210, druck.GroesseMax, 0, 0, false, druck.RahmenKein, ""},
		{297, 210, druck.GroesseMax, 0, 0, false, druck.RahmenPassepartout, "Messe 2026 · Xiaomi 15 Ultra"},
		{297, 210, druck.GroesseMax, 0, 0, false, druck.RahmenGalerie, ""},
		{297, 210, druck.GroesseFest, 150, 100, false, druck.RahmenPolaroid, "Fotomesse, Oktober 2026"},
		{297, 210, druck.GroesseMax, 0, 0, false, druck.RahmenFilm, "VERTICAL3"},
		{297, 210, druck.GroesseMax, 0, 0, false, druck.RahmenLinie, ""},
		{150, 100, druck.GroesseRandlos, 0, 0, true, druck.RahmenKein, ""},
		{297, 210, druck.GroesseFest, 300, 200, true, druck.RahmenPassepartout, ""},
		{210, 297, druck.GroesseMax, 0, 0, true, druck.RahmenPassepartout, "Hochformat-Blatt, Format füllen"},
	}
	breite := 0
	var bilder []*image.RGBA
	for _, c := range faelle {
		a := druck.Auftrag{PapierW: c.pw, PapierH: c.ph, Druckbar: druck.Rechteck{X: 4, Y: 4, W: c.pw - 8, H: c.ph - 8},
			Groesse: c.gr, FestW: c.fw, FestH: c.fh, Fuellen: c.fuellen, Rahmen: c.rahmen, LinieMM: 2, LinieFarbe: [3]uint8{0, 0, 0},
			Text: c.text, TextPt: 11}
		e := druck.Berechne(a, foto.W, foto.H)
		fmt.Printf("%-13s %-8s %4.0fx%-4.0f fuellen=%-5v Feld %.1fx%.1f@%.1f,%.1f Foto %.1fx%.1f besch=%.2f verkl=%v\n",
			c.rahmen, c.gr, c.pw, c.ph, c.fuellen, e.Feld.W, e.Feld.H, e.Feld.X, e.Feld.Y, e.Foto.W, e.Foto.H, e.Beschnitt, e.Verkleinert)
		seite := image.NewRGBA(image.Rect(0, 0, int(c.pw*s), int(c.ph*s)))
		draw.Draw(seite, seite.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		feld := druck.Komponiere(foto, a, e, s, f)
		draw.Draw(seite, feld.Bounds().Add(image.Pt(int(e.Feld.X*s+0.5), int(e.Feld.Y*s+0.5))), feld, image.Point{}, draw.Src)
		bilder = append(bilder, seite)
		breite = max(breite, seite.Bounds().Dx())
	}
	// Übersicht 3 Spalten
	zw, zh := 0, 0
	for _, b := range bilder {
		zw, zh = max(zw, b.Bounds().Dx()), max(zh, b.Bounds().Dy())
	}
	out := image.NewRGBA(image.Rect(0, 0, 3*(zw+30)+30, 3*(zh+30)+30))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.RGBA{120, 120, 120, 255}), image.Point{}, draw.Src)
	for i, b := range bilder {
		x, y := 30+(i%3)*(zw+30), 30+(i/3)*(zh+30)
		draw.Draw(out, b.Bounds().Add(image.Pt(x, y)), b, image.Point{}, draw.Src)
	}
	o, _ := os.Create(os.Args[2])
	png.Encode(o, out)
	o.Close()
}
