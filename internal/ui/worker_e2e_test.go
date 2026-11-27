package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/config"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/sessionstate"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// The wrapper observes wait subscriptions without changing production decisions.
// This gives the test a bounded handshake before a report or hook arrives.
type workerE2EHost struct {
	*controlHost
	observed chan struct{}
}

func (h *workerE2EHost) StatusChanges() ([]ctl.TabInfo, <-chan struct{}) {
	infos, changed := h.controlHost.StatusChanges()
	select {
	case h.observed <- struct{}{}:
	default:
	}
	return infos, changed
}

// All PTYs are in memory. The JSON adapter is the production control entry
// point; only the test thread drains UI work. No native window receives input.
func TestWorkerEndToEnd(t *testing.T) {
	win, _ := workerWindow(t)
	win.mgr.CloseAll()
	var ptys []*recPTY
	var ownerKey string
	win.mgr = tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) {
		p := newRecPTY()
		ptys = append(ptys, p)
		for _, e := range env {
			if strings.HasPrefix(e, "KAZE_TAB_KEY=") {
				if len(ptys) != 1 {
					t.Error("worker received owner key")
				}
				ownerKey = strings.TrimPrefix(e, "KAZE_TAB_KEY=")
			}
		}
		return p, nil
	}, 80, 24, func() {})
	t.Cleanup(win.mgr.CloseAll)
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	if _, err := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir}); err != nil {
		t.Fatal(err)
	}
	win.publishControl()
	if ownerKey == "" || win.control.OwnerKey("t1") != ownerKey {
		t.Fatal("manager key missing from host")
	}
	now := time.Now()
	win.wsr.now = func() time.Time { return now }
	deadline := time.Now().Add(8 * time.Second)
	wake := make(chan struct{}, 32)
	win.control.wake = func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	host := &workerE2EHost{controlHost: &win.control, observed: make(chan struct{}, 8)}
	adapter := controlAdapter(host)
	call := func(req ctl.Request) ctl.Reply {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		return adapter(b).(ctl.Reply)
	}
	finish := func(done <-chan ctl.Reply) ctl.Reply {
		t.Helper()
		select {
		case r := <-done:
			return r
		case <-time.After(time.Until(deadline)):
			t.Fatal("round trip deadline")
			return ctl.Reply{}
		}
	}
	begin := func(req ctl.Request) <-chan ctl.Reply {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan ctl.Reply, 1)
		go func() { done <- adapter(b).(ctl.Reply) }()
		return done
	}
	queued := func(req ctl.Request, consent *bool) ctl.Reply {
		t.Helper()
		for {
			select {
			case <-wake:
				continue
			default:
			}
			break
		}
		done := begin(req)
		// An inbox callback can also wake the frame. Drain only after this
		// request has actually entered the queue, or accept an early refusal.
		for {
			select {
			case <-wake:
				win.control.spawnMu.Lock()
				pending := len(win.control.workerRequests) != 0
				win.control.spawnMu.Unlock()
				if !pending {
					continue
				}
			case r := <-done:
				return r
			case <-time.After(time.Until(deadline)):
				t.Fatal("UI queue deadline")
			}
			break
		}
		win.drainControl()
		if consent != nil {
			c := win.consent.pending[1]
			if c == nil || !strings.Contains(c.question, "Allow workers from t1") {
				t.Fatal("consent banner missing")
			}
			select {
			case r := <-done:
				t.Fatalf("replied before consent: %+v", r)
			default:
			}
			win.answerWorkerConsent(1, *consent)
		}
		return finish(done)
	}
	ok := func(r ctl.Reply) {
		t.Helper()
		if !r.OK {
			t.Fatalf("%+v", r)
		}
	}
	code := func(r ctl.Reply, want string) {
		t.Helper()
		if r.Error == nil || r.Error.Code != want {
			t.Fatalf("reply %+v, want %s", r, want)
		}
	}
	spawn := ctl.Request{Op: "spawn", From: "t1", CallerKey: ownerKey, Agent: "claude", Task: "Review this bounded fixture", Name: "review"}
	win.orchestratorEnabled = config.Default().Orchestrator.Enabled
	code(queued(spawn, nil), ctl.CodeSpawnDisabled)
	win.orchestratorEnabled = true
	ok(queued(spawn, nil))
	spawn.Agent, spawn.Name = "codex", "tests"
	ok(queued(spawn, nil))
	if win.mgr.Active().ID != 1 {
		t.Fatal("background spawn stole active tab")
	}
	infos := call(ctl.Request{Op: "ls", From: "t1"}).Tabs
	if len(infos) != 3 {
		t.Fatalf("children: %+v", infos)
	}
	for _, info := range infos[1:] {
		if info.Parent != "t1" || info.Role != "worker" || info.FirstMessage != "pending" {
			t.Fatalf("spawn snapshot: %+v", info)
		}
	}
	hook := func(id tabs.TabID, agent string, kind notify.Kind) {
		win.noteAgent(notify.Msg{Kind: kind, Agent: agent, Tab: int(id)}, id, 0)
	}
	hook(1, "claude", notify.KindSession)
	hook(2, "claude", notify.KindPrompt)
	hook(3, "codex", notify.KindPrompt)
	model := win.cachedSide(now)
	for _, id := range []tabs.TabID{2, 3} {
		row, found := rowByID(model, id)
		if !found || !row.Worker || row.Parent != 1 {
			t.Fatalf("sidebar worker: %+v", row)
		}
	}
	if len(model.Agents) != 3 || model.Agents[0].ID != 1 || model.Agents[1].ID != 2 || model.Agents[2].ID != 3 {
		t.Fatalf("sidebar nesting: %+v", model.Agents)
	}
	spaces := buildSpaces(model, 1)
	if len(spaces.Spaces) != 1 || len(spaces.Spaces[0].Tabs) != 3 || spaces.Spaces[0].Tabs[0] != 1 {
		t.Fatalf("parent space: %+v", spaces)
	}
	for _, info := range win.control.Tabs()[1:] {
		if info.FirstMessage != "delivered" {
			t.Fatal(info)
		}
	}
	wait := ctl.Request{Op: "wait", From: "t1", CallerKey: ownerKey, Children: "all", Timeout: 2}
	// A report is delivered by the real Submit to the fake parent PTY.
	reportText := `<kaze from="t2 review" kind="report">review complete</kaze>`
	ok(call(ctl.Request{Op: "report", From: "t2", Text: "review complete"}))
	win.noteTick(now, win.control.Submit)
	waitControlCondition(t, func() bool { return ptys[0].sent() == reportText+"\r" })
	waitControlCondition(t, func() bool { return !win.control.snapshot.Load().sessions["t1"].noteSending.Load() })
	r := call(wait)
	if !r.OK || r.Reason != "report" || r.Text != reportText {
		t.Fatalf("report wait: %+v", r)
	}
	// Report also interrupts an already subscribed wait while both workers work.
	startWait := func() <-chan ctl.Reply {
		t.Helper()
		for {
			select {
			case <-host.observed:
				continue
			default:
			}
			break
		}
		done := begin(wait)
		select {
		case <-host.observed:
		case <-time.After(time.Until(deadline)):
			t.Fatal("wait did not subscribe")
		}
		select {
		case r := <-done:
			t.Fatalf("wait returned before event: %+v", r)
		case <-time.After(10 * time.Millisecond):
		}
		return done
	}
	done := startWait()
	ok(call(ctl.Request{Op: "report", From: "t3", Text: "test result"}))
	if r := finish(done); !r.OK || r.Reason != "report" {
		t.Fatalf("early report: %+v", r)
	}
	// Submit uses wall time for its pending overlay. Advance the injected UI
	// clock beyond that grace before testing the independent owner quiet rule.
	now = now.Add(controlPendingGrace + time.Second)
	win.control.recordOwnerKey(1, now)
	hook(2, "claude", notify.KindNeeds)
	sent := ptys[0].sent()
	if at, pending := win.noteTick(now, win.control.Submit); !pending || !at.Equal(now.Add(5*time.Second)) {
		t.Fatal("quiet rule", at, pending)
	}
	if at, pending := win.noteTick(now.Add(5*time.Second-time.Nanosecond), win.control.Submit); !pending || !at.Equal(now.Add(5*time.Second)) {
		t.Fatal("quiet rule ended early", at, pending)
	}
	if at, ready := win.noteTick(now.Add(5*time.Second), nil); !ready || !at.Equal(now.Add(5*time.Second)) {
		t.Fatal("note was not ready after quiet deadline", at, ready)
	}
	if ptys[0].sent() != sent {
		t.Fatal("note typed during owner quiet period")
	}
	if r := call(wait); !r.OK || r.Reason != "note" || !strings.Contains(r.Text, "approval prompt") {
		t.Fatalf("unread note: %+v", r)
	}
	for _, req := range []ctl.Request{{Op: "keys", Tab: "t2", Args: []string{"enter"}}, {Op: "send", Tab: "t2", Text: "approve", From: "t1"}} {
		code(call(req), ctl.CodeNotAllowed)
	}
	if ptys[1].sent() != "" {
		t.Fatal("approval guard wrote to worker PTY")
	}
	code(call(ctl.Request{Op: "spawn", From: "t2", Agent: "claude", Task: "nested"}), ctl.CodeNotAllowed)
	hook(2, "claude", notify.KindPrompt)
	hook(2, "claude", notify.KindDone)
	if r := call(wait); !r.OK || r.Reason != "note" {
		t.Fatalf("first child done: %+v", r)
	}
	// One idle child must not satisfy all. The second completion first returns
	// its note, then a fresh wait returns all_stopped after both are idle.
	done = startWait()
	hook(3, "codex", notify.KindDone)
	if r := finish(done); !r.OK || r.Reason != "note" {
		t.Fatalf("done notes: %+v", r)
	}
	if r := call(wait); !r.OK || r.Reason != "all_stopped" || len(r.Tabs) != 2 {
		t.Fatalf("all stopped: %+v", r)
	}
	path := filepath.Join(t.TempDir(), "sessions.json")
	var err error
	win.restore, err = sessionstate.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { win.mgr.SetStateObserver(nil); win.restore.Close() })
	win.trackRestore()
	win.captureRestore()
	if err := win.restore.Flush(); err != nil {
		t.Fatal(err)
	}
	saved, err := sessionstate.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Entries) != 1 {
		t.Fatalf("saved workers: %+v", saved)
	}
	code(queued(ctl.Request{Op: "kill", From: "t1", CallerKey: ownerKey, Tab: "t1"}, nil), ctl.CodeNotAllowed)
	ok(queued(ctl.Request{Op: "kill", From: "t1", CallerKey: ownerKey, Tab: "t2"}, nil))
	if win.mgr.Has(2) || !win.mgr.Has(3) {
		t.Fatal("kill affected wrong worker")
	}
	win.opt.Config.Orchestrator.Confirm = "first"
	spawn.Agent, spawn.Name = "claude", "denied"
	deny := false
	code(queued(spawn, &deny), "consent_denied")
	// Denial does not grant consent; the next request still needs its own banner.
	spawn.Name = "allowed"
	allow := true
	ok(queued(spawn, &allow))
	win.closeTab(1)
	if len(win.mgr.Tabs()) != 0 || len(win.workers) != 0 {
		t.Fatal("parent close left workers")
	}
}

func TestWorkerREADMEContract(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	for _, phrase := range []string{"## Agents starting agents (workers)", "Let agents start workers", "wait --children", "native claude.exe", "consent_denied"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("README missing %q", phrase)
		}
	}
}
