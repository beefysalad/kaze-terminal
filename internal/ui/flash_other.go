//go:build !windows

package ui

import "gioui.org/io/event"

// The macOS Dock bounce comes with the Mac milestone.
func viewHandle(event.Event) (uintptr, bool) { return 0, false }
func flashTaskbar(uintptr)                   {}
