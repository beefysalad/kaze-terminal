package ui

import (
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/key"
)

// A shortcut saved on the Shortcuts page works from the find bar at once, not
// only after a restart (review finding 1).
func TestFindBarTakesSavedKeys(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	win.applyKeys(map[string]string{"new_tab": "ctrl+alt+t"})
	sideFrame(t, win, r) // the save's frame, before the person presses anything
	before := len(win.mgr.Tabs())
	pressKey(t, win, r, "T", key.ModCtrl|key.ModAlt)
	if got := len(win.mgr.Tabs()); got != before+1 {
		t.Fatalf("%d sessions after the saved Ctrl+Alt+T in the find bar, want %d", got, before+1)
	}
}

// The same from the sidebar's filter field.
func TestSidebarFilterTakesSavedKeys(t *testing.T) {
	win, _, r, _ := filterWindow(t)
	focusFilter(t, win, r)
	win.applyKeys(map[string]string{"new_tab": "ctrl+alt+t"})
	sideFrame(t, win, r) // the save's frame, before the person presses anything
	before := len(win.mgr.Tabs())
	pressKey(t, win, r, "T", key.ModCtrl|key.ModAlt)
	if got := len(win.mgr.Tabs()); got != before+1 {
		t.Fatalf("%d sessions after the saved Ctrl+Alt+T in the sidebar filter, want %d", got, before+1)
	}
}

// applyKeys drops every editor's key filters built from the old keymap: the
// sidebar filter and rename, the branch launcher, find and the board.
func TestApplyKeysDropsEditorKeyCaches(t *testing.T) {
	win, _, _, _, _, _, _ := memberWindow(t)
	stale := []event.Filter{key.Filter{Name: "X"}}
	win.side.filters, win.side.renKeys, win.side.launch.filters = stale, stale, stale
	win.find.edKeys, win.board.edKeys = stale, stale
	win.applyKeys(map[string]string{"new_tab": "ctrl+alt+t"})
	for name, fs := range map[string][]event.Filter{
		"sidebar filter": win.side.filters, "sidebar rename": win.side.renKeys,
		"branch launcher": win.side.launch.filters, "find": win.find.edKeys, "board": win.board.edKeys,
	} {
		if fs != nil {
			t.Errorf("%s keeps its old key filters after a save", name)
		}
	}
}
