// Package meta liest die für die Korrektur nötigen Bilddaten – ohne exiftool:
// EXIF (Hersteller, Modell, Brennweite, Ausrichtung), Apple-Schwerkraftvektor,
// Panasonic/Leica-Neigungswinkel und Leica-XMP („Digital Shift Assistant“).
// Ist exiftool vorhanden, werden dessen Daten zusätzlich übernommen.
package meta

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
)

// Daten: Feldname → Wert (als Text). Felder gibt es mit und ohne Gruppe,
// z. B. "Panasonic:RollAngle" und "RollAngle".
type Daten map[string]string

func (d Daten) setze(gruppe, name, wert string) {
	if gruppe != "" {
		d[gruppe+":"+name] = wert
	}
	if _, da := d[name]; !da {
		d[name] = wert
	}
}

// tiff liest Werte aus einem TIFF/EXIF-Block.
type tiff struct {
	b  []byte
	bo binary.ByteOrder
}

func neuesTiff(b []byte) (*tiff, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("EXIF zu kurz")
	}
	switch string(b[:2]) {
	case "II":
		return &tiff{b, binary.LittleEndian}, nil
	case "MM":
		return &tiff{b, binary.BigEndian}, nil
	}
	return nil, fmt.Errorf("kein TIFF-Kopf")
}

type eintrag struct {
	tag, typ uint16
	anzahl   uint32
	wertPos  int // Position der Daten im Block
}

var typGroesse = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8}

// ifd liest die Einträge eines IFD; basis = Bezugspunkt für Offsets.
func (t *tiff) ifd(pos, basis int) map[uint16]eintrag {
	out := map[uint16]eintrag{}
	if pos < 0 || pos+2 > len(t.b) {
		return out
	}
	n := int(t.bo.Uint16(t.b[pos:]))
	for i := 0; i < n && n < 1000; i++ {
		p := pos + 2 + 12*i
		if p+12 > len(t.b) {
			break
		}
		e := eintrag{tag: t.bo.Uint16(t.b[p:]), typ: t.bo.Uint16(t.b[p+2:]), anzahl: t.bo.Uint32(t.b[p+4:])}
		groesse := typGroesse[e.typ] * int(e.anzahl)
		if groesse <= 4 {
			e.wertPos = p + 8
		} else {
			e.wertPos = basis + int(t.bo.Uint32(t.b[p+8:]))
		}
		if e.wertPos < 0 || e.wertPos+groesse > len(t.b) {
			continue
		}
		out[e.tag] = e
	}
	return out
}

func (t *tiff) text(e eintrag) string {
	s := t.b[e.wertPos : e.wertPos+int(e.anzahl)]
	return strings.TrimRight(string(bytes.TrimRight(s, "\x00")), " ")
}

func (t *tiff) zahl(e eintrag, i int) float64 {
	p := e.wertPos + i*typGroesse[e.typ]
	switch e.typ {
	case 1, 7:
		return float64(t.b[p])
	case 6:
		return float64(int8(t.b[p]))
	case 3:
		return float64(t.bo.Uint16(t.b[p:]))
	case 8:
		return float64(int16(t.bo.Uint16(t.b[p:])))
	case 4:
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
	}
	return 0
}

// LiesExif wertet einen EXIF-Block (beginnend mit "II*\0" oder "MM\0*") aus.
// Gibt außerdem die EXIF-Ausrichtung zurück.
func LiesExif(b []byte, d Daten) (orient int) {
	t, err := neuesTiff(b)
	if err != nil {
		return 1
	}
	ifd0 := t.ifd(int(t.bo.Uint32(b[4:])), 0)
	orient = 1
	if e, ok := ifd0[0x0112]; ok {
		orient = int(t.zahl(e, 0))
	}
	if e, ok := ifd0[0x010F]; ok {
		d.setze("IFD0", "Make", t.text(e))
	}
	if e, ok := ifd0[0x0110]; ok {
		d.setze("IFD0", "Model", t.text(e))
	}
	sub, ok := ifd0[0x8769]
	if !ok {
		return orient
	}
	exif := t.ifd(int(t.zahl(sub, 0)), 0)
	if e, ok := exif[0xA405]; ok && t.zahl(e, 0) > 0 {
		d.setze("ExifIFD", "FocalLengthIn35mmFormat", fmt.Sprint(t.zahl(e, 0)))
	}
	if e, ok := exif[0x920A]; ok && t.zahl(e, 0) > 0 {
		d.setze("ExifIFD", "FocalLength", fmt.Sprint(t.zahl(e, 0)))
	}
	if e, ok := exif[0xA434]; ok {
		d.setze("ExifIFD", "LensModel", t.text(e))
	}
	if e, ok := exif[0x9003]; ok {
		d.setze("ExifIFD", "DateTimeOriginal", t.text(e))
	}
	for tag, name := range map[uint16]string{0x829A: "ExposureTime", 0x829D: "FNumber", 0x8827: "ISO"} {
		if e, ok := exif[tag]; ok && t.zahl(e, 0) > 0 {
			d.setze("ExifIFD", name, fmt.Sprint(t.zahl(e, 0)))
		}
	}
	if e, ok := exif[0x927C]; ok {
		makernote(t, e.wertPos, int(e.anzahl), d)
	}
	return orient
}

func makernote(t *tiff, pos, laenge int, d Daten) {
	mn := t.b[pos : pos+laenge]
	switch {
	case bytes.HasPrefix(mn, []byte("Apple iOS")) && len(mn) > 16:
		// eigener Byte-Order-Kopf, Offsets relativ zum Beginn der Herstellerdaten
		at := &tiff{b: mn, bo: binary.BigEndian}
		if string(mn[12:14]) == "II" {
			at.bo = binary.LittleEndian
		}
		for tag, e := range at.ifd(14, 0) {
			if tag == 0x0008 && e.typ == 10 && e.anzahl == 3 {
				d.setze("Apple", "AccelerationVector",
					fmt.Sprintf("%.6f %.6f %.6f", at.zahl(e, 0), at.zahl(e, 1), at.zahl(e, 2)))
			}
		}
	case bytes.HasPrefix(mn, []byte("Panasonic\x00")):
		// Panasonic (auch Leica D-Lux): IFD nach 12 Byte Kopf. Byte-Order ist die
		// des TIFF – wie exiftool prüfen wir aber beide und nehmen die plausible.
		pt := t
		if n := t.bo.Uint16(t.b[pos+12:]); n == 0 || n > 300 {
			andere := binary.ByteOrder(binary.BigEndian)
			if t.bo == binary.BigEndian {
				andere = binary.LittleEndian
			}
			pt = &tiff{b: t.b, bo: andere}
		}
		for tag, e := range pt.ifd(pos+12, 0) {
			switch tag {
			case 0x90: // Roll: Grad im Uhrzeigersinn ×10
				d.setze("Panasonic", "RollAngle", fmt.Sprint(float64(int16(uint16(pt.zahl(e, 0))))/10))
			case 0x91: // Pitch: Grad nach oben ×10, gespeichert mit umgekehrtem Vorzeichen
				d.setze("Panasonic", "PitchAngle", fmt.Sprint(-float64(int16(uint16(pt.zahl(e, 0))))/10))
			}
		}
	}
}

var (
	xmpAttr = regexp.MustCompile(`xmpDSA:(\w+)\s*=\s*"([^"]*)"`)
	xmpElem = regexp.MustCompile(`<xmpDSA:(\w+)>([^<]*)</xmpDSA:(\w+)>`)
)

// LiesXMP übernimmt Leicas "Digital Shift Assistant" aus einem XMP-Block.
func LiesXMP(x []byte, d Daten) {
	for _, m := range xmpAttr.FindAllSubmatch(x, -1) {
		d.setze("XMP-xmpDSA", string(m[1]), string(m[2]))
	}
	for _, m := range xmpElem.FindAllSubmatch(x, -1) {
		if string(m[1]) == string(m[3]) {
			d.setze("XMP-xmpDSA", string(m[1]), strings.TrimSpace(string(m[2])))
		}
	}
}

// SetzeAusrichtungNormal setzt im EXIF-Block die Ausrichtung auf 1 (Kopie).
func SetzeAusrichtungNormal(b []byte) []byte {
	out := append([]byte(nil), b...)
	t, err := neuesTiff(out)
	if err != nil {
		return out
	}
	if e, ok := t.ifd(int(t.bo.Uint32(out[4:])), 0)[0x0112]; ok && e.typ == 3 {
		t.bo.PutUint16(out[e.wertPos:], 1)
	}
	return out
}
