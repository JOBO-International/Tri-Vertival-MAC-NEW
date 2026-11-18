package ki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"vertical3/kern"
)

// StabilityURL kann für Tests umgestellt werden.
var StabilityURL = "https://api.stability.ai/v2beta/stable-image/edit/inpaint"

const (
	maxPixel  = 9_000_000 // Grenze der API: 9.437.184 Pixel
	grundText = "photograph, seamless natural continuation of the surrounding scene at the image border, " +
		"same perspective, same lighting, same colors and same level of detail, realistic"
	negativText = "frame, border, vignette, text, watermark, signature, people, blurry, distorted, painting, illustration"
)

// Online füllt die leeren Bereiche über Stability AI (Inpaint).
// beschreibung (englisch oder deutsch, optional) hilft der KI beim Motiv.
func Online(ctx context.Context, b *kern.Bild, maske []uint8, schluessel, beschreibung string, fs Fortschritt) (*kern.Bild, error) {
	schluessel = strings.TrimSpace(schluessel)
	if schluessel == "" {
		return nil, errors.New("Für die Online-KI wird ein API-Schlüssel von Stability AI benötigt.\nBitte unter „API-Schlüssel …“ eintragen.")
	}
	seite := float64(b.W) / float64(b.H)
	if seite > 2.5 || seite < 1/2.5 {
		return nil, errors.New("Die Online-KI unterstützt nur Seitenverhältnisse bis 2,5 : 1.\nBitte die Offline-KI verwenden.")
	}
	melde(fs, 0.05, "Bild wird für den Versand vorbereitet …")
	f := math.Min(1, math.Sqrt(maxPixel/float64(b.W*b.H)))
	aw, ah := max(64, int(float64(b.W)*f)), max(64, int(float64(b.H)*f))
	arbeit := skaliere(b, aw, ah)
	m := weite(maskeVerkleinert(maske, b.W, b.H, aw, ah), aw, ah, 2)

	var bildDaten, maskeDaten bytes.Buffer
	if err := jpeg.Encode(&bildDaten, arbeit.RGBA(), &jpeg.Options{Quality: 93}); err != nil {
		return nil, err
	}
	gm := image.NewGray(image.Rect(0, 0, aw, ah))
	for i, v := range m {
		gm.Pix[i] = v
	}
	if err := png.Encode(&maskeDaten, gm); err != nil {
		return nil, err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	datei := func(feld, name, typ string, daten []byte) {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, feld, name))
		h.Set("Content-Type", typ)
		w, _ := mw.CreatePart(h)
		w.Write(daten)
	}
	datei("image", "bild.jpg", "image/jpeg", bildDaten.Bytes())
	datei("mask", "maske.png", "image/png", maskeDaten.Bytes())
	text := grundText
	if t := strings.TrimSpace(beschreibung); t != "" {
		text = t + ", " + grundText
	}
	mw.WriteField("prompt", text)
	mw.WriteField("negative_prompt", negativText)
	mw.WriteField("grow_mask", "8")
	mw.WriteField("output_format", "png")
	mw.Close()

	melde(fs, 0.15, fmt.Sprintf("Bild wird an Stability AI gesendet (%.1f MB) …", float64(body.Len())/1e6))
	ctx, abbruch := context.WithTimeout(ctx, 3*time.Minute)
	defer abbruch()
	req, err := http.NewRequestWithContext(ctx, "POST", StabilityURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+schluessel)
	req.Header.Set("Accept", "image/*")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("stability-client-id", "Vertical3")
	fertig := make(chan struct{})
	go func() { // Fortschritt, solange der Server rechnet
		a := 0.2
		for {
			select {
			case <-fertig:
				return
			case <-time.After(time.Second):
				a = math.Min(0.9, a+0.03)
				melde(fs, a, "Stability AI füllt die Ränder …")
			}
		}
	}()
	resp, err := http.DefaultClient.Do(req)
	close(fertig)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("Stability AI hat nicht rechtzeitig geantwortet. Bitte später erneut versuchen.")
		}
		return nil, fmt.Errorf("Keine Verbindung zu Stability AI:\n%v", err)
	}
	defer resp.Body.Close()
	daten, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiFehler(resp.StatusCode, daten)
	}
	melde(fs, 0.93, "Antwort wird eingesetzt …")
	img, _, err := image.Decode(bytes.NewReader(daten))
	if err != nil {
		return nil, fmt.Errorf("Die Antwort von Stability AI ist kein lesbares Bild: %v", err)
	}
	fuellung := kern.AusImage(img)
	return einsetzen(b, maske, fuellung), nil
}

func apiFehler(code int, daten []byte) error {
	var j struct {
		Name    string   `json:"name"`
		Errors  []string `json:"errors"`
		Message string   `json:"message"`
	}
	json.Unmarshal(daten, &j)
	detail := strings.Join(j.Errors, "\n")
	if detail == "" {
		detail = j.Message
	}
	if detail == "" && len(daten) < 300 {
		detail = strings.TrimSpace(string(daten))
	}
	var text string
	switch code {
	case 401:
		text = "Der API-Schlüssel wurde von Stability AI nicht akzeptiert. Bitte unter „API-Schlüssel …“ prüfen."
	case 402:
		text = "Das Guthaben bei Stability AI reicht nicht aus. Guthaben kann unter platform.stability.ai aufgeladen werden."
	case 403:
		text = "Stability AI hat die Anfrage abgelehnt (Inhaltsprüfung oder fehlende Berechtigung)."
	case 413:
		text = "Das Bild ist für Stability AI zu groß."
	case 429:
		text = "Zu viele Anfragen in kurzer Zeit. Bitte einen Moment warten und erneut versuchen."
	default:
		text = fmt.Sprintf("Stability AI meldet einen Fehler (Code %d).", code)
	}
	if detail != "" {
		text += "\n\n" + detail
	}
	return errors.New(text)
}
