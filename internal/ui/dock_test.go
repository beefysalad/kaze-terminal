package ui

import (
	"image"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

func TestDockSplitDefault(t *testing.T) {
	term := image.Rect(200, 42, 1200, 642)
	top, panel := dockSplit(term, 0, 1, 16, 8)
	if panel.Dy() != 240 || panel.Max != term.Max || panel.Min.X != term.Min.X {
		t.Fatalf("panel %v", panel)
	}
	if top.Min != term.Min || top.Max.Y != panel.Min.Y || top.Max.X != term.Max.X {
		t.Fatalf("top %v panel %v", top, panel)
	}
}

func TestDockSplitClamps(t *testing.T) {
	term := image.Rect(0, 0, 1000, 600)
	if _, p := dockSplit(term, 50, 1, 16, 8); p.Dy() != 120 {
		t.Fatalf("small height not raised to 120: %v", p)
	}
	if top, _ := dockSplit(term, 900, 1, 16, 8); top.Dy() != 8*16+8 {
		t.Fatalf("terminal lost its 8 rows: %v", top)
	}
	if _, p := dockSplit(term, 200, 2, 16, 8); p.Dy() != 400 {
		t.Fatalf("dp not scaled: %v", p)
	}
}

func TestDockSplitSmallWindow(t *testing.T) {
	term := image.Rect(0, 0, 800, 150)
	top, p := dockSplit(term, 300, 1, 16, 8)
	if !p.Empty() || top != term {
		t.Fatalf("too small for both: top %v panel %v", top, p)
	}
	term = image.Rect(0, 0, 800, 200) // 136 for the terminal, 64 left
	top, p = dockSplit(term, 300, 1, 16, 8)
	if top.Dy() != 136 || p.Dy() != 64 {
		t.Fatalf("terminal minimum must win: top %v panel %v", top, p)
	}
}

func TestDockToggle(t *testing.T) {
	win, _, _, a, _, _, _ := paneWindow(t)
	k := subagents.Key{Session: "s", ID: "k"}
	j := subagents.Key{Session: "s", ID: "j"}
	win.toggleDock(a.ID, k)
	if s := win.dockOf(a.ID); s == nil || s.agent != k || s.tab != dockActivity || !s.follow {
		t.Fatalf("not open: %+v", s)
	}
	win.toggleDock(a.ID, k)
	if win.dockOf(a.ID) != nil {
		t.Fatal("same agent did not close")
	}
	win.openDock(a.ID, k)
	s := win.dockOf(a.ID)
	s.tab, s.picked, s.scroll = dockReport, true, 40
	win.toggleDock(a.ID, j)
	if s := win.dockOf(a.ID); s == nil || s.agent != j || s.tab != dockActivity || s.picked || s.scroll != 0 {
		t.Fatalf("switch did not reset: %+v", s)
	}
}

func TestDockPerSession(t *testing.T) {
	win, _, _, a, b, _, _ := paneWindow(t)
	x := subagents.Key{Session: "s", ID: "x"}
	y := subagents.Key{Session: "t", ID: "y"}
	win.openDock(a.ID, x)
	win.openDock(b.ID, y)
	if win.dockOf(a.ID).agent != x || win.dockOf(b.ID).agent != y {
		t.Fatal("sessions share one dock")
	}
	win.closeDock(b.ID)
	if win.dockOf(b.ID) != nil || win.dockOf(a.ID) == nil {
		t.Fatal("close touched the wrong session")
	}
}

func TestDockShrinksOnlyItsSession(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	floatFrames(t, win, r)
	_, rows := win.mgr.Size()
	if got := ptyOf(st, 0).lastSize()[1]; got >= rows {
		t.Fatalf("docked session rows %d, shared %d", got, rows)
	}
	if got := ptyOf(st, 1).lastSize()[1]; got != rows {
		t.Fatalf("other session rows %d, want %d", got, rows)
	}
	if win.dockRect().Empty() {
		t.Fatal("no panel rect")
	}
}

func TestDockNoResizeChurn(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	floatFrames(t, win, r)
	n := ptyOf(st, 0).resizeCount()
	for range 5 {
		sideFrame(t, win, r)
	}
	if got := ptyOf(st, 0).resizeCount(); got != n {
		t.Fatalf("%d resizes on still frames", got-n)
	}
}

func TestDockCloseRestoresSize(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	floatFrames(t, win, r)
	win.closeDock(a.ID)
	floatFrames(t, win, r)
	cols, rows := win.mgr.Size()
	if got := ptyOf(st, 0).lastSize(); got != [2]int{cols, rows} {
		t.Fatalf("closed dock left %v, want %v", got, [2]int{cols, rows})
	}
	if !win.dockRect().Empty() {
		t.Fatal("panel rect kept after close")
	}
}

func TestDockPrunedWithSession(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	floatFrames(t, win, r)
	win.closeTab(a.ID)
	floatFrames(t, win, r)
	if _, ok := win.dock.by[a.ID]; ok {
		t.Fatal("closed session still has a panel")
	}
	if win.dock.sized[a.ID] {
		t.Fatal("closed session still marked sized")
	}
}

func TestDockMemberPaneStaysAbove(t *testing.T) {
	win, _, r, a, _, m1, _ := paneWindow(t)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	win.fl.open(a.ID, m1.ID, paneMember)
	p := win.fl.set(a.ID).find(m1.ID)
	p.r = image.Rect(0, 400, 560, 900) // asks to reach far below
	floatFrames(t, win, r)
	if p.hits.outer.Max.Y > win.dockRect().Min.Y {
		t.Fatalf("pane %v covers the panel %v", p.hits.outer, win.dockRect())
	}
}
