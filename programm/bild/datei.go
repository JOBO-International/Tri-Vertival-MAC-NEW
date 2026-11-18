// Package bild lädt und speichert Bilddateien inklusive EXIF und Farbprofil.
package bild

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdeng/goheif"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"vertical3/dng"
	"vertical3/kern"
	"vertical3/meta"
	rawlib "vertical3/roh"
)

// Endungen, die geöffnet werden können.
var Endungen = append([]string{".jpg", ".jpeg", ".tif", ".tiff", ".png", ".heic", ".heif", ".hif", ".webp", ".dng"}, rawlib.Endungen...)

// Filter für den Datei-öffnen-Dialog (Windows-Schreibweise).
func Filter() string {
	var l []string
	for _, e := range Endungen {
		l = append(l, "*"+e)
	}
	alle := strings.Join(l, ";")
	rohListe := []string{"*.dng"}
	for _, e := range rawlib.Endungen {
		rohListe = append(rohListe, "*"+e)
	}
	return "Bilder und RAW (" + alle + ")|" + alle + "|RAW (" + strings.Join(rohListe, ";") + ")|" + strings.Join(rohListe, ";") + "|Alle Dateien (*.*)|*.*"
}

func IstBild(name string) bool {
	e := strings.ToLower(filepath.Ext(name))
	for _, x := range Endungen {
		if e == x {
			return true
		}
	}
	return false
}

// Geladen enthält das (bereits richtig gedrehte) Bild und seine Daten.
type Geladen struct {
	Bild *kern.Bild
	Exif []byte // TIFF-Block (ohne "Exif\0\0"), Ausrichtung auf 1 gesetzt
	ICC  []byte
	Meta meta.Daten
	// Hinweis zum Laden (z. B. bei DNG nur Vorschaubild), sonst leer
	Hinweis string
}

// Lade öffnet eine Bilddatei. exiftool darf leer sein.
func Lade(pfad, exiftool string) (*Geladen, error) {
	roh, err := os.ReadFile(pfad)
	if err != nil {
		return nil, err
	}
	g := &Geladen{Meta: meta.Daten{}}
	var img image.Image
	orient := 1
	endung := strings.ToLower(filepath.Ext(pfad))
	switch {
	case dng.IstDNG(roh):
		return ladeDNG(pfad, roh, exiftool)
	case rohFormat(endung):
		return ladeRoh(pfad, roh, exiftool)
	case len(roh) > 12 && string(roh[4:8]) == "ftyp": // HEIC/HEIF – am Inhalt erkannt
		img, err = goheif.Decode(bytes.NewReader(roh))
		if err != nil {
			return nil, fmt.Errorf("HEIC-Datei konnte nicht gelesen werden: %w", err)
		}
		if ex, e2 := goheif.ExtractExif(bytes.NewReader(roh)); e2 == nil {
			g.Exif = tiffStart(ex)
		}
		g.ICC = heicICC(roh)
		meta.LiesXMP(roh, g.Meta)
	default:
		img, _, err = image.Decode(bytes.NewReader(roh))
		if err != nil {
			return nil, fmt.Errorf("Bild konnte nicht gelesen werden: %w", err)
		}
		if len(roh) > 3 && roh[0] == 0xFF && roh[1] == 0xD8 {
			ex, xmp, icc := jpegSegmente(roh)
			g.Exif, g.ICC = ex, icc
			meta.LiesXMP(xmp, g.Meta)
		} else {
			if endung == ".tif" || endung == ".tiff" {
				g.Exif = roh // TIFF ist selbst ein EXIF-Block
			}
			meta.LiesXMP(roh, g.Meta)
		}
	}
	if g.Exif != nil {
		orient = meta.LiesExif(g.Exif, g.Meta)
		if endung == ".tif" || endung == ".tiff" {
			g.Exif = nil // wird unten aus den gelesenen Daten neu gebaut
		} else {
			g.Exif = meta.SetzeAusrichtungNormal(g.Exif)
		}
	}
	_ = meta.Exiftool(exiftool, pfad, g.Meta)
	if len(roh) > 12 && string(roh[4:8]) == "ftyp" {
		// Bei HEIC gilt die Drehung des Containers, nicht die EXIF-Angabe.
		orient = heicDrehung(roh, g.Meta)
	}
	g.Bild = kern.AusImage(img).Gedreht(orient)
	echtesHEIC := len(roh) > 12 && string(roh[4:8]) == "ftyp"
	if (endung == ".heic" || endung == ".heif" || endung == ".hif") && !echtesHEIC {
		g.Hinweis = fmt.Sprintf("Hinweis: Diese Datei heißt %s, ist aber ein normales Bild (%d × %d) – vermutlich bei der Übertragung umgewandelt.", endung, g.Bild.W, g.Bild.H)
	}
	if endung == ".dng" {
		g.Hinweis = fmt.Sprintf("Achtung: Diese Datei heißt .dng, ist aber keine RAW-Datei, sondern ein normales Bild (%d × %d) – vermutlich bei der Übertragung vom Handy umgewandelt.", g.Bild.W, g.Bild.H)
	}
	if g.Exif == nil {
		g.Exif = meta.BaueExif(g.Meta)
	}
	return g, nil
}

// ladeDNG entwickelt eine DNG-Rohdatei.
func ladeDNG(pfad string, roh []byte, exiftool string) (*Geladen, error) {
	g := &Geladen{Meta: meta.Daten{}}
	orient := meta.LiesExif(roh, g.Meta) // DNG ist ein TIFF: Kameradaten direkt lesbar
	meta.LiesXMP(roh, g.Meta)
	_ = meta.Exiftool(exiftool, pfad, g.Meta)
	b, info, err := dng.Entwickle(roh)
	if err != nil {
		v, err2 := dng.Vorschau(roh)
		if err2 != nil {
			return nil, fmt.Errorf("DNG konnte nicht entwickelt werden: %w", err)
		}
		b = kern.AusImage(v)
		g.Hinweis = fmt.Sprintf("Nur Vorschaubild (%d × %d) – Rohdaten nicht lesbar: %s", b.W, b.H, err)
		g.Bild = b // Vorschaubilder sind bereits gedreht
	} else {
		g.Hinweis = fmt.Sprintf("RAW entwickelt (%s, %d Bit)", info.Typ, info.Bits)
		g.Bild = b.Gedreht(orient)
	}
	g.Exif = meta.BaueExif(g.Meta)
	return g, nil
}

// heicDrehung: HEIC speichert die Drehung im Container ("irot"), nicht in EXIF.
func heicDrehung(roh []byte, d meta.Daten) int {
	i := bytes.Index(roh, []byte("irot"))
	if i < 4 || i+5 > len(roh) {
		return 1
	}
	groesse := binary.BigEndian.Uint32(roh[i-4:])
	if groesse != 9 {
		return 1
	}
	switch roh[i+4] & 3 { // Drehung gegen den Uhrzeigersinn in 90°-Schritten
	case 1:
		return 8
	case 2:
		return 3
	case 3:
		return 6
	}
	return 1
}

func tiffStart(b []byte) []byte {
	for _, k := range [][]byte{[]byte("II*\x00"), []byte("MM\x00*")} {
		if i := bytes.Index(b, k); i >= 0 && i < 64 {
			return b[i:]
		}
	}
	return nil
}

// jpegSegmente sammelt EXIF, XMP und ICC aus den APP-Segmenten.
func jpegSegmente(b []byte) (exif, xmp, icc []byte) {
	type stueck struct {
		nr  int
		dat []byte
	}
	var iccTeile []stueck
	p := 2
	for p+4 <= len(b) && b[p] == 0xFF {
		marker := b[p+1]
		if marker == 0xD9 || marker == 0xDA {
			break
		}
		laenge := int(binary.BigEndian.Uint16(b[p+2:]))
		if p+2+laenge > len(b) || laenge < 2 {
			break
		}
		seg := b[p+4 : p+2+laenge]
		switch {
		case marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) && exif == nil:
			exif = append([]byte(nil), seg[6:]...)
		case marker == 0xE1 && bytes.HasPrefix(seg, []byte("http://ns.adobe.com/xap/1.0/\x00")):
			xmp = append(xmp, seg...)
		case marker == 0xE2 && bytes.HasPrefix(seg, []byte("ICC_PROFILE\x00")) && len(seg) > 14:
			iccTeile = append(iccTeile, stueck{int(seg[12]), seg[14:]})
		}
		p += 2 + laenge
	}
	for nr := 1; nr <= len(iccTeile); nr++ {
		for _, s := range iccTeile {
			if s.nr == nr {
				icc = append(icc, s.dat...)
			}
		}
	}
	return
}

// heicICC sucht ein eingebettetes Farbprofil ("colr"-Box vom Typ "prof").
func heicICC(b []byte) []byte {
	i := bytes.Index(b, []byte("colrprof"))
	if i < 4 {
		return nil
	}
	groesse := int(binary.BigEndian.Uint32(b[i-4:]))
	start, ende := i+8, i-4+groesse
	if groesse < 12 || ende > len(b) {
		return nil
	}
	return append([]byte(nil), b[start:ende]...)
}

// Speichere schreibt das Bild als JPEG, TIFF oder PNG. EXIF und Farbprofil
// werden bei JPEG übernommen.
func Speichere(b *kern.Bild, ziel string, exif, icc []byte) error {
	var buf bytes.Buffer
	img := b.RGBA()
	switch strings.ToLower(filepath.Ext(ziel)) {
	case ".tif", ".tiff":
		if err := tiff.Encode(&buf, img, &tiff.Options{Compression: tiff.Deflate}); err != nil {
			return err
		}
	case ".png":
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
	default:
		var roh bytes.Buffer
		if err := jpeg.Encode(&roh, img, &jpeg.Options{Quality: 95}); err != nil {
			return err
		}
		buf.Write(roh.Bytes()[:2]) // SOI
		if len(exif) > 0 && len(exif)+8 <= 65535 {
			schreibeSegment(&buf, 0xE1, append([]byte("Exif\x00\x00"), exif...))
		}
		if len(icc) > 0 {
			const max = 65519
			anzahl := (len(icc) + max - 1) / max
			for i := 0; i < anzahl; i++ {
				ende := (i + 1) * max
				if ende > len(icc) {
					ende = len(icc)
				}
				kopf := append([]byte("ICC_PROFILE\x00"), byte(i+1), byte(anzahl))
				schreibeSegment(&buf, 0xE2, append(kopf, icc[i*max:ende]...))
			}
		}
		buf.Write(roh.Bytes()[2:])
	}
	tmp := ziel + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ziel)
}

func schreibeSegment(buf *bytes.Buffer, marker byte, daten []byte) {
	buf.Write([]byte{0xFF, marker})
	var l [2]byte
	binary.BigEndian.PutUint16(l[:], uint16(len(daten)+2))
	buf.Write(l[:])
	buf.Write(daten)
}

// Zielname: Original + "_korr", HEIC/WebP werden als JPEG gespeichert.
func Zielname(pfad, ordner string) string {
	stamm := strings.TrimSuffix(filepath.Base(pfad), filepath.Ext(pfad))
	e := strings.ToLower(filepath.Ext(pfad))
	switch e {
	case ".heic", ".heif", ".hif", ".webp", ".dng":
		e = ".jpg"
	default:
		if rohFormat(e) {
			e = ".jpg"
		}
	}
	if ordner == "" {
		ordner = filepath.Dir(pfad)
	}
	return filepath.Join(ordner, stamm+"_korr"+e)
}
