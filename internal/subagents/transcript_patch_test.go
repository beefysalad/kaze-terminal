package subagents

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// patchResult is a user record whose toolUseResult carries a structuredPatch.
func patchResult(id string, isErr bool, hunks ...map[string]any) string {
	rec := map[string]any{
		"type":          "user",
		"toolUseResult": map[string]any{"structuredPatch": hunks},
		"message": map[string]any{"content": []map[string]any{
			{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": "ok"}}},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func hunk(oldStart, newStart int, lines ...string) map[string]any {
	return map[string]any{"oldStart": oldStart, "oldLines": len(lines), "newStart": newStart, "newLines": len(lines), "lines": lines}
}

func editUse(id string) string {
	return useLine(id, "Edit", map[string]any{"file_path": "/w/a.go", "old_string": "b", "new_string": "c"})
}

func TestPatchGivesRealLineNumbers(t *testing.T) {
	tr := feedAll(editUse("t1"), patchResult("t1", false, hunk(69, 69, " a", "-b", "+c", "+d")))
	s := tr.Steps[0]
	want := []DiffLine{{' ', "a", 69, 69}, {'-', "b", 70, 0}, {'+', "c", 0, 70}, {'+', "d", 0, 71}}
	if len(s.Diff) != len(want) {
		t.Fatalf("diff %+v", s.Diff)
	}
	for i, w := range want {
		if s.Diff[i] != w {
			t.Fatalf("diff[%d] %+v, want %+v", i, s.Diff[i], w)
		}
	}
	if s.Added != 2 || s.Removed != 1 {
		t.Fatalf("+%d -%d", s.Added, s.Removed)
	}
}

func TestNoPatchKeepsFallbackWithoutNumbers(t *testing.T) {
	tr := feedAll(editUse("t1"), resultLine("t1", false, "ok"))
	s := tr.Steps[0]
	if len(s.Diff) != 2 || s.Added != 1 || s.Removed != 1 {
		t.Fatalf("step %+v", s)
	}
	for _, d := range s.Diff {
		if d.Old != 0 || d.New != 0 {
			t.Fatalf("numbers in fallback: %+v", s.Diff)
		}
	}
}

func TestPendingEditHasFallbackThenNumbers(t *testing.T) {
	tr := feedAll(editUse("t1"))
	for _, d := range tr.Steps[0].Diff {
		if d.Old != 0 || d.New != 0 {
			t.Fatalf("numbers before the result: %+v", tr.Steps[0].Diff)
		}
	}
	tr.feed([]byte(patchResult("t1", false, hunk(10, 10, "-b", "+c"))))
	d := tr.Steps[0].Diff
	if len(d) != 2 || d[0].Old != 10 || d[1].New != 10 {
		t.Fatalf("after result %+v", d)
	}
}

func TestStringToolUseResultStillMarksFailure(t *testing.T) {
	rec := `{"type":"user","toolUseResult":"Error: String to replace not found","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"String to replace not found"}]}}` + "\n"
	tr := feedAll(editUse("t1"), rec)
	if s := tr.Steps[0]; !s.Done || !s.Failed {
		t.Fatalf("step %+v", s)
	}
}

func TestFailedEditDropsDiffAndKeepsError(t *testing.T) {
	tr := feedAll(editUse("t1"), resultLine("t1", true, "line one\nline two\nline three"))
	s := tr.Steps[0]
	if !s.Failed || s.Diff != nil || s.Added != 0 || s.Removed != 0 {
		t.Fatalf("step %+v", s)
	}
	if strings.Join(s.Out, "|") != "line one|line two|line three" {
		t.Fatalf("out %q", s.Out)
	}
	if total := detailBytes(&s); total != tr.stepBytes {
		t.Fatalf("tracked %d, held %d", tr.stepBytes, total)
	}
	var long strings.Builder
	for i := range 30 {
		fmt.Fprintf(&long, "e%d\n", i)
	}
	tr = feedAll(editUse("t2"), resultLine("t2", true, long.String()))
	if o := tr.Steps[0].Out; len(o) != maxOutLines || o[0] != "e0" {
		t.Fatalf("out %q", o)
	}
}

func TestPatchCappedAt40(t *testing.T) {
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = fmt.Sprintf("+l%d", i)
	}
	tr := feedAll(editUse("t1"), patchResult("t1", false, hunk(1, 1, lines...)))
	s := tr.Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 20 || s.Added != 60 {
		t.Fatalf("kept %d more %d added %d", len(s.Diff), s.DiffMore, s.Added)
	}
	if detailBytes(&s) != tr.stepBytes {
		t.Fatalf("tracked %d, held %d", tr.stepBytes, detailBytes(&s))
	}
}

func TestPatchLinesAreCleaned(t *testing.T) {
	tr := feedAll(editUse("t1"), patchResult("t1", false, hunk(1, 1, "+a\x1b[31mb\tc")))
	if got := tr.Steps[0].Diff[0].Text; got != "a[31mb    c" {
		t.Fatalf("text %q", got)
	}
}

func TestPatchRespectsTotalDetailBudget(t *testing.T) {
	tr := &Transcript{Available: true}
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = "+" + strings.Repeat("x", 190)
	}
	for i := range 200 {
		id := fmt.Sprintf("e%d", i)
		tr.feed([]byte(editUse(id) + patchResult(id, false, hunk(1, 1, lines...))))
	}
	total := 0
	for i := range tr.Steps {
		total += detailBytes(&tr.Steps[i])
		if detailBytes(&tr.Steps[i]) > maxStepDetail {
			t.Fatalf("step %d holds %d", i, detailBytes(&tr.Steps[i]))
		}
	}
	if total > maxStepsDetail || total != tr.stepBytes {
		t.Fatalf("detail %d (tracked %d), cap %d", total, tr.stepBytes, maxStepsDetail)
	}
	if len(tr.Steps[len(tr.Steps)-1].Diff) == 0 {
		t.Fatal("the newest step lost its diff")
	}
}

func TestPatchByteCapLeavesNoHole(t *testing.T) {
	var lines []string
	for range 25 {
		lines = append(lines, "+"+strings.Repeat("é", 200)) // 400 bytes each: 20 fit
	}
	for range 5 {
		lines = append(lines, "+x") // short lines after the cap must not sneak in
	}
	tr := feedAll(editUse("t1"), patchResult("t1", false, hunk(1, 1, lines...)))
	s := tr.Steps[0]
	if len(s.Diff) != 20 || s.DiffMore != 10 {
		t.Fatalf("kept %d more %d", len(s.Diff), s.DiffMore)
	}
	for i, d := range s.Diff {
		if d.New != i+1 {
			t.Fatalf("hole at %d: %+v", i, d)
		}
	}
}
