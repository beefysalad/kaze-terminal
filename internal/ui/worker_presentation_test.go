package ui

import (
	"image"
	"reflect"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestWorkerSpaceAndStripOrder(t *testing.T) {
	ts, locs := sideFixture()
	w1, w2 := sideTab(5, "review"), sideTab(6, "tests")
	w1.Parent, w1.Role = 1, "worker"
	w2.Parent, w2.Role = 1, "worker"
	ts = append(ts, w1, w2)
	// A worker with a discovered different repo must still follow its parent.
	locs[6] = locs[3]
	in := sideIn(ts, locs)
	in.Name = func(id tabs.TabID) string {
		if id == 1 {
			return "parent"
		}
		return ""
	}
	m := buildSide(in)
	if got := buildSpaces(m, 5).Spaces[0].Tabs; !reflect.DeepEqual(got, []tabs.TabID{1, 5, 6, 4}) {
		t.Fatalf("worker space/strip order %v", got)
	}
	r, _ := rowByID(m, 5)
	if r.Detail != "worker of parent" {
		t.Fatalf("worker detail %q", r.Detail)
	}
}

func TestWorkerAttentionAndNesting(t *testing.T) {
	ts, locs := sideFixture()
	w := sideTab(5, "review")
	w.Parent, w.Role = 1, "worker"
	ts = append(ts, w)
	in := sideIn(ts, locs)
	in.Agent = func(id tabs.TabID) agentState {
		if id == 1 {
			return agentState{claude: true, running: true}
		}
		return agentState{}
	}
	for _, first := range []string{"failed", "unconfirmed", "delivered"} {
		in.Worker = func(tabs.TabID) (string, string, bool) { return "codex", first, false }
		m := buildSide(in)
		row, _ := rowByID(m, 5)
		if !row.Worker || row.Parent != 1 || row.Agent != "codex" {
			t.Fatalf("worker metadata %+v", row)
		}
		if first != "delivered" && (row.Mark != markNeeds || row.Detail != "waiting for you in this tab") {
			t.Fatalf("attention %+v", row)
		}
		if len(m.Agents) != 2 || m.Agents[0].ID != 1 || m.Agents[1].ID != 5 || !m.Agents[1].Worker {
			t.Fatalf("nested rows %+v", m.Agents)
		}
	}
}

func TestWorkerFilterKeepsParentContext(t *testing.T) {
	ts, locs := sideFixture()
	w := sideTab(5, "unique-review")
	w.Parent, w.Role = 1, "worker"
	ts = append(ts, w)
	in := sideIn(ts, locs)
	in.Filter = "unique-review"
	in.Agent = func(tabs.TabID) agentState { return agentState{claude: true} }
	m := buildSide(in)
	if len(m.Agents) != 2 || m.Agents[0].ID != 1 || m.Agents[1].ID != 5 {
		t.Fatalf("filtered hierarchy %+v", m.Agents)
	}
}

func TestWorkerCachedPresentationStaysAllocationFree(t *testing.T) {
	win, _, _, parent, _, _ := spacesFixture(t)
	child := openIn(t, win, "")
	child.Role, child.Parent = "worker", parent.ID
	win.workers = map[tabs.TabID]*worker{child.ID: {parent: parent.ID, agent: "codex", firstMessage: "delivered"}}
	g := pageContext(1000, 800)
	g.Now = t0
	draw := func() {
		g.Ops.Reset()
		win.drawSide(g, image.Rect(0, 42, 260, 768))
		win.drawStrip(g, image.Rect(260, 42, 1000, 78))
	}
	for range 3 {
		draw()
	}
	builds, draws := win.side.cache.builds, win.strip.draws
	if n := testing.AllocsPerRun(50, draw); n != 0 {
		t.Fatalf("worker sidebar and strip allocated %g", n)
	}
	if win.side.cache.builds != builds || win.strip.draws != draws {
		t.Fatal("unchanged worker UI rebuilt")
	}
	win.workers[child.ID].firstMessage = "unconfirmed"
	draw()
	row, _ := rowByID(win.side.cache.base, child.ID)
	if row.Mark != markNeeds || row.Detail != "waiting for you in this tab" {
		t.Fatal("worker first-message cache did not refresh")
	}
	if win.side.cache.builds == builds {
		t.Fatal("missing first-message invalidation")
	}
}
