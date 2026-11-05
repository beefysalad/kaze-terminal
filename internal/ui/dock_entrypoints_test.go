package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

// Every member entry point docks a tab (issue #133, plan task D5). These
// replace the floating pane's member tests.

// memberShownIn reports whether session's panel shows member m.
func memberShownIn(win *window, session, m tabs.TabID) bool {
	d := win.deckOf(session)
	return d != nil && !d.hidden && d.shown() != nil && d.shown().member == m
}

func TestMemberClickDocksATab(t *testing.T) {
	win, _, r, a, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	if win.mgr.Active().ID != a.ID || !memberShownIn(win, a.ID, m1.ID) || win.dockRect().Empty() || !win.dockMemberKeys() {
		t.Fatalf("active %d deck %+v keys %v", win.mgr.Active().ID, win.deckOf(a.ID), win.dock.keys)
	}
}

func TestMemberClickAgainHidesPanel(t *testing.T) {
	win, _, r, a, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	clickMember(t, win, r, m1.ID)
	d := win.deckOf(a.ID)
	if d == nil || !d.hidden || len(d.entries) != 1 || !win.dockRect().Empty() {
		t.Fatalf("second click did not hide: %+v", d)
	}
	if win.dock.keys {
		t.Fatal("a hidden member kept the keyboard")
	}
}

func TestMemberClickWhenItIsTheMainViewDoesNothing(t *testing.T) {
	win, _, r, _, _, m1, _ := memberWindow(t)
	win.selectTab(m1.ID)
	sideFrame(t, win, r)
	clickMember(t, win, r, m1.ID)
	if win.mgr.Active().ID != m1.ID || win.memberDeck(m1.ID) != 0 {
		t.Fatalf("active %d deck %d", win.mgr.Active().ID, win.memberDeck(m1.ID))
	}
}

func TestMemberClickWhileAMemberIsShownShowsIt(t *testing.T) {
	win, _, r, _, _, m1, m2 := memberWindow(t)
	win.selectTab(m1.ID)
	sideFrame(t, win, r)
	clickMember(t, win, r, m2.ID)
	if win.mgr.Active().ID != m2.ID || win.memberDeck(m2.ID) != 0 {
		t.Fatalf("active %d deck %d", win.mgr.Active().ID, win.memberDeck(m2.ID))
	}
}

func TestExpandMakesTheMemberTheMainView(t *testing.T) {
	win, st, r, _, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	if win.dock.hits.expand.Empty() {
		t.Fatal("no Expand button on a member tab")
	}
	clickAt(t, win, r, centre(win.dock.hits.expand))
	cols, rows := win.mgr.Size()
	if win.mgr.Active().ID != m1.ID || win.memberDeck(m1.ID) != 0 || ptyOf(st, 2).lastSize() != [2]int{cols, rows} || win.side.st.shown != m1.ID {
		t.Fatalf("active %d deck %d pty %v shown %d", win.mgr.Active().ID, win.memberDeck(m1.ID), ptyOf(st, 2).lastSize(), win.side.st.shown)
	}
}

func TestPickingASessionAfterExpandGoesBack(t *testing.T) {
	win, _, r, a, _, m1, m2 := memberWindow(t)
	clickMember(t, win, r, m2.ID)
	clickMember(t, win, r, m1.ID)
	clickAt(t, win, r, centre(win.dock.hits.expand))
	sideClick(win, r, rowHit(t, win, a.ID).rect, pointer.ButtonPrimary)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != a.ID || !memberShownIn(win, a.ID, m2.ID) || win.dockRect().Empty() {
		t.Fatalf("active %d, deck %+v", win.mgr.Active().ID, win.deckOf(a.ID))
	}
}

func TestCloseTabKeepsMemberRunning(t *testing.T) {
	win, st, r, _, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	clickAt(t, win, r, centre(win.dock.hits.tabs[0].close))
	if win.memberDeck(m1.ID) != 0 || !win.mgr.Has(m1.ID) || win.memberOf(m1) == nil {
		t.Fatalf("deck %d, open %v", win.memberDeck(m1.ID), win.mgr.Has(m1.ID))
	}
	time.Sleep(20 * time.Millisecond)
	if got := ptyOf(st, 2).sent(); got != "" {
		t.Fatalf("closing the tab wrote %q", got)
	}
}

func TestOpeningAMemberInAnotherSessionMovesIt(t *testing.T) {
	win, _, r, a, b, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	win.selectTab(b.ID)
	sideFrame(t, win, r)
	clickMember(t, win, r, m1.ID)
	if win.memberDeck(m1.ID) != b.ID {
		t.Fatalf("deck %d", win.memberDeck(m1.ID))
	}
	win.selectTab(a.ID)
	twoFrames(t, win, r)
	if win.deckOf(a.ID) != nil || !win.dockRect().Empty() {
		t.Fatalf("a still shows the member: %+v", win.deckOf(a.ID))
	}
}

func TestSeventhTabReplacesOldest(t *testing.T) {
	win, st, r, a, _, m1, _ := memberWindow(t)
	ids := []tabs.TabID{m1.ID}
	for _, name := range []string{"x", "y", "z", "w", "v", "u"} {
		ids = append(ids, openMember(t, win, name).ID)
	}
	win.selectTab(a.ID)
	sideFrame(t, win, r)
	for _, id := range ids {
		clickMember(t, win, r, id)
	}
	cols, rows := win.mgr.Size()
	if n := len(win.deckOf(a.ID).entries); n != dockMaxTabs || win.memberDeck(m1.ID) != 0 || ptyOf(st, 2).lastSize() != [2]int{cols, rows} {
		t.Fatalf("%d tabs, m1 deck %d pty %v", n, win.memberDeck(m1.ID), ptyOf(st, 2).lastSize())
	}
}

func TestServerDotsDockTheDownMember(t *testing.T) {
	win, _, r, a, _, _, m2 := memberWindow(t)
	win.runs[1].members[1].run.State = workspaces.Failed
	sideFrame(t, win, r)
	if win.statusRect(grpServers).Empty() {
		t.Fatal("no server dots")
	}
	clickAt(t, win, r, centre(win.statusRect(grpServers)))
	if !memberShownIn(win, a.ID, m2.ID) || win.mgr.Active().ID != a.ID {
		t.Fatalf("deck %+v active %d", win.deckOf(a.ID), win.mgr.Active().ID)
	}
	clickAt(t, win, r, centre(win.statusRect(grpServers)))
	if !memberShownIn(win, a.ID, m2.ID) {
		t.Fatal("a second click on the dots hid the member")
	}
}

func TestDockedMemberIsHighlighted(t *testing.T) {
	win, _, r, _, _, m1, m2 := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	if win.side.st.docked != m1.ID {
		t.Fatalf("docked %d", win.side.st.docked)
	}
	clickMember(t, win, r, m2.ID)
	if win.side.st.docked != m2.ID {
		t.Fatalf("docked %d after m2", win.side.st.docked)
	}
}

// A member docked from the sidebar takes the keyboard: typing goes to it at once.
func TestTypingAfterDockingAMemberReachesIt(t *testing.T) {
	win, st, r, _, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	typeKeys(t, win, r, "q")
	waitSent(t, ptyOf(st, 2), "q")
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("the session got %q", got)
	}
}

func TestDockingAMemberMovesTheFocusReports(t *testing.T) {
	win, st, r, a, _, m1, _ := memberWindow(t)
	emitTo(t, st, 0, a, "\x1b[?1004h", func() bool { return modes(a.Session).FocusEvents })
	emitTo(t, st, 2, m1, "\x1b[?1004h", func() bool { return modes(m1.Session).FocusEvents })
	clickMember(t, win, r, m1.ID)
	waitSent(t, ptyOf(st, 0), "\x1b[O")
	waitSent(t, ptyOf(st, 2), "\x1b[I")
}

func TestSidebarCacheDockedMemberAllocatesNothing(t *testing.T) {
	win, _, r, _, _, m1, _ := memberWindow(t)
	clickMember(t, win, r, m1.ID)
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 10 {
		draw()
	}
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("sidebar with a docked member allocates %g times", n)
	}
}
