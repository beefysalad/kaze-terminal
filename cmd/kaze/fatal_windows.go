package main

import (
	"syscall"
	"unsafe"
)

var procMessageBoxW = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

const (
	mbOK            = 0x0
	mbIconError     = 0x10
	mbSetForeground = 0x10000
)

// showErrorNative shows a modal message box; it is the only output a GUI build has.
func showErrorNative(title, text string) {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbOK|mbIconError|mbSetForeground)
}
