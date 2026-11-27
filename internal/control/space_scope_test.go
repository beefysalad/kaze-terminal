package control

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func scopeHost() *fakeHost {
	return &fakeHost{tabs: []TabInfo{
		{ID: "t1", Space: "a", Agent: "claude", Status: "idle"},
		{ID: "t2", Space: "a", Agent: "codex", Status: "idle"},
		{ID: "t3", Space: "b", Agent: "codex", Name: "other", Status: "idle"},
	}}
}

func requireScopeError(t *testing.T, r Reply, code string) {
	t.Helper()
	if r.OK || r.Error == nil || r.Error.Code != code {
		t.Fatalf("reply %+v, want %s", r, code)
	}
}

func TestLsScopedToCallerSpace(t *testing.T) {
	for _, tc := range []struct {
		from  string
		cross bool
		want  []string
	}{
		{"t1", false, []string{"t1", "t2"}}, {"", false, []string{"t1", "t2", "t3"}},
		{"t1", true, []string{"t1", "t2", "t3"}},
	} {
		r := HandleContext(context.Background(), scopeHost(), Request{Op: "ls", From: tc.from}, Options{Enabled: true, CrossSpace: tc.cross})
		var ids []string
		for _, tab := range r.Tabs {
			ids = append(ids, tab.ID)
		}
		if !r.OK || r.Error != nil || !reflect.DeepEqual(ids, tc.want) {
			t.Fatalf("%+v: %+v", tc, r)
		}
	}
}

func TestExplicitIdAcrossSpacesRefused(t *testing.T) {
	for _, op := range []string{"send", "read", "keys", "interrupt"} {
		t.Run(op, func(t *testing.T) {
			h := scopeHost()
			r := Handle(h, Request{Op: op, From: "t1", Tab: "t3", Text: "hello", Args: []string{"enter"}})
			requireScopeError(t, r, CodeCrossSpace)
			if h.submitCalls != 0 || h.screenCalls != 0 {
				t.Fatalf("target reached: %+v", h)
			}
		})
	}
}

func TestCrossSpaceErrorDoesNotLeak(t *testing.T) {
	for _, address := range []string{"t3", "other", "codex"} {
		h := scopeHost()
		h.tabs = append(h.tabs[:1], h.tabs[2:]...)
		r := Handle(h, Request{Op: "read", From: "t1", Tab: address})
		requireScopeError(t, r, CodeCrossSpace)
		if r.Error.Message != "that tab is in another space; cross-space control is off" {
			t.Fatal(r.Error)
		}
		// "other" and "b" occur in the fixed sentence itself. Reject the actual
		// target's descriptive fields with distinct sentinel values instead.
		h.tabs[1].Name = "private-name"
		h.tabs[1].Agent = "private-agent"
		h.tabs[1].Space = "private-space"
		h.tabs[1].Folder = "/private-folder"
		r = Handle(h, Request{Op: "read", From: "t1", Tab: "t3"})
		requireScopeError(t, r, CodeCrossSpace)
		for _, secret := range []string{"t3", "private-name", "private-agent", "private-space", "/private-folder"} {
			if strings.Contains(r.Error.Message, secret) {
				t.Fatalf("leaked %q: %v", secret, r.Error)
			}
		}
	}
}

func TestAgentNameNeverFallsBack(t *testing.T) {
	h := scopeHost()
	h.tabs = append(h.tabs[:1], h.tabs[2:]...)
	r := Handle(h, Request{Op: "read", From: "t1", Tab: "codex"})
	requireScopeError(t, r, CodeCrossSpace)
	if h.screenCalls != 0 {
		t.Fatal("read another space")
	}
}

func TestNameInTwoSpacesPrefersOwn(t *testing.T) {
	h := scopeHost()
	h.tabs = append(h.tabs, TabInfo{ID: "t4", Space: "a", Name: "other"})
	r := Handle(h, Request{Op: "read", From: "t1", Tab: "other"})
	if !r.OK || r.Error != nil || h.id != "t4" {
		t.Fatalf("reply %+v target %q", r, h.id)
	}
}

func TestUnknownTabListsOnlyOwnSpace(t *testing.T) {
	h := scopeHost()
	r := Handle(h, Request{Op: "read", From: "t1", Tab: "missing"})
	requireScopeError(t, r, CodeUnknownTab)
	for _, id := range []string{"t1", "t2"} {
		if !strings.Contains(r.Error.Message, id) {
			t.Fatal(r.Error)
		}
	}
	for _, secret := range []string{"t3", "other", "space b"} {
		if strings.Contains(r.Error.Message, secret) {
			t.Fatal(r.Error)
		}
	}
}

func TestUnknownCallerIsUnrestricted(t *testing.T) {
	for _, from := range []string{"", "missing"} {
		h := scopeHost()
		r := Handle(h, Request{Op: "read", From: from, Tab: "t3"})
		if !r.OK || r.Error != nil || h.id != "t3" {
			t.Fatalf("reply %+v target %q", r, h.id)
		}
		r = Handle(h, Request{Op: "ls", From: from})
		if !r.OK || len(r.Tabs) != 3 {
			t.Fatalf("reply %+v", r)
		}
	}
}

func TestEmptySpaceTabsShareScope(t *testing.T) {
	h := scopeHost()
	h.tabs[0].Space = ""
	h.tabs[1].Space = ""
	r := Handle(h, Request{Op: "read", From: "t1", Tab: "t2"})
	if !r.OK || r.Error != nil || h.id != "t2" {
		t.Fatalf("reply %+v target %q", r, h.id)
	}
	r = Handle(h, Request{Op: "ls", From: "t1"})
	if !r.OK || len(r.Tabs) != 2 {
		t.Fatalf("reply %+v", r)
	}
}

func TestCrossSpaceOnKeepsWarning(t *testing.T) {
	h := scopeHost()
	r := HandleContext(context.Background(), h, Request{Op: "send", From: "t1", Tab: "t3", Text: "hello"}, Options{Enabled: true, AgentTag: true, CrossSpace: true})
	if !r.OK || r.Error != nil || h.id != "t3" || !strings.Contains(r.Warning, "not your space") {
		t.Fatalf("reply %+v target %q", r, h.id)
	}
}

func TestWaitAcrossSpacesRefused(t *testing.T) {
	h := &spaceHost{fakeHost: *scopeHost()}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := HandleContext(ctx, h, Request{Op: "wait", From: "t1", Tab: "t3", Timeout: 1})
	requireScopeError(t, r, CodeCrossSpace)
	if h.screenCalls != 0 {
		t.Fatal("read another space")
	}
}

func TestWaitSameSpaceUnchanged(t *testing.T) {
	h := &spaceHost{fakeHost: *scopeHost()}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := HandleContext(ctx, h, Request{Op: "wait", From: "t1", Tab: "codex", Timeout: 1})
	if !r.OK || r.Error != nil {
		t.Fatalf("reply %+v", r)
	}
}

type scopeMetadataHost struct {
	fakeHost
	names, labels int
}

func (h *scopeMetadataHost) Name(context.Context, string, string) *Error { h.names++; return nil }
func (h *scopeMetadataHost) Label(string, string, string) *Error         { h.labels++; return nil }

func TestNameAndLabelAcrossSpacesRefused(t *testing.T) {
	for _, op := range []string{"name", "label"} {
		for _, address := range []string{"t3", "other"} {
			t.Run(op+"/"+address, func(t *testing.T) {
				h := &scopeMetadataHost{fakeHost: *scopeHost()}
				r := Handle(h, Request{Op: op, From: "t1", Tab: address, Name: "renamed", Key: "topic", Value: "review"})
				requireScopeError(t, r, CodeCrossSpace)
				if h.names+h.labels != 0 {
					t.Fatal("metadata changed in another space")
				}
			})
		}
	}
}

type scopeChangingHost struct {
	fakeHost
	snapshots int
	move      bool
}

func (h *scopeChangingHost) StatusChanges() ([]TabInfo, <-chan struct{}) {
	h.snapshots++
	if h.snapshots == 2 {
		if h.move {
			h.tabs[1].Space = "b"
			h.tabs[1].Status = "idle"
		} else {
			h.tabs = h.tabs[:1]
		}
	}
	changed := make(chan struct{})
	// Exactly one immediate wakeup. A regression must then end by its deadline.
	if h.snapshots == 1 {
		close(changed)
	}
	return h.tabs, changed
}
func (h *scopeChangingHost) OutputChanges(string) (<-chan struct{}, func(), time.Time, error) {
	panic("working agent must not observe output")
}

func TestWaitTargetClosesMidWait(t *testing.T) {
	testWaitTargetChange(t, false, CodeUnknownTab)
}
func TestWaitTargetMovesAcrossSpacesMidWait(t *testing.T) {
	testWaitTargetChange(t, true, CodeCrossSpace)
}
func testWaitTargetChange(t *testing.T, move bool, code string) {
	t.Helper()
	h := &scopeChangingHost{fakeHost: *scopeHost(), move: move}
	h.tabs[1].Status = "working"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := HandleContext(ctx, h, Request{Op: "wait", From: "t1", Tab: "t2", Timeout: 1})
	requireScopeError(t, r, code)
	if h.snapshots != 2 {
		t.Fatalf("snapshots %d", h.snapshots)
	}
}

func TestWaitChildrenUnaffected(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		h := &childrenHost{tabs: []TabInfo{
			{ID: "t1", Space: "a"},
			{ID: "t2", Space: "a", Parent: "t1", Role: "worker", Status: "idle"},
			{ID: "t3", Space: "b", Parent: "t9", Role: "worker", Status: "working"},
		}, changed: make(chan struct{})}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		r := HandleContext(ctx, h, Request{Op: "wait", From: "t1", Children: mode, CallerKey: "secret", Timeout: 1})
		cancel()
		if !r.OK || r.Error != nil || len(r.Tabs) != 1 || r.Tabs[0].ID != "t2" {
			t.Fatalf("%s: %+v", mode, r)
		}
	}
	h := &spaceHost{fakeHost: *scopeHost()}
	h.tabs[1].Parent = "t1"
	h.tabs[1].Role = "worker"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := HandleContext(ctx, h, Request{Op: "wait", From: "t1", Tab: "t2", Timeout: 1})
	if !r.OK || r.Error != nil {
		t.Fatalf("worker wait %+v", r)
	}
}
