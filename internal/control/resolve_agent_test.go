package control

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResolveFallsBackToAUniqueAgentName(t *testing.T) {
	tabs := []TabInfo{
		{ID: "t1", Agent: "claude", Name: "main"},
		{ID: "t2", Agent: "codex"},
		{ID: "t3", Agent: "claude"},
	}
	// A tab id or an exact name always wins over an agent name.
	if tab, err := Resolve(tabs, "t2"); err != nil || tab.ID != "t2" {
		t.Fatalf("id: %v %v", tab, err)
	}
	if tab, err := Resolve(tabs, "main"); err != nil || tab.ID != "t1" {
		t.Fatalf("name: %v %v", tab, err)
	}
	// Exactly one tab runs codex: the agent name addresses it, in any case.
	for _, addr := range []string{"codex", "Codex", "CODEX"} {
		if tab, err := Resolve(tabs, addr); err != nil || tab.ID != "t2" {
			t.Fatalf("%s: %v %v", addr, tab, err)
		}
	}
	// Two tabs run claude: ambiguous, and the message names the candidates.
	_, err := Resolve(tabs, "claude")
	if err == nil || err.Code != CodeAmbiguousTab || !strings.Contains(err.Message, "t1") || !strings.Contains(err.Message, "t3") {
		t.Fatalf("claude: %+v", err)
	}
}

func TestResolveUnknownTabListsWhatExists(t *testing.T) {
	tabs := []TabInfo{{ID: "t1", Agent: "claude", Name: "main"}, {ID: "t2", Agent: "codex"}}
	_, err := Resolve(tabs, "gemini")
	if err == nil || err.Code != CodeUnknownTab {
		t.Fatalf("%+v", err)
	}
	for _, want := range []string{`"gemini"`, "t1", "main", "t2", "codex"} {
		if !strings.Contains(err.Message, want) {
			t.Fatalf("message %q lacks %q", err.Message, want)
		}
	}
	if _, err := Resolve(nil, "codex"); err == nil || err.Code != CodeUnknownTab {
		t.Fatalf("no tabs: %+v", err)
	}
}

func TestSendReplyNamesTheTarget(t *testing.T) {
	host := &fakeHost{tabs: []TabInfo{{ID: "t2", Agent: "codex", Status: "idle"}}}
	reply := Handle(host, Request{Op: "send", Tab: "codex", Text: "hi"})
	if !reply.OK || len(reply.Tabs) != 1 || reply.Tabs[0].ID != "t2" {
		t.Fatalf("%+v", reply)
	}
}

func TestResolveFromSpaces(t *testing.T) {
	tabs := []TabInfo{{ID: "t1", Agent: "claude", Space: "A"}, {ID: "t2", Agent: "codex", Space: "A"}, {ID: "t7", Agent: "codex", Space: "B"}}
	for _, tc := range []struct {
		address, from, want string
		code                string
	}{
		{"codex", "t1", "t2", ""}, {"CODEX", "t1", "t2", ""}, {"t7", "t1", "t7", ""},
		{"codex", "", "", CodeAmbiguousTab}, {"codex", "missing", "", CodeAmbiguousTab},
	} {
		got, e := ResolveFrom(tabs, tc.address, tc.from)
		if tc.code != "" {
			if e == nil || e.Code != tc.code {
				t.Fatalf("%+v: %v", tc, e)
			}
		} else if e != nil || got.ID != tc.want {
			t.Fatalf("%+v: %v %v", tc, got, e)
		}
	}
	// Exact names still take priority over an agent, even in another space.
	tabs[2].Name = "codex"
	if got, e := ResolveFrom(tabs, "codex", "t1"); e != nil || got.ID != "t7" {
		t.Fatalf("name priority: %v %v", got, e)
	}
	tabs[2].Name = ""
	tabs[1].Agent = ""
	if got, e := ResolveFrom(tabs, "codex", "t1"); e != nil || got.ID != "t7" {
		t.Fatalf("other space only: %v %v", got, e)
	}
	tabs[1].Agent = "codex"
	tabs = append(tabs, TabInfo{ID: "t3", Agent: "codex", Space: "A"})
	if _, e := ResolveFrom(tabs, "codex", "t1"); e == nil || e.Code != CodeAmbiguousTab {
		t.Fatalf("multiple local: %v", e)
	}
	_, e := Resolve(tabs, "codex")
	for _, part := range []string{"t2 (codex, space A)", "t7 (codex, space B)"} {
		if e == nil || !strings.Contains(e.Message, part) {
			t.Fatalf("missing %q: %v", part, e)
		}
	}
}

// All terminal operations go to this in-memory fake, never a live tab.
func TestHandleSpaceSafety(t *testing.T) {
	tabs := []TabInfo{{ID: "t1", Agent: "claude", Space: "kaze-terminal", Status: "idle"}, {ID: "t2", Agent: "codex", Space: "kaze-terminal", Status: "idle"}, {ID: "t7", Agent: "codex", Space: "other-api", Status: "idle"}}
	const warning = "target t7 is in space other-api, not your space kaze-terminal; use it only if the owner named that tab"
	for _, op := range []string{"send", "read", "keys", "interrupt", "wait"} {
		for _, tc := range []struct{ from, target, wantID, wantWarning string }{
			{"t1", "codex", "t2", ""}, {"t1", "t7", "t7", warning}, {"", "t7", "t7", ""}, {"missing", "t7", "t7", ""}, {"t2", "t1", "t1", ""},
		} {
			h := &spaceHost{fakeHost: fakeHost{tabs: tabs}}
			ctx, cancel := context.WithTimeout(context.Background(), 100000000)
			r := HandleContext(ctx, h, Request{Op: op, Tab: tc.target, From: tc.from, Text: "review", Args: []string{"enter"}})
			cancel()
			wantWarning := tc.wantWarning
			if op == "wait" {
				wantWarning = ""
			} // Wait uses caller resolution without a cross-space warning.
			if !r.OK || r.Warning != wantWarning {
				t.Fatalf("%s %+v: %+v", op, tc, r)
			}
			if op != "wait" && h.id != tc.wantID {
				t.Fatalf("%s routed to %s", op, h.id)
			}
		}
	}
	h := &fakeHost{tabs: append([]TabInfo(nil), tabs...)}
	h.tabs[2].Status = "working"
	r := Handle(h, Request{Op: "send", Tab: "t7", From: "t1", Text: "review"})
	if !r.OK || r.Warning != "busy; "+warning {
		t.Fatalf("busy warning: %+v", r)
	}
	if r = Handle(h, Request{Op: "send", Tab: "t7", From: "t1", Text: "review", IfIdle: true}); r.Error == nil || r.Error.Code != CodeBusy {
		t.Fatalf("busy guard: %+v", r)
	}
}

type spaceHost struct{ fakeHost }

func (h *spaceHost) StatusChanges() ([]TabInfo, <-chan struct{}) { return h.tabs, make(chan struct{}) }
func (h *spaceHost) OutputChanges(string) (<-chan struct{}, func(), time.Time, error) {
	panic("idle wait must not observe output")
}
