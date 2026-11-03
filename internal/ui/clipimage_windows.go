//go:build windows

package ui

import "golang.org/x/sys/windows"

// Standard clipboard formats (winuser.h).
const (
	cfBitmap      = 2
	cfDIB         = 8
	cfUnicodeText = 13
	cfHDrop       = 15
	cfDIBV5       = 17
)

var procIsClipboardFormatAvailable = windows.NewLazySystemDLL("user32.dll").NewProc("IsClipboardFormatAvailable")

// systemClipImage reports whether the clipboard holds an image, such as a
// screenshot from Win+Shift+S. It reads only the format list, without opening
// the clipboard.
func systemClipImage() bool {
	if procIsClipboardFormatAvailable.Find() != nil {
		return false
	}
	return clipFormatsHoldImage(func(f uint32) bool {
		r, _, _ := procIsClipboardFormatAvailable.Call(uintptr(f))
		return r != 0
	})
}

// clipFormatsHoldImage reports whether has finds a bitmap format. Windows
// offers the other bitmap formats for any one of them, so these three cover
// screenshots and images copied from apps.
func clipFormatsHoldImage(has func(uint32) bool) bool {
	return has(cfBitmap) || has(cfDIB) || has(cfDIBV5)
}
