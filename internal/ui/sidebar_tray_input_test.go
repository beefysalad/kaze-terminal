package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/unit"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// sideClickAt presses and releases at sidebar-local p, the press stamped at.
func sideClickAt(win *window, r *input.Router, p image.Point, b pointer.Buttons, at time.Duration) {
	w := sidePt(win, p)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: b, Position: w, Time: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: w, Time: at},
	)
}

// lineSpot is a point on k's tray line well left of its › button.
func lineSpot(t *testing.T, win *window, k subagents.Key) image.Point {
	t.Helper()
	h := agentHit(t, win, k)
	p := image.Pt(h.rect.Min.X+h.rect.Dx()/3, h.rect.Min.Y+h.rect.Dy()/2)
	if p.In(h.btn) {
		t.Fatalf("line spot %v falls in the button %v", p, h.btn)
	}
	return p
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

func TestAgentLineShowsSession(t *testing.T) {
	win, _, r, a, b, _, _ := paneWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	win.selectTab(b.ID)
	floatFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	floatFrames(t, win, r)
	if win.mgr.Active().ID != a.ID {
		t.Fatalf("active %d, want the agent's session %d", win.mgr.Active().ID, a.ID)
	}
	if win.dockOf(a.ID) != nil {
		t.Fatal("a click on the agent line opened its panel")
	}
}

func TestAgentButtonTogglesPanel(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	floatFrames(t, win, r)
	sideClickAt(win, r, centre(agentHit(t, win, k).btn), pointer.ButtonPrimary, 0)
	floatFrames(t, win, r)
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("› did not open the panel")
	}
	if !agentHit(t, win, k).open {
		t.Fatal("the open agent's line is not highlighted")
	}
	sideClickAt(win, r, centre(agentHit(t, win, k).btn), pointer.ButtonPrimary, 2*doubleClick)
	floatFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("a second › did not close it")
	}
}

func TestAgentLineDoubleClickOpensPanel(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	floatFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 100*time.Millisecond)
	floatFrames(t, win, r)
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("a double-click on the line did not open the panel")
	}
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 3*time.Second)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 3*time.Second+100*time.Millisecond)
	floatFrames(t, win, r)
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("a second double-click closed the panel")
	}
}

func TestAgentLineDoubleClickNeedsSameLine(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	j := startAgent(win, a.ID, "s", "y")
	floatFrames(t, win, r)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
	sideClickAt(win, r, lineSpot(t, win, j), pointer.ButtonPrimary, 100*time.Millisecond)
	floatFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("clicks on two different lines opened a panel")
	}
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 5*time.Second)
	sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 7*time.Second)
	floatFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("two slow clicks on one line opened a panel")
	}
}

// TestAgentLineMatchesRowClick: a click on an agent line leaves the window as a click
// on its session's row does.
func TestAgentLineMatchesRowClick(t *testing.T) {
	type state struct {
		active, keys tabs.TabID
		dock         bool
		panes        int
	}
	run := func(onLine bool) state {
		win, _, r, a, b, m1, _ := paneWindow(t)
		k := startAgent(win, a.ID, "s", "x")
		win.fl.open(a.ID, m1.ID, paneMember)
		floatFrames(t, win, r)
		win.selectTab(b.ID)
		floatFrames(t, win, r)
		if onLine {
			sideClickAt(win, r, lineSpot(t, win, k), pointer.ButtonPrimary, 0)
		} else {
			sideClickAt(win, r, centre(rowHit(t, win, a.ID).rect), pointer.ButtonPrimary, 0)
		}
		floatFrames(t, win, r)
		return state{win.mgr.Active().ID, win.fl.keys, win.dockOf(a.ID) != nil, len(win.fl.set(a.ID).panes)}
	}
	if line, row := run(true), run(false); line != row {
		t.Fatalf("agent line click %+v, row click %+v", line, row)
	}
}

func TestTrayChipExpandsAndFewerCollapses(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	startAgents(win, a.ID, 12)
	floatFrames(t, win, r)
	if n, chips := len(trayHits(win, a.ID, hitAgentLine)), trayHits(win, a.ID, hitAgentChip); n != trayCap || len(chips) != 1 {
		t.Fatalf("collapsed tray: %d lines, chips %v", n, chips)
	}
	sideClickAt(win, r, centre(trayHits(win, a.ID, hitAgentChip)[0].rect), pointer.ButtonPrimary, 0)
	floatFrames(t, win, r)
	fewer := trayHits(win, a.ID, hitAgentFewer)
	if n := len(trayHits(win, a.ID, hitAgentLine)); n != trayMax || len(fewer) != 1 || len(trayHits(win, a.ID, hitAgentBoard)) != 1 {
		t.Fatalf("expanded tray: %d lines, fewer %v", n, fewer)
	}
	if win.dockOf(a.ID) != nil {
		t.Fatal("the chip opened a panel")
	}
	sideClickAt(win, r, centre(fewer[0].rect), pointer.ButtonPrimary, time.Second)
	floatFrames(t, win, r)
	if n := len(trayHits(win, a.ID, hitAgentLine)); n != trayCap || len(trayHits(win, a.ID, hitAgentChip)) != 1 {
		t.Fatalf("after Show fewer: %d lines", n)
	}
}

func TestTrayFoldControl(t *testing.T) {
	win, _, r, a, b, _, _ := paneWindow(t)
	startAgents(win, a.ID, 4)
	win.selectTab(b.ID)
	floatFrames(t, win, r)
	sideClickAt(win, r, centre(rowHit(t, win, a.ID).fold), pointer.ButtonPrimary, 0)
	floatFrames(t, win, r)
	if !win.side.trayFolded[a.ID] || len(trayHits(win, a.ID, hitAgentLine)) != 0 {
		t.Fatalf("fold: folded %v, lines %d", win.side.trayFolded[a.ID], len(trayHits(win, a.ID, hitAgentLine)))
	}
	if win.mgr.Active().ID != b.ID {
		t.Fatal("the fold control selected the session")
	}
	sideClickAt(win, r, centre(rowHit(t, win, a.ID).fold), pointer.ButtonPrimary, time.Second)
	floatFrames(t, win, r)
	if win.side.trayFolded[a.ID] || len(trayHits(win, a.ID, hitAgentLine)) != trayCap {
		t.Fatal("a second click did not open the tray")
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
	sideMove(win, &r, pointer.Move, centre(agentHit(t, win, k).btn))
	sideFrame(t, win, &r)
	if win.side.hits.tip.Empty() || win.side.redraws != before+1 {
		t.Fatal("moving onto › hid the tooltip or redrew")
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
