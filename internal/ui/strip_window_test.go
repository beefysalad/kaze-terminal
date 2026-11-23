package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// spacesFixture has space alpha (a1, a2) and space beta (b1), a1 shown. Every
// test is bounded: three tabs, a handful of 1000x600 frames, no real input.
func spacesFixture(t *testing.T) (win *window, st *recStarter, r *input.Router, a1, a2, b1 *tabs.Tab) {
	t.Helper()
	win, st = sideWindow(t, true)
	a1 = openIn(t, win, osPath("/code/alpha/a"))
	a2 = openIn(t, win, osPath("/code/alpha/b"))
	b1 = openIn(t, win, osPath("/code/beta/a"))
	win.mgr.SetActive(a1.ID)
	r = new(input.Router)
	twoFrames(t, win, r)
	return
}

func activeID(win *window) tabs.TabID {
	if a := win.mgr.Active(); a != nil {
		return a.ID
	}
	return 0
}

func TestStripCarvesTheTopOfTheTerminalArea(t *testing.T) {
	win, _, _, _, _, _ := spacesFixture(t)
	s := win.strip.rect
	if s.Dy() != 36 {
		t.Fatalf("strip is %v, want 36 px high at 1 px per dp", s)
	}
	if s.Min.Y != 42 {
		t.Fatalf("strip %v is not right under the 42 px title bar", s)
	}
	if win.gridRect.Min.Y < s.Max.Y {
		t.Fatalf("terminal grid %v starts inside the strip %v", win.gridRect, s)
	}
	_, ch := win.rend.CellSize()
	_, rows := win.mgr.Size()
	if want := win.gridRect.Dy() / ch; rows != want {
		t.Fatalf("grid is %d rows, want %d for the carved area", rows, want)
	}
}

func TestSelectingASpaceActivatesItsLastUsedTab(t *testing.T) {
	win, _, r, a1, a2, b1 := spacesFixture(t)
	win.selectTab(a2.ID)
	sideFrame(t, win, r)
	win.selectTab(b1.ID)
	sideFrame(t, win, r)
	win.selectSpace(win.spaceKeyOf(a1.ID))
	if got := activeID(win); got != a2.ID {
		t.Fatalf("selecting alpha activated %d, want its last used tab %d", got, a2.ID)
	}
}

func TestClosingTheLastTabOfASpaceSelectsTheNeighbouringSpace(t *testing.T) {
	win, _, r, a1, a2, b1 := spacesFixture(t)
	win.selectTab(b1.ID)
	sideFrame(t, win, r)
	win.closeTab(b1.ID)
	sideFrame(t, win, r)
	if got := activeID(win); got != a1.ID && got != a2.ID {
		t.Fatalf("after closing beta's only tab, active is %d, want a tab of alpha", got)
	}
}

func TestClosingATabStaysInItsSpace(t *testing.T) {
	win, _, r, a1, a2, b1 := spacesFixture(t)
	win.selectTab(a1.ID)
	sideFrame(t, win, r)
	win.closeTab(a1.ID)
	if got := activeID(win); got != a2.ID {
		t.Fatalf("closing a1 selected %d, want its space mate %d (not beta's %d)", got, a2.ID, b1.ID)
	}
}

func TestCycleTabWrapsInsideTheSelectedSpace(t *testing.T) {
	win, _, r, a1, a2, b1 := spacesFixture(t)
	win.selectTab(a2.ID)
	sideFrame(t, win, r)
	win.cycleTab(1)
	if got := activeID(win); got != a1.ID {
		t.Fatalf("next from a2 went to %d, want a1 %d (wrap inside the space, not beta's %d)", got, a1.ID, b1.ID)
	}
	win.cycleTab(-1)
	if got := activeID(win); got != a2.ID {
		t.Fatalf("prev from a1 went to %d, want a2 %d", got, a2.ID)
	}
}

func TestCycleTabLeavesASingleTabSpace(t *testing.T) {
	win, _, r, a1, _, b1 := spacesFixture(t)
	win.selectTab(b1.ID)
	sideFrame(t, win, r)
	win.cycleTab(1)
	if got := activeID(win); got == b1.ID {
		t.Fatalf("a one-tab space traps the keyboard: still on %d (a1 is %d)", got, a1.ID)
	}
}

// stripPoint is the window point at the centre of a strip rect given relative to the strip.
func stripPoint(win *window, rel image.Rectangle) image.Point {
	return win.strip.rect.Min.Add(centre(rel))
}

func TestStripClickSelectsClosesAndOpensTabs(t *testing.T) {
	win, st, r, a1, a2, _ := spacesFixture(t)
	if got := len(win.strip.lay.Tabs); got != 2 {
		t.Fatalf("strip shows %d tabs, want alpha's 2", got)
	}
	// Select a2.
	clickAt(t, win, r, stripPoint(win, win.strip.lay.Tabs[1].Rect))
	if got := activeID(win); got != a2.ID {
		t.Fatalf("click on the second tab activated %d, want %d", got, a2.ID)
	}
	// Close a2 with its x.
	clickAt(t, win, r, stripPoint(win, win.strip.lay.Tabs[1].Close))
	if win.mgr.Lookup(a2.ID) != nil {
		t.Fatal("the x did not close the tab")
	}
	if got := activeID(win); got != a1.ID {
		t.Fatalf("after closing a2 active is %d, want a1 %d", got, a1.ID)
	}
	// + opens a tab in the space's folder.
	before := len(st.where)
	clickAt(t, win, r, stripPoint(win, win.strip.lay.Plus))
	if len(st.where) != before+1 {
		t.Fatalf("+ started %d shells, want 1", len(st.where)-before)
	}
	if got, want := st.where[len(st.where)-1], osPath("/code/alpha/a"); got != want {
		t.Fatalf("+ started the shell in %q, want the space's folder %q", got, want)
	}
}

func TestStripWheelScrollsOverflowingTabs(t *testing.T) {
	win, _ := sideWindow(t, true)
	for range 12 {
		openIn(t, win, osPath("/code/alpha/a"))
	}
	win.mgr.SetActive(win.mgr.Tabs()[0].ID)
	r := new(input.Router)
	twoFrames(t, win, r)
	if !win.strip.lay.Scrolls() {
		t.Fatalf("12 tabs should overflow the strip: %+v", win.strip.lay)
	}
	r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: at(stripPoint(win, win.strip.lay.Tabs[0].Rect)), Scroll: at(image.Pt(0, 120))})
	twoFrames(t, win, r)
	if win.strip.scroll <= 0 {
		t.Fatalf("wheel did not scroll the strip: scroll %d", win.strip.scroll)
	}
}

func TestStripUnchangedFramesAllocateNothingAndWakeNothing(t *testing.T) {
	win, _, _, _, _, _ := spacesFixture(t)
	g := pageContext(1000, 600)
	g.Now = time.Unix(1000, 0)
	draw := func() {
		g.Ops.Reset()
		win.drawStrip(g, win.strip.rect)
	}
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("an unchanged strip allocated %.0f times per draw, want 0", n)
	}
	if win.strip.draws == 0 || win.strip.draws > 3 {
		t.Fatalf("strip built its ops %d times for an unchanged scene", win.strip.draws)
	}
}
