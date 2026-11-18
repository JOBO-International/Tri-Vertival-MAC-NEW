package ki

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"vertical3/kern"
)

const (
	kachel     = 512  // feste Eingangsgröße des LaMa-Modells
	kRand      = 48   // Rand einer Kachel, der erst von der Nachbarkachel gefüllt wird
	arbeitsMax = 1536 // längste Kante, in der die KI rechnet
)

// Ordner mit Modell und Laufzeit (lama.onnx, onnxruntime.dll)
var KIOrdner string

var (
	ortEinmal sync.Once
	ortFehler error
	sitzung   *ort.AdvancedSession
	tEin      *ort.Tensor[float32]
	tMaske    *ort.Tensor[float32]
	tAus      *ort.Tensor[float32]
	sitzungMu sync.Mutex
)

func bibliothek() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(KIOrdner, "onnxruntime.dll")
	}
	if p := os.Getenv("V3_ORT"); p != "" {
		return p
	}
	return filepath.Join(KIOrdner, "libonnxruntime.so")
}

// Bereit prüft, ob die Offline-KI installiert ist.
func Bereit() error {
	for _, d := range []string{bibliothek()} {
		if _, err := os.Stat(d); err != nil {
			return fmt.Errorf("Die Offline-KI ist nicht vollständig installiert (%s fehlt).\nBitte das Setup „Vertical3 KI-Füllen“ erneut ausführen.", filepath.Base(d))
		}
	}
	return nil
}

func starte() error {
	ortEinmal.Do(func() {
		if err := Bereit(); err != nil {
			ortFehler = err
			return
		}
		dllOrdner(KIOrdner)
		ort.SetSharedLibraryPath(bibliothek())
		if err := ort.InitializeEnvironment(); err != nil {
			ortFehler = fmt.Errorf("Die KI-Laufzeit konnte nicht gestartet werden:\n%v", err)
			return
		}
		form := ort.NewShape(1, 3, kachel, kachel)
		var err error
		if tEin, err = ort.NewEmptyTensor[float32](form); err == nil {
			if tMaske, err = ort.NewEmptyTensor[float32](ort.NewShape(1, 1, kachel, kachel)); err == nil {
				tAus, err = ort.NewEmptyTensor[float32](form)
			}
		}
		if err != nil {
			ortFehler = err
			return
		}
		opt, err := ort.NewSessionOptions()
		if err != nil {
			ortFehler = err
			return
		}
		defer opt.Destroy()
		opt.SetIntraOpNumThreads(max(1, runtime.NumCPU()))
		sitzung, err = ort.NewAdvancedSession(ModellPfad(),
			[]string{"image", "mask"}, []string{"output"},
			[]ort.Value{tEin, tMaske}, []ort.Value{tAus}, opt)
		if err != nil {
			ortFehler = fmt.Errorf("Das KI-Modell konnte nicht geladen werden:\n%v", err)
		}
	})
	return ortFehler
}

// arbeitsFaktor: Verkleinerung für die KI (kurze Kante mind. eine Kachel).
func arbeitsFaktor(w, h int) float64 {
	f := math.Min(1, arbeitsMax/math.Max(float64(w), float64(h)))
	if float64(min(w, h))*f < kachel {
		f = kachel / float64(min(w, h))
	}
	return f
}

func positionen(laenge int) []int {
	if laenge <= kachel {
		return []int{0}
	}
	var p []int
	for x := 0; ; x += kachel - 2*kRand {
		if x+kachel >= laenge {
			p = append(p, laenge-kachel)
			break
		}
		p = append(p, x)
	}
	return p
}

// Offline füllt die leeren Bereiche (maske != 0) mit dem LaMa-Modell.
func Offline(b *kern.Bild, maske []uint8, fs Fortschritt) (*kern.Bild, error) {
	if !ModellDa() {
		if err := LadeModell(fs); err != nil {
			return nil, err
		}
	}
	melde(fs, 0, "KI wird gestartet …")
	if err := starte(); err != nil {
		return nil, err
	}
	sitzungMu.Lock()
	defer sitzungMu.Unlock()

	f := arbeitsFaktor(b.W, b.H)
	aw, ah := int(math.Round(float64(b.W)*f)), int(math.Round(float64(b.H)*f))
	arbeit := skaliere(b, aw, ah)
	if arbeit == b {
		arbeit = &kern.Bild{W: b.W, H: b.H, Pix: append([]uint8(nil), b.Pix...)}
	}
	rest := weite(maskeVerkleinert(maske, b.W, b.H, aw, ah), aw, ah, 2)

	type pos struct{ x, y int }
	var kacheln []pos
	for _, y := range positionen(ah) {
		for _, x := range positionen(aw) {
			kacheln = append(kacheln, pos{x, y})
		}
	}
	hatRest := func(p pos) bool {
		for y := p.y; y < p.y+kachel; y++ {
			for x := p.x; x < p.x+kachel; x++ {
				if rest[y*aw+x] != 0 {
					return true
				}
			}
		}
		return false
	}
	gesamt := 0
	for _, p := range kacheln {
		if hatRest(p) {
			gesamt++
		}
	}
	if gesamt == 0 {
		return b, nil
	}
	ein, mk, aus := tEin.GetData(), tMaske.GetData(), tAus.GetData()
	const ebene = kachel * kachel
	erledigt := 0
	for durchgang := 0; durchgang < 3; durchgang++ {
		for _, p := range kacheln {
			if !hatRest(p) {
				continue
			}
			// Eingabe: BGR 0…1, Maske 1 = füllen
			for y := 0; y < kachel; y++ {
				for x := 0; x < kachel; x++ {
					i := (p.y+y)*aw + p.x + x
					j := y*kachel + x
					if rest[i] != 0 {
						mk[j] = 1
						ein[j], ein[ebene+j], ein[2*ebene+j] = 0, 0, 0
					} else {
						mk[j] = 0
						ein[j] = float32(arbeit.Pix[i*3+2]) / 255
						ein[ebene+j] = float32(arbeit.Pix[i*3+1]) / 255
						ein[2*ebene+j] = float32(arbeit.Pix[i*3]) / 255
					}
				}
			}
			if err := sitzung.Run(); err != nil {
				return nil, fmt.Errorf("KI-Berechnung fehlgeschlagen: %v", err)
			}
			// Innenbereich übernehmen; Kachelränder zur Bildmitte hin bleiben
			// der Nachbarkachel überlassen (im letzten Durchgang alles).
			l, r, o, u := kRand, kachel-kRand, kRand, kachel-kRand
			if p.x == 0 || durchgang == 2 {
				l = 0
			}
			if p.y == 0 || durchgang == 2 {
				o = 0
			}
			if p.x+kachel >= aw || durchgang == 2 {
				r = kachel
			}
			if p.y+kachel >= ah || durchgang == 2 {
				u = kachel
			}
			for y := o; y < u; y++ {
				for x := l; x < r; x++ {
					i := (p.y+y)*aw + p.x + x
					if rest[i] == 0 {
						continue
					}
					j := y*kachel + x
					arbeit.Pix[i*3+2] = klemm8(aus[j])
					arbeit.Pix[i*3+1] = klemm8(aus[ebene+j])
					arbeit.Pix[i*3] = klemm8(aus[2*ebene+j])
					rest[i] = 0
				}
			}
			erledigt++
			melde(fs, math.Min(0.95, float64(erledigt)/float64(gesamt)*0.95),
				fmt.Sprintf("KI füllt die Ränder … (%d von %d)", min(erledigt, gesamt), gesamt))
		}
	}
	for _, v := range rest {
		if v != 0 {
			return nil, errors.New("Die Ränder konnten nicht vollständig gefüllt werden.")
		}
	}
	melde(fs, 0.97, "Füllung wird in voller Auflösung eingesetzt …")
	return einsetzen(b, maske, arbeit), nil
}

func klemm8(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
