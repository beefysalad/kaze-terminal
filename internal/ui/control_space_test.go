package ui

import (
	"encoding/json"
	"testing"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestControlCrossSpaceOption(t *testing.T) {
	win, _ := sideWindow(t, true)
	a1, a2, b1 := openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, "")
	win.locs = map[tabs.TabID]locEntry{
		a1.ID: {cwd: "/work/alpha", repo: "A", repoName: "alpha"},
		a2.ID: {cwd: "/work/alpha/sub", repo: "A", repoName: "alpha"},
		b1.ID: {cwd: "/work/beta", repo: "B", repoName: "beta"},
	}
	win.publishControl()
	call := func(cross bool, req ctl.Request) ctl.Reply {
		raw, _ := json.Marshal(req)
		return controlContextAdapter(&win.control, ctl.Options{Enabled: true, AgentTag: true, CrossSpace: cross})(t.Context(), raw).(ctl.Reply)
	}
	if got := call(false, ctl.Request{Op: "ls", From: "t1"}).Tabs; len(got) != 2 {
		t.Fatalf("scoped ls: %+v", got)
	}
	r := call(false, ctl.Request{Op: "read", Tab: "t3", From: "t1"})
	if r.Error == nil || r.Error.Code != ctl.CodeCrossSpace {
		t.Fatalf("cross-space read: %+v", r)
	}
	if got := call(true, ctl.Request{Op: "ls", From: "t1"}).Tabs; len(got) != 3 {
		t.Fatalf("cross_space ls: %+v", got)
	}
	if r := call(true, ctl.Request{Op: "read", Tab: "t3", From: "t1"}); !r.OK || r.Warning == "" {
		t.Fatalf("cross_space read: %+v", r)
	}
	if got := call(false, ctl.Request{Op: "ls"}).Tabs; len(got) != 3 {
		t.Fatalf("owner ls: %+v", got)
	}
}
