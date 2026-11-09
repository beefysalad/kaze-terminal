package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

// rdRead parses real transcript records through the transcript reader.
func rdRead(t *testing.T, recs ...map[string]any) *subagents.Transcript {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "agent.jsonl")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	tr := &subagents.Transcript{}
	if err := tr.Read(path); err != nil {
		t.Fatal(err)
	}
	return tr
}

func edUse(id, tool string, in map[string]any) map[string]any {
	return map[string]any{"type": "assistant", "message": map[string]any{"content": []map[string]any{
		{"type": "tool_use", "id": id, "name": tool, "input": in}}}}
}

func edResult(id string, isErr bool, text string, tur map[string]any) map[string]any {
	rec := map[string]any{"type": "user", "message": map[string]any{"content": []map[string]any{
		{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": text}}}}
	if tur != nil {
		rec["toolUseResult"] = tur
	}
	return rec
}

func TestUnchangedWriteIsOneRowWithoutAFakeCount(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 300; i++ {
		body.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	tr := rdRead(t,
		edUse("w1", "Write", map[string]any{"file_path": "/w/a.go", "content": body.String()}),
		edResult("w1", false, "ok", map[string]any{"type": "update", "structuredPatch": []any{}}))
	items := rdSteps(readableItems(rdAgent(), tr))
	s := &dockSession{}
	if len(items) != 1 || items[0].hasDetail() || s.isOpen(&items[0]) || rdMeta(items[0]) != "+0 −0" {
		t.Fatalf("%d rows, first meta %q, detail %d rows", len(items), rdMeta(items[0]), len(items[0].detail.lines))
	}
}

func rdDone(tool, id, path string) subagents.TranscriptStep {
	return subagents.TranscriptStep{Tool: tool, ID: id, Path: path, Done: true}
}

func TestHeaderOnlyEditsFoldFromATranscript(t *testing.T) {
	for _, tool := range []string{"NotebookEdit", "apply_patch"} {
		items := rdSteps(readableItems(rdAgent(), rdTr(rdDone(tool, "n1", "a.ipynb"), rdDone(tool, "n2", "b.ipynb"), rdDone(tool, "n3", "a.ipynb"))))
		s := &dockSession{}
		if len(items) != 1 || items[0].id != "n1" || rdSentence(items[0]) != "Edited 2 files" || rdMeta(items[0]) != "" || s.isOpen(&items[0]) {
			t.Fatalf("%s: %d rows: %+v", tool, len(items), items)
		}
	}
}

func TestPathlessApplyPatchesAreOneRow(t *testing.T) {
	var parts []any
	for i := 0; i < 10; i++ {
		parts = append(parts, rdDone("apply_patch", "p"+strconv.Itoa(i), ""))
	}
	items := rdSteps(readableItems(rdAgent(), rdTr(parts...)))
	if len(items) != 1 || rdMeta(items[0]) != "" || items[0].hasDetail() {
		t.Fatalf("%d rows", len(items))
	}
}

func TestInlineEditsStillLeaveTheFold(t *testing.T) {
	items := rdSteps(readableItems(rdAgent(), rdTr(
		rdDone("NotebookEdit", "n1", "a.ipynb"), rdEdit("e1", "a.go"), rdDone("NotebookEdit", "n2", "b.ipynb"),
		subagents.TranscriptStep{Tool: "Write", ID: "w1", Path: "c.go", Done: true},
		subagents.TranscriptStep{Tool: "MultiEdit", ID: "m1", Path: "d.go", Done: true},
	)))
	if len(items) != 5 || items[1].id != "e1" || items[3].id != "w1" || items[4].id != "m1" {
		t.Fatalf("%+v", items)
	}
}

func TestFailedHeaderOnlyEditOpensWithItsError(t *testing.T) {
	for _, tool := range []string{"NotebookEdit", "apply_patch"} {
		tr := rdRead(t,
			edUse("n1", tool, map[string]any{"file_path": "/w/n.ipynb"}),
			edResult("n1", true, "cell not found", nil))
		items := rdSteps(readableItems(rdAgent(), tr))
		s := &dockSession{}
		if len(items) != 1 || rdMeta(items[0]) != "failed" || len(items[0].detail.lines) != 1 ||
			items[0].detail.lines[0].Text != "cell not found" || items[0].detail.diff || !s.isOpen(&items[0]) {
			t.Fatalf("%s: %+v", tool, items)
		}
	}
}

// The error must not vanish into the neighbours' fold.
func TestFailedHeaderOnlyEditLeavesTheFold(t *testing.T) {
	failed := rdDone("NotebookEdit", "n2", "b.ipynb")
	failed.Failed, failed.Out = true, []string{"boom"}
	items := rdSteps(readableItems(rdAgent(), rdTr(rdDone("NotebookEdit", "n1", "a.ipynb"), failed, rdDone("NotebookEdit", "n3", "c.ipynb"), rdDone("NotebookEdit", "n4", "d.ipynb"))))
	s := &dockSession{}
	if len(items) != 3 || rdMeta(items[1]) != "failed" || !s.isOpen(&items[1]) || items[2].id != "n3" || rdSentence(items[2]) != "Edited 2 files" {
		t.Fatalf("%d rows: %+v", len(items), items)
	}
}

func TestSuccessfulHeaderOnlyEditStaysHeaderOnlyFromARealResult(t *testing.T) {
	tr := rdRead(t,
		edUse("n1", "NotebookEdit", map[string]any{"file_path": "/w/n.ipynb"}),
		edResult("n1", false, "Updated cell 3", nil))
	items := rdSteps(readableItems(rdAgent(), tr))
	s := &dockSession{}
	if len(items) != 1 || rdMeta(items[0]) != "" || items[0].hasDetail() || s.isOpen(&items[0]) {
		t.Fatalf("%+v", items)
	}
}
