package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func consentRequest(from string) *controlWorkerRequest {
	return &controlWorkerRequest{ctx: context.Background(), req: ctl.Request{Op: "spawn", From: from, Agent: "claude", Task: "Review the terminal\nDo not change files"}, done: make(chan ctl.Reply, 1)}
}
func consentCode(t *testing.T, r *controlWorkerRequest, code string) {
	t.Helper()
	select {
	case got := <-r.done:
		if got.Error == nil || got.Error.Code != code {
			t.Fatalf("reply %+v, want %s", got, code)
		}
	default:
		t.Fatal("missing reply")
	}
}
func TestWorkerConsentModes(t *testing.T) {
	for _, mode := range []string{"never", "first", "always", ""} {
		t.Run(mode, func(t *testing.T) {
			win, _ := workerWindow(t)
			win.opt.Config.Orchestrator.Confirm = mode
			r := consentRequest("t1")
			if held := win.holdWorkerConsent(r); held != (mode != "never") {
				t.Fatalf("held=%v", held)
			}
			if mode == "never" {
				return
			}
			if len(win.workers) != 0 {
				t.Fatal("spawned before allow")
			}
			win.answerWorkerConsent(1, false)
			consentCode(t, r, "consent_denied")
			r = consentRequest("t1")
			if !win.holdWorkerConsent(r) {
				t.Fatal("deny remembered")
			}
			win.answerWorkerConsent(1, true)
			select {
			case got := <-r.done:
				if !got.OK {
					t.Fatal(got)
				}
			default:
				t.Fatal("missing allow reply")
			}
			if held := win.holdWorkerConsent(consentRequest("t1")); held != (mode == "always") {
				t.Fatalf("after allow held=%v", held)
			}
			other, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
			if !win.holdWorkerConsent(consentRequest(workerID(other.ID))) {
				t.Fatal("allow leaked to another parent")
			}
		})
	}
}
func TestWorkerConsentDeadlinesAndCaps(t *testing.T) {
	win, _ := workerWindow(t)
	win.opt.Config.Orchestrator.Confirm = "first"
	now := t0
	win.wsr.now = func() time.Time { return now }
	if at, ok := win.consentTick(now); ok || !at.IsZero() {
		t.Fatal("idle wake")
	}
	first := consentRequest("t1")
	win.holdWorkerConsent(first)
	duplicate := consentRequest("t1")
	win.holdWorkerConsent(duplicate)
	consentCode(t, duplicate, ctl.CodeBusy)
	for range 3 {
		p, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
		win.holdWorkerConsent(consentRequest(workerID(p.ID)))
	}
	p, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
	overflow := consentRequest(workerID(p.ID))
	win.holdWorkerConsent(overflow)
	consentCode(t, overflow, ctl.CodeBusy)
	if len(win.consent.pending) != 4 {
		t.Fatal("pending cap", len(win.consent.pending))
	}
	if at, ok := win.consentTick(now); !ok || at != now.Add(time.Minute) {
		t.Fatal(at, ok)
	}
	now = now.Add(time.Minute - time.Nanosecond)
	win.consentTick(now)
	select {
	case <-first.done:
		t.Fatal("early timeout")
	default:
	}
	now = now.Add(time.Nanosecond)
	win.consentTick(now)
	consentCode(t, first, "consent_timeout")
	if n := testing.AllocsPerRun(20, func() {
		if _, ok := win.consentTick(now); ok {
			t.Fatal("settled wake")
		}
	}); n != 0 {
		t.Fatal("idle allocations", n)
	}
}
func TestWorkerConsentCloseCancelAndRevalidate(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		r := consentRequest("t1")
		win.holdWorkerConsent(r)
		win.closeTab(1)
		consentCode(t, r, ctl.CodeNotAllowed)
	})
	t.Run("cancel", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		r := consentRequest("t1")
		ctx, cancel := context.WithCancel(context.Background())
		r.ctx = ctx
		win.holdWorkerConsent(r)
		cancel()
		win.consentTick(win.wsNow())
		if len(win.consent.pending) != 0 {
			t.Fatal("retained canceled consent")
		}
		win.answerWorkerConsent(1, true)
		if len(win.workers) != 0 {
			t.Fatal("spawn after disconnect")
		}
	})
	t.Run("revalidate", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		r := consentRequest("t1")
		win.holdWorkerConsent(r)
		win.orchestratorEnabled = false
		win.answerWorkerConsent(1, true)
		consentCode(t, r, ctl.CodeSpawnDisabled)
		if len(win.workers) != 0 {
			t.Fatal("stale validation")
		}
	})
	t.Run("expired allow", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		now := t0
		win.wsr.now = func() time.Time { return now }
		r := consentRequest("t1")
		win.holdWorkerConsent(r)
		now = now.Add(time.Minute)
		win.answerWorkerConsent(1, true)
		consentCode(t, r, "consent_timeout")
		if len(win.workers) != 0 {
			t.Fatal("late allow spawned")
		}
	})
}

func TestWorkerConsentQueueSurvivesApplyTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		wake := make(chan struct{}, 4)
		win.control.wake = func() { wake <- struct{}{} }
		done := make(chan ctl.Reply, 1)
		go func() { done <- win.control.Spawn(context.Background(), consentRequest("t1").req) }()
		<-wake
		win.drainWorkers()
		synctest.Wait()
		time.Sleep(6 * time.Second)
		select {
		case got := <-done:
			t.Fatalf("consent expired at apply deadline: %+v", got)
		default:
		}
		win.answerWorkerConsent(1, false)
		select {
		case got := <-done:
			if got.Error == nil || got.Error.Code != "consent_denied" {
				t.Fatal(got)
			}
		case <-time.After(time.Second):
			t.Fatal("held socket stuck")
		}
	})
}

func TestWorkerConsentAttentionEndsWithRequest(t *testing.T) {
	win, _ := workerWindow(t)
	win.opt.Config.Orchestrator.Confirm = "first"
	win.focused = false
	r := consentRequest("t1")
	win.holdWorkerConsent(r)
	if win.alerts.markOf(1) != markNeeds || len(win.alerts.toasts) != 1 {
		t.Fatal("missing parent attention")
	}
	win.answerWorkerConsent(1, false)
	if win.alerts.markOf(1) != markNone || len(win.alerts.toasts) != 0 {
		t.Fatal("resolved consent left stale attention")
	}
}
func TestWorkerConsentPreservesNewerAgentAttention(t *testing.T) {
	win, _ := workerWindow(t)
	win.opt.Config.Orchestrator.Confirm = "first"
	win.focused = false
	win.holdWorkerConsent(consentRequest("t1"))
	win.alerts.apply(notify.Msg{Kind: notify.KindNeeds, Tab: 1, Reason: "Agent approval"}, 1, false, win.mgr.Has, win.wsNow())
	win.answerWorkerConsent(1, false)
	if win.alerts.markOf(1) != markNeeds || len(win.alerts.toasts) != 1 || win.alerts.toasts[0].Text != "Agent approval" {
		t.Fatal("newer attention lost")
	}
}

func TestWorkerConsentAttentionAfterToastEviction(t *testing.T) {
	for _, newer := range []bool{false, true} {
		t.Run(fmt.Sprint(newer), func(t *testing.T) {
			win, _ := workerWindow(t)
			win.opt.Config.Orchestrator.Confirm = "first"
			win.focused = false
			win.holdWorkerConsent(consentRequest("t1"))
			if newer {
				msg := notify.Msg{Kind: notify.KindNeeds, Tab: 1, Reason: "Agent approval"}
				win.noteAgent(msg, 1, 0)
				win.alerts.apply(msg, 0, false, win.mgr.Has, win.wsNow())
			}
			for range 3 {
				p, _ := win.mgr.NewOpts(tabs.StartOpts{Dir: win.opt.Dir})
				win.holdWorkerConsent(consentRequest(workerID(p.ID)))
			}
			win.answerWorkerConsent(1, false)
			want := markNone
			if newer {
				want = markNeeds
			}
			if win.alerts.markOf(1) != want {
				t.Fatalf("mark=%v want %v after toast eviction", win.alerts.markOf(1), want)
			}
		})
	}
}

func TestWorkerConsentRevalidatesFolderAndCap(t *testing.T) {
	t.Run("folder", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		folder := filepath.Join(win.opt.Dir, "work")
		if err := os.Mkdir(folder, 0700); err != nil {
			t.Fatal(err)
		}
		r := consentRequest("t1")
		r.req.CWD = folder
		win.holdWorkerConsent(r)
		if err := os.Remove(folder); err != nil {
			t.Fatal(err)
		}
		win.answerWorkerConsent(1, true)
		consentCode(t, r, ctl.CodeBadCWD)
	})
	t.Run("cap", func(t *testing.T) {
		win, _ := workerWindow(t)
		win.opt.Config.Orchestrator.Confirm = "first"
		r := consentRequest("t1")
		win.holdWorkerConsent(r)
		for range 4 {
			spawnWorker(t, win, "t1")
		}
		win.answerWorkerConsent(1, true)
		consentCode(t, r, ctl.CodeLimit)
		if len(win.workers) != 4 {
			t.Fatal("stale admission")
		}
	})
}
func TestWorkerConsentAttentionCompletionPaths(t *testing.T) {
	for _, reason := range []string{"allow", "deny", "timeout", "close", "cancel"} {
		t.Run(reason, func(t *testing.T) {
			win, _ := workerWindow(t)
			win.opt.Config.Orchestrator.Confirm = "first"
			win.focused = false
			now := t0
			win.wsr.now = func() time.Time { return now }
			r := consentRequest("t1")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r.ctx = ctx
			win.holdWorkerConsent(r)
			switch reason {
			case "allow":
				win.answerWorkerConsent(1, true)
			case "deny":
				win.answerWorkerConsent(1, false)
			case "timeout":
				now = now.Add(time.Minute)
				win.consentTick(now)
			case "close":
				win.closeTab(1)
			case "cancel":
				cancel()
				win.consentTick(now)
			}
			if win.alerts.markOf(1) != markNone || len(win.alerts.toasts) != 0 || len(win.consent.pending) != 0 {
				t.Fatal("stale attention after", reason)
			}
		})
	}
}

func TestWorkerConsentAttentionAfterInterrupt(t *testing.T) {
	win, _ := workerWindow(t)
	win.opt.Config.Orchestrator.Confirm = "first"
	win.focused = false
	win.holdWorkerConsent(consentRequest("t1"))
	win.noteAgent(notify.Msg{Kind: notify.KindInterrupt, Agent: "codex", Tab: 1}, 1, 0)
	win.answerWorkerConsent(1, false)
	if win.alerts.markOf(1) != markNone || len(win.alerts.toasts) != 0 {
		t.Fatal("interrupt left resolved consent attention")
	}
}

func TestWorkerConsentPublishesAttentionChanges(t *testing.T) {
	win, _ := workerWindow(t)
	win.opt.Config.Orchestrator.Confirm = "first"
	win.focused = false
	win.agents = map[tabs.TabID]agentState{1: {claude: true, running: true, shellAt: shellSeq(win.mgr.Active())}}
	win.publishControl()
	win.holdWorkerConsent(consentRequest("t1"))
	if got := win.control.Tabs()[0].Status; got != "needs" {
		t.Fatalf("pending snapshot status %q", got)
	}
	win.answerWorkerConsent(1, false)
	if got := win.control.Tabs()[0].Status; got != "working" {
		t.Fatalf("resolved snapshot status %q", got)
	}
}
