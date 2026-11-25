package ui

import (
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func pendingWindow(t *testing.T) (*window, *tabs.Tab) {
	t.Helper()
	win, _ := sideWindow(t, true)
	record := &controlRecordPTY{recPTY: newRecPTY(), writes: make(chan string, 8)}
	win.mgr = tabs.NewManager(func([]string, string) (ptyx.Session, error) { return record, nil }, 80, 24, nil)
	t.Cleanup(win.mgr.CloseAll)
	return win, openIn(t, win, "")
}

func deliverKind(win *window, tb *tabs.Tab, kind notify.Kind) {
	at := time.Now()
	win.focused = false
	win.deliver(queued{m: notify.Msg{Tab: int(tb.ID), Kind: kind, Agent: "codex"}, at: at}, at)
}

// A send must not let a following wait see the previous idle state before the
// prompt hook arrives: the tab reads as working until that hook or a grace.
func TestControlSubmitHoldsWorkingUntilPromptHook(t *testing.T) {
	win, tb := pendingWindow(t)
	deliverKind(win, tb, notify.KindDone)
	if got := win.control.Tabs()[0].Status; got != "idle" {
		t.Fatalf("before send: %s", got)
	}
	if err := win.control.Submit("t1", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := win.control.Tabs()[0].Status; got != "working" {
		t.Fatalf("after send, before hook: %s", got)
	}
	deliverKind(win, tb, notify.KindPrompt)
	if got := win.control.Tabs()[0].Status; got != "working" {
		t.Fatalf("after prompt hook: %s", got)
	}
	deliverKind(win, tb, notify.KindDone)
	if got := win.control.Tabs()[0].Status; got != "idle" {
		t.Fatalf("after done: a stale pending hold kept %s", got)
	}
}

func TestControlSubmitPendingExpires(t *testing.T) {
	old := controlPendingGrace
	controlPendingGrace = 30 * time.Millisecond
	t.Cleanup(func() { controlPendingGrace = old })
	win, tb := pendingWindow(t)
	deliverKind(win, tb, notify.KindDone)
	_, changed := win.control.StatusChanges()
	if err := win.control.Submit("t1", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, changed = win.control.StatusChanges()
	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("waiters were not woken when the hold expired")
	}
	if got := win.control.Tabs()[0].Status; got != "idle" {
		t.Fatalf("after grace: %s", got)
	}
}

func TestControlShellSubmitStaysShell(t *testing.T) {
	win, _ := pendingWindow(t)
	win.publishControl()
	if err := win.control.Submit("t1", []byte("ls")); err != nil {
		t.Fatal(err)
	}
	if got := win.control.Tabs()[0].Status; got != "shell" {
		t.Fatalf("shell tab: %s", got)
	}
}

// An agent known only by process detection has no hook state: it must not be
// reported idle, or wait returns while it is still printing.
func TestControlDetectedOnlyAgentIsUnknown(t *testing.T) {
	win, tb := pendingWindow(t)
	win.detected = map[tabs.TabID]detection{tb.ID: {agent: "codex"}}
	win.publishControl()
	got := win.control.Tabs()[0]
	if got.Agent != "codex" || got.Status != "unknown" {
		t.Fatalf("%+v", got)
	}
}
