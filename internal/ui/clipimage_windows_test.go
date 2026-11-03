package ui

import "testing"

// A screenshot (Win+Shift+S, Print Screen) puts a bitmap on the clipboard.
// The check reads only the format list, never the clipboard's data.
func TestClipFormatsHoldAnImage(t *testing.T) {
	for _, c := range []struct {
		name    string
		formats []uint32
		want    bool
	}{
		{"empty", nil, false},
		{"text only", []uint32{cfUnicodeText}, false},
		{"files", []uint32{cfHDrop}, false},
		{"bitmap", []uint32{cfBitmap}, true},
		{"dib", []uint32{cfDIB}, true},
		{"dibv5", []uint32{cfDIBV5}, true},
		{"text and bitmap", []uint32{cfUnicodeText, cfBitmap}, true},
	} {
		has := func(f uint32) bool {
			for _, g := range c.formats {
				if g == f {
					return true
				}
			}
			return false
		}
		if got := clipFormatsHoldImage(has); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
