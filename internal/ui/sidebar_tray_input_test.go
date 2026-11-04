package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// sideClickAt presses and releases at sidebar-local p, the press stamped at.
func sideClickAt(win *window, r *input.Router, p image.Point, b pointer.Buttons, at time.Duration) {
	w := sidePt(win, p)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: b, Position: w, Time: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: w, Time: at},
	)
}

// lineSpot is the centre of k's tray line.
func lineSpot(t *testing.T, win *window, k subagents.Key) image.Point {
	t.Helper()
	return centre(agentHit(t, win, k).rect)
}

// trayHits is tab's tray hits of kind.
func trayHits(win *window, tab tabs.TabID, kind agentHitKind) []sideAgentHit {
	var out []sideAgentHit
	for _, h := range win.side.hits.agents {
		if h.tab == tab && h.kind == kind {
			out = append(out, h)
		}
	}
	return out
}

func TestAgentLineTogglesPanel(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("a click on the line did not open its panel")
	}
	if !agentHit(t, win, k).open {
		t.Fatal("the open agent's line is not highlighted")
	}
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 2*doubleClick)
	twoFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("a second click on the line did not close it")
	}
}

func TestAgentLineOpensOtherSession(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != a.ID {
		t.Fatalf("active %d, want the agent's session %d", win.mgr.Active().ID, a.ID)
	}
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("the agent's panel did not open in its session")
	}
}

// TestAgentLineQuickClicksToggleTwice: no double-click rule is left, so two quick
// presses on one line open the panel and close it again.
func TestAgentLineQuickClicksToggleTwice(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 100*time.Millisecond)
	twoFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("two quick clicks left the panel open")
	}
}

func TestTrayChipOpensBoardForSession(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	startAgents(win, a.ID, 12)
	twoFrames(t, win, r)
	chips := trayHits(win, a.ID, hitAgentChip)
	if n := len(trayHits(win, a.ID, hitAgentLine)); n != trayCap || len(chips) != 1 {
		t.Fatalf("tray: %d lines, chips %v", n, chips)
	}
	sideClickAt(win, r, centre(chips[0].rect), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if !win.board.open || win.board.tab != a.ID {
		t.Fatalf("the chip did not open the board for its session: open %v tab %d", win.board.open, win.board.tab)
	}
	if win.dockOf(a.ID) != nil {
		t.Fatal("the chip opened a panel")
	}
	win.closeBoard()
	twoFrames(t, win, r)
	twoFrames(t, win, r)
	if n, chips := len(trayHits(win, a.ID, hitAgentLine)), trayHits(win, a.ID, hitAgentChip); n != trayCap || len(chips) != 1 {
		t.Fatalf("after the board: %d lines, chips %v, want the tray unchanged", n, chips)
	}
}

func TestTrayFoldControl(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	startAgents(win, a.ID, 4)
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	sideClickAt(win, r, centre(rowHit(t, win, a.ID).fold), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if !win.side.trayFolded[a.ID] || len(trayHits(win, a.ID, hitAgentLine)) != 0 {
		t.Fatalf("fold: folded %v, lines %d", win.side.trayFolded[a.ID], len(trayHits(win, a.ID, hitAgentLine)))
	}
	if win.mgr.Active().ID != b.ID {
		t.Fatal("the fold control selected the session")
	}
	sideClickAt(win, r, centre(rowHit(t, win, a.ID).fold), pointer.ButtonPrimary, time.Second)
	twoFrames(t, win, r)
	if win.side.trayFolded[a.ID] || len(trayHits(win, a.ID, hitAgentLine)) != trayCap {
		t.Fatal("a second click did not open the tray")
	}
}

// TestTrayFoldClearOfClose: on a hovered row the fold control and the × leave a dead
// zone of at least 8 dp, so a near-miss on the fold never closes the session.
func TestTrayFoldClearOfClose(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	startAgents(win, a.ID, 4)
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	sideMove(win, r, pointer.Move, centre(rowHit(t, win, a.ID).fold))
	twoFrames(t, win, r)
	h := rowHit(t, win, a.ID)
	if h.close.Empty() || h.fold.Empty() {
		t.Fatalf("hovered row: fold %v close %v, want both", h.fold, h.close)
	}
	if gap := h.close.Min.X - h.fold.Max.X; gap < gtx0Dp(8) {
		t.Fatalf("fold %v and close %v are %d px apart, want at least %d (8 dp)", h.fold, h.close, gap, gtx0Dp(8))
	}
	// One pixel right of the fold rect (Max is exclusive).
	sideClickAt(win, r, image.Pt(h.fold.Max.X, centre(h.fold).Y), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if !win.mgr.Has(a.ID) {
		t.Fatal("a press just right of the fold control closed the session")
	}
}

// stepAgent starts agent id in tab with a current step "Grep auth".
func stepAgent(win *window, tab tabs.TabID, id string) subagents.Key {
	k := startAgent(win, tab, "s", id)
	win.subs.Apply(subagents.Event{Kind: subagents.EvTool, Tab: int(tab), Src: int(tab), Session: "s", AgentID: id, AgentType: "Explore", Tool: "Grep", Summary: "auth", At: time.Now()})
	return k
}

func sideMove(win *window, r *input.Router, kind pointer.Kind, p image.Point) {
	r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: sidePt(win, p)})
}

func TestTrayTooltipShowsStep(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	k := stepAgent(win, a.ID, "x")
	var r input.Router
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	wake, wakeOK := r.WakeupTime()
	line := agentHit(t, win, k)
	if line.tip != "Grep auth" || !win.side.hits.tip.Empty() {
		t.Fatalf("before hover: tip %q, tooltip %v", line.tip, win.side.hits.tip)
	}
	before := win.side.redraws
	sideMove(win, &r, pointer.Move, centre(line.rect))
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	tip := win.side.hits.tip
	if tip.Empty() || tip.Min.Y < line.rect.Max.Y || !tip.Overlaps(win.side.hits.list) {
		t.Fatalf("tooltip %v, want it just below the line %v", tip, line.rect)
	}
	if win.side.redraws != before+1 {
		t.Fatalf("hover asked for %d redraws, want 1", win.side.redraws-before)
	}
	for range 3 {
		sideFrame(t, win, &r)
	}
	if win.side.redraws != before+1 {
		t.Fatal("idle frames with the tooltip asked for redraws")
	}
	if at, ok := r.WakeupTime(); ok != wakeOK || at != wake {
		t.Fatalf("the tooltip changed the wakeup: %v %v, was %v %v", at, ok, wake, wakeOK)
	}
	line = agentHit(t, win, k)
	sideMove(win, &r, pointer.Move, image.Pt(line.rect.Max.X-2, centre(line.rect).Y))
	sideFrame(t, win, &r)
	if win.side.hits.tip.Empty() || win.side.redraws != before+1 {
		t.Fatal("moving along the line to its elapsed time hid the tooltip or redrew")
	}
}

func TestTrayTooltipFlipsAbove(t *testing.T) {
	rows := sideRows(1, 1, "s")
	rows[0].Agents = trayAgents(1)
	rows[0].TrayTotal = 1
	m := sideModel{Loose: rows}
	st := sideState{hoverAgent: rows[0].Agents[0].Key}
	h, _ := sideDraw(t, m, st, 700)
	line := h.agents[0].rect
	if h.tip.Empty() || h.tip.Min.Y < line.Max.Y {
		t.Fatalf("tall list: tooltip %v, want it below %v", h.tip, line)
	}
	h, _ = sideDraw(t, m, st, line.Max.Y+gtx0Dp(10))
	line = h.agents[0].rect
	if h.tip.Empty() || h.tip.Max.Y > line.Min.Y || h.tip.Max.Y > h.list.Max.Y {
		t.Fatalf("line at the bottom: tooltip %v, want it above %v", h.tip, line)
	}
	rows[0].Agents[0].Action = ""
	if h, _ := sideDraw(t, sideModel{Loose: rows}, st, 700); !h.tip.Empty() {
		t.Fatal("a line with no step drew a tooltip")
	}
}

// gtx0Dp is n dp at the test pages' scale.
func gtx0Dp(n unit.Dp) int { return pageContext(sidebarW, 100).Dp(n) }

func TestTrayTooltipGoneOnLeave(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	k := stepAgent(win, a.ID, "x")
	var r input.Router
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	p := centre(agentHit(t, win, k).rect)
	sideMove(win, &r, pointer.Move, p)
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	if win.side.hits.tip.Empty() {
		t.Fatal("no tooltip on hover")
	}
	sideMove(win, &r, pointer.Leave, p)
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	if !win.side.hits.tip.Empty() {
		t.Fatalf("tooltip %v after the pointer left", win.side.hits.tip)
	}
}
