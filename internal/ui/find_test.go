package ui

import (
	"fmt"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"

	kinput "github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

func TestFindDefaultShortcut(t *testing.T) {
	for _, c := range []struct {
		goos string
		sc   kinput.Shortcut
	}{
		{"windows", kinput.Shortcut{Name: "F", Mods: key.ModCtrl | key.ModShift}},
		{"darwin", kinput.Shortcut{Name: "F", Mods: key.ModCommand}},
	} {
		km, warns := buildKeymap(c.goos, nil)
		if len(warns) != 0 || km[c.sc] != actFind {
			t.Errorf("%s: %+v maps to %v (warns %v), want actFind", c.goos, c.sc, km[c.sc], warns)
		}
	}
}

func TestPlainCtrlFGoesToShell(t *testing.T) {
	km, _ := buildKeymap("windows", nil)
	out := routeKey("windows", km, "F", key.ModCtrl, term.Modes{})
	if out.act != actNone || string(out.data) != "\x06" {
		t.Fatalf("Ctrl+F: act %v data %q, want the shell's ^F", out.act, out.data)
	}
}

func TestFindShortcutOverride(t *testing.T) {
	cs := key.ModCtrl | key.ModShift
	km, warns := buildKeymap("windows", map[string]string{"find": "ctrl+shift+g"})
	if len(warns) != 0 || km[kinput.Shortcut{Name: "G", Mods: cs}] != actFind {
		t.Fatalf("override: warns %v, Ctrl+Shift+G = %v", warns, km[kinput.Shortcut{Name: "G", Mods: cs}])
	}
	if a, ok := km[kinput.Shortcut{Name: "F", Mods: cs}]; ok {
		t.Fatalf("Ctrl+Shift+F still bound to %v after the override", a)
	}
	km, warns = buildKeymap("windows", map[string]string{"find": "ctrl+shift+t"})
	if len(warns) != 1 || !strings.Contains(warns[0], "new_tab") {
		t.Fatalf("clash: warns %v, want one naming new_tab", warns)
	}
	if km[kinput.Shortcut{Name: "F", Mods: cs}] != actFind || km[kinput.Shortcut{Name: "T", Mods: cs}] != actNewTab {
		t.Fatal("a clashing override did not keep both defaults")
	}
}

func TestFindListedInShortcuts(t *testing.T) {
	win, _ := alertWindow(t, 1)
	text := commandsPage(t, win)
	if !strings.Contains(text, "Find in the terminal") || !hasKeys(win, "Ctrl", "Shift", "F") {
		t.Fatalf("Shortcuts page lacks find:\n%s", text)
	}
	for _, g := range shortcutGroups {
		if g.title == "Editing and mouse" && strings.Join(g.configs, ",") != "copy,paste,find" {
			t.Fatalf("Editing and mouse lists %v", g.configs)
		}
	}
}

// findKey presses the find shortcut (Ctrl+Shift+F, the test keymap's).
func findKey(t *testing.T, win *window, r *input.Router) {
	t.Helper()
	pressKey(t, win, r, "F", key.ModCtrl|key.ModShift)
}

// fillHistory prints n numbered lines to pty i and waits until tab holds at
// least n-rows of them in history.
func fillHistory(t *testing.T, st *recStarter, i int, tab *tabs.Tab, n int) {
	t.Helper()
	var b strings.Builder
	for k := range n {
		fmt.Fprintf(&b, "line %d\r\n", k)
	}
	want := scrollbackLen(tab) + n - 40
	emitTo(t, st, i, tab, b.String(), func() bool { return scrollbackLen(tab) >= want })
}

func scrollbackLen(tab *tabs.Tab) int {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	return tab.Session.Screen().ScrollbackLen()
}

func setOffset(tab *tabs.Tab, n int) {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	tab.Session.Screen().SetViewOffset(n)
}

func TestFindOpensOnTheSession(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	f := &win.find
	if !f.open || !f.keys || f.member || f.tab != a.ID {
		t.Fatalf("open %v keys %v member %v tab %d, want open on %d", f.open, f.keys, f.member, f.tab, a.ID)
	}
	if win.focusTarget() != &f.ed || !r.Source().Focused(&f.ed) {
		t.Fatalf("focus target %v, want the find field", win.focusTarget())
	}
}

func TestFindOpensOnTheMemberWithTheKeyboard(t *testing.T) {
	win, _, r, _, _, m1 := dockOne(t)
	findKey(t, win, r)
	f := &win.find
	if !f.open || !f.member || f.tab != m1.ID {
		t.Fatalf("open %v member %v tab %d, want open on member %d", f.open, f.member, f.tab, m1.ID)
	}
	if !win.dockMemberKeys() {
		t.Fatal("opening find took the member's place in the panel keyboard")
	}
}

func TestFindDoesNothingInSettings(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	win.openSettings()
	twoFrames(t, win, r)
	win.openFind()
	findKey(t, win, r)
	if win.find.open {
		t.Fatal("find opened over settings")
	}
}

func TestFindKeepsTextSelected(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	win.find.ed.SetText("warn")
	win.find.ed.SetCaret(2, 2)
	win.closeFind(true)
	twoFrames(t, win, r)
	findKey(t, win, r)
	ed := &win.find.ed
	if ed.Text() != "warn" || ed.SelectionLen() != 4 {
		t.Fatalf("reopened: text %q, %d selected, want warn all selected", ed.Text(), ed.SelectionLen())
	}
}

func TestFindClosesOnTabSwitch(t *testing.T) {
	win, st, r, a, b, _, _ := memberWindow(t)
	fillHistory(t, st, 0, a, 100)
	setOffset(a, 10)
	findKey(t, win, r)
	setOffset(a, 30) // as a reveal would
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	if win.find.open {
		t.Fatal("find stayed open after switching sessions")
	}
	if got := historyOffset(a); got != 30 {
		t.Fatalf("a's view offset %d after the switch, want 30 (no restore)", got)
	}
	if win.focusTarget() != win.termTag {
		t.Fatalf("focus %v after the switch, want the terminal", win.focusTarget())
	}
}

func TestFindClosesWhenMemberHidden(t *testing.T) {
	win, _, r, a, _, _ := dockOne(t)
	findKey(t, win, r)
	if !win.find.open || !win.find.member {
		t.Fatal("did not open on the member")
	}
	win.closeDock(a.ID)
	twoFrames(t, win, r)
	if win.find.open {
		t.Fatal("find stayed open after the panel was hidden")
	}
}

func TestFindMovesToTheOtherPane(t *testing.T) {
	win, _, r, a, _, m1, _ := memberWindow(t)
	win.openDockMember(a.ID, m1.ID)
	twoFrames(t, win, r)
	findKey(t, win, r)
	if !win.find.open || win.find.member {
		t.Fatal("did not open on the session")
	}
	clickAt(t, win, r, centre(win.dock.hits.body))
	findKey(t, win, r)
	if f := &win.find; !f.open || !f.member || f.tab != m1.ID || !f.keys {
		t.Fatalf("open %v member %v tab %d keys %v, want open on the member", f.open, f.member, f.tab, f.keys)
	}
}

func TestFindEscRestoresView(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	fillHistory(t, st, 0, a, 200)
	setOffset(a, 40)
	findKey(t, win, r)
	setOffset(a, 120) // as a reveal would
	n := scrollbackLen(a)
	emitTo(t, st, 0, a, "x1\r\nx2\r\nx3\r\nx4\r\nx5\r\n", func() bool { return scrollbackLen(a) >= n+5 })
	win.closeFind(true)
	sideFrame(t, win, r)
	if got := historyOffset(a); got != 45 {
		t.Fatalf("offset %d after closing, want 45 (40 plus the 5 lines that scrolled in)", got)
	}
	setOffset(a, 0)
	findKey(t, win, r)
	setOffset(a, 120)
	win.closeFind(true)
	sideFrame(t, win, r)
	if got := historyOffset(a); got != 0 {
		t.Fatalf("opened at the bottom: offset %d after closing, want 0", got)
	}
}
