package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func TestWorkerChildWaitSpansInitialDelivery(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		t.Run(mode, func(t *testing.T) {
			win, _ := workerWindow(t)
			now := time.Now()
			win.wsr.now = func() time.Time { return now }
			if r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"}); !r.OK {
				t.Fatal(r)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			host := &workerE2EHost{controlHost: &win.control, observed: make(chan struct{}, 8)}
			done := make(chan ctl.Reply, 1)
			go func() {
				done <- ctl.HandleContext(ctx, host, ctl.Request{Op: "wait", From: "t1", CallerKey: host.OwnerKey("t1"), Children: mode, Timeout: 2})
			}()
			select {
			case <-host.observed:
			case <-time.After(time.Second):
				t.Fatal("wait subscription deadline")
			}
			for _, kind := range []notify.Kind{notify.KindSession, notify.KindPrompt} {
				win.noteAgent(notify.Msg{Kind: kind, Agent: "claude"}, 2, 0)
				want := "starting"
				if kind == notify.KindPrompt {
					want = "working"
				}
				if got := win.control.Tabs()[1].Status; got != want {
					t.Errorf("%s: status %s, want %s", kind, got, want)
				}
				select {
				case r := <-done:
					t.Fatalf("wait returned before Stop: %+v", r)
				case <-time.After(20 * time.Millisecond):
				}
			}
			win.noteAgent(notify.Msg{Kind: notify.KindDone, Agent: "claude"}, 2, 0)
			select {
			case r := <-done:
				if !r.OK || (r.Reason != "note" && r.Reason != mode+"_stopped") {
					t.Fatalf("Stop: %+v", r)
				}
			case <-time.After(time.Second):
				t.Fatal("Stop deadline")
			}
		})
	}
}

func TestWorkerStartupTimeoutStopsAndNotifies(t *testing.T) {
	for _, session := range []bool{false, true} {
		t.Run(map[bool]string{false: "no hooks", true: "session without prompt"}[session], func(t *testing.T) {
			win, _ := workerWindow(t)
			now := time.Now()
			win.wsr.now = func() time.Time { return now }
			if r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "codex", Task: "review"}); !r.OK {
				t.Fatal(r)
			}
			duration := 30 * time.Second
			want := "failed"
			if session {
				win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "codex"}, 2, 0)
				duration = 20 * time.Second
				want = "unconfirmed"
			}
			win.workerTick(now.Add(duration - time.Nanosecond))
			if got := win.control.Tabs()[1].Status; got != "starting" {
				t.Errorf("early status: %s", got)
			}
			now = now.Add(duration)
			win.workerTick(now)
			tab := win.control.Tabs()[1]
			if tab.FirstMessage != want || tab.Status != "idle" {
				t.Errorf("timeout: %+v", tab)
			}
			req := ctl.Request{Op: "wait", From: "t1", CallerKey: win.control.OwnerKey("t1"), Children: "all", Timeout: 1}
			r := ctl.Handle(&win.control, req)
			if !r.OK || r.Reason != "note" || !strings.Contains(r.Text, "trust dialog or missing hooks") {
				t.Fatalf("not_started note: %+v", r)
			}
			if r = ctl.Handle(&win.control, req); !r.OK || r.Reason != "all_stopped" {
				t.Fatalf("settled wait: %+v", r)
			}
		})
	}
}
