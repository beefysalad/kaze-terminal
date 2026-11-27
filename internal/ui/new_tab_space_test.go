package ui

import (
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Ctrl+T and the shortcut open the new tab in the space being looked at, not in
// the default start folder.
func TestNewTabShortcutOpensInCurrentSpace(t *testing.T) {
	win, st := sideWindow(t, true)
	dir := osPath("/code/repo/api")
	openIn(t, win, osPath("/code/other/web"))
	openIn(t, win, dir)
	before := len(st.where)
	win.newTab()
	if len(st.where) != before+1 || st.where[before] != dir {
		t.Fatalf("new tab started in %q (before %d), want %q", st.where, before, dir)
	}
}

func TestNewTabInShellSpaceUsesStartFolder(t *testing.T) {
	win, st := sideWindow(t, true)
	openIn(t, win, "")
	want := win.plainStartDir()
	before := len(st.where)
	win.newTab()
	if st.where[before] != want {
		t.Fatalf("new tab started in %q, want %q", st.where[before], want)
	}
}

// A new tab knows its folder at once, so it joins the space immediately instead
// of sitting in the shell space until the shell's first folder report.
func TestNewTabJoinsSpaceBeforeShellReports(t *testing.T) {
	win, _ := sideWindow(t, true)
	dir := osPath("/code/repo/api")
	a := openIn(t, win, dir)
	win.newTab()
	var fresh *tabs.Tab
	for _, tb := range win.mgr.Tabs() {
		if tb.ID != a.ID {
			fresh = tb
		}
	}
	if got := win.locOf(fresh).cwd; got != dir {
		t.Fatalf("new tab cwd %q, want %q", got, dir)
	}
	if win.spaceKeyOf(fresh.ID) != win.spaceKeyOf(a.ID) {
		t.Fatalf("new tab is in space %q, want %q", win.spaceKeyOf(fresh.ID), win.spaceKeyOf(a.ID))
	}
}
