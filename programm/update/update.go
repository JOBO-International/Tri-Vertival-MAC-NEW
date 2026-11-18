// Package update spielt Update-Pakete (.v3update) ein.
//
// Ein Update-Paket ist eine ZIP-Datei mit "update.json" und den geänderten
// Dateien, z. B. "Vertical3.exe" oder "kameraprofile/neue_kamera.json".
//
// update.json:
//
//	{"programm": "Vertical3", "version": "2.0.1", "mindestversion": "2.0",
//	 "beschreibung": "…", "dateien": ["Vertical3.exe"], "benoetigt_setup": false}
//
// Vor dem Einspielen werden die betroffenen Dateien nach
// "_sicherung/<alte Version>" kopiert; Rueckgaengig stellt sie wieder her.
// Die laufende Vertical3.exe wird umbenannt (das erlaubt Windows) und beim
// nächsten Start aufgeräumt. Ist sie gesperrt (Virenscanner, zweites Fenster),
// wird sie nach dem Beenden per Hilfsskript ausgetauscht.
package update

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	Endung        = ".v3update"
	sicherung     = "_sicherung"
	versionsdatei = "version.txt"
	altEndung     = ".alt"
)

// Manifest = Inhalt von update.json.
type Manifest struct {
	Programm       string   `json:"programm"`
	Version        string   `json:"version"`
	Mindestversion string   `json:"mindestversion,omitempty"`
	Beschreibung   string   `json:"beschreibung"`
	Dateien        []string `json:"dateien"`
	BenoetigtSetup bool     `json:"benoetigt_setup"`
}

type sicherungInfo struct {
	Version     string   `json:"version"`
	Ersetzt     string   `json:"ersetzt_durch"`
	NeueDateien []string `json:"neue_dateien"`
	Dateien     []string `json:"dateien"`
}

var versionMuster = regexp.MustCompile(`^\d+(\.\d+){0,3}$`)

func vergleiche(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 4; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// InstallierteVersion liest version.txt, sonst standard.
func InstallierteVersion(appDir, standard string) string {
	b, err := os.ReadFile(filepath.Join(appDir, versionsdatei))
	if err == nil {
		v := strings.TrimSpace(string(b))
		if versionMuster.MatchString(v) {
			return v
		}
	}
	return standard
}

func schreibeVersion(appDir, v string) error {
	return os.WriteFile(filepath.Join(appDir, versionsdatei), []byte(v), 0o644)
}

func pruefeName(n string) error {
	if strings.HasPrefix(n, "/") || strings.Contains(n, "\\") || strings.Contains(n, "..") || strings.Contains(n, ":") {
		return fmt.Errorf("Unzulässiger Dateipfad im Update: %s", n)
	}
	ordner, datei := path.Split(n)
	ordner = strings.TrimSuffix(ordner, "/")
	if datei == "" || (ordner != "" && ordner != "kameraprofile") {
		return fmt.Errorf("Unzulässiger Ordner im Update: %s", n)
	}
	e := strings.ToLower(path.Ext(datei))
	switch {
	case ordner == "" && strings.EqualFold(datei, "Vertical3.exe"):
	case e == ".json" || e == ".txt" || e == ".ico" || e == ".png":
	default:
		return fmt.Errorf("Unzulässiger Dateityp im Update: %s", n)
	}
	return nil
}

// Lies öffnet und prüft ein Update-Paket.
func Lies(pfad string) (*Manifest, error) {
	z, err := zip.OpenReader(pfad)
	if err != nil {
		return nil, errors.New("Die Datei ist beschädigt oder kein Vertical3-Update.")
	}
	defer z.Close()
	inhalt := map[string]*zip.File{}
	for _, f := range z.File {
		inhalt[f.Name] = f
	}
	mf, ok := inhalt["update.json"]
	if !ok {
		return nil, errors.New("Die Datei ist kein Vertical3-Update (update.json fehlt).")
	}
	r, err := mf.Open()
	if err != nil {
		return nil, err
	}
	roh, _ := io.ReadAll(r)
	r.Close()
	var m Manifest
	if json.Unmarshal(bytes.TrimPrefix(roh, []byte("\xef\xbb\xbf")), &m) != nil {
		return nil, errors.New("update.json ist fehlerhaft.")
	}
	if m.Programm != "Vertical3" {
		return nil, errors.New("Dieses Update ist nicht für Vertical3.")
	}
	if !versionMuster.MatchString(m.Version) || (m.Mindestversion != "" && !versionMuster.MatchString(m.Mindestversion)) {
		return nil, errors.New("Ungültige Versionsnummer im Update.")
	}
	if len(m.Dateien) == 0 {
		return nil, errors.New("Das Update enthält keine Dateien.")
	}
	for _, n := range m.Dateien {
		if err := pruefeName(n); err != nil {
			return nil, err
		}
		f, ok := inhalt[n]
		if !ok {
			return nil, fmt.Errorf("Datei fehlt im Update: %s", n)
		}
		if strings.EqualFold(n, "Vertical3.exe") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			kopf := make([]byte, 2)
			_, err = io.ReadFull(rc, kopf)
			rc.Close()
			if err != nil || string(kopf) != "MZ" {
				return nil, errors.New("Vertical3.exe im Update ist kein gültiges Programm.")
			}
		}
	}
	return &m, nil
}

// Passend prüft, ob das Update zur installierten Version passt.
func Passend(m *Manifest, aktuell string) error {
	if m.BenoetigtSetup {
		return errors.New("Dieses Update kann nur über das Setup installiert werden.")
	}
	if m.Mindestversion != "" && vergleiche(aktuell, m.Mindestversion) < 0 {
		return fmt.Errorf("Für dieses Update wird mindestens Version %s benötigt (installiert: %s). Bitte zuerst das Setup verwenden.", m.Mindestversion, aktuell)
	}
	if vergleiche(m.Version, aktuell) <= 0 {
		return fmt.Errorf("Version %s ist bereits gleich neu oder neuer als %s.", aktuell, m.Version)
	}
	return nil
}

// Ausstehend: Programmdateien, die gesperrt waren und erst nach dem Beenden
// von Vertical3 ausgetauscht werden können (Paare: neue Datei → Ziel).
var Ausstehend [][2]string

const ausstehendEndung = ".ausstehend"

func gibtEs(p string) bool { _, err := os.Stat(p); return err == nil }

// beiseite benennt eine (evtl. laufende) .exe um. Ist eine ältere .alt-Datei
// noch belegt (z. B. weil ein zweites Vertical3-Fenster offen ist), wird ein
// anderer Name genommen.
func beiseite(ziel string) error {
	if os.Getenv("V3_TEST_GESPERRT") != "" {
		return errors.New("Zugriff verweigert (Test)")
	}
	alt := ziel + altEndung
	if os.Remove(alt) != nil && gibtEs(alt) {
		for i := 2; i < 100; i++ {
			alt = fmt.Sprintf("%s.%d%s", ziel, i, altEndung)
			if !gibtEs(alt) || os.Remove(alt) == nil {
				break
			}
		}
	}
	return os.Rename(ziel, alt)
}

// Ueberschreibe ersetzt eine (nicht mehr laufende) Programmdatei.
func Ueberschreibe(ziel string, daten []byte) error {
	beiseite(ziel) // klappt das Umbenennen nicht, wird direkt überschrieben
	return os.WriteFile(ziel, daten, 0o755)
}

// ersetze schreibt eine Datei; eine laufende .exe wird vorher umbenannt.
// Lässt sie sich gar nicht umbenennen, wird sie nach dem Beenden ersetzt.
func ersetze(ziel string, daten []byte) error {
	if err := os.MkdirAll(filepath.Dir(ziel), 0o755); err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(ziel), ".exe") && gibtEs(ziel) {
		if err := beiseite(ziel); err != nil {
			neu := ziel + ausstehendEndung
			if err := os.WriteFile(neu, daten, 0o755); err != nil {
				return err
			}
			Ausstehend = append(Ausstehend, [2]string{neu, ziel})
			return nil
		}
	}
	tmp := ziel + ".neu"
	if err := os.WriteFile(tmp, daten, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, ziel)
}

func kopiere(von, nach string) error {
	b, err := os.ReadFile(von)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(nach), 0o755); err != nil {
		return err
	}
	return os.WriteFile(nach, b, 0o644)
}

// Installiere spielt das Update ein.
func Installiere(pfad, appDir, aktuell string) (*Manifest, error) {
	m, err := Lies(pfad)
	if err != nil {
		return nil, err
	}
	if err := Passend(m, aktuell); err != nil {
		return nil, err
	}
	// alle Dateien vorab in den Speicher lesen – erst dann wird etwas geändert
	z, err := zip.OpenReader(pfad)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	daten := map[string][]byte{}
	for _, f := range z.File {
		for _, n := range m.Dateien {
			if f.Name == n {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				daten[n], err = io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, err
				}
			}
		}
	}
	sich := filepath.Join(appDir, sicherung, aktuell)
	os.RemoveAll(sich)
	info := sicherungInfo{Version: aktuell, Ersetzt: m.Version, Dateien: m.Dateien}
	for _, n := range m.Dateien {
		alt := filepath.Join(appDir, filepath.FromSlash(n))
		if _, err := os.Stat(alt); err == nil {
			if err := kopiere(alt, filepath.Join(sich, filepath.FromSlash(n))); err != nil {
				return nil, fmt.Errorf("Sicherung fehlgeschlagen: %w", err)
			}
		} else {
			info.NeueDateien = append(info.NeueDateien, n)
		}
	}
	ib, _ := json.MarshalIndent(info, "", "  ")
	if err := os.MkdirAll(sich, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(sich, "sicherung.json"), ib, 0o644); err != nil {
		return nil, err
	}
	Ausstehend = nil
	for _, n := range m.Dateien {
		if err := ersetze(filepath.Join(appDir, filepath.FromSlash(n)), daten[n]); err != nil {
			for _, a := range Ausstehend {
				os.Remove(a[0])
			}
			Ausstehend = nil
			stelleWiederHer(appDir, sich)
			return nil, fmt.Errorf("Update konnte nicht geschrieben werden, alter Stand wiederhergestellt: %w", err)
		}
	}
	return m, schreibeVersion(appDir, m.Version)
}

// LetzteSicherung liefert die Version der jüngsten Sicherung ("" = keine).
func LetzteSicherung(appDir string) (string, string) {
	basis := filepath.Join(appDir, sicherung)
	eintraege, _ := os.ReadDir(basis)
	var bestV, bestO string
	var bestZeit int64
	for _, e := range eintraege {
		o := filepath.Join(basis, e.Name())
		st, err := os.Stat(filepath.Join(o, "sicherung.json"))
		if err == nil && st.ModTime().UnixNano() > bestZeit {
			bestZeit, bestV, bestO = st.ModTime().UnixNano(), e.Name(), o
		}
	}
	return bestV, bestO
}

func stelleWiederHer(appDir, sich string) (string, error) {
	b, err := os.ReadFile(filepath.Join(sich, "sicherung.json"))
	if err != nil {
		return "", err
	}
	var info sicherungInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return "", err
	}
	for _, n := range info.Dateien {
		g := filepath.Join(sich, filepath.FromSlash(n))
		if d, err := os.ReadFile(g); err == nil {
			if err := ersetze(filepath.Join(appDir, filepath.FromSlash(n)), d); err != nil {
				return "", err
			}
		}
	}
	for _, n := range info.NeueDateien {
		ziel := filepath.Join(appDir, filepath.FromSlash(n))
		if strings.EqualFold(filepath.Ext(ziel), ".exe") {
			os.Rename(ziel, ziel+altEndung)
		} else {
			os.Remove(ziel)
		}
	}
	return info.Version, schreibeVersion(appDir, info.Version)
}

// Rueckgaengig nimmt das letzte Update zurück.
func Rueckgaengig(appDir string) (string, error) {
	_, o := LetzteSicherung(appDir)
	if o == "" {
		return "", errors.New("Es gibt keine Sicherung eines früheren Stands.")
	}
	v, err := stelleWiederHer(appDir, o)
	if err != nil {
		return "", err
	}
	os.RemoveAll(o)
	return v, nil
}

// Aufraeumen entfernt beim Start übrig gebliebene alte Programmdateien.
func Aufraeumen(appDir string) {
	reste, _ := filepath.Glob(filepath.Join(appDir, "*"+altEndung))
	for _, r := range reste {
		os.Remove(r)
	}
	// Reste eines Austauschs nach dem Beenden: schon erledigt → löschen,
	// sonst jetzt nachholen (gilt ab dem nächsten Start).
	offen, _ := filepath.Glob(filepath.Join(appDir, "*"+ausstehendEndung))
	for _, neu := range offen {
		ziel := strings.TrimSuffix(neu, ausstehendEndung)
		a, e1 := os.ReadFile(neu)
		b, e2 := os.ReadFile(ziel)
		if e1 == nil && e2 == nil && bytes.Equal(a, b) {
			os.Remove(neu)
		} else if e1 == nil && beiseite(ziel) == nil {
			os.Rename(neu, ziel)
		}
	}
}

// Baue erstellt ein Update-Paket (Werkzeug für den Entwickler).
func Baue(appDir, ziel string, m Manifest) error {
	m.Programm = "Vertical3"
	f, err := os.Create(ziel)
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	mb, _ := json.MarshalIndent(m, "", "  ")
	w, _ := z.Create("update.json")
	w.Write(mb)
	for _, n := range m.Dateien {
		if err := pruefeName(n); err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(appDir, filepath.FromSlash(n)))
		if err != nil {
			return err
		}
		w, _ := z.Create(n)
		w.Write(b)
	}
	if err := z.Close(); err != nil {
		return err
	}
	f.Close()
	_, err = Lies(ziel)
	return err
}
