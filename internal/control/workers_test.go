package control

import (
	"context"
	"strings"
	"testing"
)

type workerHost struct {
	ownerKeyHost
	calls int
}

func (h *workerHost) Spawn(_ context.Context, r Request) Reply {
	h.calls++
	return Reply{OK: true, Text: "started t3 (claude)"}
}
func (h *workerHost) Kill(_ context.Context, from, id string) Reply {
	h.calls++
	return Reply{OK: true, Text: id}
}
func TestWorkerDispatch(t *testing.T) {
	h := &workerHost{}
	h.tabs = []TabInfo{{ID: "t1"}, {ID: "t2"}, {ID: "t3", Parent: "t1", Role: "worker"}}
	for _, tc := range []struct {
		op, tab string
		ok      bool
	}{{"spawn", "", true}, {"kill", "t3", true}, {"kill", "t2", false}, {"kill", "t1", false}} {
		r := Handle(h, Request{Op: tc.op, Tab: tc.tab, From: "t1", CallerKey: h.OwnerKey("t1"), Agent: "claude", Task: "review"})
		if r.OK != tc.ok {
			t.Fatalf("%s %s: %+v", tc.op, tc.tab, r)
		}
		if !tc.ok && r.Error.Code != CodeNotAllowed {
			t.Fatalf("%+v", r)
		}
	}
	if h.calls != 2 {
		t.Fatal(h.calls)
	}
}

func TestSpawnRejectsMalformedRequestBeforeHost(t *testing.T) {
	for _, tc := range []struct {
		agent, task, mode, name string
		code                    string
	}{
		{"../other", "review", "", "", CodeUnknownAgent},
		{"", "review", "", "", CodeUnknownAgent},
		{strings.Repeat("a", 33), "review", "", "", CodeUnknownAgent},
		{"claude", "", "", "", CodeInvalidRequest},
		{"claude", "review", "bypass", "", CodeInvalidRequest},
		{"claude", "review", "", "bad\nname", CodeInvalidRequest},
		{"claude", strings.Repeat("x", (16<<10)+1), "", "", CodeTextTooLarge},
	} {
		h := &workerHost{}
		r := Handle(h, Request{Op: "spawn", From: "t1", CallerKey: h.OwnerKey("t1"), Agent: tc.agent, Task: tc.task, Mode: tc.mode, Name: tc.name})
		if r.Error == nil || r.Error.Code != tc.code || h.calls != 0 {
			t.Fatalf("agent %s: %+v calls %d", tc.agent, r, h.calls)
		}
	}
}
