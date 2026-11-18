package main

import (
	"fmt"
	"os"
	"vertical3/neigung"

	"vertical3/meta"
	"vertical3/profil"
)

func main() {
	b, _ := os.ReadFile(os.Args[1])
	d := meta.Daten{}
	o := meta.LiesExif(b, d)
	meta.LiesXMP(b, d)
	P := profil.Lade("/home/claude/v3go/cmd/vertical3/standardprofile")
	p := profil.Passend(d, P)
	r, pi, ok, _ := neigung.Lies(d, p)
	fmt.Printf("Make=%q Model=%q Ausrichtung=%d Vektor=%q f35=%s | Profil %s | Roll %.2f Pitch %.2f ok=%v\n", d["Make"], d["Model"], o, d["AccelerationVector"], d["FocalLengthIn35mmFormat"], p.Name(), r, pi, ok)
}
