package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

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

// typeFind types s at the end of the find field.
func typeFind(t *testing.T, win *window, r *input.Router, s string) {
	t.Helper()
	n := win.find.ed.Len()
	r.Queue(key.EditEvent{Range: key.Range{Start: n, End: n}, Text: s})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

func TestFindTypingNeverReachesShell(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	typeFind(t, win, r, "warn")
	pressKey(t, win, r, key.NameReturn, 0)
	pressKey(t, win, r, key.NameDownArrow, 0)
	pressKey(t, win, r, key.NameReturn, key.ModShift)
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("the shell got %q while the bar had the keyboard", got)
	}
	if win.find.text != "warn" || !win.find.open {
		t.Fatalf("text %q open %v, want warn in an open bar", win.find.text, win.find.open)
	}
}

func TestFindOpeningAndTypingSameFrame(t *testing.T) {
	for _, member := range []bool{false, true} {
		var win *window
		var st *recStarter
		var r *input.Router
		pty := 0
		if member {
			win, st, r, _, _, _ = dockOne(t)
			pty = 2
		} else {
			win, st, r, _, _, _, _ = memberWindow(t)
		}
		before := ptyOf(st, pty).sent()
		r.Queue(key.Event{Name: "F", Modifiers: key.ModCtrl | key.ModShift, State: key.Press},
			key.Event{Name: "F", Modifiers: key.ModCtrl | key.ModShift, State: key.Release},
			key.EditEvent{Range: key.Range{}, Text: "warn"})
		sideFrame(t, win, r)
		sideFrame(t, win, r)
		if got := ptyOf(st, pty).sent(); got != before {
			t.Fatalf("member %v: typing after the shortcut reached the shell: %q", member, strings.TrimPrefix(got, before))
		}
		if got := win.find.ed.Text(); got != "warn" {
			t.Fatalf("member %v: field %q, want warn", member, got)
		}
	}
}

func TestFindEnterAndArrowsStep(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn", "warn", "warn")
	findFor(t, win, r, "warn")
	var got []int
	for _, k := range []struct {
		name key.Name
		mods key.Modifiers
	}{{key.NameReturn, 0}, {key.NameReturn, key.ModShift}, {key.NameEnter, 0}, {key.NameDownArrow, 0}, {key.NameUpArrow, 0}} {
		pressKey(t, win, r, k.name, k.mods)
		cur, _, _, _ := win.findCount()
		got = append(got, cur)
	}
	if fmt.Sprint(got) != "[2 1 2 3 2]" {
		t.Fatalf("current after Enter, Shift+Enter, Enter, Down, Up: %v, want [2 1 2 3 2]", got)
	}
}

func TestFindEscClosesAndGivesKeysBack(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	pressKey(t, win, r, key.NameEscape, 0)
	if win.find.open || win.focusTarget() != win.termTag || !r.Source().Focused(win.termTag) {
		t.Fatalf("after Esc: open %v, focus %v", win.find.open, win.focusTarget())
	}
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("Esc in the bar reached the shell: %q", got)
	}
	typedReaches(t, win, r, ptyOf(st, 0), "ls")
}

func TestFindEscOnMemberGivesItKeysBack(t *testing.T) {
	win, st, r, _, _, _ := dockOne(t)
	findKey(t, win, r)
	pressKey(t, win, r, key.NameEscape, 0)
	if win.find.open || !win.dockMemberKeys() || !r.Source().Focused(&win.dock.term) {
		t.Fatalf("after Esc: open %v, member keys %v", win.find.open, win.dockMemberKeys())
	}
	before := ptyOf(st, 2).sent()
	typeKeys(t, win, r, "pwd")
	waitSent(t, ptyOf(st, 2), before+"pwd")
}

func TestFindShortcutsStillWork(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	before := len(win.mgr.Tabs())
	pressKey(t, win, r, "T", key.ModCtrl|key.ModShift)
	if got := len(win.mgr.Tabs()); got != before+1 {
		t.Fatalf("%d sessions after Ctrl+Shift+T in the bar, want %d", got, before+1)
	}
}

func TestClickTerminalTakesKeysBack(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	clickAt(t, win, r, image.Pt(win.gridRect.Min.X+40, win.gridRect.Max.Y-40))
	if win.find.keys || !win.find.open || win.focusTarget() != win.termTag {
		t.Fatalf("keys %v open %v focus %v", win.find.keys, win.find.open, win.focusTarget())
	}
	typedReaches(t, win, r, ptyOf(st, 0), "ls")
}

func TestClickFieldTakesKeysAgain(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	clickAt(t, win, r, image.Pt(win.gridRect.Min.X+40, win.gridRect.Max.Y-40))
	clickAt(t, win, r, centre(win.find.hits.field))
	if !win.find.keys || win.focusTarget() != &win.find.ed || !r.Source().Focused(&win.find.ed) {
		t.Fatalf("keys %v focus %v", win.find.keys, win.focusTarget())
	}
	typeFind(t, win, r, "x")
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("the shell got %q", got)
	}
}

func TestFindCopyPrefersFieldSelection(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	var clip []string
	win.toClipboard = func(s string) { clip = append(clip, s) }
	emitTo(t, st, 0, a, "hello", func() bool { return textShown(a, 4, 'o')() })
	sideFrame(t, win, r)
	cw, ch := win.rend.CellSize()
	from := win.gridRect.Min.Add(image.Pt(cw/4, ch/2))
	dragFrom(t, win, r, from, image.Pt(4*cw+cw/2, 0))
	findKey(t, win, r)
	typeFind(t, win, r, "warn")
	pressKey(t, win, r, "C", key.ModCtrl|key.ModShift) // nothing selected in the field: the terminal's
	win.find.ed.SetCaret(win.find.ed.Len(), 0)
	pressKey(t, win, r, "C", key.ModCtrl|key.ModShift)
	win.find.ed.SetCaret(win.find.ed.Len(), win.find.ed.Len())
	pressKey(t, win, r, "C", key.ModCtrl|key.ModShift)
	if fmt.Sprint(clip) != "[hello warn hello]" {
		t.Fatalf("copied %q, want [hello warn hello]", clip)
	}
}

func TestEscInTerminalGoesToShell(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	clickAt(t, win, r, image.Pt(win.gridRect.Min.X+40, win.gridRect.Max.Y-40))
	pressKey(t, win, r, key.NameEscape, 0)
	waitSent(t, ptyOf(st, 0), "\x1b")
	if !win.find.open {
		t.Fatal("Esc in the terminal closed the bar")
	}
}

func TestFindKeysLeaveWhenSidebarMemberClicked(t *testing.T) {
	win, _, r, _, _, m1, _ := memberWindow(t)
	findKey(t, win, r)
	clickMember(t, win, r, m1.ID)
	if !win.dockMemberKeys() {
		t.Fatal("member click did not give the member the keyboard")
	}
	if win.find.keys || win.focusTarget() == &win.find.ed {
		t.Fatalf("find kept the keyboard: keys %v focus %v", win.find.keys, win.focusTarget())
	}
}

func TestFindKeysLeaveWhenPanelBodyClicked(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	if win.find.keys || win.focusTarget() == &win.find.ed {
		t.Fatalf("find kept the keyboard: keys %v focus %v", win.find.keys, win.focusTarget())
	}
}

func TestFindFromSidebarFilterTakesKeyboard(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	sideClick(win, r, win.side.hits.filter, pointer.ButtonPrimary)
	twoFrames(t, win, r)
	if !win.side.filtering {
		t.Fatal("filter did not open")
	}
	findKey(t, win, r)
	if win.side.filtering || win.focusTarget() != &win.find.ed {
		t.Fatalf("filtering %v focus %v, want the find field", win.side.filtering, win.focusTarget())
	}
}
