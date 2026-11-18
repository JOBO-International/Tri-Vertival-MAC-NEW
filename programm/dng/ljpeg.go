// Package dng entwickelt DNG-Rohdaten (RAW) zu einem RGB-Bild.
package dng

import (
	"errors"
	"fmt"
)

// Verlustfreies JPEG (ITU T.81 Prozess 14, "LJ92"), wie in DNG verwendet.

type huffman struct {
	// schnelle Tabelle für Codes bis 9 Bit: Wert = (Länge<<8)|Symbol, 0 = nicht drin
	schnell [512]uint16
	// kanonische Tabellen für längere Codes
	maxcode [18]int32
	valptr  [17]int32
	mincode [17]int32
	werte   []uint8
}

func neuerHuffman(anzahl [16]uint8, werte []uint8) *huffman {
	h := &huffman{werte: werte}
	code := int32(0)
	k := int32(0)
	for l := 1; l <= 16; l++ {
		n := int32(anzahl[l-1])
		h.valptr[l] = k
		h.mincode[l] = code
		for i := int32(0); i < n; i++ {
			if l <= 9 {
				// alle 9-Bit-Muster mit diesem Präfix eintragen
				basis := code << (9 - l)
				for f := int32(0); f < 1<<(9-l); f++ {
					h.schnell[basis+f] = uint16(l)<<8 | uint16(werte[k+i])
				}
			}
			code++
		}
		k += n
		h.maxcode[l] = code - 1
		if n == 0 {
			h.maxcode[l] = -1
		}
		code <<= 1
	}
	h.maxcode[17] = 0x7fffffff
	return h
}

type bitleser struct {
	d    []byte
	pos  int
	puf  uint64
	bits uint
	ende bool
}

func (b *bitleser) fuelle() {
	for b.bits <= 56 {
		var c byte
		if b.pos < len(b.d) {
			c = b.d[b.pos]
			if c == 0xFF {
				if b.pos+1 < len(b.d) && b.d[b.pos+1] == 0x00 {
					b.pos += 2
				} else { // Marker: ab hier Nullen liefern
					c = 0
					b.ende = true
				}
			} else {
				b.pos++
			}
		} else {
			b.ende = true
		}
		b.puf |= uint64(c) << (56 - b.bits)
		b.bits += 8
	}
}

func (b *bitleser) schau(n uint) uint32 {
	if b.bits < n {
		b.fuelle()
	}
	return uint32(b.puf >> (64 - n))
}

func (b *bitleser) weg(n uint) {
	b.puf <<= n
	b.bits -= n
}

func (b *bitleser) lies(n uint) uint32 {
	if n == 0 {
		return 0
	}
	v := b.schau(n)
	b.weg(n)
	return v
}

func (h *huffman) dekodiere(b *bitleser) (int, error) {
	if b.bits < 16 {
		b.fuelle()
	}
	if e := h.schnell[b.puf>>55]; e != 0 {
		b.weg(uint(e >> 8))
		return int(e & 0xFF), nil
	}
	code := int32(0)
	for l := 1; l <= 16; l++ {
		code = code<<1 | int32(b.lies(1))
		if code <= h.maxcode[l] && h.maxcode[l] >= 0 {
			return int(h.werte[h.valptr[l]+code-h.mincode[l]]), nil
		}
	}
	return 0, errors.New("ungültiger Huffman-Code")
}

// LJ92 dekodiert verlustfreies JPEG. Ergebnis: Werte zeilenweise,
// breite = Spalten × Komponenten.
func LJ92(d []byte) (werte []uint16, breite, hoehe, komponenten int, err error) {
	if len(d) < 4 || d[0] != 0xFF || d[1] != 0xD8 {
		return nil, 0, 0, 0, errors.New("kein verlustfreies JPEG")
	}
	tabellen := map[int]*huffman{}
	var praez, x, y, nf int
	var compTab []int
	var pred, pt int
	pos := 2
	restart := 0
	for {
		if pos+4 > len(d) {
			return nil, 0, 0, 0, errors.New("JPEG unvollständig")
		}
		if d[pos] != 0xFF {
			pos++
			continue
		}
		m := d[pos+1]
		if m == 0xFF {
			pos++
			continue
		}
		l := int(d[pos+2])<<8 | int(d[pos+3])
		seg := d[pos+4 : min(len(d), pos+2+l)]
		switch m {
		case 0xC4: // DHT
			p := 0
			for p+17 <= len(seg) {
				id := int(seg[p] & 0x0F)
				var anz [16]uint8
				gesamt := 0
				for i := 0; i < 16; i++ {
					anz[i] = seg[p+1+i]
					gesamt += int(anz[i])
				}
				if p+17+gesamt > len(seg) {
					return nil, 0, 0, 0, errors.New("Huffman-Tabelle defekt")
				}
				tabellen[id] = neuerHuffman(anz, append([]uint8(nil), seg[p+17:p+17+gesamt]...))
				p += 17 + gesamt
			}
		case 0xC3: // SOF3
			praez = int(seg[0])
			y = int(seg[1])<<8 | int(seg[2])
			x = int(seg[3])<<8 | int(seg[4])
			nf = int(seg[5])
		case 0xDD:
			restart = int(seg[0])<<8 | int(seg[1])
		case 0xDA: // SOS
			ns := int(seg[0])
			compTab = make([]int, ns)
			for i := 0; i < ns; i++ {
				compTab[i] = int(seg[2+2*i] >> 4)
			}
			pred = int(seg[1+2*ns])
			pt = int(seg[3+2*ns] & 0x0F)
			pos = pos + 2 + l
			if nf == 0 || x == 0 || y == 0 || ns != nf {
				return nil, 0, 0, 0, fmt.Errorf("JPEG-Format nicht unterstützt (%d/%d Komponenten)", ns, nf)
			}
			for _, t := range compTab {
				if tabellen[t] == nil {
					return nil, 0, 0, 0, errors.New("Huffman-Tabelle fehlt")
				}
			}
			werte, err = dekodiereDaten(d[pos:], tabellen, compTab, x, y, nf, praez, pred, pt, restart)
			return werte, x * nf, y, nf, err
		case 0xD9:
			return nil, 0, 0, 0, errors.New("keine Bilddaten")
		}
		pos += 2 + l
	}
}

func dekodiereDaten(d []byte, tab map[int]*huffman, compTab []int, x, y, nf, praez, pred, pt, restart int) ([]uint16, error) {
	out := make([]uint16, x*y*nf)
	b := &bitleser{d: d}
	zeile := x * nf
	start := 1 << (praez - pt - 1)
	hs := make([]*huffman, nf)
	for i := range hs {
		hs[i] = tab[compTab[i]]
	}
	mcu := 0
	neuerAbschnitt := true
	for r := 0; r < y; r++ {
		for c := 0; c < x; c++ {
			if restart > 0 && mcu > 0 && mcu%restart == 0 {
				// Restart-Marker überspringen
				b.puf, b.bits = 0, 0
				for b.pos+1 < len(b.d) && !(b.d[b.pos] == 0xFF && b.d[b.pos+1] >= 0xD0 && b.d[b.pos+1] <= 0xD7) {
					b.pos++
				}
				b.pos += 2
				b.ende = false
				neuerAbschnitt = true
			}
			mcu++
			for k := 0; k < nf; k++ {
				s, err := hs[k].dekodiere(b)
				if err != nil {
					return out, err
				}
				var diff int
				if s == 16 {
					diff = 32768
				} else if s > 0 {
					v := int(b.lies(uint(s)))
					if v < 1<<(s-1) {
						v -= (1 << s) - 1
					}
					diff = v
				}
				i := r*zeile + c*nf + k
				var p int
				switch {
				case neuerAbschnitt && c == 0 && (r == 0 || restart > 0):
					p = start
				case r == 0 || (restart > 0 && neuerAbschnitt):
					p = int(out[i-nf])
				case c == 0:
					p = int(out[i-zeile])
				default:
					ra, rb, rc := int(out[i-nf]), int(out[i-zeile]), int(out[i-zeile-nf])
					switch pred {
					case 1:
						p = ra
					case 2:
						p = rb
					case 3:
						p = rc
					case 4:
						p = ra + rb - rc
					case 5:
						p = ra + ((rb - rc) >> 1)
					case 6:
						p = rb + ((ra - rc) >> 1)
					case 7:
						p = (ra + rb) >> 1
					default:
						p = ra
					}
				}
				out[i] = uint16((p + diff) & 0xFFFF)
			}
			if c == x-1 {
				neuerAbschnitt = false
			}
		}
	}
	if pt > 0 {
		for i := range out {
			out[i] <<= uint(pt)
		}
	}
	return out, nil
}
