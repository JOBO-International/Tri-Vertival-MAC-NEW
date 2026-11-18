package main

// Anbindung an die Windows-Druckerverwaltung: Drucker, Papiere, Treiberdialog,
// Seitengeometrie und Ausgabe eines Bildes über GDI.

import (
	"errors"
	"image"
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"vertical3/druck"
)

var (
	winspool          = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinter   = winspool.NewProc("OpenPrinterW")
	procClosePrinter  = winspool.NewProc("ClosePrinter")
	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	procStretchDIBits = gdi32.NewProc("StretchDIBits")
	procSetBrushOrgEx = gdi32.NewProc("SetBrushOrgEx")
)

const (
	dmOrientation = 0x1
	dmPaperSize   = 0x2
	dmCopies      = 0x100
	dmOutBuffer   = 2
	dmInPrompt    = 4
	dmInBuffer    = 8
	dcPapers      = 2
	dcPaperSize   = 3
	dcPaperNames  = 16
)

type papier struct {
	id   int16
	name string
	w, h float64 // mm, Hochformat wie vom Treiber gemeldet
}

type drucker struct {
	name     string
	dm       []byte // DEVMODE inkl. Treiberdaten
	papiere  []papier
	papierW  float64 // aktuelle Seite (mm, gedreht)
	papierH  float64
	druckbar druck.Rechteck
	dpiX     int
	dpiY     int
}

func utf16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// druckerListe: alle installierten Drucker und der Standarddrucker.
func druckerListe() (namen []string, standard string) {
	var noetig, anzahl uint32
	flags := uint32(2 | 4) // LOCAL | CONNECTIONS
	win.EnumPrinters(flags, nil, 4, nil, 0, &noetig, &anzahl)
	if noetig > 0 {
		buf := make([]byte, noetig)
		if win.EnumPrinters(flags, nil, 4, &buf[0], noetig, &noetig, &anzahl) {
			type info4 struct {
				name   *uint16
				server *uint16
				attr   uint32
			}
			groesse := unsafe.Sizeof(info4{})
			for i := uint32(0); i < anzahl; i++ {
				e := (*info4)(unsafe.Pointer(&buf[uintptr(i)*groesse]))
				namen = append(namen, win.UTF16PtrToString(e.name))
			}
		}
	}
	var n uint32 = 512
	b := make([]uint16, n)
	if win.GetDefaultPrinter(&b[0], &n) {
		standard = syscall.UTF16ToString(b)
	}
	return
}

func oeffne(name string) (win.HANDLE, error) {
	var h win.HANDLE
	r, _, e := procOpenPrinter.Call(uintptr(unsafe.Pointer(utf16(name))), uintptr(unsafe.Pointer(&h)), 0)
	if r == 0 {
		return 0, e
	}
	return h, nil
}

func schliesse(h win.HANDLE) { procClosePrinter.Call(uintptr(h)) }

func (d *drucker) devmode() *win.DEVMODE {
	if len(d.dm) == 0 {
		return nil
	}
	return (*win.DEVMODE)(unsafe.Pointer(&d.dm[0]))
}

// neuerDrucker lädt Standardeinstellungen und Papierliste des Treibers.
func neuerDrucker(hwnd win.HWND, name string) (*drucker, error) {
	d := &drucker{name: name}
	h, err := oeffne(name)
	if err != nil {
		return nil, err
	}
	defer schliesse(h)
	n := win.DocumentProperties(hwnd, h, utf16(name), nil, nil, 0)
	if n <= 0 {
		return nil, errors.New("Druckertreiber antwortet nicht")
	}
	d.dm = make([]byte, n)
	if win.DocumentProperties(hwnd, h, utf16(name), d.devmode(), nil, dmOutBuffer) < 0 {
		return nil, errors.New("Druckereinstellungen nicht lesbar")
	}
	d.ladePapiere()
	d.messe()
	return d, nil
}

func (d *drucker) ladePapiere() {
	d.papiere = nil
	n := int32(win.DeviceCapabilities(utf16(d.name), nil, dcPapers, nil, nil))
	if n <= 0 || n > 1000 {
		return
	}
	ids := make([]uint16, n)
	win.DeviceCapabilities(utf16(d.name), nil, dcPapers, &ids[0], nil)
	namen := make([]uint16, 64*n)
	win.DeviceCapabilities(utf16(d.name), nil, dcPaperNames, &namen[0], nil)
	groessen := make([]int32, 2*n)
	win.DeviceCapabilities(utf16(d.name), nil, dcPaperSize, (*uint16)(unsafe.Pointer(&groessen[0])), nil)
	for i := int32(0); i < n; i++ {
		nm := syscall.UTF16ToString(namen[64*i : 64*i+64])
		w, h := float64(groessen[2*i])/10, float64(groessen[2*i+1])/10
		if nm == "" || w <= 0 || h <= 0 {
			continue
		}
		d.papiere = append(d.papiere, papier{id: int16(ids[i]), name: nm, w: w, h: h})
	}
}

// setze übernimmt Papier und Ausrichtung (1 = hoch, 2 = quer) in die Treibereinstellungen.
func (d *drucker) setze(hwnd win.HWND, papierID int16, ausrichtung int16) {
	dm := d.devmode()
	if dm == nil {
		return
	}
	if papierID != 0 {
		dm.DmPaperSize = papierID
		dm.DmFields |= dmPaperSize
	}
	dm.DmOrientation = ausrichtung
	dm.DmFields |= dmOrientation
	dm.DmCopies = 1
	dm.DmFields |= dmCopies
	if h, err := oeffne(d.name); err == nil {
		ein := append([]byte(nil), d.dm...)
		win.DocumentProperties(hwnd, h, utf16(d.name), d.devmode(), (*win.DEVMODE)(unsafe.Pointer(&ein[0])), dmInBuffer|dmOutBuffer)
		schliesse(h)
	}
	d.messe()
}

// dialog zeigt den Einstellungsdialog des Druckertreibers.
func (d *drucker) dialog(hwnd win.HWND) bool {
	h, err := oeffne(d.name)
	if err != nil {
		return false
	}
	defer schliesse(h)
	ein := append([]byte(nil), d.dm...)
	r := win.DocumentProperties(hwnd, h, utf16(d.name), d.devmode(), (*win.DEVMODE)(unsafe.Pointer(&ein[0])), dmInBuffer|dmInPrompt|dmOutBuffer)
	d.messe()
	return r == win.IDOK
}

// messe liest Blattgröße, druckbaren Bereich und Auflösung aus dem Treiber.
func (d *drucker) messe() {
	ic := win.CreateIC(nil, utf16(d.name), nil, d.devmode())
	if ic == 0 {
		d.papierW, d.papierH, d.dpiX, d.dpiY = 210, 297, 300, 300
		d.druckbar = druck.Rechteck{X: 5, Y: 5, W: 200, H: 287}
		return
	}
	defer win.DeleteDC(ic)
	d.dpiX = int(win.GetDeviceCaps(ic, win.LOGPIXELSX))
	d.dpiY = int(win.GetDeviceCaps(ic, win.LOGPIXELSY))
	mmX := func(px int32) float64 { return float64(px) * 25.4 / float64(d.dpiX) }
	mmY := func(px int32) float64 { return float64(px) * 25.4 / float64(d.dpiY) }
	d.papierW = mmX(win.GetDeviceCaps(ic, 110)) // PHYSICALWIDTH
	d.papierH = mmY(win.GetDeviceCaps(ic, 111)) // PHYSICALHEIGHT
	d.druckbar = druck.Rechteck{
		X: mmX(win.GetDeviceCaps(ic, 112)), Y: mmY(win.GetDeviceCaps(ic, 113)), // PHYSICALOFFSETX/Y
		W: mmX(win.GetDeviceCaps(ic, win.HORZRES)), H: mmY(win.GetDeviceCaps(ic, win.VERTRES)),
	}
}

// drucke gibt das Feldbild an seiner Stelle auf dem Blatt aus.
func (d *drucker) drucke(img *image.RGBA, feld druck.Rechteck, kopien int, titel string) error {
	hdc := win.CreateDC(nil, utf16(d.name), nil, d.devmode())
	if hdc == 0 {
		return errors.New("Drucker konnte nicht angesprochen werden")
	}
	defer win.DeleteDC(hdc)
	dpiX := float64(win.GetDeviceCaps(hdc, win.LOGPIXELSX))
	dpiY := float64(win.GetDeviceCaps(hdc, win.LOGPIXELSY))
	offX := float64(win.GetDeviceCaps(hdc, 112))
	offY := float64(win.GetDeviceCaps(hdc, 113))
	x := int32(feld.X*dpiX/25.4 - offX + 0.5)
	y := int32(feld.Y*dpiY/25.4 - offY + 0.5)
	w := int32(feld.W*dpiX/25.4 + 0.5)
	h := int32(feld.H*dpiY/25.4 + 0.5)

	// Bild als 32-Bit-DIB (BGRA, von oben nach unten)
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	bits := make([]byte, iw*ih*4)
	for yy := 0; yy < ih; yy++ {
		q := img.Pix[yy*img.Stride:]
		z := bits[yy*iw*4:]
		for xx := 0; xx < iw; xx++ {
			z[4*xx], z[4*xx+1], z[4*xx+2], z[4*xx+3] = q[4*xx+2], q[4*xx+1], q[4*xx], 0
		}
	}
	var bmi win.BITMAPINFO
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(iw)
	bmi.BmiHeader.BiHeight = -int32(ih)
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = win.BI_RGB

	di := win.DOCINFO{LpszDocName: utf16(titel)}
	di.CbSize = int32(unsafe.Sizeof(di))
	if win.StartDoc(hdc, &di) <= 0 {
		return errors.New("Druckauftrag konnte nicht gestartet werden")
	}
	for i := 0; i < max(1, kopien); i++ {
		if win.StartPage(hdc) <= 0 {
			win.AbortDoc(hdc)
			return errors.New("Seite konnte nicht begonnen werden")
		}
		win.SetStretchBltMode(hdc, 4) // HALFTONE
		procSetBrushOrgEx.Call(uintptr(hdc), 0, 0, 0)
		r, _, _ := procStretchDIBits.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			0, 0, uintptr(iw), uintptr(ih), uintptr(unsafe.Pointer(&bits[0])), uintptr(unsafe.Pointer(&bmi)), 0, 0x00CC0020)
		if int32(r) <= 0 {
			win.AbortDoc(hdc)
			return errors.New("Bild konnte nicht an den Drucker übergeben werden")
		}
		win.EndPage(hdc)
	}
	win.EndDoc(hdc)
	return nil
}
