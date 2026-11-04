package ui

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/app"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Twenty frames of the 50-agent board inside one poll window read no
// transcript and parse no markdown again: a still board does no new work.
func TestBoardFrame50Steady(t *testing.T) {
	win := boardBenchWindow(t)
	keys := boardFixture50(t, win)
	win.openBoard(0)
	win.boardSelect(keys[boardFixtureTranscript])
	start := time.Now()
	gtx := pageContext(1400, 860)
	frame := func(at time.Duration) {
		gtx.Ops.Reset()
		gtx.Now = start.Add(at)
		win.drawBoard(gtx, image.Rect(0, 0, 1400, 860))
	}
	for i := range 5 {
		frame(time.Duration(i) * time.Millisecond)
	}
	if !boardHas(win, "50 agents in 5 sessions") {
		t.Fatalf("fixture: texts %q", win.board.texts)
	}
	p := &win.board.prev
	if p.tr == nil || !p.tr.Available || p.cache.parses == 0 {
		t.Fatalf("the preview did not read and lay out the transcript (tr %v, parses %d)", p.tr, p.cache.parses)
	}
	parses, at := p.cache.parses, p.trAt
	for i := range 20 {
		frame(time.Duration(10+i) * time.Millisecond)
	}
	if p.cache.parses != parses || !p.trAt.Equal(at) {
		t.Fatalf("still frames: parses %d -> %d, transcript read at %v -> %v", parses, p.cache.parses, at, p.trAt)
	}
	n := 0
	allocs := testing.AllocsPerRun(20, func() { frame(time.Duration(40+n) * time.Millisecond); n++ })
	t.Logf("50 agents, wide board with a running preview: %.0f allocs per still frame", allocs)
	if allocs > 400 {
		t.Fatalf("%.0f allocs per still frame, want at most 400", allocs)
	}
	// A search that keeps every row costs about what no search does: matching
	// allocates nothing per row.
	win.board.ed.SetText("E")
	frame(time.Duration(70) * time.Millisecond)
	if !boardHas(win, "50 agents in 5 sessions") {
		t.Fatalf("the search hid rows: texts %q", win.board.texts)
	}
	searched := testing.AllocsPerRun(20, func() { frame(time.Duration(80+n) * time.Millisecond); n++ })
	t.Logf("with a search: %.0f allocs per still frame", searched)
	if searched > allocs+10 {
		t.Fatalf("a search adds %.0f allocs per frame (%.0f, %.0f without)", searched-allocs, searched, allocs)
	}
}

// boardFixtureTranscript is the index in boardFixture50's keys of the running
// agent with a 20-message transcript.
const boardFixtureTranscript = 1

// boardBenchWindow is a window with five sessions and what onNotify reads
// published; it takes a testing.TB so benchmarks can use it.
func boardBenchWindow(tb testing.TB) *window {
	tb.Helper()
	st := &recStarter{}
	km, _ := buildKeymap("windows", nil)
	binds, _ := resolveBindings("windows", nil)
	win := &window{w: new(app.Window), km: km, binds: binds, sels: map[tabs.TabID]*tabSel{}}
	win.mgr = tabs.NewManager(st.start, 80, 24, nil)
	win.lc.fn = func(context.Context, string) (int, int, error) { return 0, 0, nil } // never run real git
	tb.Cleanup(win.mgr.CloseAll)
	win.ui = newChrome()
	win.focused = true
	for range 5 {
		if _, err := win.mgr.New(); err != nil {
			tb.Fatal(err)
		}
	}
	win.mgr.SetActive(win.mgr.Tabs()[0].ID)
	win.chime = func(time.Time) {}
	win.flash = func() {}
	win.liveMgr.Store(win.mgr)
	win.publish(win.mgr.Active(), true)
	return win
}

// boardFixture50 starts 50 subagents in win's first five sessions, ten each:
// one waiting, six running and three done. keys[boardFixtureTranscript] is a
// running agent with a 20-message transcript.
func boardFixture50(tb testing.TB, win *window) []subagents.Key {
	tb.Helper()
	tasks := []string{"Find the auth middleware", "Map the checkout flow", "Read the tenant scoping", "Draft the refund tests",
		"Trace webhook retries", "Check 3D Secure helper", "List the cache keys", "Audit the session store"}
	types := []string{"Explore", "general-purpose", "Plan"}
	main := filepath.Join(tb.TempDir(), "s.jsonl")
	var keys []subagents.Key
	for s, t := range win.mgr.Tabs()[:5] {
		session := fmt.Sprintf("s%d", s)
		send := func(m notify.Msg) { m.Tab, m.Session = int(t.ID), session; win.onNotify(m) }
		for i := range 10 {
			id := fmt.Sprintf("a%d", i)
			k := subagents.Key{Session: session, ID: id}
			tr := ""
			if len(keys) == boardFixtureTranscript {
				tr = main
				var b strings.Builder
				for j := range 20 {
					b.WriteString(fixtureAssistant(fmt.Sprintf("Step %d: looking at **panes.go** and `board.go`, then the sidebar.", j+1)))
				}
				path := subagents.TranscriptPath(main, id)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					tb.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
					tb.Fatal(err)
				}
			}
			task := tasks[(s*10+i)%len(tasks)]
			send(notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: types[i%len(types)], Description: task})
			send(notify.Msg{Kind: notify.KindAgentStart, AgentID: id, AgentType: types[i%len(types)], Transcript: tr})
			send(notify.Msg{Kind: notify.KindTool, AgentID: id, AgentType: types[i%len(types)], Tool: "Grep", Summary: "\"tenantID\" internal/store"})
			switch {
			case i == 0:
				send(notify.Msg{Kind: notify.KindNeeds, AgentID: id})
			case i >= 7:
				send(notify.Msg{Kind: notify.KindAgentStop, AgentID: id, Report: "Found it in internal/store.\n\nMore detail."})
			}
			keys = append(keys, k)
		}
	}
	win.drainSubs(time.Now())
	return keys
}

// One wide board frame with 50 agents and a running agent's preview.
func BenchmarkBoardFrame50(b *testing.B) {
	win := boardBenchWindow(b)
	keys := boardFixture50(b, win)
	win.openBoard(0)
	win.boardSelect(keys[boardFixtureTranscript])
	gtx := pageContext(1400, 860)
	gtx.Now = time.Now()
	r := image.Rect(0, 0, 1400, 860)
	win.drawBoard(gtx, r) // the first frame reads the transcript and lays it out
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		gtx.Ops.Reset()
		win.drawBoard(gtx, r)
	}
}
