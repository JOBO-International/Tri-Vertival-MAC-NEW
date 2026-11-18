// Package filter: Bildlooks (Filter) und Feinabstimmung.
//
// Jeder Filter ist ein Satz Einstellungen. Die Stärke blendet linear zwischen
// „neutral“ und dem Filter; die Feinabstimmung wird obendrauf gerechnet.
// Die Tonwert- und Weißabgleichsschritte werden je Farbkanal zu einer Tabelle
// (LUT) zusammengefasst, der Rest läuft je Bildpunkt – schnell genug für die
// Live-Vorschau und für volle Auflösung.
package filter

import (
	"math"
	"runtime"
	"sync"

	"vertical3/kern"
)

// Kanalverschiebung: Kurvenanhebung eines Farbkanals in Tiefen und Lichtern.
type Kanalverschiebung struct {
	Tiefe float64 `json:"tiefe"`
	Licht float64 `json:"licht"`
}

// Einstellung: alle Regler eines Looks (0 = neutral).
type Einstellung struct {
	Belichtung float64              `json:"belichtung"` // Blendenstufen
	Kontrast   float64              `json:"kontrast"`   // −1 … 1
	Lichter    float64              `json:"lichter"`    // −1 … 1
	Tiefen     float64              `json:"tiefen"`     // −1 … 1
	Saettigung float64              `json:"saettigung"` // −1 … 1
	Dynamik    float64              `json:"dynamik"`    // −1 … 1
	Temperatur float64              `json:"temperatur"` // −1 (kühl) … 1 (warm)
	Toenung    float64              `json:"toenung"`    // −1 (grün) … 1 (magenta)
	SW         float64              `json:"sw"`         // 0 … 1 Schwarzweiß-Anteil
	SWMix      [3]float64           `json:"swmix"`      // Kanalanteile für S/W
	TonLichter [3]float64           `json:"tonlichter"` // Tonungsfarbe Lichter (0…1, 0,5 = neutral)
	TonTiefen  [3]float64           `json:"tontiefen"`  // Tonungsfarbe Tiefen
	TonStaerke float64              `json:"tonstaerke"` // 0 … 1
	Matt       float64              `json:"matt"`       // 0 … 1 (verblasste Schwärzen)
	Klarheit   float64              `json:"klarheit"`   // −1 … 1 (lokaler Kontrast)
	Vignette   float64              `json:"vignette"`   // −1 … 1 (+ = dunkle Ecken)
	Koernung   float64              `json:"koernung"`   // 0 … 1
	Kanal      [3]Kanalverschiebung `json:"kanal"`
}

// Neutral: keine Veränderung.
func Neutral() Einstellung {
	return Einstellung{SWMix: [3]float64{0.30, 0.59, 0.11}, TonLichter: [3]float64{.5, .5, .5}, TonTiefen: [3]float64{.5, .5, .5}}
}

func mischF(a, b, t float64) float64 { return a + (b-a)*t }

// Mische: zwischen a (t = 0) und b (t = 1).
func Mische(a, b Einstellung, t float64) Einstellung {
	r := b
	r.Belichtung = mischF(a.Belichtung, b.Belichtung, t)
	r.Kontrast = mischF(a.Kontrast, b.Kontrast, t)
	r.Lichter = mischF(a.Lichter, b.Lichter, t)
	r.Tiefen = mischF(a.Tiefen, b.Tiefen, t)
	r.Saettigung = mischF(a.Saettigung, b.Saettigung, t)
	r.Dynamik = mischF(a.Dynamik, b.Dynamik, t)
	r.Temperatur = mischF(a.Temperatur, b.Temperatur, t)
	r.Toenung = mischF(a.Toenung, b.Toenung, t)
	r.SW = mischF(a.SW, b.SW, t)
	r.TonStaerke = mischF(a.TonStaerke, b.TonStaerke, t)
	r.Matt = mischF(a.Matt, b.Matt, t)
	r.Klarheit = mischF(a.Klarheit, b.Klarheit, t)
	r.Vignette = mischF(a.Vignette, b.Vignette, t)
	r.Koernung = mischF(a.Koernung, b.Koernung, t)
	for i := 0; i < 3; i++ {
		r.Kanal[i].Tiefe = mischF(a.Kanal[i].Tiefe, b.Kanal[i].Tiefe, t)
		r.Kanal[i].Licht = mischF(a.Kanal[i].Licht, b.Kanal[i].Licht, t)
	}
	if b.SW == 0 {
		r.SWMix = a.SWMix
	}
	return r
}

// Plus: Feinabstimmung zu einem Look addieren.
func Plus(a, f Einstellung) Einstellung {
	r := a
	r.Belichtung += f.Belichtung
	r.Kontrast += f.Kontrast
	r.Lichter += f.Lichter
	r.Tiefen += f.Tiefen
	r.Saettigung += f.Saettigung
	r.Dynamik += f.Dynamik
	r.Temperatur += f.Temperatur
	r.Toenung += f.Toenung
	r.Matt = math.Min(1, r.Matt+f.Matt)
	r.Klarheit += f.Klarheit
	r.Vignette += f.Vignette
	r.Koernung = math.Min(1, r.Koernung+f.Koernung)
	return r
}

func klemm01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func srgbZuLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func linearZuSrgb(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func glatt(s float64) float64 { return s * s * (3 - 2*s) }

// tabellen: je Kanal Belichtung, Weißabgleich und Tonkurve.
func tabellen(e Einstellung) [3][256]float32 {
	var lut [3][256]float32
	fakt := math.Pow(2, e.Belichtung)
	wb := [3]float64{1 + 0.22*e.Temperatur + 0.04*e.Toenung, 1 - 0.12*e.Toenung, 1 - 0.22*e.Temperatur + 0.04*e.Toenung}
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			s := float64(i) / 255
			lin := srgbZuLinear(s) * fakt * wb[c]
			s = linearZuSrgb(math.Min(1, lin))
			if lin > 1 { // sanfter Übergang statt hartem Ausfressen
				s = 1
			}
			// Kontrast: S-Kurve um die Mitte
			if k := e.Kontrast; k != 0 {
				lin := 0.5 + (s-0.5)*(1+0.5*k)
				if k > 0 {
					s = mischF(lin, glatt(klemm01(lin)), math.Min(1, k*0.7))
				} else {
					s = lin
				}
				s = klemm01(s)
			}
			// Tiefen und Lichter
			wT := 4 * s * (1 - s) * (1 - s) // Spitze bei ~0,33
			wL := 4 * s * s * (1 - s)       // Spitze bei ~0,67
			s += e.Tiefen*0.30*wT + e.Lichter*0.30*wL
			// Kanalkurven (Cross, Film)
			s += e.Kanal[c].Tiefe*0.15*wT + e.Kanal[c].Licht*0.15*wL
			s = klemm01(s)
			// Matt: Schwärzen anheben, Weiß leicht dämpfen
			if e.Matt > 0 {
				s = e.Matt*0.13 + s*(1-e.Matt*0.13-e.Matt*0.04)
			}
			lut[c][i] = float32(klemm01(s))
		}
	}
	return lut
}

// Anwenden erzeugt ein neues Bild mit dem Look.
func Anwenden(b *kern.Bild, e Einstellung) *kern.Bild {
	out := kern.NeuesBild(b.W, b.H)
	lut := tabellen(e)
	W, H := b.W, b.H
	n := W * H

	// 1. Tonwerte je Kanal
	f := make([]float32, n*3)
	parallel(H, func(y0, y1 int) {
		for i := y0 * W * 3; i < y1*W*3; i += 3 {
			f[i] = lut[0][b.Pix[i]]
			f[i+1] = lut[1][b.Pix[i+1]]
			f[i+2] = lut[2][b.Pix[i+2]]
		}
	})

	// 2. Klarheit: lokaler Kontrast über eine weichgezeichnete Helligkeit
	var weich []float32
	if math.Abs(e.Klarheit) > 0.005 {
		hell := make([]float32, n)
		for i := 0; i < n; i++ {
			hell[i] = 0.2126*f[3*i] + 0.7152*f[3*i+1] + 0.0722*f[3*i+2]
		}
		radius := max(2, int(float64(max(W, H))*0.015))
		weich = kastenWeich(hell, W, H, radius)
	}

	// Tonung und S/W vorbereiten
	mix := e.SWMix
	if s := mix[0] + mix[1] + mix[2]; math.Abs(s) > 1e-6 {
		mix = [3]float64{mix[0] / s, mix[1] / s, mix[2] / s}
	}
	korn := max(1, max(W, H)/1500) // Korngröße wächst mit der Auflösung
	cx, cy := float64(W)/2, float64(H)/2
	diag := math.Hypot(cx, cy)

	parallel(H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < W; x++ {
				i := y*W + x
				r, g, bl := float64(f[3*i]), float64(f[3*i+1]), float64(f[3*i+2])
				L := 0.2126*r + 0.7152*g + 0.0722*bl
				// Klarheit
				if weich != nil {
					d := L - float64(weich[i])
					Ln := L + e.Klarheit*d*1.2
					if L > 1e-4 {
						q := klemm01(Ln) / L
						r, g, bl = r*q, g*q, bl*q
					} else {
						r, g, bl = Ln, Ln, Ln
					}
					L = 0.2126*r + 0.7152*g + 0.0722*bl
				}
				// Sättigung und Dynamik
				if e.Saettigung != 0 || e.Dynamik != 0 {
					mx := math.Max(r, math.Max(g, bl))
					mn := math.Min(r, math.Min(g, bl))
					fakt := 1 + e.Saettigung + e.Dynamik*(1-(mx-mn))*0.9
					fakt = math.Max(0, fakt)
					r, g, bl = L+(r-L)*fakt, L+(g-L)*fakt, L+(bl-L)*fakt
				}
				// Schwarzweiß
				if e.SW > 0 {
					sw := klemm01(mix[0]*r + mix[1]*g + mix[2]*bl)
					r, g, bl = mischF(r, sw, e.SW), mischF(g, sw, e.SW), mischF(bl, sw, e.SW)
				}
				// Tonung (Lichter und Tiefen getrennt)
				if e.TonStaerke > 0 {
					l := klemm01(0.2126*r + 0.7152*g + 0.0722*bl)
					t := e.TonStaerke * 0.5
					r += t * ((e.TonLichter[0]-0.5)*l + (e.TonTiefen[0]-0.5)*(1-l))
					g += t * ((e.TonLichter[1]-0.5)*l + (e.TonTiefen[1]-0.5)*(1-l))
					bl += t * ((e.TonLichter[2]-0.5)*l + (e.TonTiefen[2]-0.5)*(1-l))
				}
				// Vignette
				if e.Vignette != 0 {
					d := math.Hypot(float64(x)-cx, float64(y)-cy) / diag
					w := glatt(klemm01((d - 0.35) / 0.65))
					if e.Vignette > 0 {
						q := 1 - e.Vignette*0.65*w
						r, g, bl = r*q, g*q, bl*q
					} else {
						q := -e.Vignette * 0.65 * w
						r, g, bl = r+(1-r)*q, g+(1-g)*q, bl+(1-bl)*q
					}
				}
				// Körnung
				if e.Koernung > 0 {
					n := rauschen(x/korn, y/korn) * e.Koernung * 0.06
					l := klemm01(0.2126*r + 0.7152*g + 0.0722*bl)
					n *= 0.4 + 0.6*4*l*(1-l) // in den Mitteltönen am stärksten
					r, g, bl = r+n, g+n, bl+n
				}
				o := 3 * i
				out.Pix[o] = uint8(klemm01(r)*255 + 0.5)
				out.Pix[o+1] = uint8(klemm01(g)*255 + 0.5)
				out.Pix[o+2] = uint8(klemm01(bl)*255 + 0.5)
			}
		}
	})
	return out
}

// rauschen: gleichbleibendes Pseudozufallsrauschen −1 … 1 je Bildpunkt.
func rauschen(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffff)/32767.5 - 1
}

// kastenWeich: dreifacher Kastenfilter ≈ Gaußscher Weichzeichner.
func kastenWeich(q []float32, W, H, r int) []float32 {
	a := append([]float32(nil), q...)
	b := make([]float32, len(q))
	for durchgang := 0; durchgang < 3; durchgang++ {
		// waagerecht a → b
		parallel(H, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				zeile := a[y*W : (y+1)*W]
				ziel := b[y*W : (y+1)*W]
				var sum float32
				for x := -r; x <= r; x++ {
					sum += zeile[max(0, min(W-1, x))]
				}
				inv := 1 / float32(2*r+1)
				for x := 0; x < W; x++ {
					ziel[x] = sum * inv
					sum += zeile[min(W-1, x+r+1)] - zeile[max(0, x-r)]
				}
			}
		})
		// senkrecht b → a
		parallel(W, func(x0, x1 int) {
			inv := 1 / float32(2*r+1)
			for x := x0; x < x1; x++ {
				var sum float32
				for y := -r; y <= r; y++ {
					sum += b[max(0, min(H-1, y))*W+x]
				}
				for y := 0; y < H; y++ {
					a[y*W+x] = sum * inv
					sum += b[min(H-1, y+r+1)*W+x] - b[max(0, y-r)*W+x]
				}
			}
		})
	}
	return a
}

func parallel(n int, f func(a, b int)) {
	k := runtime.NumCPU()
	if k > n {
		k = max(1, n)
	}
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		a, b := n*i/k, n*(i+1)/k
		wg.Add(1)
		go func() { defer wg.Done(); f(a, b) }()
	}
	wg.Wait()
}
