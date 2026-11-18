package meta

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"strconv"
)

// BaueExif erzeugt einen kleinen EXIF-Block (TIFF, Little Endian) mit den
// wichtigsten Kameradaten – z. B. für JPEGs aus DNG- oder TIFF-Dateien.
// Die Ausrichtung ist immer 1 (Bild ist bereits gedreht).
func BaueExif(d Daten) []byte {
	type feld struct {
		tag  uint16
		typ  uint16
		anz  uint32
		wert []byte
	}
	le := binary.LittleEndian
	text := func(tag uint16, s string) *feld {
		if s == "" {
			return nil
		}
		b := append([]byte(s), 0)
		return &feld{tag, 2, uint32(len(b)), b}
	}
	kurz := func(tag uint16, v int) *feld {
		b := make([]byte, 2)
		le.PutUint16(b, uint16(v))
		return &feld{tag, 3, 1, b}
	}
	bruch := func(tag uint16, s string) *feld {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f <= 0 {
			return nil
		}
		var n, z uint32
		if f < 1 { // Belichtungszeit als 1/x
			n, z = 1, uint32(math.Round(1/f))
		} else {
			n, z = uint32(math.Round(f*100)), 100
		}
		b := make([]byte, 8)
		le.PutUint32(b, n)
		le.PutUint32(b[4:], z)
		return &feld{tag, 5, 1, b}
	}
	zahl := func(s string) int {
		f, _ := strconv.ParseFloat(s, 64)
		return int(math.Round(f))
	}
	var ifd0, exif []*feld
	for _, f := range []*feld{text(0x010F, d["Make"]), text(0x0110, d["Model"]), kurz(0x0112, 1)} {
		if f != nil {
			ifd0 = append(ifd0, f)
		}
	}
	for _, f := range []*feld{
		bruch(0x829A, d["ExposureTime"]), bruch(0x829D, d["FNumber"]), text(0x9003, d["DateTimeOriginal"]),
		bruch(0x920A, d["FocalLength"]), text(0xA434, d["LensModel"]),
	} {
		if f != nil {
			exif = append(exif, f)
		}
	}
	if v := zahl(d["ISO"]); v > 0 {
		exif = append(exif, kurz(0x8827, v))
	}
	if v := zahl(d["FocalLengthIn35mmFormat"]); v > 0 {
		exif = append(exif, kurz(0xA405, v))
	}
	if len(exif) > 0 {
		ifd0 = append(ifd0, &feld{0x8769, 4, 1, make([]byte, 4)}) // Zeiger, unten gesetzt
	}
	sort.Slice(ifd0, func(i, j int) bool { return ifd0[i].tag < ifd0[j].tag })
	sort.Slice(exif, func(i, j int) bool { return exif[i].tag < exif[j].tag })

	var buf bytes.Buffer
	buf.Write([]byte("II*\x00"))
	binary.Write(&buf, le, uint32(8))
	groesse := func(l []*feld) int {
		g := 2 + 12*len(l) + 4
		for _, f := range l {
			if len(f.wert) > 4 {
				g += len(f.wert) + len(f.wert)%2
			}
		}
		return g
	}
	exifPos := 8 + groesse(ifd0)
	schreibe := func(l []*feld, start int) {
		extra := start + 2 + 12*len(l) + 4
		var daten bytes.Buffer
		binary.Write(&buf, le, uint16(len(l)))
		for _, f := range l {
			if f.tag == 0x8769 {
				le.PutUint32(f.wert, uint32(exifPos))
			}
			binary.Write(&buf, le, f.tag)
			binary.Write(&buf, le, f.typ)
			binary.Write(&buf, le, f.anz)
			if len(f.wert) <= 4 {
				w := make([]byte, 4)
				copy(w, f.wert)
				buf.Write(w)
			} else {
				binary.Write(&buf, le, uint32(extra+daten.Len()))
				daten.Write(f.wert)
				if len(f.wert)%2 == 1 {
					daten.WriteByte(0)
				}
			}
		}
		binary.Write(&buf, le, uint32(0))
		buf.Write(daten.Bytes())
	}
	schreibe(ifd0, 8)
	if len(exif) > 0 {
		schreibe(exif, exifPos)
	}
	return buf.Bytes()
}
