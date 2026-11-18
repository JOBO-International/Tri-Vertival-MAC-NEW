package kern

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// V3LISTE="datei|f35|roll|pitch;..."
func TestOrientDebug(t *testing.T) {
	l := os.Getenv("V3LISTE")
	if l == "" {
		t.Skip()
	}
	if v := os.Getenv("V3KG"); v != "" {
		kohGrossMin, _ = strconv.ParseFloat(v, 64)
	}
	if v := os.Getenv("V3WG"); v != "" {
		waagerechtGewicht, _ = strconv.ParseFloat(v, 64)
	}
	if v := os.Getenv("V3WS"); v != "" {
		waagSpitzeMin, _ = strconv.ParseFloat(v, 64)
	}
	if v := os.Getenv("V3GG"); v != "" {
		grossGlaettung, _ = strconv.Atoi(v)
	}
	for _, e := range strings.Split(l, ";") {
		f := strings.Split(e, "|")
		fh, err := os.Open(f[0])
		if err != nil {
			fmt.Println(err)
			continue
		}
		img, _, _ := image.Decode(fh)
		fh.Close()
		b := AusImage(img)
		f35, _ := strconv.ParseFloat(f[1], 64)
		if os.Getenv("V3RECHTECK") != "" {
			am, err := AutoMarken(b, f35, false)
			fmt.Printf("%-28.28s RECHTECKE ist %+6.2f %+6.2f  %d Formen  %v\n", f[0][strings.LastIndex(f[0], "/")+1:], am.Loesung.Roll, am.Loesung.Pitch, len(am.Marken), err)
			continue
		}
		t0 := time.Now()
		r, err := AutoOrientierung(b, f35, os.Getenv("V3FRONTAL") != "")
		fmt.Printf("%-28.28s soll %6s %6s  ist %+6.2f %+6.2f yaw %+6.2f güte %.3f nurRoll=%v unsicher=%v  %v  %v\n", f[0][strings.LastIndex(f[0], "/")+1:], f[2], f[3], r.Roll, r.Pitch, r.Yaw, r.Guete, r.NurRoll, r.Unsicher, time.Since(t0).Round(time.Millisecond), err)
	}
}
