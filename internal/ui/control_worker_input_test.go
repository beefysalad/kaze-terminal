package ui

import (
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Capture the handler's eligible snapshot before its host call waits for send.
type inputSnapshotHost struct {
	*controlHost
	observed chan struct{}
}

func (h *inputSnapshotHost) Tabs() []ctl.TabInfo {
	infos := h.controlHost.Tabs()
	select {
	case h.observed <- struct{}{}:
	default:
	}
	return infos
}

func TestWorkerInputRechecksAfterSendLock(t *testing.T) {
	for _, op := range []string{"send", "keys", "interrupt"} {
		t.Run(op, func(t *testing.T) {
			win, st := workerWindow(t)
			if r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"}); !r.OK {
				t.Fatal(r)
			}
			win.noteAgent(notify.Msg{Kind: notify.KindPrompt, Agent: "claude"}, 2, 0)
			target := win.control.snapshot.Load().sessions["t2"]
			target.send.Lock()
			locked := true
			defer func() {
				if locked {
					target.send.Unlock()
				}
			}()
			host := &inputSnapshotHost{controlHost: &win.control, observed: make(chan struct{}, 1)}
			done := make(chan ctl.Reply, 1)
			go func() { done <- ctl.Handle(host, ctl.Request{Op: op, Tab: "t2", Text: "yes", Args: []string{"enter"}}) }()
			select {
			case <-host.observed:
			case <-time.After(time.Second):
				t.Fatal("snapshot deadline")
			}
			select {
			case r := <-done:
				t.Fatalf("did not wait for send lock: %+v", r)
			case <-time.After(10 * time.Millisecond):
			}
			win.noteAgent(notify.Msg{Kind: notify.KindNeeds, Agent: "claude"}, 2, 0)
			target.send.Unlock()
			locked = false
			select {
			case r := <-done:
				if r.Error == nil || r.Error.Code != ctl.CodeNotAllowed {
					t.Fatalf("stale approval accepted: %+v", r)
				}
			case <-time.After(time.Second):
				t.Fatal("input deadline")
			}
			if got := st.ptys[1].sent(); got != "" {
				t.Fatalf("input reached approval: %q", got)
			}
		})
	}
}

func TestWorkerSendRechecksBeforeEnter(t *testing.T) {
	win, _ := workerWindow(t)
	win.mgr.CloseAll()
	record := &controlRecordPTY{recPTY: newRecPTY(), writes: make(chan string, 4)}
	win.mgr = tabs.NewManagerWithArgv(func([]string, []string, string) (ptyx.Session, error) { return record, nil }, 80, 24, nil)
	t.Cleanup(win.mgr.CloseAll)
	tb, err := win.mgr.NewOpts(tabs.StartOpts{Role: "worker", Parent: 99})
	if err != nil {
		t.Fatal(err)
	}
	win.workers = map[tabs.TabID]*worker{tb.ID: {agent: "claude", firstMessage: "delivered", hooked: true}}
	win.publishControl()
	done := make(chan ctl.Reply, 1)
	go func() { done <- ctl.Handle(&win.control, ctl.Request{Op: "send", Tab: "t1", Text: "review"}) }()
	select {
	case <-record.writes:
	case <-time.After(time.Second):
		t.Fatal("paste deadline")
	}
	win.noteAgent(notify.Msg{Kind: notify.KindNeeds, Agent: "claude"}, tb.ID, 0)
	select {
	case r := <-done:
		if r.Error == nil || r.Error.Code != ctl.CodeNotAllowed {
			t.Fatalf("Enter accepted: %+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("send deadline")
	}
	// Flush the session queue by closing only after the request has returned.
	select {
	case got := <-record.writes:
		t.Fatalf("extra input after needs: %q", got)
	case <-time.After(20 * time.Millisecond):
	}
}
