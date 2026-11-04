package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func rowIDs(rs []boardRow) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.key.ID)
	}
	return out
}

func cardTabs(v boardView) []tabs.TabID {
	out := []tabs.TabID{}
	for _, c := range v.cards {
		out = append(out, c.tab)
	}
	return out
}

func TestBoardViewNeedsCardAndCards(t *testing.T) {
	win, id := boardWindow(t)
	title, _, _ := win.sessionMeta(id)
	v := win.boardView()
	if len(v.needs) != 1 || v.needs[0].key.ID != "wait" || !strings.HasPrefix(v.needs[0].step, title+" · ") {
		t.Fatalf("needs %+v (title %q)", v.needs, title)
	}
	if len(v.cards) != 1 {
		t.Fatalf("%d cards", len(v.cards))
	}
	c := v.cards[0]
	if c.tab != id || strings.Join(rowIDs(c.rows), ",") != "run" || c.done != 1 || !c.folded {
		t.Fatalf("card %+v", c)
	}
	if c.meter != (subagents.TrayCounts{Waiting: 1, Running: 1, Done: 1}) {
		t.Fatalf("meter %+v", c.meter)
	}
	if strings.Join(rowIDs(v.order), ",") != "wait,run" {
		t.Fatalf("order %v", rowIDs(v.order))
	}
}

func TestBoardViewSidebarOrder(t *testing.T) {
	win, id := boardWindow(t)
	other := win.mgr.Tabs()[0].ID
	startAgent(win, other, "o", "x")
	win.drainSubs(t0)
	v := win.boardView()
	want := []tabs.TabID{}
	for _, s := range win.sessionIDs() {
		if s == id || s == other {
			want = append(want, s)
		}
	}
	if !slices.Equal(cardTabs(v), want) || len(want) != 2 {
		t.Fatalf("cards %v, sidebar %v", cardTabs(v), want)
	}
}

func TestBoardViewFilters(t *testing.T) {
	win, _ := boardWindow(t)
	for _, tc := range []struct {
		f    boardFilter
		want string
	}{{boardNeeds, "wait"}, {boardRunning, "run"}, {boardDone, "done"}} {
		win.board.filter = tc.f
		v := win.boardView()
		if len(v.needs) != 0 || len(v.cards) != 1 || strings.Join(rowIDs(v.cards[0].rows), ",") != tc.want {
			t.Fatalf("filter %d: needs %v cards %+v", tc.f, rowIDs(v.needs), v.cards)
		}
		if tc.f == boardDone && v.cards[0].folded {
			t.Fatal("Done filter folded its rows")
		}
	}
}

func TestBoardViewSearch(t *testing.T) {
	win, id := boardWindow(t)
	title, _, _ := win.sessionMeta(id)
	all := func(v boardView) []string {
		ids := rowIDs(v.needs)
		for _, c := range v.cards {
			ids = append(ids, rowIDs(c.rows)...)
			if c.folded {
				ids = append(ids, "folded")
			}
		}
		return ids
	}
	for _, tc := range []struct{ q, want string }{
		{"PLAN", "wait"},
		{strings.ToUpper(title), "wait,run,folded"},
		{"zzz", ""},
	} {
		win.board.ed.SetText(tc.q)
		v := win.boardView()
		if strings.Join(all(v), ",") != tc.want {
			t.Fatalf("search %q: %v", tc.q, all(v))
		}
		if v.counts != [4]int{3, 1, 1, 1} {
			t.Fatalf("search %q counts %v", tc.q, v.counts)
		}
	}
	if !boardMatch(boardRow{typ: "Explore"}, "s", "kaze-terminal", "terminal") || boardMatch(boardRow{typ: "Explore"}, "s", "kaze", "terminal") {
		t.Fatal("repo match")
	}
	if !boardMatch(boardRow{task: "Split Pane keys"}, "", "", "pane") || !boardMatch(boardRow{}, "", "", "") {
		t.Fatal("task or empty match")
	}
}

func TestBoardViewSessionToken(t *testing.T) {
	win, id := boardWindow(t)
	other := win.mgr.Tabs()[0].ID
	startAgent(win, other, "o", "x")
	win.drainSubs(t0)
	win.board.tab = id
	v := win.boardView()
	if len(v.cards) != 1 || v.cards[0].tab != id {
		t.Fatalf("cards %v", cardTabs(v))
	}
	if v.counts[boardAll] != 3 || v.agents != 4 || v.sessions != 2 {
		t.Fatalf("counts %v agents %d sessions %d", v.counts, v.agents, v.sessions)
	}
}

func TestBoardViewShowDone(t *testing.T) {
	win, id := boardWindow(t)
	win.board.showDone = map[tabs.TabID]bool{id: true}
	v := win.boardView()
	c := v.cards[0]
	if strings.Join(rowIDs(c.rows), ",") != "run,done" || c.folded {
		t.Fatalf("card %+v", c)
	}
	if o := rowIDs(v.order); o[len(o)-1] != "done" {
		t.Fatalf("order %v", o)
	}
}

func TestBoardViewSessionWait(t *testing.T) {
	win, id := boardWindow(t)
	win.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds, Session: "s"})
	win.drainSubs(t0)
	v := win.boardView()
	i := slices.IndexFunc(v.needs, func(r boardRow) bool { return r.session })
	if i < 0 || v.needs[i].typ != "Session" || v.needs[i].tab != id {
		t.Fatalf("needs %+v", v.needs)
	}
	if slices.ContainsFunc(v.order, func(r boardRow) bool { return r.session }) {
		t.Fatal("a session wait is selectable")
	}
}

func TestBoardSummary(t *testing.T) {
	if boardSummary(11, 3) != "11 agents in 3 sessions" || boardSummary(1, 1) != "1 agent in 1 session" || boardSummary(0, 0) != "" {
		t.Fatal(boardSummary(11, 3), boardSummary(1, 1), boardSummary(0, 0))
	}
}

func TestMeterSplit(t *testing.T) {
	n, r, d := meterSplit(subagents.TrayCounts{Waiting: 1, Running: 2, Done: 1}, 100, 2)
	if n+r+d+4 != 100 || r < 2*n-1 || r > 2*n+1 {
		t.Fatalf("{1,2,1}: %d %d %d", n, r, d)
	}
	if n, r, d := meterSplit(subagents.TrayCounts{Running: 5}, 100, 2); n != 0 || r != 100 || d != 0 {
		t.Fatalf("{0,5,0}: %d %d %d", n, r, d)
	}
	if n, r, d := meterSplit(subagents.TrayCounts{Waiting: 1, Running: 1000}, 100, 2); n < 1 || n+r+d+2 != 100 {
		t.Fatalf("{1,1000,0}: %d %d %d", n, r, d)
	}
	if n, r, d := meterSplit(subagents.TrayCounts{}, 100, 2); n+r+d != 0 {
		t.Fatalf("empty: %d %d %d", n, r, d)
	}
}

// Matching a row against a search allocates nothing, whatever the case.
func TestBoardMatchNoAllocs(t *testing.T) {
	r := boardRow{typ: "General-Purpose", task: "Trace Webhook RETRIES"}
	for _, tc := range []struct {
		q    string
		want bool
	}{{"retries", true}, {"kaze", true}, {"webhook r", true}, {"zzz", false}, {"élan", true}, {"ÉLAN", false}} {
		if got := boardMatch(r, "Kaze Terminal", "Élan", tc.q); got != tc.want {
			t.Errorf("%q: %v, want %v", tc.q, got, tc.want)
		}
	}
	if n := testing.AllocsPerRun(50, func() { boardMatch(r, "Kaze Terminal", "Élan", "retries") }); n != 0 {
		t.Fatalf("%.0f allocs per match", n)
	}
}
