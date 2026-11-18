// Package profil verwaltet die Kameraprofile (*.json im Ordner "kameraprofile")
// – im selben Format wie bisher, damit vorhandene .v3cam-Dateien weiter gelten.
package profil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"vertical3/meta"
)

const Endung = ".v3cam"

// Profil ist ein Kameraprofil; die Felder entsprechen den JSON-Schlüsseln.
type Profil struct {
	Roh   map[string]any
	Datei string
}

func (p *Profil) Name() string { return text(p.Roh["name"]) }
func (p *Profil) Match() map[string]any {
	m, _ := p.Roh["match"].(map[string]any)
	return m
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

func liste(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, text(e))
		}
		return out
	}
	return nil
}

func (p *Profil) zahl(k string, standard float64) float64 {
	if v, ok := p.Roh[k]; ok {
		if f, err := strconv.ParseFloat(text(v), 64); err == nil {
			return f
		}
	}
	return standard
}

func (p *Profil) wahr(k string) bool {
	switch x := p.Roh[k].(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return strings.EqualFold(x, "true") || x == "1"
	}
	return false
}

// Lade liest alle Profile; das allgemeine (ohne "match") kommt zuletzt.
func Lade(ordner string) []*Profil {
	var out []*Profil
	eintraege, _ := os.ReadDir(ordner)
	for _, e := range eintraege {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(ordner, e.Name()))
		if err != nil {
			continue
		}
		var roh map[string]any
		if json.Unmarshal(entferneBOM(b), &roh) != nil {
			continue
		}
		out = append(out, &Profil{Roh: roh, Datei: e.Name()})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := len(out[i].Match()) == 0, len(out[j].Match()) == 0
		if ai != aj {
			return !ai
		}
		return out[i].Datei < out[j].Datei
	})
	if len(out) == 0 || len(out[len(out)-1].Match()) != 0 {
		out = append(out, &Profil{Roh: map[string]any{"name": "Allgemein", "match": map[string]any{}}})
	}
	return out
}

func entferneBOM(b []byte) []byte {
	return []byte(strings.TrimPrefix(string(b), "\ufeff"))
}

// Passend sucht das erste Profil, dessen Erkennungsmuster alle zutreffen.
func Passend(d meta.Daten, profile []*Profil) *Profil {
	for _, p := range profile {
		ok := true
		for feld, muster := range p.Match() {
			re, err := regexp.Compile("(?i)" + text(muster))
			if err != nil || !re.MatchString(d[feld]) {
				ok = false
				break
			}
		}
		if ok {
			return p
		}
	}
	return profile[len(profile)-1]
}

func findeWert(d meta.Daten, tags []string, endung string) (float64, bool) {
	for _, t := range tags {
		if v, ok := d[t]; ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return f, true
			}
		}
	}
	// Rückfall: irgendein Feld mit diesem Namen (Nikon, OM System, Pentax …)
	schluessel := make([]string, 0, len(d))
	for k := range d {
		schluessel = append(schluessel, k)
	}
	sort.Strings(schluessel)
	for _, k := range schluessel {
		if strings.HasSuffix(strings.ToLower(k), strings.ToLower(endung)) {
			if f, err := strconv.ParseFloat(strings.TrimSpace(d[k]), 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// KBBrennweite: Kleinbild-Brennweite aus den Daten oder dem Profil.
func KBBrennweite(d meta.Daten, p *Profil) float64 {
	for _, k := range append(liste(p.Roh["brennweite_tag"]), "FocalLengthIn35mmFormat", "FocalLengthIn35mmFilm") {
		if f, err := strconv.ParseFloat(strings.TrimSpace(d[k]), 64); err == nil && f > 0 {
			return f
		}
	}
	if f, err := strconv.ParseFloat(d["FocalLength"], 64); err == nil && f > 0 {
		if c := p.zahl("cropfaktor", 0); c > 0 {
			return f * c
		}
		// Ohne Kleinbild-Angabe: echte Brennweite verwenden (stimmt bei Vollformat,
		// ist sonst näher dran als ein fester Wert). Sehr kleine Werte stammen von
		// Handys/Kompaktkameras – dann lieber den Profilwert.
		if f >= 12 {
			return f
		}
	}
	return p.zahl("kb_brennweite_standard", 28)
}

// Pruefe: Fehlermeldung oder nil.
func Pruefe(roh map[string]any) error {
	if strings.TrimSpace(text(roh["name"])) == "" {
		return errors.New("Dem Profil fehlt ein Name ('name').")
	}
	m, ok := roh["match"].(map[string]any)
	if !ok || len(m) == 0 {
		return errors.New("Dem Profil fehlt die Kameraerkennung ('match').")
	}
	for feld, muster := range m {
		if _, err := regexp.Compile(text(muster)); err != nil {
			return fmt.Errorf("Ungültiges Suchmuster bei '%s'.", feld)
		}
	}
	p := &Profil{Roh: roh}
	if !(p.wahr("keine_neigungsdaten") || text(roh["schwerkraft_tag"]) != "" ||
		(len(liste(roh["roll_tag"])) > 0 && len(liste(roh["pitch_tag"])) > 0)) {
		return errors.New("Das Profil muss 'roll_tag' und 'pitch_tag', 'schwerkraft_tag' oder 'keine_neigungsdaten' enthalten.")
	}
	for _, k := range []string{"roll_faktor", "pitch_faktor", "kb_brennweite_standard", "cropfaktor"} {
		if v, da := roh[k]; da {
			if _, err := strconv.ParseFloat(text(v), 64); err != nil {
				return fmt.Errorf("'%s' muss eine Zahl sein.", k)
			}
		}
	}
	return nil
}

var nichtErlaubt = regexp.MustCompile(`[^a-z0-9]+`)

// Importiere übernimmt eine .v3cam/.json-Datei. Status: "neu", "ersetzt", "vorhanden".
func Importiere(quelle, ordner string, ueberschreiben bool) (status, name string, err error) {
	b, err := os.ReadFile(quelle)
	if err != nil {
		return "", "", err
	}
	var roh map[string]any
	if json.Unmarshal(entferneBOM(b), &roh) != nil {
		return "", "", errors.New("Die Datei ist kein gültiges Kameraprofil.")
	}
	if err := Pruefe(roh); err != nil {
		return "", "", err
	}
	name = text(roh["name"])
	if err := os.MkdirAll(ordner, 0o755); err != nil {
		return "", name, err
	}
	stamm := strings.Trim(nichtErlaubt.ReplaceAllString(strings.ToLower(name), "_"), "_")
	if len(stamm) > 60 {
		stamm = stamm[:60]
	}
	if stamm == "" {
		stamm = "kamera"
	}
	ziel := filepath.Join(ordner, stamm+".json")
	for _, p := range Lade(ordner) {
		if p.Datei != "" && reflect.DeepEqual(p.Match(), roh["match"]) {
			ziel = filepath.Join(ordner, p.Datei)
			break
		}
	}
	status = "neu"
	if _, e := os.Stat(ziel); e == nil {
		if !ueberschreiben {
			return "vorhanden", name, nil
		}
		status = "ersetzt"
	}
	aus, _ := json.MarshalIndent(roh, "", "  ")
	return status, name, os.WriteFile(ziel, aus, 0o644)
}

// Text liefert ein beliebiges Textfeld des Profils.
func (p *Profil) Text(k string) string { return text(p.Roh[k]) }

// Zeile der Kameraübersicht.
type Zeile struct {
	Hersteller, Modelle, Neigung, Formate, Datei string
}

var nurBuchstaben = regexp.MustCompile(`[^\p{L}\p{N} \-]+`)

func lesbar(muster string) string {
	s := strings.TrimSpace(nurBuchstaben.ReplaceAllString(muster, " "))
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Uebersicht liefert je Profil Hersteller, Modelle und Art der Neigungsdaten.
// Fehlende Angaben (z. B. bei selbst erstellten Profilen) werden abgeleitet.
func Uebersicht(profile []*Profil) []Zeile {
	var out []Zeile
	for _, p := range profile {
		z := Zeile{Hersteller: p.Text("hersteller"), Modelle: p.Text("modelle"),
			Neigung: p.Text("neigung"), Formate: p.Text("formate"), Datei: p.Datei}
		m := p.Match()
		if z.Hersteller == "" {
			z.Hersteller = lesbar(text(m["Make"]))
			if z.Hersteller == "" {
				z.Hersteller = p.Name()
			}
		}
		if z.Modelle == "" {
			if mod := lesbar(text(m["Model"])); mod != "" {
				z.Modelle = strings.ToUpper(mod)
			} else {
				z.Modelle = "alle Modelle"
			}
		}
		if z.Neigung == "" {
			switch {
			case p.wahr("keine_neigungsdaten"):
				z.Neigung = "keine – per Regler oder Kanten markieren"
			case text(p.Roh["schwerkraft_tag"]) != "":
				z.Neigung = "automatisch (Schwerkraftsensor)"
			default:
				z.Neigung = "automatisch"
			}
		}
		if z.Formate == "" {
			z.Formate = "—"
		}
		out = append(out, z)
	}
	return out
}

// Für das Zusatzprogramm Kameraneigung:

// Wahr, Zahl, Liste und FindeWert lesen Profilangaben bzw. Bilddaten.
func (p *Profil) Wahr(k string) bool                      { return p.wahr(k) }
func (p *Profil) Zahl(k string, standard float64) float64 { return p.zahl(k, standard) }
func (p *Profil) Liste(k string) []string                 { return liste(p.Roh[k]) }
func FindeWert(d meta.Daten, tags []string, endung string) (float64, bool) {
	return findeWert(d, tags, endung)
}
