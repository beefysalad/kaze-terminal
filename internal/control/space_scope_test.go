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

func TestUnknownCallerRefusedWhenScoped(t *testing.T) {
	for _, op := range []string{"ls", "read", "send", "keys", "interrupt", "name", "label"} {
		h := scopeHost()
		r := Handle(h, Request{Op: op, From: "missing", Tab: "t3", Text: "x", Args: []string{"enter"}, Key: "k", Value: "v"})
		if r.OK || r.Error == nil || r.Error.Code != CodeUnknownTab {
			t.Fatalf("%s: %+v", op, r)
		}
		if h.submitCalls != 0 || h.screenCalls != 0 || len(r.Tabs) != 0 {
			t.Fatalf("%s reached the host: %+v", op, r)
		}
	}
	// The owner (no from) and cross_space mode stay unrestricted.
	if r := Handle(scopeHost(), Request{Op: "ls"}); !r.OK || len(r.Tabs) != 3 {
		t.Fatalf("owner ls: %+v", r)
	}
	if r := HandleContext(context.Background(), scopeHost(), Request{Op: "ls", From: "missing"}, Options{Enabled: true, CrossSpace: true}); !r.OK || len(r.Tabs) != 3 {
		t.Fatalf("cross_space ls: %+v", r)
	}
}

func TestSpaceKeySeparatesSameNamedSpaces(t *testing.T) {
	h := &fakeHost{tabs: []TabInfo{
		{ID: "t1", Space: "project", SpaceKey: "/a/project", Agent: "claude"},
		{ID: "t2", Space: "project", SpaceKey: "/b/project", Agent: "codex"},
		{ID: "t3", Space: "Shell", Agent: "codex"},
		{ID: "t4", Space: "Shell", SpaceKey: "/c/Shell", Agent: "codex"},
	}}
	if r := Handle(h, Request{Op: "ls", From: "t1"}); len(r.Tabs) != 1 || r.Tabs[0].ID != "t1" {
		t.Fatalf("same-named repos shared a space: %+v", r.Tabs)
	}
	requireScopeError(t, Handle(h, Request{Op: "read", From: "t1", Tab: "t2"}), CodeCrossSpace)
	requireScopeError(t, Handle(h, Request{Op: "read", From: "t3", Tab: "t4"}), CodeCrossSpace)
	if h.screenCalls != 0 {
		t.Fatal("target reached")
	}
}

func TestRawSendIsNotTaggedButStillScoped(t *testing.T) {
	h := scopeHost()
	if r := Handle(h, Request{Op: "send", From: "t1", Tab: "t2", Text: "hi", Raw: true}); !r.OK {
		t.Fatalf("%+v", r)
	}
	if got := string(h.input); strings.Contains(got, "<kaze") || !strings.Contains(got, "hi") {
		t.Fatalf("raw send was tagged: %q", got)
	}
	requireScopeError(t, Handle(h, Request{Op: "send", From: "t1", Tab: "t3", Text: "hi", Raw: true}), CodeCrossSpace)
}

type scopeWorkerHost struct {
	workerHost
	names []string
}

func (h *scopeWorkerHost) Name(_ context.Context, id, name string) *Error {
	h.names = append(h.names, id+"="+name)
	return nil
}
func (h *scopeWorkerHost) Label(string, string, string) *Error { return nil }

func newScopeWorkerHost() *scopeWorkerHost {
	h := &scopeWorkerHost{}
	h.tabs = []TabInfo{
		{ID: "t1", Space: "a", Agent: "claude"},
		{ID: "t2", Space: "a", Agent: "codex", Parent: "t1", Role: "worker"},
		{ID: "t3", Space: "b", Agent: "codex", Name: "taken", Parent: "t9", Role: "worker"},
		{ID: "t4", Space: "b", Agent: "claude", Name: "elsewhere"},
	}
	return h
}

func TestKillIsScopedToCallerSpace(t *testing.T) {
	h := newScopeWorkerHost()
	key := h.OwnerKey("t1")
	requireScopeError(t, Handle(h, Request{Op: "kill", From: "t1", CallerKey: key, Tab: "t3"}), CodeCrossSpace)
	r := Handle(h, Request{Op: "kill", From: "t1", CallerKey: key, Tab: "nope"})
	if r.Error == nil || r.Error.Code != CodeUnknownTab || strings.Contains(r.Error.Message, "elsewhere") || strings.Contains(r.Error.Message, "t4") || strings.Contains(r.Error.Message, "taken") {
		t.Fatalf("unknown kill leaked other spaces: %+v", r)
	}
	// "codex" is also a worker in space b: the caller's own worker must win.
	if r = Handle(h, Request{Op: "kill", From: "t1", CallerKey: key, Tab: "codex"}); !r.OK || r.Text != "t2" {
		t.Fatalf("kill codex: %+v", r)
	}
}

func TestNameCollisionIsPerSpace(t *testing.T) {
	h := newScopeWorkerHost()
	key := h.OwnerKey("t1")
	// "taken" exists only in space b: spawning and renaming it in space a must work.
	if r := Handle(h, Request{Op: "spawn", From: "t1", CallerKey: key, Agent: "claude", Task: "review", Name: "taken"}); !r.OK {
		t.Fatalf("spawn: %+v", r)
	}
	if r := Handle(h, Request{Op: "name", From: "t1", Tab: "t2", Name: "taken"}); !r.OK {
		t.Fatalf("rename collided across spaces: %+v", r)
	}
	// A collision in the caller's own space is still refused.
	h.tabs[0].Name = "mine"
	if r := Handle(h, Request{Op: "spawn", From: "t1", CallerKey: key, Agent: "claude", Task: "review", Name: "mine"}); r.OK {
		t.Fatalf("same-space collision accepted: %+v", r)
	}
}

func TestOwnerRenameChecksOnlyTargetSpace(t *testing.T) {
	h := &scopeWorkerHost{}
	h.tabs = []TabInfo{
		{ID: "t1", Space: "a", Agent: "claude"},
		{ID: "t2", Space: "b", Agent: "codex", Name: "reviewer"},
	}
	// The owner sees every tab, but names only have to be unique inside the target's space.
	if r := Handle(h, Request{Op: "name", Tab: "t1", Name: "reviewer"}); !r.OK {
		t.Fatalf("owner rename refused by another space's name: %+v", r)
	}
	h.tabs = append(h.tabs, TabInfo{ID: "t3", Space: "a", Agent: "codex", Name: "reviewer"})
	if r := Handle(h, Request{Op: "name", Tab: "t1", Name: "reviewer"}); r.OK {
		t.Fatalf("same-space collision accepted: %+v", r)
	}
}

func TestCrossSpaceWhenOnlyOtherSpacesMatch(t *testing.T) {
	h := &fakeHost{tabs: []TabInfo{
		{ID: "t1", Space: "a", Agent: "claude"},
		{ID: "t2", Space: "b", Agent: "codex", Name: "twin"},
		{ID: "t3", Space: "c", Agent: "codex", Name: "twin"},
	}}
	for _, address := range []string{"codex", "twin", "t2"} {
		requireScopeError(t, Handle(h, Request{Op: "read", From: "t1", Tab: address}), CodeCrossSpace)
	}
	if r := Handle(h, Request{Op: "read", From: "t1", Tab: "nothing"}); r.Error == nil || r.Error.Code != CodeUnknownTab {
		t.Fatalf("%+v", r)
	}
	if h.screenCalls != 0 {
		t.Fatal("target reached")
	}
}

type callerHost struct {
	scopeWorkerHost
	from string
}

func (h *callerHost) Name(ctx context.Context, id, name string) *Error {
	h.from = CallerFrom(ctx)
	return nil
}

func TestRenameCarriesCallerToTheHost(t *testing.T) {
	h := &callerHost{}
	h.tabs = []TabInfo{{ID: "t1", Space: "a"}, {ID: "t2", Space: "a"}}
	if r := Handle(h, Request{Op: "name", From: "t1", Tab: "t2", Name: "x"}); !r.OK || h.from != "t1" {
		t.Fatalf("%+v from %q", r, h.from)
	}
	if r := Handle(h, Request{Op: "name", Tab: "t2", Name: "y"}); !r.OK || h.from != "" {
		t.Fatalf("owner: %+v from %q", r, h.from)
	}
}
