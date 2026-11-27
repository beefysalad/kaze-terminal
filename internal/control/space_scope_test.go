package control

import (
	"context"
	"reflect"
	"strings"
	"testing"
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
