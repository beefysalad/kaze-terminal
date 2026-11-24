package ui

import (
	"context"
	"encoding/json"
	"fmt"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestControlWaitPublication(t *testing.T) {
	f := newControlSpike(t)
	_, changed := f.win.control.StatusChanges()
	f.win.publishControl()
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("publish did not broadcast")
	}
	tabs, next := f.win.control.StatusChanges()
	if len(tabs) != 2 {
		t.Fatal("missing snapshot")
	}
	select {
	case <-next:
		t.Fatal("new channel already closed")
	default:
	}
}
func TestControlWaitEndToEnd(t *testing.T) {
	f := newControlSpike(t)
	id := fmt.Sprintf("t%d", f.b.ID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// A separate observer provides a bounded readiness handshake for real output.
	wake, remove, _, err := f.win.control.OutputChanges(id)
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	result := make(chan ctl.Reply, 1)
	go func() { result <- f.request(t, ctl.Request{Op: "wait", Tab: id, For: "finished", Timeout: 1}) }()
	waitControlCondition(t, func() bool { return f.b.Session.OutputObserverCount() == 2 })
	written := make(chan error, 1)
	go func() { _, err := f.ptys[1].w.Write([]byte("finished")); written <- err }()
	select {
	case err := <-written:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("PTY blocked")
	}
	select {
	case <-wake:
	case <-ctx.Done():
		t.Fatal("no output wake")
	}
	select {
	case reply := <-result:
		if !reply.OK {
			t.Fatalf("%+v", reply)
		}
	case <-ctx.Done():
		t.Fatal("wait blocked")
	}
	remove()
	if n := f.b.Session.OutputObserverCount(); n != 0 {
		t.Fatalf("leaked %d observers", n)
	}
}

// Bounded sampling is test instrumentation only. Production wait has no poll.
func waitControlCondition(t *testing.T, check func() bool) {
	t.Helper()
	limit := time.Now().Add(time.Second)
	for !check() {
		if time.Now().After(limit) {
			t.Fatal("condition deadline")
		}
		time.Sleep(time.Millisecond)
	}
}
func controlWaitWorkers() int {
	buf := make([]byte, 256<<10)
	n := runtime.Stack(buf, true)
	count := 0
	for _, stack := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(stack, "internal/control.wait(") || strings.Contains(stack, "internal/notify.(*Server).handle(") || strings.Contains(stack, "internal/notify.(*Server).handle.func") {
			count++
		}
	}
	return count
}
func TestControlWaitDisconnectCleanup(t *testing.T) {
	f := newControlSpike(t)
	id := fmt.Sprintf("t%d", f.b.ID)
	before := runtime.NumGoroutine()
	beforeWorkers := controlWaitWorkers()
	c, err := net.DialTimeout("tcp", f.srv.Addr(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(ctl.Request{Op: "wait", Tab: id, For: "absent", Timeout: 10, Token: f.srv.Token()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Write(wire); err != nil {
		t.Fatal(err)
	}
	waitControlCondition(t, func() bool { return f.b.Session.OutputObserverCount() == 1 })
	active := controlWaitWorkers()
	if active <= beforeWorkers {
		t.Fatal("no active wait workers measured")
	}
	c.Close()
	waitControlCondition(t, func() bool { return f.b.Session.OutputObserverCount() == 0 && controlWaitWorkers() == beforeWorkers })
	after := runtime.NumGoroutine()
	t.Logf("observers before=0 active=1 after=%d; control workers before=%d active=%d after=%d; total goroutines before=%d after=%d", f.b.Session.OutputObserverCount(), beforeWorkers, active, controlWaitWorkers(), before, after)
}
