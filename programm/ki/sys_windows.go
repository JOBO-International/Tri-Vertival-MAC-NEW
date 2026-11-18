package ki

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	setDllDirectoryW = kernel32.NewProc("SetDllDirectoryW")
)

// dllOrdner: abhängige DLLs (Visual-C++-Laufzeit) im KI-Ordner finden.
func dllOrdner(d string) {
	p, err := syscall.UTF16PtrFromString(d)
	if err == nil {
		setDllDirectoryW.Call(uintptr(unsafe.Pointer(p)))
	}
}
