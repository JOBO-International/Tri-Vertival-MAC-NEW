package ki

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
)

type kiEinstellungen struct {
	Schluessel   string `json:"schluessel,omitempty"` // verschlüsselt (Windows: DPAPI), Base64
	Online       bool   `json:"online"`
	Beschreibung string `json:"beschreibung,omitempty"`
}

func einstellungsDatei() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "Vertical3", "ki.json")
}

func ladeEinst() kiEinstellungen {
	var e kiEinstellungen
	if b, err := os.ReadFile(einstellungsDatei()); err == nil {
		json.Unmarshal(b, &e)
	}
	return e
}

func speichereEinst(e kiEinstellungen) error {
	p := einstellungsDatei()
	os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.MarshalIndent(e, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// Schluessel liefert den gespeicherten API-Schlüssel ("" wenn keiner).
func Schluessel() string {
	e := ladeEinst()
	if e.Schluessel == "" {
		return ""
	}
	roh, err := base64.StdEncoding.DecodeString(e.Schluessel)
	if err != nil {
		return ""
	}
	klar, err := entschluesseln(roh)
	if err != nil {
		return ""
	}
	return string(klar)
}

// SetzeSchluessel speichert den API-Schlüssel verschlüsselt ("" = löschen).
func SetzeSchluessel(s string) error {
	e := ladeEinst()
	if s == "" {
		e.Schluessel = ""
		return speichereEinst(e)
	}
	v, err := verschluesseln([]byte(s))
	if err != nil {
		return err
	}
	e.Schluessel = base64.StdEncoding.EncodeToString(v)
	return speichereEinst(e)
}

// Wahl: zuletzt gewählte Variante und Motivbeschreibung.
func Wahl() (online bool, beschreibung string) {
	e := ladeEinst()
	return e.Online, e.Beschreibung
}

func SetzeWahl(online bool, beschreibung string) {
	e := ladeEinst()
	e.Online, e.Beschreibung = online, beschreibung
	speichereEinst(e)
}
