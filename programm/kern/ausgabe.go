package kern

import "math"

// Ausgabemass berechnet die Größe des gespeicherten Bildes. art:
// "original", "lang" (längste Seite a px), "breite", "hoehe", "box" (passt
// in a×b px), "druck" (a×b cm bei 300 dpi), "prozent". Das
// Seitenverhältnis bleibt erhalten; bei box/druck wird der Kasten an Hoch-
// bzw. Querformat angepasst.
func Ausgabemass(art string, a, b float64, w, h int, nichtVergroessern bool) (int, int) {
	fw, fh := float64(w), float64(h)
	f := 1.0
	box := func(bw, bh float64) float64 {
		if bw <= 0 || bh <= 0 {
			return 1
		}
		if (fw >= fh) != (bw >= bh) {
			bw, bh = bh, bw
		}
		return math.Min(bw/fw, bh/fh)
	}
	switch art {
	case "lang":
		f = a / math.Max(fw, fh)
	case "breite":
		f = a / fw
	case "hoehe":
		f = a / fh
	case "box":
		f = box(a, b)
	case "druck":
		f = box(a/2.54*300, b/2.54*300)
	case "prozent":
		f = a / 100
	}
	if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		f = 1
	}
	if nichtVergroessern && f > 1 {
		f = 1
	}
	return max(1, int(math.Round(fw*f))), max(1, int(math.Round(fh*f)))
}
