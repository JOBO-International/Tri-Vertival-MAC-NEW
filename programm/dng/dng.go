package dng

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"runtime"
	"sort"
	"sync"
)

// ------------------------------------------------------------------ TIFF

type tiff struct {
	b  []byte
	bo binary.ByteOrder
}

type eintrag struct {
	typ    uint16
	anzahl int
	pos    int
}

type ifd map[uint16]eintrag

var groesse = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4, 16: 8}

func (t *tiff) lies(pos int) ifd {
	out := ifd{}
	if pos <= 0 || pos+2 > len(t.b) {
		return out
	}
	n := int(t.bo.Uint16(t.b[pos:]))
	for i := 0; i < n && i < 2000; i++ {
		p := pos + 2 + 12*i
		if p+12 > len(t.b) {
			break
		}
		tag, typ, anz := t.bo.Uint16(t.b[p:]), t.bo.Uint16(t.b[p+2:]), int(t.bo.Uint32(t.b[p+4:]))
		g := groesse[typ] * anz
		w := p + 8
		if g > 4 {
			w = int(t.bo.Uint32(t.b[p+8:]))
		}
		if g == 0 || w < 0 || w+g > len(t.b) {
			continue
		}
		out[tag] = eintrag{typ, anz, w}
	}
	return out
}

func (t *tiff) zahl(e eintrag, i int) float64 {
	p := e.pos + i*groesse[e.typ]
	switch e.typ {
	case 1, 7:
		return float64(t.b[p])
	case 6:
		return float64(int8(t.b[p]))
	case 3:
		return float64(t.bo.Uint16(t.b[p:]))
	case 8:
		return float64(int16(t.bo.Uint16(t.b[p:])))
	case 4, 13:
		return float64(t.bo.Uint32(t.b[p:]))
	case 9:
		return float64(int32(t.bo.Uint32(t.b[p:])))
	case 5:
		n, d := t.bo.Uint32(t.b[p:]), t.bo.Uint32(t.b[p+4:])
		if d == 0 {
			return 0
		}
		return float64(n) / float64(d)
	case 10:
		n, d := int32(t.bo.Uint32(t.b[p:])), int32(t.bo.Uint32(t.b[p+4:]))
		if d == 0 {
			return 0
		}
		return float64(n) / float64(d)
	case 11:
		return float64(math.Float32frombits(t.bo.Uint32(t.b[p:])))
	case 12:
		return math.Float64frombits(t.bo.Uint64(t.b[p:]))
	}
	return 0
}

func (t *tiff) zahlen(d ifd, tag uint16) []float64 {
	e, ok := d[tag]
	if !ok {
		return nil
	}
	v := make([]float64, e.anzahl)
	for i := range v {
		v[i] = t.zahl(e, i)
	}
	return v
}

func (t *tiff) wert(d ifd, tag uint16, standard float64) float64 {
	if v := t.zahlen(d, tag); len(v) > 0 {
		return v[0]
	}
	return standard
}

// ------------------------------------------------------------------ Rohbild

const (
	cfa       = 32803
	linearRaw = 34892
)

// Info beschreibt das gefundene Rohbild.
type Info struct {
	Breite, Hoehe int
	Kompression   int
	Typ           string
	Bits          int
}

type rohbild struct {
	w, h, spp   int
	daten       []uint16 // w*h*spp
	photometric int
	cfaMuster   [4]int // 2×2: 0=R 1=G 2=B
	schwarz     []float64
	schwarzDim  [2]int
	weiss       []float64
	linear      []uint16
	aktiv       [4]int // oben, links, unten, rechts
	opcodes     map[int][]abbildung
	crop        [4]float64
	info        Info
}

// Ist DNG? (TIFF mit DNGVersion-Tag)
func IstDNG(b []byte) bool {
	t, err := neu(b)
	if err != nil {
		return false
	}
	_, ok := t.lies(int(t.bo.Uint32(b[4:])))[0xC612]
	return ok
}

func neu(b []byte) (*tiff, error) {
	if len(b) < 8 {
		return nil, errors.New("Datei zu kurz")
	}
	switch string(b[:4]) {
	case "II*\x00":
		return &tiff{b, binary.LittleEndian}, nil
	case "MM\x00*":
		return &tiff{b, binary.BigEndian}, nil
	}
	return nil, errors.New("keine TIFF/DNG-Datei")
}

// findeRohIFD sucht das Hauptbild (NewSubFileType = 0) mit CFA oder LinearRaw.
func (t *tiff) findeRohIFD() (ifd, ifd, error) {
	ifd0 := t.lies(int(t.bo.Uint32(t.b[4:])))
	kandidaten := []ifd{ifd0}
	if sub, ok := ifd0[0x014A]; ok {
		for i := 0; i < sub.anzahl; i++ {
			kandidaten = append(kandidaten, t.lies(int(t.zahl(sub, i))))
		}
	}
	var beste ifd
	besteFl := 0.0
	for _, d := range kandidaten {
		if t.wert(d, 0x00FE, 0) != 0 {
			continue
		}
		ph := int(t.wert(d, 0x0106, 0))
		if ph != cfa && ph != linearRaw {
			continue
		}
		fl := t.wert(d, 0x0100, 0) * t.wert(d, 0x0101, 0)
		if fl > besteFl {
			beste, besteFl = d, fl
		}
	}
	if beste == nil {
		return nil, ifd0, errors.New("in der DNG-Datei wurden keine Rohdaten gefunden")
	}
	return beste, ifd0, nil
}

func (t *tiff) ladeRoh(d ifd) (*rohbild, error) {
	r := &rohbild{
		w:           int(t.wert(d, 0x0100, 0)),
		h:           int(t.wert(d, 0x0101, 0)),
		spp:         int(t.wert(d, 0x0115, 1)),
		photometric: int(t.wert(d, 0x0106, 0)),
	}
	bits := int(t.wert(d, 0x0102, 16))
	komp := int(t.wert(d, 0x0103, 1))
	r.info = Info{Breite: r.w, Hoehe: r.h, Kompression: komp, Bits: bits}
	if r.photometric == cfa {
		r.info.Typ = "Sensormuster (CFA)"
		if r.spp != 1 {
			return nil, errors.New("CFA-Daten mit mehreren Kanälen werden nicht unterstützt")
		}
		dim := t.zahlen(d, 0x828D)
		muster := t.zahlen(d, 0x828E)
		if len(dim) == 2 && (dim[0] != 2 || dim[1] != 2) {
			return nil, errors.New("dieses Sensormuster (z. B. Fujifilm X-Trans) wird nicht unterstützt")
		}
		if len(muster) < 4 {
			muster = []float64{0, 1, 1, 2}
		}
		for i := 0; i < 4; i++ {
			if muster[i] > 2 {
				return nil, errors.New("nur Rot/Grün/Blau-Sensoren werden unterstützt")
			}
			r.cfaMuster[i] = int(muster[i])
		}
	} else {
		r.info.Typ = "lineare Rohdaten"
		if r.spp != 3 {
			return nil, fmt.Errorf("lineare Rohdaten mit %d Kanälen werden nicht unterstützt", r.spp)
		}
	}
	if r.w <= 0 || r.h <= 0 || r.w*r.h > 400_000_000 {
		return nil, errors.New("ungültige Bildgröße")
	}
	r.daten = make([]uint16, r.w*r.h*r.spp)

	// Kacheln oder Streifen
	var offs, laengen []float64
	kw, kh := r.w, 0
	if _, ok := d[0x0144]; ok {
		offs, laengen = t.zahlen(d, 0x0144), t.zahlen(d, 0x0145)
		kw, kh = int(t.wert(d, 0x0142, 0)), int(t.wert(d, 0x0143, 0))
	} else {
		offs, laengen = t.zahlen(d, 0x0111), t.zahlen(d, 0x0117)
		kh = int(t.wert(d, 0x0116, float64(r.h)))
		if kh > r.h || kh <= 0 {
			kh = r.h
		}
	}
	if len(offs) == 0 || len(offs) != len(laengen) || kw <= 0 || kh <= 0 {
		return nil, errors.New("Bilddaten-Aufteilung fehlt")
	}
	spalten := (r.w + kw - 1) / kw
	var fehler error
	var mu sync.Mutex
	parallel(len(offs), func(a, b int) {
		for i := a; i < b; i++ {
			o, l := int(offs[i]), int(laengen[i])
			if o < 0 || o+l > len(t.b) {
				mu.Lock()
				fehler = errors.New("Bilddaten außerhalb der Datei")
				mu.Unlock()
				return
			}
			x0, y0 := (i%spalten)*kw, (i/spalten)*kh
			if err := r.kachel(t, t.b[o:o+l], komp, bits, x0, y0, kw, kh); err != nil {
				mu.Lock()
				fehler = err
				mu.Unlock()
				return
			}
		}
	})
	if fehler != nil {
		return nil, fehler
	}

	r.schwarz = t.zahlen(d, 0xC61A)
	if len(r.schwarz) == 0 {
		r.schwarz = []float64{0}
	}
	if dim := t.zahlen(d, 0xC619); len(dim) == 2 {
		r.schwarzDim = [2]int{int(dim[0]), int(dim[1])}
	} else {
		r.schwarzDim = [2]int{1, 1}
	}
	r.weiss = t.zahlen(d, 0xC61D)
	if len(r.weiss) == 0 {
		r.weiss = []float64{math.Pow(2, float64(bits)) - 1}
	}
	if lt := t.zahlen(d, 0xC618); len(lt) > 1 {
		r.linear = make([]uint16, len(lt))
		for i, v := range lt {
			r.linear[i] = uint16(v)
		}
	}
	if e, ok := d[0xC741]; ok {
		r.opcodes = leseOpcodes(t.b[e.pos:e.pos+e.anzahl*groesse[e.typ]], r.spp)
	}
	r.aktiv = [4]int{0, 0, r.h, r.w}
	if a := t.zahlen(d, 0xC68D); len(a) == 4 {
		r.aktiv = [4]int{int(a[0]), int(a[1]), int(a[2]), int(a[3])}
	}
	aw, ah := float64(r.aktiv[3]-r.aktiv[1]), float64(r.aktiv[2]-r.aktiv[0])
	r.crop = [4]float64{0, 0, aw, ah}
	if o := t.zahlen(d, 0xC61F); len(o) == 2 {
		r.crop[0], r.crop[1] = o[0], o[1]
	}
	if s := t.zahlen(d, 0xC620); len(s) == 2 {
		r.crop[2], r.crop[3] = s[0], s[1]
	}
	return r, nil
}

// kachel entpackt eine Kachel/einen Streifen an Position x0,y0.
func (r *rohbild) kachel(t *tiff, d []byte, komp, bits, x0, y0, kw, kh int) error {
	setze := func(x, y, k int, v uint16) {
		if x < r.w && y < r.h {
			r.daten[(y*r.w+x)*r.spp+k] = v
		}
	}
	switch komp {
	case 1: // unkomprimiert
		n := kw * r.spp
		if bits == 16 {
			for y := 0; y < kh; y++ {
				for x := 0; x < n; x++ {
					p := (y*n + x) * 2
					if p+2 > len(d) {
						return nil
					}
					setze(x0+x/r.spp, y0+y, x%r.spp, t.bo.Uint16(d[p:]))
				}
			}
			return nil
		}
		zeilenBytes := (n*bits + 7) / 8
		for y := 0; y < kh; y++ {
			var puf uint64
			var nb uint
			p := y * zeilenBytes
			for x := 0; x < n; x++ {
				for nb < uint(bits) {
					var c byte
					if p < len(d) {
						c = d[p]
					}
					p++
					puf = puf<<8 | uint64(c)
					nb += 8
				}
				v := uint16(puf >> (nb - uint(bits)) & (1<<uint(bits) - 1))
				nb -= uint(bits)
				setze(x0+x/r.spp, y0+y, x%r.spp, v)
			}
		}
		return nil
	case 7: // verlustfreies JPEG
		werte, breite, hoehe, _, err := LJ92(d)
		if err != nil {
			return err
		}
		n := kw * r.spp
		// Werte füllen die Kachel zeilenweise
		gesamt := breite * hoehe
		for i := 0; i < gesamt && i < n*kh; i++ {
			y, x := i/n, i%n
			setze(x0+x/r.spp, y0+y, x%r.spp, werte[i])
		}
		return nil
	case 34892: // verlustbehaftetes JPEG (8 Bit)
		img, err := jpeg.Decode(bytes.NewReader(d))
		if err != nil {
			return fmt.Errorf("JPEG-Kachel defekt: %w", err)
		}
		b := img.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
		for y := 0; y < b.Dy() && y < kh; y++ {
			for x := 0; x < b.Dx() && x < kw; x++ {
				o := y*rgba.Stride + x*4
				for k := 0; k < r.spp && k < 3; k++ {
					setze(x0+x, y0+y, k, uint16(rgba.Pix[o+k]))
				}
			}
		}
		return nil
	case 52546:
		return errors.New("diese DNG-Datei ist mit JPEG XL komprimiert (z. B. neuere iPhone-ProRAW) – das wird noch nicht unterstützt")
	}
	return fmt.Errorf("DNG-Kompression %d wird nicht unterstützt", komp)
}

func parallel(n int, f func(a, b int)) {
	k := runtime.NumCPU()
	if k > n {
		k = n
	}
	if k <= 1 {
		f(0, n)
		return
	}
	var wg sync.WaitGroup
	block := (n + k - 1) / k
	for a := 0; a < n; a += block {
		b := min(a+block, n)
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			f(a, b)
		}(a, b)
	}
	wg.Wait()
}

// perzentil liefert näherungsweise das p-Quantil (Stichprobe).
func perzentil(v []float32, p float64) float64 {
	if len(v) == 0 {
		return 1
	}
	s := append([]float32(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return float64(s[int(math.Min(float64(len(s)-1), p*float64(len(s))))])
}

// NurGamma (nur für Tests): keine automatische Belichtung, keine S-Kurve.
var NurGamma bool

// RohDaten liefert die entpackten Rohwerte (für Tests).
func RohDaten(datei []byte) ([]uint16, int, int, int, error) {
	t, err := neu(datei)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	d, _, err := t.findeRohIFD()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	r, err := t.ladeRoh(d)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return r.daten, r.w, r.h, r.spp, nil
}

// abbildung ist eine Umrechnung aus der OpcodeList2 (MapPolynomial/MapTable),
// angewandt auf normierte Werte 0…1 eines Kanals.
type abbildung struct {
	poly    []float64
	tabelle []uint16
}

func (a abbildung) wende(x float64) float64 {
	if a.poly != nil {
		s, p := 0.0, 1.0
		for _, c := range a.poly {
			s += c * p
			p *= x
		}
		return s
	}
	if len(a.tabelle) > 0 {
		i := int(math.Round(x * 65535))
		if i < 0 {
			i = 0
		}
		if i >= len(a.tabelle) {
			i = len(a.tabelle) - 1
		}
		return float64(a.tabelle[i]) / 65535
	}
	return x
}

// leseOpcodes wertet eine OpcodeList (immer Big-Endian) aus.
// Unterstützt: MapTable (6), MapPolynomial (8); andere werden übersprungen.
func leseOpcodes(b []byte, kanaele int) map[int][]abbildung {
	out := map[int][]abbildung{}
	be := binary.BigEndian
	if len(b) < 4 {
		return out
	}
	n := int(be.Uint32(b))
	p := 4
	for i := 0; i < n && p+16 <= len(b); i++ {
		id := be.Uint32(b[p:])
		groesse := int(be.Uint32(b[p+12:]))
		par := b[p+16 : min(len(b), p+16+groesse)]
		p += 16 + groesse
		if len(par) < 36 {
			continue
		}
		ebene, ebenen := int(be.Uint32(par[16:])), int(be.Uint32(par[20:]))
		var a abbildung
		switch id {
		case 8: // MapPolynomial
			grad := int(be.Uint32(par[32:]))
			if grad > 8 || len(par) < 36+8*(grad+1) {
				continue
			}
			for k := 0; k <= grad; k++ {
				a.poly = append(a.poly, math.Float64frombits(be.Uint64(par[36+8*k:])))
			}
		case 6: // MapTable
			anz := int(be.Uint32(par[32:]))
			if anz <= 0 || len(par) < 36+2*anz {
				continue
			}
			a.tabelle = make([]uint16, anz)
			for k := range a.tabelle {
				a.tabelle[k] = be.Uint16(par[36+2*k:])
			}
		default:
			continue
		}
		for e := ebene; e < ebene+ebenen && e < kanaele; e++ {
			out[e] = append(out[e], a)
		}
	}
	return out
}

// Vorschau liefert das größte eingebettete JPEG-Vorschaubild (Rückfallebene,
// wenn die Rohdaten nicht entwickelt werden können).
func Vorschau(datei []byte) (image.Image, error) {
	t, err := neu(datei)
	if err != nil {
		return nil, err
	}
	ifd0 := t.lies(int(t.bo.Uint32(datei[4:])))
	liste := []ifd{ifd0}
	if sub, ok := ifd0[0x014A]; ok {
		for i := 0; i < sub.anzahl; i++ {
			liste = append(liste, t.lies(int(t.zahl(sub, i))))
		}
	}
	var beste image.Image
	bestFl := 0
	versuche := func(o, l int) {
		if o <= 0 || l <= 0 || o+l > len(datei) {
			return
		}
		img, err := jpeg.Decode(bytes.NewReader(datei[o : o+l]))
		if err == nil && img.Bounds().Dx()*img.Bounds().Dy() > bestFl {
			beste, bestFl = img, img.Bounds().Dx()*img.Bounds().Dy()
		}
	}
	for _, d := range liste {
		if int(t.wert(d, 0x0103, 0)) == 7 && int(t.wert(d, 0x0106, 0)) == 6 {
			if o, l := t.zahlen(d, 0x0111), t.zahlen(d, 0x0117); len(o) == 1 && len(l) == 1 {
				versuche(int(o[0]), int(l[0]))
			}
		}
		if o, l := t.wert(d, 0x0201, 0), t.wert(d, 0x0202, 0); o > 0 {
			versuche(int(o), int(l))
		}
	}
	if beste == nil {
		return nil, errors.New("kein Vorschaubild in der DNG-Datei")
	}
	return beste, nil
}
