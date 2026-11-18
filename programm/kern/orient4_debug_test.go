package kern

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"os"
	"strconv"
	"strings"
	"testing"
)

// V3RENDER="datei|f35|ziel;..."
func TestOrientRender(t *testing.T) {
	l := os.Getenv("V3RENDER")
	if l == "" {
		t.Skip()
	}
	for _, e := range strings.Split(l, ";") {
		f := strings.Split(e, "|")
		fh, _ := os.Open(f[0])
		img, _, _ := image.Decode(fh)
		fh.Close()
		b := AusImage(img)
		f35, _ := strconv.ParseFloat(f[1], 64)
		r, err := AutoOrientierung(b, f35, false)
		fmt.Println(f[2], r.Roll, r.Pitch, err)
		if err != nil {
			continue
		}
		par, anteil := BegrenzeKorrektur(b.W, b.H, Parameter{Roll: r.Roll, Pitch: r.Pitch, F35: f35, Zuschneiden: true}, 1.6)
		fmt.Println("  begrenzt", par.Pitch, anteil)
		k, err := Korrigiere(b, par, [3]uint8{255, 255, 255}, true)
		if err != nil {
			fmt.Println(err)
			continue
		}
		w, _ := os.Create(f[2])
		jpeg.Encode(w, k.RGBA(), &jpeg.Options{Quality: 88})
		w.Close()
	}
}
