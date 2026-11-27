package ui

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

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

func TestControlSnapshotUnresolvedLocationIsIsolated(t *testing.T) {
	win, _ := sideWindow(t, true)
	a1, b1, s1, s2, c1, c2 := openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, ""), openIn(t, win, "")
	win.locs = map[tabs.TabID]locEntry{
		a1.ID: {cwd: "/a", pending: true}, // repository lookup still running
		b1.ID: {cwd: "/b", pending: true},
		s1.ID: {cwd: "/tmp/x"}, // looked up: genuinely not in a repository
		s2.ID: {cwd: "/tmp/y"},
		c1.ID: {cwd: "/c", repo: "C", repoName: "c", pending: true}, // known repo, refreshing
		c2.ID: {cwd: "/c/sub", repo: "C", repoName: "c"},
	}
	win.publishControl()
	id := func(tab *tabs.Tab) string { return "t" + strconv.Itoa(int(tab.ID)) }
	ids := func(from *tabs.Tab) []string {
		var out []string
		for _, tab := range ctl.Handle(&win.control, ctl.Request{Op: "ls", From: id(from)}).Tabs {
			out = append(out, tab.ID)
		}
		return out
	}
	if got := ids(a1); len(got) != 1 || got[0] != id(a1) {
		t.Fatalf("a tab whose repository is unknown must see only itself, saw %v", got)
	}
	if got := ids(s1); len(got) != 2 {
		t.Fatalf("looked-up non-repository tabs share the Shell space, saw %v", got)
	}
	if got := ids(c1); len(got) != 2 {
		t.Fatalf("a refreshing tab with a known repository keeps its space, saw %v", got)
	}
}

func TestControlRenameReauthorizesAtApply(t *testing.T) {
	win, _ := sideWindow(t, true)
	a1, a2 := openIn(t, win, ""), openIn(t, win, "")
	win.locs = map[tabs.TabID]locEntry{
		a1.ID: {cwd: "/a", repo: "A", repoName: "a"},
		a2.ID: {cwd: "/a/sub", repo: "A", repoName: "a"},
	}
	win.publishControl()
	from, target := "t"+strconv.Itoa(int(a1.ID)), "t"+strconv.Itoa(int(a2.ID))
	reply := make(chan ctl.Reply, 1)
	go func() {
		reply <- ctl.HandleContext(t.Context(), &win.control, ctl.Request{Op: "name", From: from, Tab: target, Name: "moved"}, ctl.Options{Enabled: true})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		win.control.renameMu.Lock()
		queued := len(win.control.renames)
		win.control.renameMu.Unlock()
		if queued == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rename never queued")
		}
		time.Sleep(time.Millisecond)
	}
	// The target changes project after the handler approved the rename.
	win.locs[a2.ID] = locEntry{cwd: "/b", repo: "B", repoName: "b"}
	win.publishControl()
	win.drainControl()
	r := <-reply
	if r.Error == nil || r.Error.Code != ctl.CodeCrossSpace {
		t.Fatalf("%+v", r)
	}
	if win.names[a2.ID] != "" {
		t.Fatalf("rename applied across spaces: %q", win.names[a2.ID])
	}
}
