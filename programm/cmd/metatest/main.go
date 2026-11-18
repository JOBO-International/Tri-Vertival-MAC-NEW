package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"vertical3/neigung"

	"vertical3/bild"
	"vertical3/kern"
	"vertical3/meta"
	"vertical3/profil"
)

func main() {
	ordner, profOrdner, out := os.Args[1], os.Args[2], os.Args[3]
	profile := profil.Lade(profOrdner)
	dateien, _ := filepath.Glob(filepath.Join(ordner, "*"))
	sort.Strings(dateien)
	for _, mit := range []bool{true, false} {
		et := ""
		if mit {
			et = meta.FindeExiftool("/nonexistent")
			fmt.Println("=== MIT exiftool")
		} else {
			fmt.Println("=== OHNE exiftool")
		}
		for _, f := range dateien {
			if !bild.IstBild(f) {
				continue
			}
			g, err := bild.Lade(f, et)
			if err != nil {
				fmt.Println(f, "FEHLER", err)
				continue
			}
			p := profil.Passend(g.Meta, profile)
			r, pi, ok, h := neigung.Lies(g.Meta, p)
			rs, ps := "—", "—"
			if ok {
				rs, ps = fmt.Sprintf("%+.1f", r), fmt.Sprintf("%+.1f", pi)
			}
			fmt.Printf("%-50s %-28.28s Roll %5s Pitch %5s f %.0f  %s\n", filepath.Base(f), p.Name(), rs, ps, profil.KBBrennweite(g.Meta, p), h)
		}
	}
	g, _ := bild.Lade(filepath.Join(ordner, "iPhone16Pro_Fassade.heic"), "")
	p := profil.Passend(g.Meta, profile)
	r, pi, _, _ := neigung.Lies(g.Meta, p)
	k, _ := kern.Korrigiere(g.Bild, kern.Parameter{Roll: r, Pitch: pi, F35: profil.KBBrennweite(g.Meta, p), Zuschneiden: true}, [3]uint8{255, 255, 255}, false)
	fmt.Println("Speichern:", bild.Speichere(k, out, g.Exif, g.ICC), "ICC Bytes", len(g.ICC))
}
