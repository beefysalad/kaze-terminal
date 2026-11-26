package ui

import (
	"context"
	"fmt"
	"github.com/beefysalad/kaze-terminal/internal/sessionstate"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func workerWindow(t *testing.T) (*window, *recStarter) {
	t.Helper()
	win, st := sideWindow(t, true)
	win.mgr.CloseAll()
	win.mgr = tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) { return st.start(env, dir) }, 10, 2, func() {})
	t.Cleanup(win.mgr.CloseAll)
	win.opt.Shell = []string{"/bin/zsh"}
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "claude.exe"), nil, 0600)
		win.opt.ShellEnv = func(env []string) []string { return append(env, "PATH="+dir, "PATHEXT=.EXE", "KAZE_CODEX_SHIM="+dir) }
	}
	win.opt.Dir = t.TempDir()
	win.orchestratorEnabled = true
	_, err := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
	if err != nil {
		t.Fatal(err)
	}
	win.publishControl()
	return win, st
}
func TestWorkerSpawnState(t *testing.T) {
	win, _ := workerWindow(t)
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	win.wsr.now = func() time.Time { return now }
	active := win.mgr.Active().ID
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review", Name: "review"})
	if !r.OK || !strings.Contains(r.Text, "started t2") {
		t.Fatalf("%+v", r)
	}
	if win.mgr.Active().ID != active {
		t.Fatal("stole active tab")
	}
	old := win.control.Tabs()
	if old[1].Parent != "t1" || old[1].Role != "worker" || old[1].FirstMessage != "pending" || old[1].Status != "starting" {
		t.Fatalf("%+v", old)
	}
	if at, ok := win.workerTick(now); !ok || !at.Equal(now.Add(30*time.Second)) {
		t.Fatal(at, ok)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 2, 0)
	if at, ok := win.workerTick(now); !ok || !at.Equal(now.Add(20*time.Second)) {
		t.Fatal(at, ok)
	}
	now = now.Add(20 * time.Second)
	win.workerTick(now)
	if win.workers[2].firstMessage != "unconfirmed" {
		t.Fatal(win.workers[2])
	}
	if _, ok := win.workerTick(now); ok {
		t.Fatal("idle wake")
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	if old[1].FirstMessage != "pending" {
		t.Fatal("mutated old snapshot")
	}
}
func TestWorkerDisabledAndOwnership(t *testing.T) {
	win, _ := workerWindow(t)
	win.orchestratorEnabled = false
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if r.Error == nil || r.Error.Code != ctl.CodeSpawnDisabled {
		t.Fatalf("%+v", r)
	}
	win.orchestratorEnabled = true
	r = win.applyWorker(ctl.Request{Op: "kill", From: "t1", Tab: "t1"})
	if r.Error == nil || r.Error.Code != ctl.CodeNotAllowed {
		t.Fatalf("%+v", r)
	}
}
func TestWorkerQueueFullAndCancellation(t *testing.T) {
	h := &controlHost{}
	for range 8 {
		h.workerRequests = append(h.workerRequests, &controlWorkerRequest{})
	}
	if r := h.Spawn(context.Background(), ctl.Request{}); r.Error == nil || r.Error.Code != ctl.CodeBusy {
		t.Fatalf("%+v", r)
	}
	h.workerRequests = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := h.Spawn(ctx, ctl.Request{}); r.Error == nil || r.Error.Code != "timeout" {
		t.Fatalf("%+v", r)
	}
	if len(h.workerRequests) != 0 {
		t.Fatal("retained canceled request")
	}
}

func TestWorkerParentEnvironment(t *testing.T) {
	win, _ := workerWindow(t)
	var argvGot, envGot []string
	win.mgr.CloseAll()
	win.mgr = tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) {
		argvGot = append([]string(nil), argv...)
		envGot = append([]string(nil), env...)
		return newRecPTY(), nil
	}, 10, 2, func() {})
	t.Cleanup(win.mgr.CloseAll)
	win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
	win.publishControl()
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "line 1\nline 2"})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	if len(argvGot) == 0 {
		t.Fatal("no explicit argv")
	}
	hasParent, hasBrief := false, false
	for _, e := range envGot {
		if e == "KAZE_PARENT=t1" {
			hasParent = true
		}
		if strings.HasPrefix(e, "KAZE_TAB_KEY=") {
			t.Fatal("worker received owner key")
		}
		if strings.HasPrefix(e, "KAZE_TASK=") && strings.Contains(e, "Your tab is t2") && strings.Contains(e, "line 1\nline 2") {
			hasBrief = true
		}
	}
	if !hasParent {
		t.Fatal("parent must be addressable by send and report")
	}
	if runtime.GOOS != "windows" && !hasBrief {
		t.Fatal("brief missing actual worker id or exact task")
	}
}
func TestWorkerExitAfterDeliveryPublishes(t *testing.T) {
	win, st := workerWindow(t)
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	if win.workers[2].firstMessage != "delivered" {
		t.Fatal("not delivered")
	}
	st.ptys[1].Close()
	deadline := time.Now().Add(time.Second)
	for exited, _ := win.mgr.Lookup(2).Session.Exited(); !exited; exited, _ = win.mgr.Lookup(2).Session.Exited() {
		if time.Now().After(deadline) {
			t.Fatal("session did not exit")
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := win.workerTick(win.wsNow()); ok {
		t.Fatal("exit scheduled wake")
	}
	if tab := win.control.Tabs()[1]; tab.Status != "exited" || tab.FirstMessage != "delivered" {
		t.Fatalf("%+v", tab)
	}
}
func TestWorkerLabelPreservesAuthentication(t *testing.T) {
	win, _ := workerWindow(t)
	key := win.control.OwnerKey("t1")
	if e := win.control.Label("t1", "role", "lead"); e != nil {
		t.Fatal(e)
	}
	if got := win.control.OwnerKey("t1"); got != key || got == "" {
		t.Fatal("label erased owner authentication")
	}
}
func TestWorkerRestoreSkipsWorkers(t *testing.T) {
	win, _ := workerWindow(t)
	p := filepath.Join(t.TempDir(), "sessions.json")
	var err error
	win.restore, err = sessionstate.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { win.mgr.SetStateObserver(nil); win.restore.Close() })
	win.trackRestore()
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	win.mgr.SetActive(2)
	win.captureRestore()
	if err := win.restore.Flush(); err != nil {
		t.Fatal(err)
	}
	saved, err := sessionstate.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Entries) != 1 {
		t.Fatalf("workers persisted: %+v", saved)
	}
}
func TestWorkerCloseParent(t *testing.T) {
	win, _ := workerWindow(t)
	for range 2 {
		if r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"}); !r.OK {
			t.Fatal(r)
		}
	}
	other, err := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
	if err != nil {
		t.Fatal(err)
	}
	win.closeTab(1)
	if len(win.workers) != 0 || win.mgr.Has(1) || win.mgr.Has(2) || win.mgr.Has(3) || !win.mgr.Has(other.ID) {
		t.Fatal("incorrect close cascade")
	}
}
func TestWorkerApplyTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := &controlHost{}
		now := time.Now()
		r := h.Spawn(context.Background(), ctl.Request{})
		if r.Error == nil || r.Error.Code != "timeout" || time.Since(now) != 5*time.Second {
			t.Fatalf("%+v elapsed %v", r, time.Since(now))
		}
		if len(h.workerRequests) != 0 {
			t.Fatal("timed out request retained")
		}
	})
}

func spawnWorker(t *testing.T, win *window, from string) tabs.TabID {
	t.Helper()
	r := win.applyWorker(ctl.Request{Op: "spawn", From: from, Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	return parseWorkerID(r.Tabs[0].ID)
}
func TestWorkerLimitsAndDepth(t *testing.T) {
	t.Run("parent", func(t *testing.T) {
		win, _ := workerWindow(t)
		for range 4 {
			spawnWorker(t, win, "t1")
		}
		r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
		if r.Error == nil || r.Error.Code != ctl.CodeLimit {
			t.Fatalf("%+v", r)
		}
		r = ctl.Handle(&win.control, ctl.Request{Op: "spawn", From: "t2", CallerKey: win.control.OwnerKey("t1"), Agent: "claude", Task: "nested"})
		if r.Error == nil || r.Error.Code != ctl.CodeNotAllowed {
			t.Fatal(r)
		}
		r = win.applyWorker(ctl.Request{Op: "spawn", From: "t2", Agent: "claude", Task: "nested"})
		if r.Error == nil || r.Error.Code != ctl.CodeNotAllowed {
			t.Fatal(r)
		}
	})
	t.Run("total", func(t *testing.T) {
		win, _ := workerWindow(t)
		p2, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
		p3, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
		for range 4 {
			spawnWorker(t, win, "t1")
			spawnWorker(t, win, workerID(p2.ID))
		}
		r := win.applyWorker(ctl.Request{Op: "spawn", From: workerID(p3.ID), Agent: "claude", Task: "review"})
		if r.Error == nil || r.Error.Code != ctl.CodeLimit {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("rate", func(t *testing.T) {
		win, _ := workerWindow(t)
		now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
		win.wsr.now = func() time.Time { return now }
		for range 6 {
			id := spawnWorker(t, win, "t1")
			r := win.applyWorker(ctl.Request{Op: "kill", From: "t1", Tab: workerID(id)})
			if !r.OK {
				t.Fatal(r)
			}
		}
		r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
		if r.Error == nil || r.Error.Code != ctl.CodeRateLimited {
			t.Fatalf("%+v", r)
		}
		now = now.Add(time.Minute)
		spawnWorker(t, win, "t1")
	})
}
func TestWorkerFirstMessageTransitions(t *testing.T) {
	for _, kind := range []string{"prompt", "session", "silent", "exit"} {
		t.Run(kind, func(t *testing.T) {
			win, st := workerWindow(t)
			now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
			win.wsr.now = func() time.Time { return now }
			spawnWorker(t, win, "t1")
			expected := "failed"
			switch kind {
			case "prompt":
				win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
				expected = "delivered"
			case "session":
				now = now.Add(5 * time.Second)
				win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 2, 0)
				now = now.Add(20 * time.Second)
				expected = "unconfirmed"
			case "silent":
				now = now.Add(30 * time.Second)
			case "exit":
				st.ptys[1].Close()
				deadline := time.Now().Add(time.Second)
				for exited, _ := win.mgr.Lookup(2).Session.Exited(); !exited; exited, _ = win.mgr.Lookup(2).Session.Exited() {
					if time.Now().After(deadline) {
						t.Fatal("exit timeout")
					}
					time.Sleep(time.Millisecond)
				}
			}
			if _, ok := win.workerTick(now); ok {
				t.Fatal("wake after final state")
			}
			if got := win.workers[2].firstMessage; got != expected {
				t.Fatal(got, expected)
			}
			if n := testing.AllocsPerRun(20, func() { win.workerTick(now) }); n != 0 {
				t.Fatal("settled workers allocated", n)
			}
		})
	}
	win := &window{}
	if n := testing.AllocsPerRun(20, func() {
		if _, ok := win.workerTick(time.Time{}); ok {
			t.Fatal("idle wake")
		}
	}); n != 0 {
		t.Fatal("idle allocations", n)
	}
}
func TestWorkerQueueAppliedOnlyOnFrameAndConcurrentSnapshots(t *testing.T) {
	win, _ := workerWindow(t)
	wakes := make(chan struct{}, 1)
	win.control.wake = func() { wakes <- struct{}{} }
	done := make(chan ctl.Reply, 1)
	req := ctl.Request{Op: "spawn", From: "t1", CallerKey: win.control.OwnerKey("t1"), Agent: "claude", Task: "review", Labels: map[string]string{"role": "review"}}
	go func() { done <- ctl.Handle(&win.control, req) }()
	select {
	case <-wakes:
	case <-time.After(time.Second):
		t.Fatal("no frame requested")
	}
	if len(win.workers) != 0 || len(win.mgr.Tabs()) != 1 {
		t.Fatal("socket mutated UI state")
	}
	readers := make(chan struct{})
	go func() {
		defer close(readers)
		for range 200 {
			for _, tab := range win.control.Tabs() {
				_ = tab.Parent
				_ = tab.FirstMessage
			}
			ctl.Handle(&win.control, ctl.Request{Op: "ls"})
		}
	}()
	win.drainControl()
	select {
	case r := <-done:
		if !r.OK {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("no reply")
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	select {
	case <-readers:
	case <-time.After(time.Second):
		t.Fatal("snapshot reader stuck")
	}
	if got := win.control.Tabs()[1].Labels["role"]; got != "review" {
		t.Fatal(got)
	}
}

func TestWorkerWaitStartUsesSnapshots(t *testing.T) {
	for _, kind := range []notify.Kind{notify.KindPrompt, notify.KindSession} {
		t.Run(string(kind), func(t *testing.T) {
			win, _ := workerWindow(t)
			now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
			win.wsr.now = func() time.Time { return now }
			wake := make(chan struct{}, 1)
			win.control.wake = func() { wake <- struct{}{} }
			done := make(chan ctl.Reply, 1)
			go func() {
				done <- ctl.Handle(&win.control, ctl.Request{Op: "spawn", From: "t1", CallerKey: win.control.OwnerKey("t1"), Agent: "claude", Task: "review", WaitStart: true})
			}()
			select {
			case <-wake:
			case <-time.After(time.Second):
				t.Fatal("no wake")
			}
			win.drainControl()
			select {
			case r := <-done:
				t.Fatalf("returned pending: %+v", r)
			default:
			}
			win.noteAgent(notify.Msg{Kind: kind, Agent: "claude"}, 2, 0)
			if kind == notify.KindSession {
				now = now.Add(20 * time.Second)
				win.workerTick(now)
			}
			select {
			case r := <-done:
				if kind == notify.KindPrompt && (!r.OK || r.Tabs[0].FirstMessage != "delivered") {
					t.Fatalf("%+v", r)
				}
				if kind == notify.KindSession && (r.Error == nil || r.Error.Code != ctl.CodeSpawnFailed) {
					t.Fatalf("%+v", r)
				}
			case <-time.After(time.Second):
				t.Fatal("wait_start stuck")
			}
		})
	}
}
func TestWorkerValidationAndLaunchFailures(t *testing.T) {
	win, st := workerWindow(t)
	for _, tc := range []struct {
		req  ctl.Request
		code string
	}{
		{ctl.Request{Agent: "shell", Task: "review"}, ctl.CodeUnknownAgent},
		{ctl.Request{Agent: "claude", Task: strings.Repeat("x", (16<<10)+1)}, ctl.CodeTextTooLarge},
		{ctl.Request{Agent: "claude", Task: "review", CWD: filepath.Join(t.TempDir(), "missing")}, ctl.CodeBadCWD},
		{ctl.Request{Agent: "claude", Task: "review", CWD: t.TempDir()}, ctl.CodeBadCWD},
	} {
		req := tc.req
		req.Op = "spawn"
		req.From = "t1"
		r := win.applyWorker(req)
		if r.Error == nil || r.Error.Code != tc.code {
			t.Fatalf("%+v want %s", r, tc.code)
		}
	}
	st.fail = fmt.Errorf("fake launch failure")
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if r.Error == nil || r.Error.Code != ctl.CodeSpawnFailed {
		t.Fatalf("%+v", r)
	}
	if len(win.workerStarts[1]) != 0 || len(win.workers) != 0 {
		t.Fatal("failed start consumed admission")
	}
	st.fail = nil
	id := spawnWorker(t, win, "t1")
	if id != 3 {
		t.Fatal("failed-start id not skipped", id)
	}
}

func TestWorkerLateHookCannotRevivePendingStart(t *testing.T) {
	win, _ := workerWindow(t)
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	win.wsr.now = func() time.Time { return now }
	spawnWorker(t, win, "t1")
	now = now.Add(31 * time.Second)
	win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 2, 0)
	if got := win.workers[2].firstMessage; got != "failed" {
		t.Fatal("late session revived start", got)
	}
}
func TestWorkerFolderUsesCanonicalParentRepo(t *testing.T) {
	win, _ := workerWindow(t)
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	sibling := filepath.Join(root, "sibling")
	for _, dir := range []string{sub, sibling, filepath.Join(root, ".git")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(sub, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	parent, err := win.mgr.NewOpts(tabs.StartOpts{Dir: link})
	if err != nil {
		t.Fatal(err)
	}
	win.publishControl()
	r := win.applyWorker(ctl.Request{Op: "spawn", From: workerID(parent.ID), Agent: "claude", Task: "review", CWD: sibling})
	if !r.OK {
		t.Fatalf("sibling inside canonical repo refused: %+v", r)
	}
}

func TestWorkerDisconnectDuringLaunchRollsBack(t *testing.T) {
	win, _ := workerWindow(t)
	win.mgr.CloseAll()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	win.mgr = tabs.NewManagerWithArgv(func(argv, env []string, dir string) (ptyx.Session, error) {
		if len(argv) > 0 {
			cancel()
		}
		return newRecPTY(), nil
	}, 10, 2, func() {})
	t.Cleanup(win.mgr.CloseAll)
	win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
	win.publishControl()
	wake := make(chan struct{}, 1)
	win.control.wake = func() { wake <- struct{}{} }
	done := make(chan ctl.Reply, 1)
	go func() { done <- win.control.Spawn(ctx, ctl.Request{From: "t1", Agent: "claude", Task: "review"}) }()
	select {
	case <-wake:
	case <-time.After(time.Second):
		t.Fatal("no wake")
	}
	win.drainControl()
	select {
	case r := <-done:
		if r.Error == nil || r.Error.Code != "timeout" {
			t.Fatalf("%+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("request stuck")
	}
	if win.mgr.Has(2) || len(win.workers) != 0 || len(win.workerStarts[1]) != 0 {
		t.Fatal("canceled start left a worker")
	}
}

func TestWorkerCloseRemovesEntryKeepsParentRate(t *testing.T) {
	for _, kill := range []bool{false, true} {
		win, _ := workerWindow(t)
		id := spawnWorker(t, win, "t1")
		starts := append([]time.Time(nil), win.workerStarts[1]...)
		if kill {
			if r := win.applyWorker(ctl.Request{Op: "kill", From: "t1", Tab: workerID(id)}); !r.OK {
				t.Fatal(r)
			}
		} else {
			win.closeTab(id)
		}
		if win.workers[id] != nil || win.mgr.Has(id) {
			t.Fatal("closed worker retained")
		}
		if len(starts) != 1 || len(win.workerStarts[1]) != 1 || !starts[0].Equal(win.workerStarts[1][0]) {
			t.Fatal("parent rate history lost")
		}
	}
}
