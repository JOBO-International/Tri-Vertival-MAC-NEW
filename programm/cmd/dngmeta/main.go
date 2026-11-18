package main

import (
	"fmt"
	"os"
	"vertical3/neigung"

	"vertical3/bild"
	"vertical3/profil"
)

func main() {
	P := profil.Lade("/home/claude/v3go/cmd/vertical3/standardprofile")
	for _, f := range os.Args[1:] {
		g, err := bild.Lade(f, "")
		if err != nil {
			fmt.Println(f, "FEHLER:", err)
			continue
		}
		p := profil.Passend(g.Meta, P)
		r, pi, ok, _ := neigung.Lies(g.Meta, p)
		fmt.Printf("%s: %dx%d | %s | Profil %s | Neigung ok=%v r=%.1f p=%.1f f35=%.0f | EXIF %d Bytes\n", f, g.Bild.W, g.Bild.H, g.Hinweis, p.Name(), ok, r, pi, profil.KBBrennweite(g.Meta, p), len(g.Exif))
	}
}
