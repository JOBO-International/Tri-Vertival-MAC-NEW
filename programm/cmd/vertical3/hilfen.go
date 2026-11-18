package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

func syscallUTF16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// alleTastenFuer sorgt dafür, dass das Fenster die Tasten Esc und Enter selbst erhält. Sonst verarbeitet die Windows-Dialogverwaltung Esc als
// "Abbrechen" und das Bild bekommt die Taste nie zu sehen.
func alleTastenFuer(hwnd win.HWND) {
	var alt uintptr
	neu := syscall.NewCallback(func(h win.HWND, msg uint32, wp, lp uintptr) uintptr {
		if msg == win.WM_GETDLGCODE && lp != 0 {
			// nur Esc, Enter und Pfeil links/rechts (Strg+Pfeil = drehen) selbst haben wollen – Tab usw. bleiben bei Windows
			if m := (*win.MSG)(unsafe.Pointer(lp)); m.Message == win.WM_KEYDOWN && (m.WParam == win.VK_ESCAPE || m.WParam == win.VK_RETURN || m.WParam == win.VK_LEFT || m.WParam == win.VK_RIGHT) {
				return win.DLGC_WANTMESSAGE | win.CallWindowProc(alt, h, msg, wp, lp)
			}
		}
		return win.CallWindowProc(alt, h, msg, wp, lp)
	})
	alt = win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, neu)
}
