package main

import (
	"fmt"
	"vertical3/neigung"

	"vertical3/meta"
	"vertical3/profil"
)

func main() {
	P := profil.Lade("/home/claude/v3go/cmd/vertical3/standardprofile")
	for _, d := range []meta.Daten{
		{"Make": "NIKON CORPORATION", "Model": "NIKON Z 9", "Nikon:RollAngle": "-1.5", "RollAngle": "-1.5", "Nikon:PitchAngle": "12.3", "PitchAngle": "12.3"},
		{"Make": "NIKON CORPORATION", "Model": "NIKON Z 9"},
		{"Make": "NIKON CORPORATION", "Model": "NIKON D850", "RollAngle": "268.0", "PitchAngle": "-4"},
	} {
		p := profil.Passend(d, P)
		r, pi, ok, _ := neigung.Lies(d, p)
		fmt.Printf("%-40s roll %.1f pitch %.1f ok=%v | ohne exiftool: %q\n", p.Name(), r, pi, ok, p.Text("hinweis_ohne_exiftool")[:40])
	}
	fmt.Println(profil.Pruefe(P[3].Roh))
}
