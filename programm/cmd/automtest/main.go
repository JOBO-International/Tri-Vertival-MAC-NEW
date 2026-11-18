package main

import (
	"fmt"
	"os"
	"time"

	"vertical3/bild"
	"vertical3/kern"
)

func main() {
	for _, f := range os.Args[1:] {
		g, err := bild.Lade(f, "")
		if err != nil {
			fmt.Println(f, err)
			continue
		}
		for _, fr := range []bool{false, true} {
			t := time.Now()
			e, err := kern.AutoMarken(g.Bild, 28, fr)
			l := e.Loesung
			fmt.Printf("%-40s frontal=%v  Roll %+.1f Pitch %+.1f Yaw %+.1f  Formen %d verwendet %d  %v  err=%v\n", f[len(f)-min(40, len(f)):], fr, l.Roll, l.Pitch, l.Yaw, e.Gefunden, len(e.Marken), time.Since(t).Round(time.Millisecond), err)
		}
	}
}
