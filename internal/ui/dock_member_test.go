package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/pointer"
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
