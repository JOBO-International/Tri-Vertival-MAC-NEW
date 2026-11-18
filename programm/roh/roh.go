// Package roh entwickelt RAW-Dateien der gängigen Kameras (NEF, CR2, CR3, ARW,
// RW2, ORF, RAF, PEF, SRW …) mit der Bibliothek LibRaw (https://www.libraw.org,
// Lizenz CDDL-1.0 / LGPL-2.1; Texte in roh/quelle). LibRaw ist fest eingebunden.
//
// Das Ergebnis ist ein 8-Bit-sRGB-Bild mit Kamera-Weißabgleich, bereits so
// gedreht, wie die Kamera es vorgesehen hat.
package roh

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo CXXFLAGS: -I${SRCDIR}
#cgo linux LDFLAGS: -L${SRCDIR}/lib/linux -lraw -lstdc++ -lm
#cgo windows LDFLAGS: -L${SRCDIR}/lib/windows -lraw -lstdc++ -lws2_32
#cgo darwin LDFLAGS: -L${SRCDIR}/lib/darwin -lraw -lc++ -lm
#include <stdlib.h>
#include "libraw/libraw.h"

static unsigned char* daten(libraw_processed_image_t* p) { return p->data; }
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"vertical3/kern"
)

// Endungen der unterstützten RAW-Formate (DNG hat einen eigenen Weg).
var Endungen = []string{
	".nef", ".nrw", // Nikon
	".cr2", ".cr3", ".crw", // Canon
	".arw", ".sr2", ".srf", // Sony
	".rw2", ".raw", ".rwl", // Panasonic, Leica
	".orf", ".ori", // Olympus / OM System
	".raf",         // Fujifilm
	".pef",         // Pentax
	".srw",         // Samsung
	".3fr", ".fff", // Hasselblad
	".iiq", ".cap", // Phase One
	".erf",                 // Epson
	".mef",                 // Mamiya
	".mos",                 // Leaf
	".mrw",                 // Minolta
	".kdc", ".dcr", ".k25", // Kodak
	".x3f", // Sigma
	".rwz", ".gpr",
}

// IstRoh: Dateiendung eines unterstützten RAW-Formats.
func IstRoh(endung string) bool {
	e := strings.ToLower(endung)
	for _, x := range Endungen {
		if e == x {
			return true
		}
	}
	return false
}

// Info über die entwickelte Datei.
type Info struct {
	Marke, Modell string
	Objektiv      string
	Breite, Hoehe int     // Größe des Ergebnisbildes
	Brennweite    float64 // mm
	Brennweite35  float64 // Kleinbild-äquivalent, 0 = unbekannt
	ISO           float64
	Belichtung    float64 // s
	Blende        float64
	Zeitstempel   int64 // Unix-Zeit der Aufnahme, 0 = unbekannt
	Ausrichtung   int   // EXIF-Ausrichtung der Kamera (schon angewandt)
	Rohbreite     int   // Größe der Rohdaten
	Rohhoehe      int
}

func fehler(rc C.int, was string) error {
	return fmt.Errorf("%s: %s", was, C.GoString(C.libraw_strerror(rc)))
}

func cstr(p *C.char) string { return strings.TrimSpace(C.GoString(p)) }

// Entwickle liest die RAW-Datei aus dem Speicher und entwickelt sie.
func Entwickle(daten []byte) (*kern.Bild, Info, error) {
	var info Info
	if len(daten) < 1024 {
		return nil, info, fmt.Errorf("Datei zu klein")
	}
	d := C.libraw_init(0)
	if d == nil {
		return nil, info, fmt.Errorf("LibRaw konnte nicht gestartet werden")
	}
	defer C.libraw_close(d)

	// LibRaw hält den Zeiger bis zum Ende – deshalb eine Kopie im C-Speicher.
	puffer := C.CBytes(daten)
	defer C.free(puffer)
	if rc := C.libraw_open_buffer(d, puffer, C.size_t(len(daten))); rc != 0 {
		return nil, info, fehler(rc, "RAW-Format nicht erkannt")
	}

	info.Marke = cstr(&d.idata.make[0])
	info.Modell = cstr(&d.idata.model[0])
	if info.Modell == "" {
		info.Modell = cstr(&d.idata.normalized_model[0])
	}
	info.Objektiv = cstr(&d.lens.Lens[0])
	info.Brennweite = float64(d.other.focal_len)
	info.Brennweite35 = float64(d.lens.FocalLengthIn35mmFormat)
	info.ISO = float64(d.other.iso_speed)
	info.Belichtung = float64(d.other.shutter)
	info.Blende = float64(d.other.aperture)
	info.Zeitstempel = int64(d.other.timestamp)
	info.Rohbreite, info.Rohhoehe = int(d.sizes.raw_width), int(d.sizes.raw_height)
	switch d.sizes.flip {
	case 3:
		info.Ausrichtung = 3
	case 5:
		info.Ausrichtung = 8
	case 6:
		info.Ausrichtung = 6
	default:
		info.Ausrichtung = 1
	}

	// Entwicklung wie eine Kamera-JPEG: Kamera-Weißabgleich, sRGB, 8 Bit,
	// Helligkeit automatisch angepasst, hochwertige Farbinterpolation (AHD bzw.
	// X-Trans-Verfahren für Fujifilm).
	p := &d.params
	p.output_bps = 8
	p.output_color = 1 // sRGB
	p.use_camera_wb = 1
	p.use_camera_matrix = 3
	p.user_qual = 3
	p.no_auto_bright = 0
	p.highlight = 0
	p.gamm[0] = 1.0 / 2.4 // sRGB-Kennlinie
	p.gamm[1] = 12.92
	p.fbdd_noiserd = 0

	if rc := C.libraw_unpack(d); rc != 0 {
		return nil, info, fehler(rc, "Rohdaten nicht lesbar")
	}
	if rc := C.libraw_dcraw_process(d); rc != 0 {
		return nil, info, fehler(rc, "Entwicklung fehlgeschlagen")
	}
	var rc C.int
	bild := C.libraw_dcraw_make_mem_image(d, &rc)
	if bild == nil || rc != 0 {
		return nil, info, fehler(rc, "Bild konnte nicht erzeugt werden")
	}
	defer C.libraw_dcraw_clear_mem(bild)

	w, h, farben := int(bild.width), int(bild.height), int(bild.colors)
	if bild._type != C.LIBRAW_IMAGE_BITMAP || bild.bits != 8 || farben != 3 || w <= 0 || h <= 0 {
		return nil, info, fmt.Errorf("unerwartetes Ergebnisformat (%d×%d, %d Farben, %d Bit)", w, h, farben, int(bild.bits))
	}
	if int(bild.data_size) < w*h*3 {
		return nil, info, fmt.Errorf("Bilddaten unvollständig")
	}
	quelle := unsafe.Slice((*byte)(unsafe.Pointer(C.daten(bild))), w*h*3)
	b := kern.NeuesBild(w, h)
	copy(b.Pix, quelle)
	info.Breite, info.Hoehe = w, h
	return b, info, nil
}

// Vorschau liefert das größte eingebettete JPEG der RAW-Datei (Notlösung, wenn
// die Rohdaten nicht entwickelt werden können).
func Vorschau(daten []byte) ([]byte, error) {
	if len(daten) < 1024 {
		return nil, fmt.Errorf("Datei zu klein")
	}
	d := C.libraw_init(0)
	if d == nil {
		return nil, fmt.Errorf("LibRaw konnte nicht gestartet werden")
	}
	defer C.libraw_close(d)
	puffer := C.CBytes(daten)
	defer C.free(puffer)
	if rc := C.libraw_open_buffer(d, puffer, C.size_t(len(daten))); rc != 0 {
		return nil, fehler(rc, "RAW-Format nicht erkannt")
	}
	if rc := C.libraw_unpack_thumb(d); rc != 0 {
		return nil, fehler(rc, "keine Vorschau enthalten")
	}
	var rc C.int
	t := C.libraw_dcraw_make_mem_thumb(d, &rc)
	if t == nil || rc != 0 {
		return nil, fehler(rc, "Vorschau nicht lesbar")
	}
	defer C.libraw_dcraw_clear_mem(t)
	if t._type != C.LIBRAW_IMAGE_JPEG {
		return nil, fmt.Errorf("Vorschau ist kein JPEG")
	}
	return C.GoBytes(unsafe.Pointer(C.daten(t)), C.int(t.data_size)), nil
}
