package ki

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Das LaMa-Modell (Apache 2.0, OpenCV Model Zoo) wird beim ersten Gebrauch
// einmalig geladen, damit das Setup klein bleibt.
const (
	ModellURL    = "https://media.githubusercontent.com/media/opencv/opencv_zoo/main/models/inpainting_lama/inpainting_lama_2025jan.onnx"
	modellSHA256 = "7df918ac3921d3daf0aae1d219776cf0dc4e4935f035af81841b40adcf74fdf2"
	ModellMB     = 93
)

// benutzerOrdner: %LOCALAPPDATA%\Vertical3\ki (immer beschreibbar).
func benutzerOrdner() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "Vertical3", "ki")
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "Vertical3", "ki")
}

// ModellPfad: vorhandenes Modell (Programmordner oder Benutzerordner).
func ModellPfad() string {
	for _, p := range []string{filepath.Join(KIOrdner, "lama.onnx"), filepath.Join(benutzerOrdner(), "lama.onnx")} {
		if fi, err := os.Stat(p); err == nil && fi.Size() > 50<<20 {
			return p
		}
	}
	return filepath.Join(benutzerOrdner(), "lama.onnx")
}

func ModellDa() bool {
	fi, err := os.Stat(ModellPfad())
	return err == nil && fi.Size() > 50<<20
}

// LadeModell lädt das KI-Modell herunter und prüft die Prüfsumme.
func LadeModell(fs Fortschritt) error {
	ziel := filepath.Join(benutzerOrdner(), "lama.onnx")
	if err := os.MkdirAll(filepath.Dir(ziel), 0o755); err != nil {
		return err
	}
	melde(fs, 0, fmt.Sprintf("KI-Modell wird einmalig geladen (%d MB) …", ModellMB))
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(ModellURL)
	if err != nil {
		return ladeFehler(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ladeFehler(fmt.Errorf("Server antwortet mit Code %d", resp.StatusCode))
	}
	teil := ziel + ".teil"
	f, err := os.Create(teil)
	if err != nil {
		return err
	}
	h := sha256.New()
	gesamt := resp.ContentLength
	if gesamt <= 0 {
		gesamt = ModellMB << 20
	}
	var geladen int64
	puffer := make([]byte, 256<<10)
	letzte := time.Now()
	for {
		n, rerr := resp.Body.Read(puffer)
		if n > 0 {
			if _, werr := f.Write(puffer[:n]); werr != nil {
				f.Close()
				os.Remove(teil)
				return werr
			}
			h.Write(puffer[:n])
			geladen += int64(n)
			if time.Since(letzte) > 300*time.Millisecond {
				letzte = time.Now()
				melde(fs, float64(geladen)/float64(gesamt), fmt.Sprintf("KI-Modell wird geladen … %d von %d MB", geladen>>20, gesamt>>20))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(teil)
			return ladeFehler(rerr)
		}
	}
	f.Close()
	if hex.EncodeToString(h.Sum(nil)) != modellSHA256 {
		os.Remove(teil)
		return errors.New("Das heruntergeladene KI-Modell ist beschädigt (Prüfsumme stimmt nicht). Bitte erneut versuchen.")
	}
	os.Remove(ziel)
	return os.Rename(teil, ziel)
}

func ladeFehler(err error) error {
	return fmt.Errorf("Das KI-Modell konnte nicht geladen werden:\n%v\n\n"+
		"Bitte Internetverbindung prüfen und erneut versuchen. Alternativ die Datei\n%s\n"+
		"herunterladen, in „lama.onnx“ umbenennen und hier ablegen:\n%s", err, ModellURL, benutzerOrdner())
}
