package ui

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/vt"
)

func TestControlSnapshotHooks(t *testing.T) {
	win, _ := sideWindow(t, true)
	tb := openIn(t, win, osPath("/code/repo/api"))
	tb.FixedTitle = "review"
	win.cachedSide(time.Now())
	win.publishControl()
	old := win.control.Tabs()
	if len(old) != 1 || old[0].ID != "t1" || old[0].Name != "review" || old[0].Space != "repo" || old[0].Folder != osPath("/code/repo/api") || old[0].Status != "shell" {
		t.Fatalf("%+v", old)
	}
	at := time.Now()
	for _, tc := range []struct {
		kind   notify.Kind
		status string
	}{
		{notify.KindPrompt, "working"}, {notify.KindDone, "idle"}, {notify.KindNeeds, "needs"},
	} {
		win.focused = false
		win.deliver(queued{m: notify.Msg{Tab: int(tb.ID), Kind: tc.kind, Agent: "codex"}, at: at}, at)
		got := win.control.Tabs()[0]
		if got.Agent != "codex" || got.Status != tc.status {
			t.Fatalf("%+v", got)
		}
		if tc.kind == notify.KindPrompt && !got.Since.Equal(at) {
			t.Fatalf("since %v", got.Since)
		}
	}
	if old[0].Status != "shell" {
		t.Fatal("published snapshot mutated")
	}
	tb.FixedTitle = ""
	win.names = map[tabs.TabID]string{tb.ID: "new name"}
	win.publishControl()
	if win.control.Tabs()[0].Name != "new name" {
		t.Fatal("rename missing")
	}
	member := openMember(t, win, "member")
	win.publishControl()
	if !win.control.Tabs()[1].Workspace {
		t.Fatal("workspace flag missing")
	}
	win.mgr.Close(member.ID)
	win.publishControl()
	if len(win.control.Tabs()) != 1 {
		t.Fatal("closed member retained")
	}
}

func TestControlPublishDoesNotWake(t *testing.T) {
	win, st := sideWindow(t, true)
	openIn(t, win, "")
	var wakes atomic.Int64
	mgr := tabs.NewManager(st.start, 10, 2, func() { wakes.Add(1) })
	defer mgr.CloseAll()
	win.mgr = mgr
	win.w = nil // Any direct Gio invalidation from publish would panic.
	if _, err := mgr.New(); err != nil {
		t.Fatal(err)
	}
	win.publishControl()
	before := wakes.Load()
	for range 10 {
		win.publishControl()
	}
	timer := time.NewTimer(30 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	if got := wakes.Load() - before; got != 0 {
		t.Fatalf("publish requested %d frames", got)
	}
}

func TestControlAdapter(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	win.publishControl()
	r := controlAdapter(&win.control)(json.RawMessage(`{"op":"ls"}`))
	reply, ok := r.(ctl.Reply)
	if !ok || !reply.OK || len(reply.Tabs) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestControlSubmitAndClosed(t *testing.T) {
	win, _ := sideWindow(t, true)
	record := &controlRecordPTY{recPTY: newRecPTY(), writes: make(chan string, 4)}
	win.mgr = tabs.NewManager(func([]string, string) (ptyx.Session, error) { return record, nil }, 80, 24, nil)
	t.Cleanup(win.mgr.CloseAll)
	tb := openIn(t, win, "")
	tb.Session.Lock()
	vt.NewParser(tb.Session.Applier()).Feed([]byte("\x1b[?2004h"))
	tb.Session.Unlock()
	win.publishControl()
	done := make(chan error, 1)
	started := time.Now()
	go func() { done <- win.control.Submit("t1", []byte("first\nsecond")) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("submit timeout")
	}
	if time.Since(started) < controlPasteEnterDelay {
		t.Fatal("Enter did not wait for the no-echo deadline")
	}
	for _, want := range []string{"\x1b[200~first\rsecond\x1b[201~", "\r"} {
		select {
		case got := <-record.writes:
			if got != want {
				t.Fatalf("write %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("write timeout")
		}
	}
	if len(record.writes) != 0 {
		t.Fatal("extra write")
	}
	record.Close()
	deadline := time.Now().Add(time.Second)
	for {
		exited, _ := tb.Session.Exited()
		if exited {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("exit timeout")
		}
		time.Sleep(time.Millisecond)
	}
	if err := win.control.Submit("t1", []byte("exited")); err == nil {
		t.Fatal("exited session accepted input")
	}
	tb.Session.Close()
	if err := win.control.Submit("t1", []byte("closed")); err == nil {
		t.Fatal("closed session accepted input")
	}
	win.mgr.Close(tb.ID)
	win.publishControl()
	if err := win.control.Submit("t1", []byte("removed")); err == nil {
		t.Fatal("removed tab accepted input")
	}
}

func TestControlScreen(t *testing.T) {
	win, _ := sideWindow(t, true)
	tb := openIn(t, win, "")
	tb.Session.Lock()
	p := vt.NewParser(tb.Session.Applier())
	for range ctl.MaxLines + 50 {
		p.Feed([]byte("row\r\n"))
	}
	p.Feed([]byte("last"))
	tb.Session.Unlock()
	win.publishControl()
	text, err := win.control.Screen("t1", ctl.MaxLines+50)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(text, "\n"); got != ctl.MaxLines+23 {
		t.Fatalf("newlines %d", got)
	}
	if _, err := win.control.Screen("missing", 0); err == nil {
		t.Fatal("missing screen accepted")
	}
}

type controlRecordPTY struct {
	*recPTY
	writes chan string
}

func (p *controlRecordPTY) Write(b []byte) (int, error) {
	select {
	case p.writes <- string(b):
	default:
	}
	return p.recPTY.Write(b)
}

// Publishing cannot start location discovery, whose completion requests a frame.
func TestControlPublishDoesNotDiscoverLocation(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo/api"))
	win.repoFn = func(string) (string, string) { t.Fatal("publish discovered repo"); return "", "" }
	win.branchFn = func(string) string { t.Fatal("publish discovered branch"); return "" }
	win.publishControl()
	if got := win.control.Tabs()[0].Folder; got != osPath("/code/repo/api") {
		t.Fatalf("folder %q", got)
	}
}

// A tab whose agent is only found by looking at the program in front of it (a
// Codex whose hooks never reached Kaze) shows the Codex logo in the strip, so
// peers must see it as codex too.
func TestControlSnapshotFollowsDetectedAgent(t *testing.T) {
	win, _ := sideWindow(t, true)
	tb := openIn(t, win, osPath("/code/repo/api"))
	win.cachedSide(time.Now())
	win.publishControl()
	if got := win.control.Tabs()[0]; got.Agent != "" || got.Status != "shell" {
		t.Fatalf("before detection: %+v", got)
	}
	win.probe = agentProbe{
		fg:   func(*tabs.Tab) int { return 4242 },
		out:  func(*tabs.Tab) uint64 { return 7 },
		name: func(int) string { return "codex" },
	}
	win.detectAgents(time.Now())
	if win.detected[tb.ID].agent != "codex" {
		t.Fatalf("scene: detection %+v", win.detected[tb.ID])
	}
	win.cachedSide(time.Now()) // the frame refreshes the models after detection
	if got := win.control.Tabs()[0]; got.Agent != "codex" {
		t.Fatalf("peers still see %+v after the program was detected", got)
	}
}
