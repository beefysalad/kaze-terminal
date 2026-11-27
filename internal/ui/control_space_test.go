package ui

import (
	"encoding/json"
	"strconv"
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

func TestControlSnapshotSpaceIdentity(t *testing.T) {
	win, _ := sideWindow(t, true)
	a1, b1, w := openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, "")
	w.Parent, w.Role = a1.ID, "worker" // a fresh worker has no location of its own yet
	win.locs = map[tabs.TabID]locEntry{
		a1.ID: {cwd: "/a/project", repo: "/a/project/.git", repoName: "project"},
		b1.ID: {cwd: "/b/project", repo: "/b/project/.git", repoName: "project"},
	}
	win.publishControl()
	byID := map[string]ctl.TabInfo{}
	for _, tab := range win.control.Tabs() {
		byID[tab.ID] = tab
	}
	ida, idb, idw := "t"+strconv.Itoa(int(a1.ID)), "t"+strconv.Itoa(int(b1.ID)), "t"+strconv.Itoa(int(w.ID))
	if byID[ida].Space != "project" || byID[idb].Space != "project" {
		t.Fatalf("names: %+v", byID)
	}
	if byID[ida].SpaceKey == "" || byID[ida].SpaceKey == byID[idb].SpaceKey {
		t.Fatalf("same-named repos must have different space keys: %q vs %q", byID[ida].SpaceKey, byID[idb].SpaceKey)
	}
	if got := byID[idw]; got.Space != byID[ida].Space || got.SpaceKey != byID[ida].SpaceKey {
		t.Fatalf("worker must share its parent's space: %+v vs %+v", got, byID[ida])
	}
	r := ctl.Handle(&win.control, ctl.Request{Op: "ls", From: ida})
	if len(r.Tabs) != 2 { // the parent and its worker, not the other project
		t.Fatalf("scoped ls: %+v", r.Tabs)
	}
}
