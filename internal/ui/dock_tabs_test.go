package ui

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// Tests for the panel's tab row (issue #133, plan task D2).

// dockAgents starts n agents in tab's session, each with a task, and opens them all.
func dockAgents(t *testing.T, win *window, tab tabs.TabID, n int, task string) []subagents.Key {
	t.Helper()
	win.liveMgr.Store(win.mgr)
	var ks []subagents.Key
	for i := range n {
		id := fmt.Sprintf("a%d", i)
		win.onNotify(notify.Msg{Tab: int(tab), Kind: notify.KindTool, Session: "s", Tool: "Agent", AgentType: "Explore", Description: task})
		win.onNotify(notify.Msg{Tab: int(tab), Kind: notify.KindAgentStart, Session: "s", AgentID: id, AgentType: "Explore"})
		k := subagents.Key{Session: "s", ID: id}
		win.openDock(tab, k)
		ks = append(ks, k)
	}
	return ks
}

func TestDockDrawsTabRow(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	win.liveMgr.Store(win.mgr)
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStart, Session: "s", AgentID: "x", AgentType: "Explore"})
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStart, Session: "s", AgentID: "y", AgentType: "Plan"})
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "y"})
	twoFrames(t, win, r)
	h := win.dock.hits
	if len(h.tabs) != 2 {
		t.Fatalf("%d tabs drawn", len(h.tabs))
	}
	lo, hi := pageContext(1, 1).Dp(dockTabMinDp), pageContext(1, 1).Dp(dockTabMaxDp)
	for i, x := range h.tabs {
		if x.rect.Dx() < lo || x.rect.Dx() > hi {
			t.Fatalf("tab %d width %d not in [%d, %d]", i, x.rect.Dx(), lo, hi)
		}
		if i > 0 && x.rect.Min.X < h.tabs[i-1].rect.Max.X {
			t.Fatalf("tabs not left to right: %v", h.tabs)
		}
	}
	for _, s := range []string{"Explore", "Plan", "Hide ⌄"} {
		if !slices.Contains(win.dock.texts, s) {
			t.Fatalf("%q not drawn in %q", s, win.dock.texts)
		}
	}
	if h.hide.Empty() || h.hide.Min.X < h.tabs[1].rect.Max.X {
		t.Fatalf("hide %v", h.hide)
	}
}

func TestDockTabsShrinkThenCut(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	dockAgents(t, win, a.ID, 6, strings.Repeat("long task words ", 5))
	twoFrames(t, win, r)
	h := win.dock.hits
	if len(h.tabs) != 6 {
		t.Fatalf("%d tabs drawn", len(h.tabs))
	}
	lo := pageContext(1, 1).Dp(dockTabMinDp)
	for i, x := range h.tabs {
		if x.rect.Dx() < lo {
			t.Fatalf("tab %d is %d px, under the least %d", i, x.rect.Dx(), lo)
		}
	}
	if last := h.tabs[5].rect; last.Max.X > h.hide.Min.X {
		t.Fatalf("last tab %v runs into Hide %v", last, h.hide)
	}
}

func TestDockTabClickSwitches(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	ks := dockAgents(t, win, a.ID, 2, "task")
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.tabs[0].rect))
	if s := win.dockOf(a.ID); s == nil || s.agent != ks[0] {
		t.Fatalf("tab click did not show the first agent: %+v", s)
	}
}

func TestDockTabCloseKeepsOthers(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	ks := dockAgents(t, win, a.ID, 2, "task")
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.tabs[1].close))
	d := win.deckOf(a.ID)
	if d == nil || len(d.entries) != 1 || win.dockOf(a.ID).agent != ks[0] {
		t.Fatalf("× did not close only its tab: %+v", d)
	}
	if win.subs.Get(ks[1]) == nil {
		t.Fatal("× touched the agent")
	}
}

func TestDockHideKeepsTabs(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	ks := dockAgents(t, win, a.ID, 2, "task")
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.hide))
	if !win.dockRect().Empty() {
		t.Fatal("Hide left the panel drawn")
	}
	if d := win.deckOf(a.ID); d == nil || len(d.entries) != 2 || !d.hidden {
		t.Fatalf("Hide lost the tabs: %+v", d)
	}
	win.showAgent(a.ID, ks[0], false)
	twoFrames(t, win, r)
	if win.dockRect().Empty() || win.dockOf(a.ID).agent != ks[0] {
		t.Fatal("showing an agent did not bring the panel back")
	}
}

func TestDockHoverRedrawsOnlyOnChange(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	dockAgents(t, win, a.ID, 2, "task")
	twoFrames(t, win, r)
	move := func(p image.Point) {
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at(p)})
		sideFrame(t, win, r)
	}
	t0, t1 := win.dock.hits.tabs[0].rect, win.dock.hits.tabs[1].rect
	before := win.dock.redraws
	move(centre(t0))
	move(centre(t0).Add(image.Pt(1, 0)))
	if got := win.dock.redraws - before; got != 1 || win.dock.hover != 0 {
		t.Fatalf("redraws %d hover %d over tab 0", got, win.dock.hover)
	}
	move(centre(t1))
	if got := win.dock.redraws - before; got != 2 || win.dock.hover != 1 {
		t.Fatalf("redraws %d hover %d over tab 1", got, win.dock.hover)
	}
}

func TestDockHiddenAgentReadsNothing(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := dockScene(t, win, int(a.ID), "running")
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	s := win.deckOf(a.ID).shown().view
	read := s.trAt
	if read.IsZero() {
		t.Fatal("the shown agent never read its transcript")
	}
	win.closeDock(a.ID)
	for i := range 5 {
		gtx := pageContext(1000, 600)
		gtx.Source = r.Source()
		gtx.Now = time.Unix(1000, 0).Add(time.Duration(i+1) * 600 * time.Millisecond)
		if err := win.frame(gtx); err != nil {
			t.Fatal(err)
		}
		r.Frame(gtx.Ops)
	}
	if s.trAt != read {
		t.Fatalf("a hidden panel read its transcript at %v", s.trAt)
	}
}
