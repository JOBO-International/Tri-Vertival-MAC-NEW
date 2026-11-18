package bild

import (
	"bytes"
	"fmt"
	"image"
	"strconv"

	"vertical3/kern"
	"vertical3/meta"
	rawlib "vertical3/roh"
)

// rohFormat: Endung eines RAW-Formats, das über LibRaw entwickelt wird (außer DNG).
func rohFormat(endung string) bool { return rawlib.IstRoh(endung) }

func zahlText(f float64, stellen int) string {
	return strconv.FormatFloat(f, 'f', stellen, 64)
}

// ladeRoh entwickelt eine RAW-Datei (NEF, CR2, CR3, ARW, RW2, ORF, RAF, PEF …).
func ladeRoh(pfad string, daten []byte, exiftool string) (*Geladen, error) {
	g := &Geladen{Meta: meta.Daten{}}
	// Viele RAW-Formate sind TIFF-Dateien: Kameradaten lassen sich direkt lesen.
	if ts := tiffStart(daten); ts != nil {
		meta.LiesExif(ts, g.Meta)
	}
	meta.LiesXMP(daten, g.Meta)
	_ = meta.Exiftool(exiftool, pfad, g.Meta)

	b, info, err := rawlib.Entwickle(daten)
	if err != nil {
		// Notlösung: eingebettetes Vorschaubild
		jpg, err2 := rawlib.Vorschau(daten)
		if err2 != nil {
			return nil, fmt.Errorf("RAW-Datei konnte nicht gelesen werden: %w", err)
		}
		img, _, err3 := image.Decode(bytes.NewReader(jpg))
		if err3 != nil {
			return nil, fmt.Errorf("RAW-Datei konnte nicht gelesen werden: %w", err)
		}
		orient := 1
		if ex, _, _ := jpegSegmente(jpg); ex != nil {
			orient = meta.LiesExif(ex, g.Meta)
		}
		g.Bild = kern.AusImage(img).Gedreht(orient)
		g.Hinweis = fmt.Sprintf("Nur Vorschaubild (%d × %d) – die Rohdaten konnten nicht entwickelt werden: %s", g.Bild.W, g.Bild.H, err)
		g.Exif = meta.BaueExif(g.Meta)
		return g, nil
	}
	// fehlende Angaben aus LibRaw ergänzen
	setze := func(k, v string) {
		if v != "" && g.Meta[k] == "" {
			g.Meta[k] = v
		}
	}
	setze("Make", info.Marke)
	setze("Model", info.Modell)
	setze("LensModel", info.Objektiv)
	if info.Brennweite > 0 {
		setze("FocalLength", zahlText(info.Brennweite, 1)+" mm")
	}
	if info.Brennweite35 > 0 {
		setze("FocalLengthIn35mmFormat", zahlText(info.Brennweite35, 0)+" mm")
	}
	if info.ISO > 0 {
		setze("ISO", zahlText(info.ISO, 0))
	}
	if info.Blende > 0 {
		setze("FNumber", zahlText(info.Blende, 1))
	}
	g.Bild = b // LibRaw hat die Kameradrehung schon angewandt
	g.Hinweis = fmt.Sprintf("RAW entwickelt (%s %s, %d × %d)", info.Marke, info.Modell, b.W, b.H)
	g.Exif = meta.BaueExif(g.Meta)
	return g, nil
}
