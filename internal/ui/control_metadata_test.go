package ui

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"gioui.org/io/input"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func TestControlNameFrame(t *testing.T) {
	win, _ := sideWindow(t, true)
	tb := openIn(t, win, "")
	win.initControl()
	wake := make(chan struct{}, 4)
	win.control.wake = func() { wake <- struct{}{} }
	win.publishControl()
	done := make(chan ctl.Reply, 1)
	go func() { done <- ctl.Handle(&win.control, ctl.Request{Op: "name", Tab: "t1", Name: " review "}) }()
	select {
	case <-wake:
	case <-time.After(time.Second):
		t.Fatal("no frame requested")
	}
	if win.control.Tabs()[0].Name != "" || win.names[tb.ID] != "" {
		t.Fatal("rename applied before frame")
	}
	select {
	case r := <-done:
		t.Fatalf("replied before apply: %+v", r)
	default:
	}
	sideFrame(t, win, new(input.Router))
	select {
	case r := <-done:
		if !r.OK {
			t.Fatalf("%+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("no reply")
	}
	row, ok := rowByID(win.side.cache.base, tb.ID)
	if !ok || row.Title != "review" || win.control.Tabs()[0].Name != "review" {
		t.Fatalf("model %+v snapshot %+v", row, win.control.Tabs())
	}
	// A fixed-title workspace member must use the same visible rename.
	tb.FixedTitle = "member"
	win.publishControl()
	go func() { done <- ctl.Handle(&win.control, ctl.Request{Op: "name", Tab: "t1", Name: "renamed member"}) }()
	select {
	case <-wake:
	case <-time.After(time.Second):
		t.Fatal("no frame")
	}
	sideFrame(t, win, new(input.Router))
	select {
	case r := <-done:
		if !r.OK {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("no reply")
	}
	row, _ = rowByID(win.side.cache.base, tb.ID)
	if row.Title != "renamed member" {
		t.Fatalf("fixed title model %+v", row)
	}
}
func TestControlNameRechecksUniqueness(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	openIn(t, win, "")
	win.initControl()
	wakes := make(chan struct{}, 2)
	win.control.wake = func() { wakes <- struct{}{} }
	win.publishControl()
	replies := make(chan ctl.Reply, 2)
	for _, id := range []string{"t1", "t2"} {
		go func() { replies <- ctl.Handle(&win.control, ctl.Request{Op: "name", Tab: id, Name: "same"}) }()
	}
	for range 2 {
		select {
		case <-wakes:
		case <-time.After(time.Second):
			t.Fatal("queue timeout")
		}
	}
	sideFrame(t, win, new(input.Router))
	successes, duplicates := 0, 0
	for range 2 {
		select {
		case r := <-replies:
			if r.OK {
				successes++
			} else if r.Error != nil && r.Error.Code == ctl.CodeAmbiguousTab {
				duplicates++
			}
		case <-time.After(time.Second):
			t.Fatal("reply timeout")
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatalf("success %d duplicate %d", successes, duplicates)
	}
}
func TestControlNameTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := &controlHost{}
		start := time.Now()
		if e := h.Name(context.Background(), "t1", "review"); e == nil || e.Code != "timeout" {
			t.Fatalf("%v", e)
		}
		if time.Since(start) != 2*time.Second {
			t.Fatal("clock")
		}
		h.renameMu.Lock()
		n := len(h.renames)
		h.renameMu.Unlock()
		if n != 0 {
			t.Fatal("timed out rename retained")
		}
	})
}
func TestControlLabels(t *testing.T) {
	win, _ := sideWindow(t, true)
	tb := openIn(t, win, "")
	win.publishControl()
	old := win.control.Tabs()
	for i := range 8 {
		r := ctl.Handle(&win.control, ctl.Request{Op: "label", Tab: "t1", Key: fmt.Sprint("k", i), Value: "v"})
		if !r.OK {
			t.Fatalf("%+v", r)
		}
	}
	if old[0].Labels != nil {
		t.Fatal("old snapshot changed")
	}
	r := ctl.Handle(&win.control, ctl.Request{Op: "label", Tab: "t1", Key: "ninth", Value: "v"})
	if r.Error == nil || r.Error.Code != ctl.CodeInvalidRequest {
		t.Fatalf("%+v", r)
	}
	r = ctl.Handle(&win.control, ctl.Request{Op: "label", Tab: "t1", Key: "k0", Value: "updated"})
	if !r.OK {
		t.Fatal(r)
	}
	old = win.control.Tabs()
	win.publishControl()
	r = ctl.Handle(&win.control, ctl.Request{Op: "ls"})
	if len(r.Tabs[0].Labels) != 8 || r.Tabs[0].Labels["k0"] != "updated" {
		t.Fatal(r)
	}
	if e := win.control.Label("t1", "bad key", "v"); e == nil {
		t.Fatal("direct invalid label")
	}
	win.mgr.Close(tb.ID)
	win.publishControl()
	if e := win.control.Label("t1", "k", "v"); e == nil || e.Code != ctl.CodeUnknownTab {
		t.Fatalf("closed label %v", e)
	}
	if len(old[0].Labels) != 8 {
		t.Fatal("old labels mutated")
	}
}

func TestControlDisabledKeepsHooks(t *testing.T) {
	f := newControlSpike(t)
	f.srv.SetControlContext(controlContextAdapter(&f.win.control, ctl.Options{}))
	for _, op := range []string{"ls", "read", "send", "keys", "interrupt", "wait", "name", "label", "bogus"} {
		r := f.request(t, ctl.Request{Op: op, Tab: "t2"})
		if r.Error == nil || r.Error.Code != ctl.CodeDisabled {
			t.Fatalf("%s: %+v", op, r)
		}
	}
	f.win.liveMgr.Store(f.win.mgr)
	if e := notify.Send(f.srv.Addr(), f.srv.Token(), notify.Msg{Tab: int(f.b.ID), Kind: notify.KindPrompt, Agent: "codex"}, time.Second); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for f.win.inbox.size() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("hook lost with control disabled")
		}
		time.Sleep(time.Millisecond)
	}
	sideFrame(t, f.win, new(input.Router))
	if got := f.win.control.Tabs()[1].Status; got != "working" {
		t.Fatalf("hook status %q", got)
	}
	// Re-enabling only control leaves the same hook listener and state intact.
	f.srv.SetControlContext(controlContextAdapter(&f.win.control, ctl.Options{Enabled: true, AgentTag: false}))
	if r := f.request(t, ctl.Request{Op: "ls"}); !r.OK {
		t.Fatal(r)
	}
	r := f.request(t, ctl.Request{Op: "send", Tab: "t2", From: "t1", Text: "hello"})
	if !r.OK || r.Warning != "busy" {
		t.Fatal(r)
	}
	for _, want := range []string{"hello", "\r"} {
		select {
		case got := <-f.ptys[1].writes:
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("write timeout")
		}
	}
}
