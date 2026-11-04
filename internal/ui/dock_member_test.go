package ui

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// Tests for workspace members as panel tabs (issue #133, plan tasks D3 to D5).

// memberGridWant is the PTY size a member gets in the panel drawn at panel,
// at scale 1: the panel minus its handle and tab row, less the terminal's pad.
func memberGridWant(win *window, panel image.Rectangle) [2]int {
	cw, ch := win.rend.CellSize()
	body := image.Rect(panel.Min.X, panel.Min.Y+dockHandleDp+dockTabsDp, panel.Max.X, panel.Max.Y)
	c, r := gridSize(body.Dx()-2*8, body.Dy()-8, cw, ch)
	return [2]int{c, r}
}

func TestDockMemberGetsPanelGrid(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	panel := win.dockRect()
	if panel.Empty() {
		t.Fatal("no panel for the member")
	}
	if got, want := ptyOf(st, 2).lastSize(), memberGridWant(win, panel); got != want {
		t.Fatalf("member pty %v, want the panel body's %v", got, want)
	}
	cw, ch := win.rend.CellSize()
	top := image.Rect(panel.Min.X, 42, panel.Max.X, panel.Min.Y)
	c, rows := gridSize(top.Dx()-16, top.Dy()-8, cw, ch)
	if got := ptyOf(st, 0).lastSize(); got != [2]int{c, rows} {
		t.Fatalf("session pty %v, want the area above %v", got, [2]int{c, rows})
	}
	if win.shownMember() != m1.ID || win.dock.drawn != m1.ID || win.dock.rend == nil {
		t.Fatalf("member not drawn: shown %d drawn %d", win.shownMember(), win.dock.drawn)
	}
}

func TestDockMemberDragResizesOnRelease(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	n := ptyOf(st, 2).resizeCount()
	p := centre(win.dock.hits.handle)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p)})
	sideFrame(t, win, r)
	for i := 1; i <= 3; i++ {
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p.Add(image.Pt(0, -20*i)))})
		sideFrame(t, win, r)
	}
	if got := ptyOf(st, 2).resizeCount(); got != n {
		t.Fatalf("%d member resizes during the drag", got-n)
	}
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at(p.Add(image.Pt(0, -60)))})
	floatFrames(t, win, r)
	if got := ptyOf(st, 2).resizeCount(); got != n+1 {
		t.Fatalf("%d member resizes after the release, want 1", got-n)
	}
	if got, want := ptyOf(st, 2).lastSize(), memberGridWant(win, win.dockRect()); got != want {
		t.Fatalf("member pty %v, want %v", got, want)
	}
}

func TestDockMemberCloseDocksSize(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	win.closeDockTab(a.ID, 0)
	floatFrames(t, win, r)
	cols, rows := win.mgr.Size()
	if got := ptyOf(st, 2).lastSize(); got != [2]int{cols, rows} {
		t.Fatalf("closed member pty %v, want the shared %v", got, [2]int{cols, rows})
	}
	if win.dock.drawn != 0 || win.dock.rend != nil {
		t.Fatalf("closed member still drawn: %d", win.dock.drawn)
	}
	if !win.mgr.Has(m1.ID) {
		t.Fatal("closing the tab closed the member")
	}
}

func TestDockHiddenMemberDrawsNothing(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	sized := ptyOf(st, 2).lastSize()
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
	if win.dock.rend != nil || win.dock.drawn != 0 {
		t.Fatalf("an agent tab shows but the member is drawn: %d", win.dock.drawn)
	}
	if got := ptyOf(st, 2).lastSize(); got != sized {
		t.Fatalf("a member behind another tab lost its panel size: %v, want %v", got, sized)
	}
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	if win.dock.drawn != m1.ID {
		t.Fatal("member not drawn again")
	}
	win.closeDock(a.ID)
	floatFrames(t, win, r)
	if win.dock.rend != nil || win.dock.drawn != 0 {
		t.Fatalf("a hidden panel still draws its member: %d", win.dock.drawn)
	}
	if got := ptyOf(st, 2).lastSize(); got != sized {
		t.Fatalf("hiding reflowed the member: %v, want %v", got, sized)
	}
}

func TestDockMemberMovesBetweenSessions(t *testing.T) {
	win, _, r, a, b, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	win.selectTab(b.ID)
	win.openDockMember(b.ID, m1.ID)
	floatFrames(t, win, r)
	if win.deckOf(a.ID) != nil {
		t.Fatalf("a still holds the member: %+v", win.deckOf(a.ID))
	}
	if win.memberDeck(m1.ID) != b.ID || win.shownMember() != m1.ID {
		t.Fatalf("member deck %d shown %d", win.memberDeck(m1.ID), win.shownMember())
	}
}

func TestDockExitedMemberKeepsTab(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	ptyOf(st, 2).Close()
	deadline := time.Now().Add(3 * time.Second)
	for done, _ := m1.Session.Exited(); !done; done, _ = m1.Session.Exited() {
		if time.Now().After(deadline) {
			t.Fatal("member shell never exited")
		}
		time.Sleep(time.Millisecond)
	}
	floatFrames(t, win, r)
	d := win.deckOf(a.ID)
	if d == nil || len(d.entries) != 1 || d.entries[0].member != m1.ID {
		t.Fatalf("exited member lost its tab: %+v", d)
	}
	if l := win.memberTabLabel(m1.ID); !l.down || l.name != "api" {
		t.Fatalf("label %+v, want api down", l)
	}
	if win.dock.drawn != m1.ID {
		t.Fatal("exited member's last output not drawn")
	}
}

// dockOne docks m1 in a and gives it the keyboard with a click on its body.
func dockOne(t *testing.T) (*window, *recStarter, *input.Router, *tabs.Tab, *tabs.Tab, *tabs.Tab) {
	t.Helper()
	win, st, r, a, b, m1, _ := paneWindow(t)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	return win, st, r, a, b, m1
}

func TestDockMemberClickTakesKeys(t *testing.T) {
	win, st, r, _, _, _ := dockOne(t)
	if win.focusTarget() != &win.dock.term || !r.Source().Focused(&win.dock.term) {
		t.Fatalf("focus target %v", win.focusTarget())
	}
	typeKeys(t, win, r, "ls\r")
	waitSent(t, ptyOf(st, 2), "ls\r")
	if got := ptyOf(st, 0).sent(); strings.Contains(got, "ls") {
		t.Fatalf("the session got %q", got)
	}
}

func TestDockTerminalClickTakesKeysBackFromMember(t *testing.T) {
	win, st, r, _, _, _ := dockOne(t)
	clickAt(t, win, r, image.Pt(win.gridRect.Min.X+40, win.gridRect.Min.Y+40))
	if win.dock.keys || win.focusTarget() != win.termTag {
		t.Fatalf("keys %v focus %v", win.dock.keys, win.focusTarget())
	}
	member := ptyOf(st, 2).sent()
	typeKeys(t, win, r, "pwd")
	waitSent(t, ptyOf(st, 0), "pwd")
	if got := ptyOf(st, 2).sent(); got != member {
		t.Fatalf("the member got %q", strings.TrimPrefix(got, member))
	}
}

func TestDockMemberShortcutsWork(t *testing.T) {
	win, _, r, _, _, _ := dockOne(t)
	r.Queue(shortcutFor(t, win, actPalette))
	sideFrame(t, win, r)
	if !win.palette.open {
		t.Fatal("the palette shortcut did nothing from the member")
	}
}

func TestDockMemberFocusReports(t *testing.T) {
	win, st, r, a, _, m1, _ := paneWindow(t)
	emitTo(t, st, 0, a, "\x1b[?1004h", func() bool { return modes(a.Session).FocusEvents })
	emitTo(t, st, 2, m1, "\x1b[?1004h", func() bool { return modes(m1.Session).FocusEvents })
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	waitSent(t, ptyOf(st, 0), "\x1b[O")
	waitSent(t, ptyOf(st, 2), "\x1b[I")
}

// Review focus 1: the floating pane and the docked member never both have
// the keyboard.
func TestDockKeysAndPaneKeysExclusive(t *testing.T) {
	win, _, r, a, _, m1, m2 := paneWindow(t)
	p, _ := win.fl.open(a.ID, m2.ID, paneMember)
	win.openDockMember(a.ID, m1.ID)
	floatFrames(t, win, r)
	win.focusPane(m2.ID)
	floatFrames(t, win, r)
	if win.fl.keys != m2.ID {
		t.Fatal("pane did not take the keyboard")
	}
	win.dockFocusMember(true)
	floatFrames(t, win, r)
	if win.fl.keys != 0 || !win.dock.keys {
		t.Fatalf("pane keys %d dock keys %v", win.fl.keys, win.dock.keys)
	}
	clickAt(t, win, r, centre(p.hits.body))
	if win.dock.keys || win.fl.keys != m2.ID {
		t.Fatalf("pane keys %d dock keys %v after a pane click", win.fl.keys, win.dock.keys)
	}
}

func TestDockMemberKeysLeaveOnTabSwitch(t *testing.T) {
	win, st, r, _, b, _ := dockOne(t)
	win.selectTab(b.ID)
	floatFrames(t, win, r)
	if win.dock.keys || win.focusTarget() != win.termTag {
		t.Fatalf("keys %v focus %v after switching", win.dock.keys, win.focusTarget())
	}
	member := ptyOf(st, 2).sent()
	typeKeys(t, win, r, "q")
	waitSent(t, ptyOf(st, 1), "q")
	if ptyOf(st, 2).sent() != member {
		t.Fatal("typing after the switch reached the member")
	}
}

func TestShiftEnterReachesDockMember(t *testing.T) {
	for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
		t.Run(string(name), func(t *testing.T) {
			win, st, r, _, _, _ := dockOne(t)
			r.Queue(key.Event{Name: name, Modifiers: key.ModShift, State: key.Press})
			sideFrame(t, win, r)
			waitSent(t, ptyOf(st, 2), "\x1b\r")
			for _, other := range []int{0, 1} {
				if ptyOf(st, other).sent() != "" {
					t.Fatalf("input reached unfocused terminal %d", other)
				}
			}
		})
	}
}

func TestDockMemberSelectionAutoScrolls(t *testing.T) {
	win, st, r, a, _, m1 := dockOne(t)
	for i, tab := range []*tabs.Tab{a, m1} {
		emitTo(t, st, []int{0, 2}[i], tab, strings.Repeat("line\r\n", 100), func() bool {
			tab.Session.Lock()
			defer tab.Session.Unlock()
			return tab.Session.Screen().ScrollbackLen() > 20
		})
	}
	floatFrames(t, win, r)
	v, ok := win.dockMemberView()
	if !ok {
		t.Fatal("no member view")
	}
	cw, ch := win.rend.CellSize()
	from := v.grid.Min.Add(image.Pt(cw/2, ch/2))
	to := image.Pt(from.X, v.grid.Min.Y-ch)
	win.probe = agentProbe{fg: func(*tabs.Tab) int { return 0 }, out: func(*tabs.Tab) uint64 { return 0 }, name: func(int) string { return "" }}
	now := time.Unix(1000, 0)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(from)})
	floatFrameAt(t, win, r, now)
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(to)})
	floatFrameAt(t, win, r, now)
	before := historyOffset(m1)
	floatFrameAt(t, win, r, now.Add(autoScrollEvery))
	if got := historyOffset(m1); got <= before {
		t.Fatalf("offset %d after %d: the member's selection did not auto-scroll", got, before)
	}
	if historyOffset(a) != 0 {
		t.Fatal("the member's selection scrolled the session")
	}
}

// Ruling 11: while a member tab shows, a 2 dp accent line tops whichever
// terminal has the keyboard; with no member tab there is none.
func TestDockFocusLineFollowsKeys(t *testing.T) {
	win, _, r, a, _, m1 := dockOne(t)
	if l := win.dock.focusLine; l.Empty() || l.Min.Y != win.dock.hits.body.Min.Y || l.Dy() != 2 {
		t.Fatalf("member has the keyboard: line %v, body %v", l, win.dock.hits.body)
	}
	clickAt(t, win, r, image.Pt(win.gridRect.Min.X+40, win.gridRect.Min.Y+40))
	if l := win.dock.focusLine; l.Empty() || l.Min.Y != win.dock.top.Min.Y {
		t.Fatalf("session has the keyboard: line %v, top %v", l, win.dock.top)
	}
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
	if !win.dock.focusLine.Empty() {
		t.Fatalf("agent tab: line %v", win.dock.focusLine)
	}
	_ = m1
}
