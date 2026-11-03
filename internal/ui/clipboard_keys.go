package ui

import (
	"gioui.org/io/key"

	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// warpClipKey is what plain Ctrl+C and Ctrl+V do on Windows, the way Warp and
// Windows Terminal do it: Ctrl+V pastes, and Ctrl+C copies when text is
// selected. With no selection, Ctrl+C stays the interrupt. Ctrl+Shift+C and
// Ctrl+Shift+V remain the keymap's copy and paste. macOS uses Cmd+C / Cmd+V,
// so its Ctrl keys always reach the shell.
func warpClipKey(goos string, name key.Name, mods key.Modifiers, hasSel bool) action {
	if goos != "windows" || mods != key.ModCtrl {
		return actNone
	}
	switch {
	case name == "V":
		return actPaste
	case name == "C" && hasSel:
		return actCopy
	}
	return actNone
}

// hasSel reports whether id has selected text.
func (win *window) hasSel(id tabs.TabID) bool {
	ts := win.sels[id]
	return ts != nil && ts.sel.Active
}
