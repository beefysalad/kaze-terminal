package ui

import (
	"testing"

	"gioui.org/io/key"

	"github.com/patrick-gogo/kaze-terminal/internal/term"
)

// The scratch shell is removed (issue #133, plan task D6).

func TestNoScratchAction(t *testing.T) {
	if _, ok := actionByName["scratch_shell"]; ok {
		t.Fatal("scratch_shell is still an action")
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for name, sc := range defaultBindings(goos) {
			if sc.Name == "`" {
				t.Fatalf("%s: %s is bound to backtick", goos, name)
			}
		}
	}
	// Ctrl+Shift+` (Ctrl+` on macOS) is no kaze action any more: it is the shell's.
	for goos, mods := range map[string]key.Modifiers{"windows": key.ModCtrl | key.ModShift, "darwin": key.ModCtrl} {
		km, _ := buildKeymap(goos, nil)
		if out := routeKey(goos, km, "`", mods, term.Modes{}); out.act != actNone {
			t.Fatalf("%s: the old scratch key runs %v", goos, out.act)
		}
	}
	win, _, r, _, _, _, _ := paneWindow(t)
	n := len(win.mgr.Tabs())
	r.Queue(key.Event{Name: "`", Modifiers: key.ModCtrl | key.ModShift, State: key.Press})
	floatFrames(t, win, r)
	if len(win.mgr.Tabs()) != n {
		t.Fatal("the old scratch key opened a tab")
	}
}

// Closing a session docks its members back on the shared size; they keep running.
func TestClosingSessionDocksItsMembersBack(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	win.closeTab(a.ID)
	floatFrames(t, win, r)
	cols, rows := win.mgr.Size()
	if got := ptyOf(st, 2).lastSize(); got != [2]int{cols, rows} {
		t.Fatalf("member pty %v, want the shared %v", got, [2]int{cols, rows})
	}
	if !win.mgr.Has(m1.ID) || win.memberDeck(m1.ID) != 0 || win.dock.drawn != 0 {
		t.Fatalf("open %v deck %d drawn %d", win.mgr.Has(m1.ID), win.memberDeck(m1.ID), win.dock.drawn)
	}
}
