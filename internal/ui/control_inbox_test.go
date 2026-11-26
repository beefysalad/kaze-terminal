package ui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestWorkerNotesAndQuietWindow(t *testing.T) {
	win, _ := workerWindow(t)
	now := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	win.wsr.now = func() time.Time { return now }
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review", Name: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	win.control.recordOwnerKey(1, now.Add(-2*time.Second))
	win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 1, 0)
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	win.workerHook(2, notify.KindNeeds)
	win.workerHook(2, notify.KindNeeds)
	infos := win.control.Tabs()
	if infos[0].Unread != 1 || !strings.Contains(infos[0].Inbox[0].Text, `kind="note" child="t2"`) || !strings.Contains(infos[0].Inbox[0].Text, "approval prompt") {
		t.Fatalf("%+v", infos[0])
	}
	win.control.recordOwnerKey(1, now.Add(-2*time.Second))
	if at, ok := win.noteTick(now, nil); !ok || !at.Equal(now.Add(3*time.Second)) {
		t.Fatal(at, ok)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 1, 0)
	if _, ok := win.noteTick(now, nil); ok {
		t.Fatal("working parent must not schedule quiet wake")
	}
	win.noteAgent(notify.Msg{Kind: notify.KindDone, Agent: "claude"}, 1, 0)
	win.control.recordOwnerKey(1, now.Add(-6*time.Second))
	// Dry decision mode verifies readiness without submitting terminal input.
	if at, ok := win.noteTick(now, nil); !ok || !at.Equal(now) {
		t.Fatal(at, ok)
	}
	win.workerHook(2, notify.KindPrompt)
	win.workerHook(2, notify.KindNeeds)
	win.workerHook(2, notify.KindPrompt)
	win.workerHook(2, notify.KindDone)
	win.workerHook(2, notify.KindDone)
	if got := win.control.Tabs()[0].Unread; got != 3 {
		t.Fatal("re-arm / done", got)
	}
	old := win.control.Tabs()
	messages := win.control.ConsumeInbox("t1", old[0].Inbox[1].Sequence)
	if len(messages) != 2 || win.control.Tabs()[0].Unread != 1 || old[0].Unread != 3 {
		t.Fatal("consumption or snapshot mutation", messages)
	}
	win.control.ConsumeInbox("t1", old[0].Inbox[2].Sequence)
	if _, ok := win.noteTick(now, nil); ok {
		t.Fatal("empty inbox wake")
	}
}
func TestWorkerFailureAndExitNotes(t *testing.T) {
	win, _ := workerWindow(t)
	now := time.Now()
	win.wsr.now = func() time.Time { return now }
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	now = now.Add(31 * time.Second)
	win.workerTick(now)
	win.workerTick(now)
	if got := win.control.Tabs()[0].Inbox; len(got) != 1 || !strings.Contains(got[0].Text, "not confirmed") {
		t.Fatal(got)
	}
	win.closeWorkers(2)
	if got := win.control.Tabs()[0].Inbox; len(got) != 2 || !strings.Contains(got[1].Text, "exited") {
		t.Fatal(got)
	}
}
func TestInboxReportWaitAndConcurrentReaders(t *testing.T) {
	win, _ := workerWindow(t)
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	r = ctl.Handle(&win.control, ctl.Request{Op: "report", From: "t2", Text: "result"})
	if !r.OK {
		t.Fatal(r)
	}
	old := win.control.Tabs()
	if old[0].Unread != 1 {
		t.Fatal(old)
	}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for range 50 {
				win.control.Tabs()
				win.control.StatusChanges()
			}
		})
	}
	for range 10 {
		win.publishControl()
	}
	wg.Wait()
	r = ctl.HandleContext(context.Background(), &win.control, ctl.Request{Op: "wait", Children: "all", From: "t1", CallerKey: win.control.OwnerKey("t1"), Timeout: 1})
	if !r.OK || r.Reason != "report" || !strings.Contains(r.Text, "result") || win.control.Tabs()[0].Unread != 0 || old[0].Unread != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestNoteDeliveryUsesCallbackAndPendingGrace(t *testing.T) {
	win, _ := workerWindow(t)
	now := time.Now()
	win.wsr.now = func() time.Time { return now }
	win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 1, 0)
	if err := win.control.EnqueueInbox("t1", "note", "a note"); err != nil {
		t.Fatal(err)
	}
	target := win.control.snapshot.Load().sessions["t1"]
	target.pending.Store(now.UnixNano())
	if at, ok := win.noteTick(now, nil); !ok || !at.Equal(now.Add(controlPendingGrace)) {
		t.Fatalf("pending note lost its wake: %v %v", at, ok)
	}
	target.pending.Store(0)
	delivered := make(chan string, 1)
	submit := func(id string, text []byte) error { delivered <- id + ":" + string(text); return nil }
	if _, ok := win.noteTick(now, submit); ok {
		t.Fatal("delivered note scheduled wake")
	}
	select {
	case text := <-delivered:
		if text != "t1:a note" {
			t.Fatal(text)
		}
	case <-time.After(time.Second):
		t.Fatal("delivery timeout")
	}
	if _, ok := win.noteTick(now, submit); ok {
		t.Fatal("duplicate note wake")
	}
	select {
	case text := <-delivered:
		t.Fatal("duplicate", text)
	default:
	}
	if n := testing.AllocsPerRun(20, func() { win.noteTick(now, nil) }); n != 0 {
		t.Fatal("settled inbox allocations", n)
	}
}

func TestNeedsWorkerSnapshotSurvivesPendingOverlay(t *testing.T) {
	s := &controlSnapshot{tabs: []ctl.TabInfo{{ID: "t2", Role: "worker", Status: "needs"}}, sessions: map[string]*controlSession{"t2": {}}}
	now := time.Now().UnixNano()
	s.sessions["t2"].pending.Store(now)
	if got := overlayPending(s, now)[0].Status; got != "needs" {
		t.Fatal("approval state was hidden", got)
	}
}
func TestNoteSubmissionRechecksParent(t *testing.T) {
	win, _ := workerWindow(t)
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 1, 0)
	if err := win.control.submitNote("t1", []byte("note")); err != errNoteDeferred {
		t.Fatal("working parent accepted note", err)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindDone, Agent: "claude"}, 1, 0)
	win.control.recordOwnerKey(1, time.Now())
	if err := win.control.submitNote("t1", []byte("note")); err != errNoteDeferred {
		t.Fatal("recent owner key accepted note", err)
	}
}

func TestWorkerNeedsGuardDoesNotDependOnUnreadAlert(t *testing.T) {
	win, _ := workerWindow(t)
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	win.noteAgent(notify.Msg{Kind: notify.KindNeeds, Agent: "claude"}, 2, 0)
	win.alerts.visited(2)
	win.publishControl()
	if got := win.control.Tabs()[1].Status; got != "needs" {
		t.Fatal("viewing approval cleared guard", got)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	if got := win.control.Tabs()[1].Status; got != "working" {
		t.Fatal(got)
	}
}

func TestWorkerDoneClearsNeedsEvenWithUnreadAlert(t *testing.T) {
	win, _ := workerWindow(t)
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
	win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
	win.noteAgent(notify.Msg{Kind: notify.KindNeeds, Agent: "claude"}, 2, 0)
	win.alerts.marks = map[tabs.TabID]mark{2: markNeeds}
	win.noteAgent(notify.Msg{Kind: notify.KindDone, Agent: "claude"}, 2, 0)
	if got := win.control.Tabs()[1].Status; got != "idle" {
		t.Fatal("stale alert hid actual status", got)
	}
}

func TestWorkerExitNotesByCloseCause(t *testing.T) {
	for _, cause := range []string{"parent kill", "owner close", "natural exit"} {
		t.Run(cause, func(t *testing.T) {
			win, st := workerWindow(t)
			if r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"}); !r.OK {
				t.Fatal(r)
			}
			win.workerHook(2, notify.KindPrompt)
			switch cause {
			case "parent kill":
				if r := win.applyWorker(ctl.Request{Op: "kill", From: "t1", Tab: "t2"}); !r.OK {
					t.Fatal(r)
				}
			case "owner close":
				win.closeTab(2)
			case "natural exit":
				st.ptys[1].Close()
				deadline := time.Now().Add(time.Second)
				for exited, _ := win.mgr.Lookup(2).Session.Exited(); !exited; exited, _ = win.mgr.Lookup(2).Session.Exited() {
					if time.Now().After(deadline) {
						t.Fatal("session did not exit")
					}
					time.Sleep(time.Millisecond)
				}
			}
			win.workerTick(win.wsNow())
			win.workerTick(win.wsNow())
			got := win.control.Tabs()[0].Inbox
			if cause == "parent kill" {
				if len(got) != 0 {
					t.Fatalf("parent kill sent notes: %+v", got)
				}
			} else if len(got) != 1 || !strings.Contains(got[0].Text, "exited") {
				t.Fatalf("exit notes: %+v", got)
			}
		})
	}
}
