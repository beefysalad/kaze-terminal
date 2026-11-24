package control

import (
	"strings"
	"testing"
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
