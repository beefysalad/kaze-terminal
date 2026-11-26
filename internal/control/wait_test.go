package control

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type waitFixture struct {
	Host
	mu        sync.Mutex
	tab       TabInfo
	changed   chan struct{}
	output    chan struct{}
	text      string
	observers int
	ready     chan struct{}
}

func newWaitFixture(status string) *waitFixture {
	return &waitFixture{tab: TabInfo{ID: "t1", Status: status}, changed: make(chan struct{}), output: make(chan struct{}, 1), ready: make(chan struct{}, 1)}
}
func (h *waitFixture) Tabs() []TabInfo { h.mu.Lock(); defer h.mu.Unlock(); return []TabInfo{h.tab} }
func (h *waitFixture) StatusChanges() ([]TabInfo, <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return []TabInfo{h.tab}, h.changed
}
func (h *waitFixture) OutputChanges(string) (<-chan struct{}, func(), time.Time, error) {
	h.mu.Lock()
	h.observers++
	h.mu.Unlock()
	h.ready <- struct{}{}
	return h.output, func() { h.mu.Lock(); h.observers--; h.mu.Unlock() }, time.Time{}, nil
}
func (h *waitFixture) Screen(_ string, lines int) (string, error) {
	if lines != 200 {
		panic("unbounded history")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.text, nil
}
func (h *waitFixture) status(s string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tab.Status = s
	close(h.changed)
	h.changed = make(chan struct{})
}
func TestWait(t *testing.T) {
	for _, status := range []string{"idle", "needs"} {
		t.Run(status, func(t *testing.T) {
			h := newWaitFixture(status)
			if r := HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1"}); !r.OK {
				t.Fatalf("%+v", r)
			}
			if h.observers != 0 {
				t.Fatal("observer leaked")
			}
		})
	}
	t.Run("working then idle", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := newWaitFixture("working")
			done := make(chan Reply, 1)
			go func() { done <- HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1"}) }()
			synctest.Wait()
			select {
			case r := <-done:
				t.Fatalf("early %+v", r)
			default:
			}
			h.status("idle")
			synctest.Wait()
			if r := <-done; !r.OK {
				t.Fatalf("%+v", r)
			}
		})
	})
	for _, existing := range []bool{false, true} {
		t.Run("text", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newWaitFixture("working")
				if existing {
					h.text = "done"
				}
				done := make(chan Reply, 1)
				go func() { done <- HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1", For: "done"}) }()
				synctest.Wait()
				if !existing {
					h.mu.Lock()
					h.text = "done"
					h.mu.Unlock()
					h.output <- struct{}{}
					synctest.Wait()
				}
				if r := <-done; !r.OK {
					t.Fatalf("%+v", r)
				}
				if h.observers != 0 {
					t.Fatal("leaked")
				}
			})
		})
	}
	t.Run("timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := newWaitFixture("idle")
			start := time.Now()
			r := HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1", For: "absent", Timeout: 2})
			if r.Error == nil || r.Error.Code != "timeout" || time.Since(start) != 2*time.Second {
				t.Fatalf("%+v elapsed %v", r, time.Since(start))
			}
			if h.observers != 0 {
				t.Fatal("leaked")
			}
		})
	})
	t.Run("quiet resets", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := newWaitFixture("shell")
			done := make(chan Reply, 1)
			go func() { done <- HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1", Quiet: 1500}) }()
			synctest.Wait()
			time.Sleep(time.Second)
			h.output <- struct{}{}
			synctest.Wait()
			time.Sleep(time.Second)
			select {
			case <-done:
				t.Fatal("early quiet")
			default:
			}
			time.Sleep(500 * time.Millisecond)
			synctest.Wait()
			if r := <-done; !r.OK {
				t.Fatalf("%+v", r)
			}
			if h.observers != 0 {
				t.Fatal("leaked")
			}
		})
	})
	t.Run("cancel", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := newWaitFixture("shell")
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan Reply, 1)
			go func() { done <- HandleContext(ctx, h, Request{Op: "wait", Tab: "t1"}) }()
			synctest.Wait()
			cancel()
			synctest.Wait()
			<-done
			if h.observers != 0 {
				t.Fatal("leaked")
			}
		})
	})
}

func TestWaitShellWorkingShell(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newWaitFixture("shell")
		done := make(chan Reply, 1)
		go func() { done <- HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1", Timeout: 5}) }()
		synctest.Wait()
		h.status("working")
		synctest.Wait()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("working considered quiet")
		default:
		}
		h.status("shell")
		synctest.Wait()
		select {
		case r := <-done:
			if !r.OK {
				t.Fatalf("%+v", r)
			}
		default:
			t.Fatal("quiet shell did not return")
		}
	})
}

// An agent with no hook state ("unknown") is waited on by output quiescence,
// like a shell, not returned at once like a hook-reported idle.
func TestWaitUnknownUsesQuiescence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newWaitFixture("unknown")
		done := make(chan Reply, 1)
		go func() { done <- HandleContext(context.Background(), h, Request{Op: "wait", Tab: "t1", Timeout: 5}) }()
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("unknown returned before the output went quiet")
		default:
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		select {
		case r := <-done:
			if !r.OK {
				t.Fatalf("%+v", r)
			}
		default:
			t.Fatal("quiet unknown tab did not return")
		}
	})
}
