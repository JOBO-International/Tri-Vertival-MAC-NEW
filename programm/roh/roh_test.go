package roh

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// V3RAWORDNER=Ordner mit RAW-Dateien
func TestRaw(t *testing.T) {
	ord := os.Getenv("V3RAWORDNER")
	if ord == "" {
		t.Skip()
	}
	dat, _ := filepath.Glob(filepath.Join(ord, "*"))
	for _, f := range dat {
		if !IstRoh(filepath.Ext(f)) {
			continue
		}
		b, _ := os.ReadFile(f)
		img, info, err := Entwickle(b)
		w, h := 0, 0
		if img != nil {
			w, h = img.W, img.H
		}
		fmt.Printf("%-22s %-10s %-14s %5dx%-5d ausr %d  f %.1f/%.0f  %v\n", filepath.Base(f), info.Marke, info.Modell, w, h, info.Ausrichtung, info.Brennweite, info.Brennweite35, err)
	}
}

func TestRawBild(t *testing.T) {
	f := os.Getenv("V3RAWDATEI")
	if f == "" {
		t.Skip()
	}
	b, _ := os.ReadFile(f)
	img, _, err := Entwickle(b)
	if err != nil {
		t.Fatal(err)
	}
	o, _ := os.Create(os.Getenv("V3RAWPNG"))
	defer o.Close()
	png.Encode(o, img.RGBA())
}
