//go:build windows

package client

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	setClipboardData = user32.NewProc("SetClipboardData")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	globalSize       = kernel32.NewProc("GlobalSize")
	moveMemory       = kernel32.NewProc("RtlMoveMemory")
)

const cfUnicodeText = 13

// clipRead returns the clipboard text (the callback gets "" on failure).
func clipRead(done func(string)) {
	if r, _, _ := openClipboard.Call(0); r == 0 {
		done("")
		return
	}
	defer closeClipboard.Call()
	h, _, _ := getClipboardData.Call(cfUnicodeText)
	if h == 0 {
		done("")
		return
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		done("")
		return
	}
	defer globalUnlock.Call(h)
	n, _, _ := globalSize.Call(h)
	if n < 2 || n > 64<<20 {
		done("")
		return
	}
	u := make([]uint16, n/2)
	moveMemory.Call(uintptr(unsafe.Pointer(&u[0])), p, n)
	done(syscall.UTF16ToString(u))
}

func clipWrite(s string) bool {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return false
	}
	if r, _, _ := openClipboard.Call(0); r == 0 {
		return false
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	h, _, _ := globalAlloc.Call(0x0002, uintptr(len(u)*2)) // GMEM_MOVEABLE
	if h == 0 {
		return false
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		return false
	}
	moveMemory.Call(p, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2))
	globalUnlock.Call(h)
	r, _, _ := setClipboardData.Call(cfUnicodeText, h)
	return r != 0
}
