package ki

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	crypt32       = syscall.NewLazyDLL("crypt32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree = kernel32.NewProc("LocalFree")
)

type datenBlob struct {
	n uint32
	p *byte
}

func blob(b []byte) *datenBlob {
	if len(b) == 0 {
		return &datenBlob{}
	}
	return &datenBlob{n: uint32(len(b)), p: &b[0]}
}

func (d *datenBlob) bytes() []byte {
	out := make([]byte, d.n)
	copy(out, unsafe.Slice(d.p, d.n))
	return out
}

const cryptUIForbidden = 0x1

// verschluesseln: Windows-DPAPI, nur für den angemeldeten Benutzer lesbar.
func verschluesseln(klar []byte) ([]byte, error) {
	var aus datenBlob
	r, _, err := procProtect.Call(uintptr(unsafe.Pointer(blob(klar))), 0, 0, 0, 0, cryptUIForbidden, uintptr(unsafe.Pointer(&aus)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(aus.p)))
	return aus.bytes(), nil
}

func entschluesseln(v []byte) ([]byte, error) {
	if len(v) == 0 {
		return nil, errors.New("leer")
	}
	var aus datenBlob
	r, _, err := procUnprotect.Call(uintptr(unsafe.Pointer(blob(v))), 0, 0, 0, 0, cryptUIForbidden, uintptr(unsafe.Pointer(&aus)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(aus.p)))
	return aus.bytes(), nil
}
