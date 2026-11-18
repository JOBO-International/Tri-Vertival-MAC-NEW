package belichter

import (
	"bufio"
	"encoding/binary"
	"image"
	"io"
	"math"
	"os"
)

// SchreibeTIFF speichert ein unkomprimiertes 8-Bit-RGB-TIFF (3 Kanäle, kein
// Alphakanal, keine Komprimierung) mit Auflösungsangabe – das Format, das
// Belichter- und RIP-Software am zuverlässigsten liest.
func SchreibeTIFF(pfad string, img *image.RGBA, dpi float64) error {
	f, err := os.Create(pfad)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	if err := schreibeTIFF(w, img, dpi); err != nil {
		f.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func schreibeTIFF(w io.Writer, img *image.RGBA, dpi float64) error {
	b := img.Bounds()
	bw, bh := b.Dx(), b.Dy()
	daten := uint32(bw * bh * 3)
	le := binary.LittleEndian
	type eintrag struct {
		tag, typ uint16
		anzahl   uint32
		wert     uint32
	}
	const nTags = 14
	ifdStart := uint32(8)
	ifdLaenge := uint32(2 + nTags*12 + 4)
	bpsOff := ifdStart + ifdLaenge // BitsPerSample (3 × SHORT)
	xresOff := bpsOff + 6          // RATIONAL
	yresOff := xresOff + 8         // RATIONAL
	softOff := yresOff + 8         // ASCII
	soft := []byte("Tri-Vertical Belichter\x00")
	datOff := softOff + uint32(len(soft))
	if datOff%2 == 1 {
		datOff++
	}
	eintraege := []eintrag{
		{256, 4, 1, uint32(bw)},              // ImageWidth
		{257, 4, 1, uint32(bh)},              // ImageLength
		{258, 3, 3, bpsOff},                  // BitsPerSample 8,8,8
		{259, 3, 1, 1},                       // Compression: keine
		{262, 3, 1, 2},                       // Photometric: RGB
		{273, 4, 1, datOff},                  // StripOffsets
		{277, 3, 1, 3},                       // SamplesPerPixel
		{278, 4, 1, uint32(bh)},              // RowsPerStrip
		{279, 4, 1, daten},                   // StripByteCounts
		{282, 5, 1, xresOff},                 // XResolution
		{283, 5, 1, yresOff},                 // YResolution
		{284, 3, 1, 1},                       // PlanarConfiguration: zusammen
		{296, 3, 1, 2},                       // ResolutionUnit: Zoll
		{305, 2, uint32(len(soft)), softOff}, // Software
	}
	kopf := make([]byte, 0, datOff)
	kopf = append(kopf, 'I', 'I', 42, 0)
	kopf = le.AppendUint32(kopf, ifdStart)
	kopf = le.AppendUint16(kopf, nTags)
	for _, e := range eintraege {
		kopf = le.AppendUint16(kopf, e.tag)
		kopf = le.AppendUint16(kopf, e.typ)
		kopf = le.AppendUint32(kopf, e.anzahl)
		if e.typ == 3 && e.anzahl == 1 {
			kopf = le.AppendUint16(kopf, uint16(e.wert))
			kopf = le.AppendUint16(kopf, 0)
		} else {
			kopf = le.AppendUint32(kopf, e.wert)
		}
	}
	kopf = le.AppendUint32(kopf, 0) // kein weiteres IFD
	kopf = le.AppendUint16(kopf, 8)
	kopf = le.AppendUint16(kopf, 8)
	kopf = le.AppendUint16(kopf, 8)
	z := uint32(math.Round(dpi * 100))
	kopf = le.AppendUint32(kopf, z)
	kopf = le.AppendUint32(kopf, 100)
	kopf = le.AppendUint32(kopf, z)
	kopf = le.AppendUint32(kopf, 100)
	kopf = append(kopf, soft...)
	for uint32(len(kopf)) < datOff {
		kopf = append(kopf, 0)
	}
	if _, err := w.Write(kopf); err != nil {
		return err
	}
	zeile := make([]byte, bw*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		s := img.Pix[img.PixOffset(b.Min.X, y):]
		for x := 0; x < bw; x++ {
			zeile[x*3], zeile[x*3+1], zeile[x*3+2] = s[x*4], s[x*4+1], s[x*4+2]
		}
		if _, err := w.Write(zeile); err != nil {
			return err
		}
	}
	return nil
}
