package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

// pairWorld is a window with one agent tab, a main transcript path and a
// counter on every prompt read.
type pairWorld struct {
	win   *window
	tab   int
	main  string
	reads *int
}

func newPairWorld(t *testing.T) pairWorld {
	t.Helper()
	win, _, _ := liveWindow(t, 2)
	reads := 0
	orig := firstPrompt
	t.Cleanup(func() { firstPrompt = orig })
	firstPrompt = func(path string) (string, bool) {
		reads++
		return orig(path)
	}
	return pairWorld{win: win, tab: int(win.mgr.Tabs()[1].ID), main: filepath.Join(t.TempDir(), "s.jsonl"), reads: &reads}
}

func (w pairWorld) send(engine string, ms ...notify.Msg) {
	for _, m := range ms {
		m.Tab, m.Session, m.Agent, m.Transcript = w.tab, "s1", engine, w.main
		w.win.onNotify(m)
	}
	w.win.drainSubs(t0)
}

func (w pairWorld) call(desc, typ, prompt string) notify.Msg {
	return notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: typ, Description: desc, TaskPrompt: prompt}
}

func (w pairWorld) start(id, typ string) notify.Msg {
	return notify.Msg{Kind: notify.KindAgentStart, AgentID: id, AgentType: typ}
}

func (w pairWorld) step(id, typ string) notify.Msg {
	return notify.Msg{Kind: notify.KindTool, AgentID: id, AgentType: typ, Tool: "Read", Summary: "x.go"}
}

// transcript writes agent id's transcript starting with prompt.
func (w pairWorld) transcript(t *testing.T, id, prompt string) {
	t.Helper()
	p := subagents.TranscriptPath(w.main, id)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": prompt}})
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (w pairWorld) task(id string) string {
	if a := w.win.subs.Get(subagents.Key{Session: "s1", ID: id}); a != nil {
		return a.Task
	}
	return "<none>"
}

const gp = "general-purpose"

var lanes = [...][3]string{{"d1", "Lane A", "Build lane A"}, {"d2", "Lane D", "Build lane D"}, {"d3", "Lane L", "Build lane L"}}

// Issue #135: three agents of one type, started in one message, fire their
// starts in reverse; each still ends up with its own name.
func TestThreeAgentsStartedTogetherKeepTheirNames(t *testing.T) {
	w := newPairWorld(t)
	for _, l := range lanes {
		w.send("claude", w.call(l[1], gp, l[2]))
	}
	for i := len(lanes) - 1; i >= 0; i-- {
		w.send("claude", w.start(lanes[i][0], gp))
	}
	for _, l := range lanes {
		w.transcript(t, l[0], l[2])
	}
	for _, l := range lanes {
		w.send("claude", w.step(l[0], gp))
	}
	for _, l := range lanes {
		if got := w.task(l[0]); got != l[1] {
			t.Fatalf("%s is %q, want %q", l[0], got, l[1])
		}
	}
	rows := map[string]string{}
	for _, r := range w.win.boardView().order {
		rows[r.key.ID] = r.task
	}
	for _, l := range lanes {
		if rows[l[0]] != l[1] {
			t.Fatalf("board row %s reads %q, want %q", l[0], rows[l[0]], l[1])
		}
	}
	if *w.reads != len(lanes) {
		t.Fatalf("%d prompt reads, want %d", *w.reads, len(lanes))
	}
}

func TestNoTranscriptFallsBackWithoutStealing(t *testing.T) {
	w := newPairWorld(t)
	for _, l := range lanes {
		w.send("claude", w.call(l[1], gp, l[2]))
	}
	// d1 starts first and takes Lane A by order, which is right; d3 and d2
	// start reversed, so by order d3 has Lane D and d2 has Lane L.
	w.send("claude", w.start("d1", gp), w.start("d3", gp), w.start("d2", gp))
	w.transcript(t, "d2", lanes[1][2])
	w.transcript(t, "d3", lanes[2][2]) // d1 never writes a transcript
	for i := 0; i < 3; i++ {
		w.send("claude", w.step("d1", gp))
	}
	if got := w.task("d1"); got != "Lane A" {
		t.Fatalf("d1 lost its order pairing: %q", got)
	}
	w.send("claude", w.step("d2", gp), w.step("d3", gp))
	for _, l := range lanes {
		if got := w.task(l[0]); got != l[1] {
			t.Fatalf("%s is %q, want %q", l[0], got, l[1])
		}
	}
	w.send("claude", w.step("d1", gp)) // given up: no fourth read
	if *w.reads != 3+2 {               // three misses for d1, one read each for d2 and d3
		t.Fatalf("%d prompt reads, want 5", *w.reads)
	}
}

func TestSingleCallOpensNoFile(t *testing.T) {
	w := newPairWorld(t)
	w.send("claude", w.call("Find auth", "Explore", "Find the auth code."), w.start("a1", "Explore"))
	for i := 0; i < 5; i++ {
		w.send("claude", w.step("a1", "Explore"))
	}
	if *w.reads != 0 || w.task("a1") != "Find auth" {
		t.Fatalf("%d reads, task %q", *w.reads, w.task("a1"))
	}
}

func TestCodexHelpersNeverRead(t *testing.T) {
	w := newPairWorld(t)
	w.send("codex", w.call("one", "explorer", "p1"), w.call("two", "explorer", "p2"))
	w.send("codex", w.start("h1", "explorer"), w.start("h2", "explorer"))
	for i := 0; i < 5; i++ {
		w.send("codex", w.step("h1", "explorer"), w.step("h2", "explorer"))
	}
	if *w.reads != 0 || w.task("h1") != "one" || w.task("h2") != "two" {
		t.Fatalf("%d reads, tasks %q %q", *w.reads, w.task("h1"), w.task("h2"))
	}
}

func TestSettleReadsAtMostThreeTimes(t *testing.T) {
	w := newPairWorld(t)
	w.send("claude", w.call("one", gp, "p1"), w.call("two", gp, "p2"), w.start("a1", gp))
	p := subagents.TranscriptPath(w.main, "a1")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		w.send("claude", w.step("a1", gp))
	}
	if *w.reads != 3 || w.task("a1") != "one" {
		t.Fatalf("%d reads, task %q", *w.reads, w.task("a1"))
	}
}
