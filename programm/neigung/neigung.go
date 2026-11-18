// Paket neigung: Roll und Pitch aus den Neigungsdaten, die die Kamera in der
// Bilddatei hinterlegt hat. Wird nur vom Zusatzprogramm „Vertical3
// Kameraneigung“ verwendet – das Hauptprogramm enthält diesen Teil nicht.
package neigung

import (
	"math"
	"strconv"
	"strings"

	"vertical3/kern"
	"vertical3/meta"
	"vertical3/profil"
)

// Lies liefert Roll und Pitch aus den Bilddaten (ok=false: keine Daten)
// und ggf. einen Hinweis für die Anzeige.
func Lies(d meta.Daten, p *profil.Profil) (roll, pitch float64, ok bool, hinweis string) {
	if p.Wahr("keine_neigungsdaten") {
		return 0, 0, false, ""
	}
	if tag := p.Text("bereits_korrigiert_tag"); tag != "" {
		v := strings.ToLower(strings.TrimSpace(d[tag]))
		if v == "true" || v == "1" || v == "yes" {
			return 0, 0, false, "Die Kamera hat dieses Bild bereits selbst entzerrt."
		}
	}
	if tag := p.Text("schwerkraft_tag"); tag != "" {
		felder := strings.Fields(strings.ReplaceAll(d[tag], ",", " "))
		if len(felder) < 3 {
			return 0, 0, false, ""
		}
		var v [3]float64
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(felder[i], 64)
			if err != nil {
				return 0, 0, false, ""
			}
			v[i] = f
		}
		achsen := [3]float64{1, -1, -1}
		if a := p.Liste("schwerkraft_achsen"); len(a) == 3 {
			for i := range a {
				achsen[i], _ = strconv.ParseFloat(a[i], 64)
			}
		}
		r, pi, gut := NeigungAusSchwerkraft(v, achsen)
		if !gut {
			return 0, 0, false, ""
		}
		return kern.NormiereRoll(r * p.Zahl("roll_faktor", 1)), pi * p.Zahl("pitch_faktor", 1), true, ""
	}
	r, ok1 := profil.FindeWert(d, p.Liste("roll_tag"), "RollAngle")
	pi, ok2 := profil.FindeWert(d, p.Liste("pitch_tag"), "PitchAngle")
	if !ok1 || !ok2 {
		return 0, 0, false, ""
	}
	return kern.NormiereRoll(r * p.Zahl("roll_faktor", 1)), pi * p.Zahl("pitch_faktor", 1), true, ""
}

// NeigungAusSchwerkraft: Schwerkraftvektor (z. B. iPhone) → Roll, Pitch.
// achsen bildet die Geräteachsen auf das Kamerabild ab (x rechts, y unten,
// z Blickrichtung). Hoch-/Querformat wird automatisch erkannt.
func NeigungAusSchwerkraft(v [3]float64, achsen [3]float64) (float64, float64, bool) {
	g := [3]float64{v[0] * achsen[0], v[1] * achsen[1], v[2] * achsen[2]}
	n := math.Sqrt(g[0]*g[0] + g[1]*g[1] + g[2]*g[2])
	if n < 0.3 {
		return 0, 0, false
	}
	for i := range g {
		g[i] /= n
	}
	kand := [4][2]float64{{g[0], g[1]}, {g[1], -g[0]}, {-g[0], -g[1]}, {-g[1], g[0]}}
	beste := kand[0]
	for _, k := range kand[1:] {
		if k[1] > beste[1] {
			beste = k
		}
	}
	roll := grad(math.Atan2(beste[0], beste[1]))
	pitch := grad(math.Asin(math.Max(-1, math.Min(1, -g[2]))))
	return kern.NormiereRoll(roll), pitch, true
}

func grad(r float64) float64 { return r * 180 / math.Pi }
