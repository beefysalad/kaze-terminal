package subagents

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Only Bash-like steps show their output, so only they keep it.
func TestResultOutputOnlyForShellSteps(t *testing.T) {
	tr := feedAll(
		useLine("g", "Grep", map[string]any{"pattern": "x"}),
		resultLine("g", false, "a.go:1: x\nb.go:2: x"),
		useLine("w", "WebFetch", map[string]any{"url": "https://example.com"}),
		resultLine("w", false, "page text"),
		useLine("a", "Agent", map[string]any{"description": "look"}),
		resultLine("a", false, "the report"),
		useLine("b", "Bash", map[string]any{"command": "ls"}),
		resultLine("b", false, "one\ntwo"),
	)
	for _, s := range tr.Steps[:3] {
		if !s.Done || len(s.Out) != 0 {
			t.Fatalf("%s kept output %q (done %v)", s.Tool, s.Out, s.Done)
		}
	}
	if s := tr.Steps[3]; strings.Join(s.Out, "|") != "one|two" {
		t.Fatalf("Bash out %q", s.Out)
	}
	if tr.stepBytes != 6 {
		t.Fatalf("detail bytes %d, want only the Bash output's 6", tr.stepBytes)
	}
}

// Trimmed messages or a skipped oversized line are not dropped steps.
func TestStepsDroppedOnlyForTrimmedSteps(t *testing.T) {
	big := strings.Repeat("x", MaxTranscriptLine+10) + "\n"
	tr := feedAll(useLine("r", "Read", map[string]any{"file_path": "/w/a.go"}), big)
	if !tr.Dropped || tr.StepsDropped {
		t.Fatalf("skipped line: dropped %v steps dropped %v", tr.Dropped, tr.StepsDropped)
	}
	var lines []string
	for i := range MaxTranscriptSteps + 1 {
		id := "s" + strconv.Itoa(i)
		lines = append(lines, useLine(id, "Read", map[string]any{"file_path": "/w/a.go"}))
	}
	tr = feedAll(lines...)
	if !tr.StepsDropped || len(tr.Steps) != MaxTranscriptSteps {
		t.Fatalf("steps dropped %v, %d steps", tr.StepsDropped, len(tr.Steps))
	}
}

// Nothing but a regular file is opened: a FIFO or a directory is turned
// away before any open.
func TestReadersOpenOnlyRegularFiles(t *testing.T) {
	opened := 0
	was := openFile
	openFile = func(p string) (*os.File, error) { opened++; return os.Open(p) }
	defer func() { openFile = was }()
	dir := t.TempDir()
	if _, ok := FirstPrompt(dir); ok {
		t.Fatal("a directory gave a prompt")
	}
	var tr Transcript
	if tr.Read(dir) == nil || tr.Available {
		t.Fatal("a directory read as a transcript")
	}
	if opened != 0 {
		t.Fatalf("opened a directory %d times", opened)
	}
	f := filepath.Join(dir, "t.jsonl")
	os.WriteFile(f, []byte(userLine("hello")), 0o644)
	if p, ok := FirstPrompt(f); !ok || p != "hello" {
		t.Fatalf("regular file: %q %v", p, ok)
	}
	if tr.Read(f) != nil || !tr.Available {
		t.Fatal("regular transcript not read")
	}
}

// A settle that changes nothing leaves the revision alone.
func TestSettlePromptRevisionOnlyOnChange(t *testing.T) {
	var tr Tracker
	tr.Apply(promptCall("one", "Explore", "p1"))
	tr.Apply(ev(EvStart, "a1", "Explore"))
	rev := tr.Revision()
	tr.SettlePrompt(k("a1"), "p1") // not contested
	tr.SettlePrompt(k("nobody"), "p1")
	if tr.Revision() != rev {
		t.Fatalf("revision %d, want %d after settles that changed nothing", tr.Revision(), rev)
	}
	tr.Apply(promptCall("two", "Plan", "p2"))
	tr.Apply(promptCall("three", "Plan", "p3"))
	tr.Apply(ev(EvStart, "a2", "Plan"))
	rev = tr.Revision()
	tr.SettlePrompt(k("a2"), "")   // a miss
	tr.SettlePrompt(k("a2"), "p2") // already right
	if tr.Revision() != rev {
		t.Fatalf("revision %d, want %d", tr.Revision(), rev)
	}
	tr.Apply(promptCall("four", "Plan", "p4"))
	tr.Apply(promptCall("five", "Plan", "p5"))
	tr.Apply(ev(EvStart, "a3", "Plan"))
	rev = tr.Revision()
	if !tr.SettlePrompt(k("a3"), "p5") || tr.Revision() == rev {
		t.Fatalf("a real swap kept revision %d", rev)
	}
}
