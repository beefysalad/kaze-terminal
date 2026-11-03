package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

func TestBuildSideAddsAgents(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Agents = func(id tabs.TabID) sideTray {
		if id == 1 {
			return sideTray{Shown: []sideAgent{{Type: "Explore", Task: "Find the auth middleware"}},
				Hidden: subagents.TrayCounts{Running: 2}, Total: 3, Active: 3}
		}
		if id == 4 {
			return sideTray{Total: 1, Active: 1}
		}
		return sideTray{}
	}
	m := buildSide(in)
	g := m.Groups[0]
	r := g.Rows[0]
	if r.ID != 1 || len(r.Agents) != 1 || r.Hidden != (subagents.TrayCounts{Running: 2}) || r.TrayTotal != 3 || r.AgentCount != 3 {
		t.Fatalf("row %+v", r)
	}
	if g.AgentCount != 4 {
		t.Fatalf("group count %d", g.AgentCount)
	}
}

func TestTrayTexts(t *testing.T) {
	for _, c := range []struct {
		total  int
		folded bool
		want   string
	}{{20, false, "20 agents ⌄"}, {20, true, "20 agents ›"}, {1, false, "1 agent ⌄"}} {
		if got := trayFold(c.total, c.folded); got != c.want {
			t.Errorf("trayFold(%d, %v) = %q, want %q", c.total, c.folded, got, c.want)
		}
	}
	for _, c := range []struct {
		h                subagents.TrayCounts
		lead, needs, rst string
	}{
		{subagents.TrayCounts{Running: 15, Done: 2}, "+17 more", "", " · 15 running, 2 done ⌄"},
		{subagents.TrayCounts{Waiting: 1, Running: 3, Done: 1}, "+5 more", " · 1 needs you", ", 3 running, 1 done ⌄"},
		{subagents.TrayCounts{Waiting: 2, Running: 1}, "+3 more", " · 2 need you", ", 1 running ⌄"},
		{subagents.TrayCounts{Waiting: 2}, "+2 more", " · 2 need you", " ⌄"},
		{subagents.TrayCounts{Done: 4}, "+4 more", "", " · 4 done ⌄"},
	} {
		lead, needs, rest := trayChip(c.h)
		if lead != c.lead || needs != c.needs || rest != c.rst {
			t.Errorf("trayChip(%+v) = (%q, %q, %q), want (%q, %q, %q)", c.h, lead, needs, rest, c.lead, c.needs, c.rst)
		}
	}
	if f, b := trayFooter(0); f != "Show fewer" || b != "" {
		t.Errorf("trayFooter(0) = %q %q", f, b)
	}
	if f, b := trayFooter(13); f != "Show fewer" || b != " · 13 more on the board" {
		t.Errorf("trayFooter(13) = %q %q", f, b)
	}
	if got := (sideRow{TrayTotal: 4, AgentCount: 2, Mark: markNeeds}).stateWord(); got != "4 agents ⌄" {
		t.Errorf("a row with a tray reads %q, want the fold control", got)
	}
	if got := (sideRow{TrayTotal: 4, TrayFolded: true, Mark: markDone}).stateWord(); got != "4 agents ›" {
		t.Errorf("a folded tray reads %q", got)
	}
	if got := (sideRow{Mark: markNeeds}).stateWord(); got != "needs you" {
		t.Errorf("a row without a tray reads %q", got)
	}
}

func TestTrayCaretsDraw(t *testing.T) {
	for _, r := range trayOpenCaret + trayShutCaret {
		if !render.BundledCovers(r) {
			t.Errorf("caret %q is not in the bundled fonts", string(r))
		}
	}
}

// trayRow is tab id's row in the window's current sidebar model.
func trayRow(t *testing.T, win *window, id tabs.TabID) sideRow {
	t.Helper()
	for _, r := range sessionRows(win.cachedSide(time.Now())) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row for %d", id)
	return sideRow{}
}

func startAgents(win *window, tab tabs.TabID, n int) {
	for i := range n {
		startAgent(win, tab, "s", fmt.Sprintf("x%02d", i))
	}
}

func TestTrayStateLimits(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	var router input.Router
	sideFrame(t, win, &router)
	startAgents(win, a.ID, 12)
	sideFrame(t, win, &router)
	if r := trayRow(t, win, a.ID); len(r.Agents) != trayCap || r.Hidden.Total() != 9 || r.TrayTotal != 12 || r.AgentCount != 12 {
		t.Fatalf("default tray: %d lines, hidden %+v, total %d", len(r.Agents), r.Hidden, r.TrayTotal)
	}
	win.side.trayOpen = map[tabs.TabID]bool{a.ID: true}
	if r := trayRow(t, win, a.ID); len(r.Agents) != trayMax || r.Hidden.Total() != 4 || !r.TrayExpanded {
		t.Fatalf("expanded tray: %d lines, hidden %+v", len(r.Agents), r.Hidden)
	}
	win.side.trayFolded = map[tabs.TabID]bool{a.ID: true}
	if r := trayRow(t, win, a.ID); len(r.Agents) != 0 || r.Hidden.Total() != 0 || r.TrayTotal != 12 || !r.TrayFolded {
		t.Fatalf("folded tray: %d lines, hidden %+v, total %d", len(r.Agents), r.Hidden, r.TrayTotal)
	}
}

func TestTrayStateRebuildsModel(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	var router input.Router
	startAgents(win, a.ID, 4)
	sideFrame(t, win, &router)
	sideFrame(t, win, &router)
	builds := win.side.cache.builds
	sideFrame(t, win, &router)
	if win.side.cache.builds != builds {
		t.Fatal("an unchanged frame rebuilt the model")
	}
	win.side.trayOpen = map[tabs.TabID]bool{a.ID: true}
	sideFrame(t, win, &router)
	if win.side.cache.builds != builds+1 {
		t.Fatalf("expanding the tray: builds %d, want %d", win.side.cache.builds, builds+1)
	}
	win.side.trayFolded = map[tabs.TabID]bool{a.ID: true}
	sideFrame(t, win, &router)
	if win.side.cache.builds != builds+2 {
		t.Fatalf("folding the tray: builds %d, want %d", win.side.cache.builds, builds+2)
	}
}

func TestTrayStatePrunedWithSession(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	openIn(t, win, osPath("/code/repo/web"))
	var router input.Router
	sideFrame(t, win, &router)
	win.side.trayOpen = map[tabs.TabID]bool{a.ID: true}
	win.side.trayFolded = map[tabs.TabID]bool{a.ID: true}
	win.closeTab(a.ID)
	sideFrame(t, win, &router)
	if win.side.trayOpen[a.ID] || win.side.trayFolded[a.ID] || len(win.side.trayOpen)+len(win.side.trayFolded) != 0 {
		t.Fatalf("closed session's tray state kept: %v %v", win.side.trayOpen, win.side.trayFolded)
	}
}

func TestFilterMatchesAgentTask(t *testing.T) {
	r := sideRow{Title: "x", Agents: []sideAgent{{Type: "Explore", Task: "Find the auth Middleware"}}}
	if !r.matches("middleware") || !r.matches("explore") || r.matches("nothing") {
		t.Fatal("filter")
	}
}

func TestElapsedText(t *testing.T) {
	a := &subagents.Agent{Started: t0, State: subagents.Running}
	for _, c := range []struct {
		d    time.Duration
		want string
	}{{30 * time.Second, "<1m"}, {4*time.Minute + 10*time.Second, "4m"}, {75 * time.Minute, "75m"}} {
		if got := agentElapsed(a, t0.Add(c.d)); got != c.want {
			t.Errorf("%v: %q want %q", c.d, got, c.want)
		}
	}
	a.State, a.Ended = subagents.Done, t0.Add(6*time.Minute+5*time.Second)
	if got := agentElapsed(a, t0.Add(time.Hour)); got != "took 6m" {
		t.Errorf("done: %q", got)
	}
	a.Ended = t0.Add(20 * time.Second)
	if got := agentElapsed(a, t0.Add(time.Hour)); got != "took <1m" {
		t.Errorf("quick: %q", got)
	}
}

func TestAgentChipText(t *testing.T) {
	if agentChip(0) != "" || agentChip(1) != "1 agent" || agentChip(3) != "3 agents" {
		t.Fatal("chip text")
	}
}

func TestSideAgentFromTracker(t *testing.T) {
	a := &subagents.Agent{Key: subagents.Key{Session: "s", ID: "a"}, Type: "Explore", Task: "Find", Started: t0, Depth: 1,
		Steps: []subagents.Step{{Tool: "Grep", Summary: "auth in src"}}}
	s := sideAgentOf(a, t0.Add(2*time.Minute))
	if s.Key != a.Key || s.Action != "Grep auth in src" || s.Elapsed != "2m" || s.Depth != 1 || s.Type != "Explore" {
		t.Fatalf("%+v", s)
	}
	a.Steps = nil
	if s := sideAgentOf(a, t0); s.Action != "Starting" {
		t.Fatalf("no steps: %q", s.Action)
	}
	a.State, a.Report = subagents.Done, "Found it in auth.ts\nmore"
	if s := sideAgentOf(a, t0); s.Action != "Found it in auth.ts" {
		t.Fatalf("done: %q", s.Action)
	}
}

// trayAgents is n tray lines with keys a0, a1, ...
func trayAgents(n int) []sideAgent {
	out := make([]sideAgent, n)
	for i := range out {
		out[i] = sideAgent{Key: subagents.Key{ID: fmt.Sprintf("a%d", i)}, Type: "Explore", Task: "Find the auth middleware", Action: "Grep auth", Elapsed: "4m"}
	}
	return out
}

func TestSidebarDrawsTray(t *testing.T) {
	rows := sideRows(1, 2, "s")
	rows[0].Agents = trayAgents(3)
	rows[0].Agents[1].State = subagents.Waiting
	rows[0].Hidden, rows[0].TrayTotal, rows[0].AgentCount = subagents.TrayCounts{Running: 15, Done: 2}, 20, 18
	m := sideModel{Loose: rows}
	gtx := pageContext(sidebarW, 700)
	h, size := sideDraw(t, m, sideState{}, 700)
	if len(h.agents) != 4 {
		t.Fatalf("agent hits %d, want 3 lines and a chip", len(h.agents))
	}
	first, second := h.rows[0].rect, h.rows[1].rect
	for i, a := range h.agents {
		if a.tab != 1 || a.rect.Min.Y < first.Max.Y || a.rect.Max.Y > second.Min.Y || a.rect.Max.X > size.X {
			t.Fatalf("hit %d %+v not between %v and %v", i, a, first, second)
		}
	}
	for i, a := range h.agents[:3] {
		if a.kind != hitAgentLine || a.key.ID != fmt.Sprintf("a%d", i) || a.rect.Dy() != gtx.Dp(sideAgentH) {
			t.Fatalf("line %d %+v, want a %ddp agent line", i, a, sideAgentH)
		}
		if !a.btn.In(a.rect) || a.btn.Max.X != a.rect.Max.X || a.btn.Empty() || a.btn.Dx() >= a.rect.Dx()/3 {
			t.Fatalf("line %d button %v not at the right end of %v", i, a.btn, a.rect)
		}
		if i > 0 && a.rect.Min.Y != h.agents[i-1].rect.Max.Y {
			t.Fatalf("line %d does not follow line %d", i, i-1)
		}
		if a.tip != "Grep auth" {
			t.Fatalf("line %d tip %q", i, a.tip)
		}
	}
	if got, want := h.agents[0].rect.Min.Y, first.Max.Y+gtx.Dp(sideTrayGap); got != want {
		t.Fatalf("first line top %d, want the row's bottom + %ddp = %d", got, sideTrayGap, want)
	}
	if h.agents[0].rect.Min.X <= first.Min.X+gtx.Dp(16) {
		t.Fatalf("tray line %v is not indented under row %v", h.agents[0].rect, first)
	}
	if c := h.agents[3]; c.kind != hitAgentChip || c.rect.Min.Y < h.agents[2].rect.Max.Y {
		t.Fatalf("chip %+v", c)
	}
}

func TestSidebarDrawsTrayFooter(t *testing.T) {
	rows := sideRows(1, 2, "s")
	rows[0].Agents = trayAgents(trayMax)
	rows[0].Hidden, rows[0].TrayTotal, rows[0].TrayExpanded = subagents.TrayCounts{Running: 13}, 21, true
	h, _ := sideDraw(t, sideModel{Loose: rows}, sideState{}, 900)
	if len(h.agents) != trayMax+2 {
		t.Fatalf("agent hits %d, want %d lines, Show fewer and the board link", len(h.agents), trayMax)
	}
	fewer, board := h.agents[trayMax], h.agents[trayMax+1]
	if fewer.kind != hitAgentFewer || board.kind != hitAgentBoard || board.rect.Min.X < fewer.rect.Max.X ||
		fewer.rect.Min.Y != board.rect.Min.Y || fewer.rect.Min.Y < h.agents[trayMax-1].rect.Max.Y {
		t.Fatalf("footer %+v %+v", fewer, board)
	}
	rows[0].Hidden, rows[0].TrayTotal = subagents.TrayCounts{}, trayMax
	h, _ = sideDraw(t, sideModel{Loose: rows}, sideState{}, 900)
	if len(h.agents) != trayMax+1 || h.agents[trayMax].kind != hitAgentFewer {
		t.Fatalf("with nothing beyond, hits %+v, want only Show fewer after the lines", h.agents[trayMax:])
	}
	rows[0].Agents, rows[0].TrayTotal = trayAgents(2), 2
	h, _ = sideDraw(t, sideModel{Loose: rows}, sideState{}, 900)
	if len(h.agents) != 2 {
		t.Fatalf("an expanded tray of two shows %d hits, want just the lines", len(h.agents))
	}
}

func TestSidebarFoldedTrayHasNoLines(t *testing.T) {
	rows := sideRows(1, 2, "s")
	rows[0].TrayTotal, rows[0].TrayFolded, rows[0].AgentCount = 5, true, 5
	gtx := pageContext(sidebarW, 700)
	h, _ := sideDraw(t, sideModel{Loose: rows}, sideState{}, 700)
	if len(h.agents) != 0 {
		t.Fatalf("folded tray drew %d agent hits", len(h.agents))
	}
	if h.rows[0].fold.Empty() || !h.rows[0].fold.Overlaps(h.rows[0].rect) {
		t.Fatalf("no fold control on the row: %+v", h.rows[0])
	}
	if span := h.rows[1].rect.Min.Y - h.rows[0].rect.Min.Y; span != gtx.Dp(sideRowH) || rowSpan(gtx, rows[0]) != gtx.Dp(sideRowH) {
		t.Fatalf("folded row spans %d, want %d", span, gtx.Dp(sideRowH))
	}
	if h, _ := sideDraw(t, sideModel{Loose: sideRows(1, 1, "s")}, sideState{}, 700); !h.rows[0].fold.Empty() {
		t.Fatal("a row without a tray has a fold control")
	}
}

func TestRevealScrollCountsAgentLines(t *testing.T) {
	rows := sideRows(1, 6, "s")
	for i := range rows[:5] {
		rows[i].Agents = []sideAgent{{Type: "X"}, {Type: "Y"}}
	}
	m := sideModel{Loose: rows}
	gtx := pageContext(sidebarW, 300)
	st := sideState{shown: 6}
	h := newChrome().layoutSidebar(gtx, m, st, image.Pt(sidebarW, 300))
	sc, ok := revealScroll(gtx, m, st, h)
	if !ok {
		t.Fatal("not found")
	}
	st.scroll = sc
	h = newChrome().layoutSidebar(gtx, m, st, image.Pt(sidebarW, 300))
	for _, r := range h.rows {
		if r.id == 6 && r.rect.Dy() == gtx.Dp(sideRowH) {
			return
		}
	}
	t.Fatalf("row 6 not shown whole at scroll %d", sc)
}

// chipText is the chip text as drawn into the sidebar's hits.
func chipText(h sideHits) string { return h.chip[0] + h.chip[1] + h.chip[2] }

func TestTrayChipKeepsCaretAtDefaultWidth(t *testing.T) {
	for _, c := range []struct {
		name  string
		h     subagents.TrayCounts
		needs string
	}{
		{"running", subagents.TrayCounts{Running: 15, Done: 2}, ""},
		{"waiting", subagents.TrayCounts{Waiting: 2, Running: 13, Done: 2}, "2 need you"},
	} {
		rows := sideRows(1, 2, "s")
		rows[0].Agents = trayAgents(3)
		rows[0].Hidden, rows[0].TrayTotal, rows[0].AgentCount = c.h, 20, 18
		h, _ := sideDraw(t, sideModel{Loose: rows}, sideState{}, 700)
		got := chipText(h)
		if !strings.HasSuffix(got, trayOpenCaret) || !strings.Contains(got, "+17 more") || strings.Contains(got, "…") {
			t.Errorf("%s: chip drew %q, want it whole with \"+17 more\" and the caret", c.name, got)
		}
		if c.needs != "" && !strings.Contains(h.chip[1], c.needs) {
			t.Errorf("%s: chip drew %q, want %q kept in its amber part", c.name, got, c.needs)
		}
	}
}
