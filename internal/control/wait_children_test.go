package control

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type childrenHost struct {
	Host
	mu       sync.Mutex
	tabs     []TabInfo
	changed  chan struct{}
	consumed uint64
}

func (h *childrenHost) OwnerKey(id string) string {
	if id == "t1" {
		return "secret"
	}
	return ""
}
func (h *childrenHost) Tabs() []TabInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]TabInfo(nil), h.tabs...)
}
func (h *childrenHost) StatusChanges() ([]TabInfo, <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]TabInfo(nil), h.tabs...), h.changed
}
func (h *childrenHost) OutputChanges(string) (<-chan struct{}, func(), time.Time, error) {
	panic("children must not observe output")
}
func (h *childrenHost) ConsumeInbox(id string, seq uint64) []InboxMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consumed = seq
	return h.tabs[0].Inbox
}
func TestWaitChildren(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		for _, event := range []string{"stopped", "report", "note", "timeout"} {
			t.Run(mode+"/"+event, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					h := &childrenHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t2", Parent: "t1", Role: "worker", Status: "working"}, {ID: "t3", Parent: "t1", Role: "worker", Status: "working"}, {ID: "t4", Parent: "t9", Role: "worker", Status: "idle"}}, changed: make(chan struct{})}
					req := Request{Op: "wait", Children: mode, From: "t1", CallerKey: "secret", Timeout: 2}
					done := make(chan Reply, 1)
					go func() { done <- Handle(h, req) }()
					synctest.Wait()
					select {
					case r := <-done:
						t.Fatalf("early: %+v", r)
					default:
					}
					want := event
					if event == "timeout" {
						time.Sleep(2 * time.Second)
					} else {
						h.mu.Lock()
						if event == "stopped" {
							h.tabs[1].Status = "needs"
							if mode == "all" {
								h.tabs[2].Status = "exited"
							}
							want = mode + "_stopped"
						} else {
							h.tabs[0].Inbox = []InboxMessage{{Sequence: 1, Kind: event, Text: "message"}}
							h.tabs[0].Unread = 1
						}
						close(h.changed)
						h.changed = make(chan struct{})
						h.mu.Unlock()
					}
					synctest.Wait()
					select {
					case r := <-done:
						if r.Reason != want || len(r.Tabs) != 2 {
							t.Fatalf("%+v", r)
						}
						if event == "note" || event == "report" {
							if h.consumed != 1 || r.Text != "message" {
								t.Fatalf("not consumed: %+v", r)
							}
						}
						if event == "timeout" && (r.Error == nil || r.Error.Code != "timeout") {
							t.Fatal(r)
						}
					default:
						t.Fatal("wait did not return")
					}
				})
			})
		}
	}
}
func TestWaitChildrenKeyAndValidation(t *testing.T) {
	h := &childrenHost{tabs: []TabInfo{{ID: "t1"}}, changed: make(chan struct{})}
	for _, key := range []string{"", "wrong"} {
		r := Handle(h, Request{Op: "wait", Children: "all", From: "t1", CallerKey: key})
		if r.Error == nil || r.Error.Code != CodeNotAllowed {
			t.Fatal(r)
		}
	}
	r := Handle(h, Request{Op: "wait", Children: "bad", From: "t1", CallerKey: "secret"})
	if r.Error == nil || r.Error.Code != CodeInvalidRequest {
		t.Fatal(r)
	}
}
func TestNeedsWorkerInputGuard(t *testing.T) {
	for _, op := range []string{"keys", "send", "interrupt"} {
		for _, status := range []string{"needs", "idle", "working"} {
			for _, role := range []string{"worker", ""} {
				h := &fakeHost{tabs: []TabInfo{{ID: "t2", Parent: "t1", Role: role, Status: status}}}
				r := Handle(h, Request{Op: op, Tab: "t2", From: "t1", Text: "text", Args: []string{"enter"}})
				if status == "needs" && role == "worker" {
					if r.Error == nil || r.Error.Code != CodeNotAllowed || h.submitCalls != 0 || h.id != "" {
						t.Fatalf("%s: %+v", op, r)
					}
				} else if !r.OK {
					t.Fatalf("%s: %+v", op, r)
				}
			}
		}
	}
}

func TestWaitChildrenAllRequiresLastChild(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := &childrenHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t2", Parent: "t1", Role: "worker", Status: "idle"}, {ID: "t3", Parent: "t1", Role: "worker", Status: "working"}}, changed: make(chan struct{})}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan Reply, 1)
		go func() {
			done <- HandleContext(ctx, h, Request{Op: "wait", Children: "all", From: "t1", CallerKey: "secret", Timeout: 2})
		}()
		synctest.Wait()
		select {
		case r := <-done:
			t.Fatalf("returned with one child working: %+v", r)
		default:
		}
		cancel()
		synctest.Wait()
		select {
		case r := <-done:
			if r.Error == nil || r.Error.Code != "cancelled" {
				t.Fatal(r)
			}
		default:
			t.Fatal("cancel did not return")
		}
	})
}
