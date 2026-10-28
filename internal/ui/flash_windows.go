//go:build windows

package ui

import (
	"syscall"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
)

var procFlashWindowEx = syscall.NewLazyDLL("user32.dll").NewProc("FlashWindowEx")

// flashInfo is FLASHWINFO.
type flashInfo struct {
	size    uint32
	hwnd    uintptr
	flags   uint32
	count   uint32
	timeout uint32
}

const (
	flashTray      = 0x2 // FLASHW_TRAY
	flashUntilFore = 0xc // FLASHW_TIMERNOFG: until the window comes to the front
)

// viewHandle picks the HWND out of Gio's view event.
func viewHandle(e event.Event) (uintptr, bool) {
	v, ok := e.(app.Win32ViewEvent)
	return v.HWND, ok
}

// flashTaskbar flashes the taskbar button; it works while minimized and
// stops by itself when the owner brings kaze to the front.
func flashTaskbar(hwnd uintptr) {
	if hwnd == 0 || procFlashWindowEx.Find() != nil {
		return
	}
	fi := flashInfo{hwnd: hwnd, flags: flashTray | flashUntilFore}
	fi.size = uint32(unsafe.Sizeof(fi))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
}
