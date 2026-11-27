package ui

import "testing"

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
