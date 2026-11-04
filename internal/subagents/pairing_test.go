package subagents

import (
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
)

// promptCall is an Agent call of typ named desc that asks prompt.
func promptCall(desc, typ, prompt string) Event {
	e := agentCall(desc, typ)
	e.Prompt = prompt
	return e
}

func k(id string) Key { return Key{"s1", id} }

// Issue #135: three agents of one type started in one message start in
// reverse order. The order pairing names them wrong; their prompts settle it.
func TestReverseStartsSettleToTheirOwnPrompts(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("Lane A", "general-purpose", "pA"))
	tr.Apply(promptCall("Lane D", "general-purpose", "pD"))
	tr.Apply(promptCall("Lane L", "general-purpose", "pL"))
	tr.Apply(ev(EvStart, "d3", "general-purpose"))
	tr.Apply(ev(EvStart, "d2", "general-purpose"))
	tr.Apply(ev(EvStart, "d1", "general-purpose"))
	if a := tr.Get(k("d3")); a.Task != "Lane A" {
		t.Fatalf("setup: d3 %q", a.Task)
	}
	for _, id := range []string{"d1", "d2", "d3"} {
		if !tr.PromptCheck(k(id)) {
			t.Fatalf("%s not contested", id)
		}
	}
	if !tr.SettlePrompt(k("d3"), "pL") {
		t.Fatal("d3 settle changed nothing")
	}
	if tr.SettlePrompt(k("d2"), "pD") {
		t.Fatal("d2 was already right")
	}
	if tr.SettlePrompt(k("d1"), "pA") {
		t.Fatal("d1 was already right after the swap")
	}
	want := map[string][2]string{"d1": {"Lane A", "pA"}, "d2": {"Lane D", "pD"}, "d3": {"Lane L", "pL"}}
	for id, w := range want {
		if a := tr.Get(k(id)); a.Task != w[0] || a.Prompt != w[1] {
			t.Fatalf("%s: %q %q, want %v", id, a.Task, a.Prompt, w)
		}
	}
}

func TestSettleTakesAStillQueuedCall(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(ev(EvStart, "a1", "Explore"))
	if a := tr.Get(k("a1")); a.Task != "one" {
		t.Fatalf("setup %q", a.Task)
	}
	if !tr.SettlePrompt(k("a1"), "p2") {
		t.Fatal("settle changed nothing")
	}
	if a := tr.Get(k("a1")); a.Task != "two" || a.Prompt != "p2" {
		t.Fatalf("a1 %q %q", a.Task, a.Prompt)
	}
	tr.Apply(ev(EvStart, "a2", "Explore"))
	if b := tr.Get(k("a2")); b.Task != "one" || b.Prompt != "p1" {
		t.Fatalf("a2 %q %q", b.Task, b.Prompt)
	}
}

func TestSettleNeverTakesASettledName(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(ev(EvStart, "b", "Explore"))
	if tr.SettlePrompt(k("a"), "p1") {
		t.Fatal("a was right")
	}
	if tr.SettlePrompt(k("b"), "p1") { // a corrupted transcript claims a's prompt
		t.Fatal("b took a settled name")
	}
	if a, b := tr.Get(k("a")), tr.Get(k("b")); a.Task != "one" || b.Task != "two" {
		t.Fatalf("tasks %q %q", a.Task, b.Task)
	}
}

func TestSingleCallIsNotContested(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(ev(EvStart, "a1", "Explore"))
	if tr.PromptCheck(k("a1")) {
		t.Fatal("one call is contested")
	}
	tr.Apply(promptCall("plan", "Plan", "p2"))
	tr.Apply(promptCall("look", "Explore", "p3"))
	tr.Apply(ev(EvStart, "a2", "Plan"))
	tr.Apply(ev(EvStart, "a3", "Explore"))
	if tr.PromptCheck(k("a2")) || tr.PromptCheck(k("a3")) {
		t.Fatal("calls of different types are contested")
	}
	if tr.PromptCheck(k("nobody")) {
		t.Fatal("an unknown agent is contested")
	}
}

func TestEmptyPromptGivesUpAfterThreeTries(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(ev(EvStart, "b", "Explore"))
	for i := 0; i < maxPromptChecks; i++ {
		if !tr.PromptCheck(k("a")) {
			t.Fatalf("gave up after %d tries", i)
		}
		if tr.SettlePrompt(k("a"), "") {
			t.Fatal("an empty prompt changed something")
		}
	}
	if tr.PromptCheck(k("a")) {
		t.Fatal("still checking after three empty reads")
	}
	if a, b := tr.Get(k("a")), tr.Get(k("b")); a.Task != "one" || b.Task != "two" || !tr.PromptCheck(k("b")) {
		t.Fatalf("tasks %q %q, b checked %v", a.Task, b.Task, tr.PromptCheck(k("b")))
	}
}

func TestSettleIsOnce(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(ev(EvStart, "b", "Explore"))
	tr.SettlePrompt(k("a"), "p1")
	if tr.PromptCheck(k("a")) || tr.SettlePrompt(k("a"), "p2") {
		t.Fatal("settled twice")
	}
	if a := tr.Get(k("a")); a.Task != "one" {
		t.Fatalf("task %q", a.Task)
	}
}

func TestSettleStaysInItsSession(t *testing.T) {
	var tr Tracker
	other := func(e Event) Event { e.Session = "s2"; return e }
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(other(promptCall("three", "Explore", "p3")))
	tr.Apply(other(promptCall("four", "Explore", "p4")))
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(other(ev(EvStart, "x", "Explore")))
	if tr.SettlePrompt(k("a"), "p3") || tr.SettlePrompt(k("a"), "p4") {
		t.Fatal("settled across sessions")
	}
	if a, x := tr.Get(k("a")), tr.Get(Key{"s2", "x"}); a.Task != "one" || x.Task != "three" {
		t.Fatalf("tasks %q %q", a.Task, x.Task)
	}
}

func TestPromptKeyNormalizes(t *testing.T) {
	long := strings.Repeat("do the thing <b> & \"quote\"\n", 800) // about 20 KiB
	base := strings.Repeat("x", promptKeyBytes)
	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{"crlf", "line one\r\nline two", "line one\nline two", true},
		{"trailing space and blank lines", "do it\n\n  \n", "do it", true},
		{"fitted to the hook cap", long, notify.FitReport(long, notify.MaxPrompt), true},
		{"differs only after 2 KiB", base + "tail one", base + "tail two", true},
		{"different first line", "Lane A: alert cards", "Lane D: dock tabs", false},
	}
	for _, tc := range tests {
		if got := promptKey(tc.a) == promptKey(tc.b); got != tc.same {
			t.Errorf("%s: same %v, want %v", tc.name, got, tc.same)
		}
	}
	if n := len(promptKey(long)); n > promptKeyBytes {
		t.Fatalf("key %d bytes", n)
	}
}

func TestUntypedFallbackContested(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "", "p1"))
	tr.Apply(promptCall("two", "", "p2"))
	tr.Apply(ev(EvStart, "a", "general-purpose"))
	tr.Apply(ev(EvStart, "b", "general-purpose"))
	if !tr.PromptCheck(k("a")) || !tr.PromptCheck(k("b")) {
		t.Fatal("untyped fallback not contested")
	}
	if !tr.SettlePrompt(k("a"), "p2") {
		t.Fatal("no swap")
	}
	if a, b := tr.Get(k("a")), tr.Get(k("b")); a.Task != "two" || b.Task != "one" || a.Prompt != "p2" || b.Prompt != "p1" {
		t.Fatalf("a %q %q, b %q %q", a.Task, a.Prompt, b.Task, b.Prompt)
	}
}

func TestOwnerPromptClearsContested(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(promptCall("two", "Explore", "p2"))
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(ev(EvStart, "b", "Explore"))
	tr.Apply(ev(EvPrompt, "", ""))
	tr.Apply(promptCall("three", "Explore", "p3"))
	tr.Apply(ev(EvStart, "c", "Explore"))
	if tr.PromptCheck(k("c")) {
		t.Fatal("the type stayed contested past the owner's prompt")
	}
}

// A settle that swaps keeps each agent's parent in step with its call.
func TestSettleMovesTheParent(t *testing.T) {
	var tr Tracker
	tr.Apply(ev(EvStart, "p", "Plan"))
	tr.Apply(promptCall("top", "Explore", "p1"))
	child := promptCall("nested", "Explore", "p2")
	child.AgentID = "p"
	tr.Apply(child)
	tr.Apply(ev(EvStart, "a", "Explore"))
	tr.Apply(ev(EvStart, "b", "Explore"))
	if !tr.SettlePrompt(k("a"), "p2") {
		t.Fatal("no swap")
	}
	a, b := tr.Get(k("a")), tr.Get(k("b"))
	if a.Parent != k("p") || a.Depth != 1 || b.Parent != (Key{}) || b.Depth != 0 {
		t.Fatalf("a %+v %d, b %+v %d", a.Parent, a.Depth, b.Parent, b.Depth)
	}
}
