package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

// rdTr builds an available transcript from steps and messages, numbering
// their Seq in the order given.
func rdTr(parts ...any) *subagents.Transcript {
	tr := &subagents.Transcript{Available: true}
	for i, p := range parts {
		switch v := p.(type) {
		case subagents.TranscriptStep:
			v.Seq = uint64(i)
			if v.ID == "" {
				v.ID = "id" + string(rune('a'+i))
			}
			tr.Steps = append(tr.Steps, v)
		case string:
			tr.Messages = append(tr.Messages, subagents.TranscriptMessage{Text: v, Seq: uint64(i)})
		}
	}
	return tr
}

func rdAgent() *subagents.Agent {
	return &subagents.Agent{State: subagents.Running, Started: tl0}
}

// rdSteps are the step items of a readable list.
func rdSteps(items []readItem) []readItem {
	var out []readItem
	for _, it := range items {
		if it.kind == readStep {
			out = append(out, it)
		}
	}
	return out
}

func rdSentence(it readItem) string {
	return strings.TrimSpace(it.verb + " " + it.obj)
}

func rdMeta(it readItem) string {
	var parts []string
	for _, m := range it.metas() {
		parts = append(parts, m.text)
	}
	return strings.Join(parts, " ")
}

func TestReadableBashSentence(t *testing.T) {
	for _, c := range []struct {
		name           string
		st             subagents.TranscriptStep
		sentence, meta string
		mono           bool
	}{
		{"described failing", subagents.TranscriptStep{Tool: "Bash", Desc: "Run go vet and the ui tests", Command: "go vet ./...", Done: true, Failed: true, Fails: 1}, "Ran go vet and the ui tests", "failed · 1 test", false},
		{"two fails", subagents.TranscriptStep{Tool: "Bash", Desc: "Run tests", Command: "go test ./...", Done: true, Failed: true, Fails: 2}, "Ran tests", "failed · 2 tests", false},
		{"failed, no Go fails", subagents.TranscriptStep{Tool: "Bash", Desc: "Build it", Command: "make", Done: true, Failed: true}, "Built it", "failed", false},
		{"listed", subagents.TranscriptStep{Tool: "Bash", Desc: "List files", Command: "ls", Done: true}, "Listed files", "done", false},
		{"lowercase verb", subagents.TranscriptStep{Tool: "Bash", Desc: "check the branch", Command: "git branch", Done: true}, "Checked the branch", "done", false},
		{"unknown verb", subagents.TranscriptStep{Tool: "Bash", Desc: "Worktree state", Command: "git status", Done: true}, "Worktree state", "done", false},
		{"no description", subagents.TranscriptStep{Tool: "Bash", Command: "git status\ngit log", Done: true}, "Ran git status", "done", true},
		{"test passed", subagents.TranscriptStep{Tool: "Bash", Command: "go test ./...", Done: true}, "Ran go test ./...", "passed", true},
		{"background", subagents.TranscriptStep{Tool: "Bash", Command: "npm run dev", Background: true, Done: true}, "Ran npm run dev", "started", true},
		{"running", subagents.TranscriptStep{Tool: "Bash", Desc: "Run the ui tests", Command: "go test ./internal/ui/"}, "Ran the ui tests", "", false},
	} {
		got := rdSteps(readableItems(rdAgent(), rdTr(c.st)))
		if len(got) != 1 {
			t.Fatalf("%s: %d items", c.name, len(got))
		}
		if s := rdSentence(got[0]); s != c.sentence {
			t.Errorf("%s: sentence %q, want %q", c.name, s, c.sentence)
		}
		if m := rdMeta(got[0]); m != c.meta {
			t.Errorf("%s: meta %q, want %q", c.name, m, c.meta)
		}
		if got[0].objMono != c.mono {
			t.Errorf("%s: mono %v", c.name, got[0].objMono)
		}
	}
}

func TestReadableCommit(t *testing.T) {
	got := rdSteps(readableItems(rdAgent(), rdTr(subagents.TranscriptStep{Tool: "Bash", Command: `git commit -m "x"`, Done: true,
		Commit: "c3f9a12", Subject: "refactor(ui): Remove the scratch shell key"})))
	if len(got) != 1 || rdSentence(got[0]) != `Committed "refactor(ui): Remove the scratch shell key"` || rdMeta(got[0]) != "c3f9a12" || got[0].metas()[0].tone != toneOK {
		t.Fatalf("%+v", got)
	}
}

func TestReadableEditSentence(t *testing.T) {
	diff := []subagents.DiffLine{{Kind: '-', Text: "old"}, {Kind: '+', Text: "new"}}
	got := rdSteps(readableItems(rdAgent(), rdTr(subagents.TranscriptStep{Tool: "Edit", Path: "internal/ui/scratch_gone_test.go", Added: 8, Removed: 5, Diff: diff, Done: true})))
	if len(got) != 1 || rdSentence(got[0]) != "Edited internal/ui/scratch_gone_test.go" || !got[0].objMono {
		t.Fatalf("%+v", got)
	}
	m := got[0].metas()
	if len(m) != 2 || m[0].text != "+8" || m[0].tone != toneAdd || m[1].text != "−5" || m[1].tone != toneDel {
		t.Fatalf("meta %+v", m)
	}
	if d := got[0].detail.lines; len(d) != 2 || d[0].Kind != '-' || d[1].Text != "new" {
		t.Fatalf("detail %+v", d)
	}
}

func TestReadableFoldsReads(t *testing.T) {
	got := rdSteps(readableItems(rdAgent(), rdTr(
		subagents.TranscriptStep{Tool: "Read", Path: "a.go"},
		subagents.TranscriptStep{Tool: "ToolSearch"},
		subagents.TranscriptStep{Tool: "Read", Path: "b.go"},
		subagents.TranscriptStep{Tool: "Read", Path: "a.go"},
	)))
	if len(got) != 1 || rdSentence(got[0]) != "Read 2 files" || got[0].objMono {
		t.Fatalf("%+v", got)
	}
	if d := got[0].detail.lines; len(d) != 2 || d[0].Text != "a.go" || d[1].Text != "b.go" {
		t.Fatalf("detail %+v", d)
	}
	// One file reads as itself.
	got = rdSteps(readableItems(rdAgent(), rdTr(subagents.TranscriptStep{Tool: "Read", Path: "a.go"})))
	if rdSentence(got[0]) != "Read a.go" || !got[0].objMono {
		t.Fatalf("%+v", got)
	}
}

func TestReadableFoldsSearches(t *testing.T) {
	var parts []any
	for _, p := range []string{"scratch", "win.fl", "func routeKey", "paneWindow"} {
		parts = append(parts, subagents.TranscriptStep{Tool: "Grep", Pattern: p, Path: "internal/ui"})
	}
	got := rdSteps(readableItems(rdAgent(), rdTr(parts...)))
	if len(got) != 1 || rdSentence(got[0]) != "Searched the code 4 times" {
		t.Fatalf("%+v", got)
	}
	if d := got[0].detail.lines; len(d) != 4 || d[0].Text != "scratch in internal/ui" {
		t.Fatalf("detail %+v", d)
	}
	got = rdSteps(readableItems(rdAgent(), rdTr(subagents.TranscriptStep{Tool: "Glob", Pattern: "*.go"})))
	if rdSentence(got[0]) != "Searched the code" {
		t.Fatalf("%+v", got)
	}
}

func TestReadableFoldsEdits(t *testing.T) {
	got := rdSteps(readableItems(rdAgent(), rdTr(
		subagents.TranscriptStep{Tool: "Edit", Path: "internal/ui/commands.go", Removed: 12},
		subagents.TranscriptStep{Tool: "Edit", Path: "internal/ui/logic.go", Added: 2, Removed: 9},
		subagents.TranscriptStep{Tool: "Write", Path: "internal/config/config.go", Added: 4, Removed: 20},
	)))
	if len(got) != 1 || rdSentence(got[0]) != "Edited 3 files" || rdMeta(got[0]) != "+6 −41" {
		t.Fatalf("%+v", got)
	}
	if d := got[0].detail.lines; len(d) != 3 || d[1].Text != "internal/ui/logic.go  +2 −9" {
		t.Fatalf("detail %+v", d)
	}
	// The same file twice: one file's sentence, counts summed, diffs joined.
	got = rdSteps(readableItems(rdAgent(), rdTr(
		subagents.TranscriptStep{Tool: "Edit", Path: "a.go", Added: 1, Removed: 1, Diff: []subagents.DiffLine{{Kind: '+', Text: "x"}}},
		subagents.TranscriptStep{Tool: "Edit", Path: "a.go", Added: 2, Diff: []subagents.DiffLine{{Kind: '+', Text: "y"}}},
	)))
	if len(got) != 1 || rdSentence(got[0]) != "Edited a.go" || rdMeta(got[0]) != "+3 −1" || len(got[0].detail.lines) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestReadableNoteBreaksFold(t *testing.T) {
	items := readableItems(rdAgent(), rdTr(
		subagents.TranscriptStep{Tool: "Read", Path: "a.go"},
		"Now the other one.",
		subagents.TranscriptStep{Tool: "Read", Path: "b.go"},
	))
	var kinds []readKind
	for _, it := range items {
		kinds = append(kinds, it.kind)
	}
	if len(kinds) != 4 || kinds[0] != readStep || kinds[1] != readNote || kinds[2] != readStep || kinds[3] != readLive {
		t.Fatalf("kinds %v", kinds)
	}
}

func TestReadablePlumbingHidden(t *testing.T) {
	tr := rdTr(subagents.TranscriptStep{Tool: "Monitor"}, subagents.TranscriptStep{Tool: "TaskStop"}, "hi")
	if got := rdSteps(readableItems(rdAgent(), tr)); len(got) != 0 {
		t.Fatalf("plumbing shown: %+v", got)
	}
	if s, m := readableCounts(rdAgent(), tr); s != 2 || m != 1 {
		t.Fatalf("counts %d %d", s, m)
	}
}

func TestReadableOtherTools(t *testing.T) {
	got := rdSteps(readableItems(rdAgent(), rdTr(
		subagents.TranscriptStep{Tool: "WebFetch", Query: "https://go.dev/doc/install"},
		subagents.TranscriptStep{Tool: "WebSearch", Query: "gio headless"},
		subagents.TranscriptStep{Tool: "TodoWrite"},
		subagents.TranscriptStep{Tool: "TodoWrite"},
		subagents.TranscriptStep{Tool: "mcp__x__y"},
	)))
	want := []string{"Fetched go.dev", "Searched the web for gio headless", "Updated the to-do list", "Used mcp__x__y"}
	if len(got) != len(want) {
		t.Fatalf("%d items %+v", len(got), got)
	}
	for i, w := range want {
		if rdSentence(got[i]) != w {
			t.Errorf("item %d %q, want %q", i, rdSentence(got[i]), w)
		}
	}
	if got[2].hasDetail() {
		t.Error("to-do list has detail")
	}
}

func TestReadableBashDetail(t *testing.T) {
	got := rdSteps(readableItems(rdAgent(), rdTr(subagents.TranscriptStep{Tool: "Bash", Desc: "Run it", Command: "cd x && go test ./...",
		Done: true, Out: []string{"--- FAIL: TestX", "FAIL"}})))
	d := got[0].detail
	if d.cap != "$ cd x && go test ./..." || len(d.lines) != 2 || d.lines[1].Text != "FAIL" || d.lines[0].Kind != ' ' {
		t.Fatalf("detail %+v", d)
	}
}

func TestReadableHookFallback(t *testing.T) {
	a := rdAgent()
	a.Steps = []subagents.Step{{At: tl0, Tool: "Bash", Summary: "go test"}, {At: tl0.Add(time.Second), Tool: "Read", Summary: "x.go"}, {At: tl0.Add(2 * time.Second), Tool: "Read", Summary: "y.go"}}
	got := rdSteps(readableItems(a, nil))
	if len(got) != 2 || rdSentence(got[0]) != "Ran go test" || !got[0].objMono || rdSentence(got[1]) != "Read 2 files" {
		t.Fatalf("%+v", got)
	}
	if rdMeta(got[0]) != "" || got[0].hasDetail() {
		t.Fatalf("hook Bash with meta or detail: %+v", got[0])
	}
	// A transcript with notes but no steps still uses the hook steps and keeps the notes.
	tr := avail(subagents.TranscriptMessage{Text: "between", At: tl0.Add(500 * time.Millisecond)})
	items := readableItems(a, tr)
	if len(items) != 4 || items[1].kind != readNote || items[1].text != "between" || rdSentence(items[2]) != "Read 2 files" {
		t.Fatalf("items %+v", items)
	}
}

func TestReadableMessagesLead(t *testing.T) {
	items := readableItems(rdAgent(), rdTr(
		"Starting **D6**.",
		subagents.TranscriptStep{Tool: "Bash", Desc: "Merge the branch", Command: "git merge x", Done: true},
		"RED as expected.",
	))
	if len(items) != 4 || items[0].kind != readNote || items[0].text != "Starting **D6**." || items[0].msg != 0 ||
		items[1].kind != readStep || items[2].text != "RED as expected." || items[2].msg != 1 {
		t.Fatalf("items %+v", items)
	}
}

func TestReadableIDsStable(t *testing.T) {
	tr := rdTr(
		subagents.TranscriptStep{Tool: "Bash", ID: "b1", Command: "ls"},
		subagents.TranscriptStep{Tool: "Edit", ID: "e1", Path: "a.go"},
		subagents.TranscriptStep{Tool: "Bash", ID: "b2", Command: "pwd"},
	)
	before := rdSteps(readableItems(rdAgent(), tr))
	tr.Steps = tr.Steps[1:]
	after := rdSteps(readableItems(rdAgent(), tr))
	if len(after) != 2 || after[0].id != before[1].id || after[1].id != before[2].id || after[0].id != "e1" {
		t.Fatalf("before %v after %v", before, after)
	}
}

func TestReadableEndRowsAndDropped(t *testing.T) {
	a := rdAgent()
	a.State = subagents.Done
	a.Ended = tl0.Add(90 * time.Second)
	tr := rdTr(subagents.TranscriptStep{Tool: "Read", Path: "a.go"})
	tr.Dropped = true
	items := readableItems(a, tr)
	if items[0].kind != readDropped || items[0].text != "… earlier steps hidden" || items[len(items)-1].kind != readFinished || items[len(items)-1].text != "Finished in 1m" {
		t.Fatalf("items %+v", items)
	}
}
