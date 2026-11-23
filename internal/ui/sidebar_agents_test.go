package ui

import (
	"fmt"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
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
	// The sidebar no longer draws the tray, but the model still counts it for the rail and the filter.
	if r := trayRow(t, win, a.ID); len(r.Agents) != trayCap || r.Hidden.Total() != 9 || r.TrayTotal != 12 || r.AgentCount != 12 {
		t.Fatalf("tray: %d lines, hidden %+v, total %d", len(r.Agents), r.Hidden, r.TrayTotal)
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
	// A markdown report shows its first line of prose, without the marks.
	for report, want := range map[string]string{
		"## Summary\n\n**No existing binding** for `split`.\n": "No existing binding for split.",
		"# Report\n\n- Cmd+D is free\n":                        "Cmd+D is free",
		"> Quoted\n":                                           "Quoted",
		"```go\nfunc x() {}\n```\nAfter the code\n":            "After the code",
		"## Only a heading\n":                                  "Only a heading",
		"#125 landed, tests pass\n":                            "#125 landed, tests pass",
		"#\n\n#7 is next\n":                                    "#7 is next",
	} {
		a.Report = report
		if s := sideAgentOf(a, t0); s.Action != want {
			t.Fatalf("report %q: %q, want %q", report, s.Action, want)
		}
	}
}
