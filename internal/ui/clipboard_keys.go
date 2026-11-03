package ui

import (
	"strings"

	"gioui.org/io/key"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
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

// hasSel reports whether t has a selection with text worth copying. A
// selection over empty cells has none, so Ctrl+C interrupts instead.
func (win *window) hasSel(t *tabs.Tab) bool {
	ts := win.sels[t.ID]
	if ts == nil || !ts.sel.Active {
		return false
	}
	t.Session.Lock()
	text := render.SelectedText(t.Session.Screen(), ts.sel)
	t.Session.Unlock()
	return strings.TrimSpace(text) != ""
}

// dropSelOnInput clears t's selection when input goes to the shell, as
// Windows Terminal does, so a stale selection never turns the next Ctrl+C
// into a copy.
func (win *window) dropSelOnInput(t *tabs.Tab) {
	if ts := win.sels[t.ID]; ts != nil && ts.sel.Active {
		ts.clear()
		win.w.Invalidate()
	}
}
