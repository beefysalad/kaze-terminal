package subagents

import (
	"encoding/json"
	"strings"
	"testing"
)

// resultRecord is a user record holding one tool_result per id and tur as its
// toolUseResult (nil leaves the field out).
func resultRecord(tur map[string]any, isErr bool, content string, ids ...string) string {
	blocks := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": content})
	}
	rec := map[string]any{"type": "user", "message": map[string]any{"content": blocks}}
	if tur != nil {
		rec["toolUseResult"] = tur
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func editBlock(id, path, old, nw string) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": "Edit",
		"input": map[string]any{"file_path": path, "old_string": old, "new_string": nw}}
}

func writeBlock(id, content string) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": "Write",
		"input": map[string]any{"file_path": "/w/" + id, "content": content}}
}

func assertNoNumbers(t *testing.T, s *TranscriptStep) {
	t.Helper()
	for _, d := range s.Diff {
		if d.Old != 0 || d.New != 0 {
			t.Fatalf("%s has numbers: %+v", s.ID, s.Diff)
		}
	}
}

// One patch cannot be told apart between two results, so neither gets it.
func TestSeveralResultsInARecordKeepTheirFallbackDiffs(t *testing.T) {
	tr := feedAll(
		stepRecord("assistant", "", editBlock("t1", "/w/a.go", "one", "two"), editBlock("t2", "/w/b.go", "p\nq", "r")),
		resultRecord(map[string]any{"type": "update", "structuredPatch": []any{hunk(69, 69, "-one", "+two")}}, false, "ok", "t1", "t2"),
	)
	a, b := &tr.Steps[0], &tr.Steps[1]
	if !a.Done || !b.Done {
		t.Fatalf("results not applied: %+v", tr.Steps)
	}
	assertNoNumbers(t, b)
	assertNoNumbers(t, a)
	if a.Added != 1 || a.Removed != 1 || b.Added != 1 || b.Removed != 2 {
		t.Fatalf("counts a +%d -%d, b +%d -%d", a.Added, a.Removed, b.Added, b.Removed)
	}
	if kinds(b.Diff) != "--+" || b.Diff[1].Text != "q" || b.Diff[2].Text != "r" {
		t.Fatalf("second file shows %+v", b.Diff)
	}
}

func TestSeveralResultsIgnoreTheRecordsWriteType(t *testing.T) {
	for _, kind := range []string{"create", "update"} {
		tr := feedAll(
			stepRecord("assistant", "", writeBlock("t1", "a\nb"), writeBlock("t2", "c")),
			resultRecord(map[string]any{"type": kind, "structuredPatch": []any{}}, false, "ok", "t1", "t2"),
		)
		for i := range tr.Steps {
			assertNoNumbers(t, &tr.Steps[i])
		}
		if tr.Steps[0].Added != 2 || len(tr.Steps[0].Diff) != 2 || tr.Steps[1].Added != 1 {
			t.Fatalf("%s: %+v", kind, tr.Steps)
		}
	}
}

func TestSingleResultStillGetsThePatch(t *testing.T) {
	tr := feedAll(
		stepRecord("assistant", "", editBlock("t1", "/w/a.go", "one", "two")),
		resultRecord(map[string]any{"structuredPatch": []any{hunk(69, 69, "-one", "+two")}}, false, "ok", "t1"),
	)
	if d := tr.Steps[0].Diff; len(d) != 2 || d[0].Old != 69 || d[1].New != 69 {
		t.Fatalf("%+v", d)
	}
}

// A rewrite with identical content: the tool says update and the patch is
// there but empty.
func TestUnchangedWriteShowsNothing(t *testing.T) {
	tr := feedAll(writeUse("t1", numbered("l", 300)), writeResult("t1", "update", false))
	s := tr.Steps[0]
	if s.Diff != nil || s.DiffMore != 0 || s.Added != 0 || s.Removed != 0 {
		t.Fatalf("diff %d rows, more %d, +%d -%d", len(s.Diff), s.DiffMore, s.Added, s.Removed)
	}
	if !s.Done || s.Failed {
		t.Fatalf("done %v failed %v", s.Done, s.Failed)
	}
	if detailBytes(&s) != tr.stepBytes || tr.stepBytes != 0 {
		t.Fatalf("tracked %d, held %d", tr.stepBytes, detailBytes(&s))
	}
}

// "No patch" is not "an empty patch": only the second proves nothing changed.
func TestWriteWithoutAPatchKeepsItsFallback(t *testing.T) {
	for name, res := range map[string]string{
		"field missing": resultRecord(map[string]any{"type": "update"}, false, "ok", "t1"),
		"null":          resultRecord(map[string]any{"type": "update", "structuredPatch": nil}, false, "ok", "t1"),
		"no record":     resultLine("t1", false, "ok"),
	} {
		s := feedAll(writeUse("t1", "a\nb"), res).Steps[0]
		if len(s.Diff) != 2 || s.Added != 2 {
			t.Fatalf("%s: +%d %+v", name, s.Added, s.Diff)
		}
		assertNoNumbers(t, &s)
	}
}

func TestEmptyPatchLeavesAnEditsFallback(t *testing.T) {
	s := feedAll(editUse("t1"), writeResult("t1", "update", false)).Steps[0]
	if len(s.Diff) != 2 || s.Added != 1 || s.Removed != 1 {
		t.Fatalf("+%d -%d %+v", s.Added, s.Removed, s.Diff)
	}
}

func TestWriteUpdateWithAPatchUsesIt(t *testing.T) {
	tr := feedAll(writeUse("t1", "a\nb"), resultRecord(map[string]any{"type": "update", "structuredPatch": []any{hunk(3, 3, "-x", "+y")}}, false, "ok", "t1"))
	if s := tr.Steps[0]; len(s.Diff) != 2 || s.Diff[0].Old != 3 || s.Added != 1 || s.Removed != 1 {
		t.Fatalf("%+v", s)
	}
}

func headerOnlyUse(id, tool string) string {
	return useLine(id, tool, map[string]any{"file_path": "/w/n.ipynb"})
}

func TestFailedHeaderOnlyEditKeepsItsError(t *testing.T) {
	for _, tool := range []string{"NotebookEdit", "apply_patch"} {
		tr := feedAll(headerOnlyUse("t1", tool), resultLine("t1", true, "cell not found\n\nsecond line"))
		s := tr.Steps[0]
		if !s.Failed || len(s.Out) != 2 || s.Out[0] != "cell not found" || s.Out[1] != "second line" {
			t.Fatalf("%s: %+v", tool, s)
		}
		if detailBytes(&s) != tr.stepBytes {
			t.Fatalf("%s: tracked %d, held %d", tool, tr.stepBytes, detailBytes(&s))
		}
	}
}

func TestFailedHeaderOnlyEditStaysWithinTheCaps(t *testing.T) {
	long := strings.Repeat(strings.Repeat("e", 3*maxDetailLine)+"\n", 3*maxOutLines)
	tr := feedAll(headerOnlyUse("t1", "NotebookEdit"), resultLine("t1", true, long))
	s := tr.Steps[0]
	if len(s.Out) != maxOutLines || detailBytes(&s) > maxStepDetail || detailBytes(&s) != tr.stepBytes {
		t.Fatalf("%d lines, %d bytes, tracked %d", len(s.Out), detailBytes(&s), tr.stepBytes)
	}
	for _, o := range s.Out {
		if len([]rune(o)) > maxDetailLine {
			t.Fatalf("line of %d runes", len([]rune(o)))
		}
	}
}

// A step already past the detail budget gets no error back.
func TestFailedHeaderOnlyEditRespectsTheTotalBudget(t *testing.T) {
	lines := []string{headerOnlyUse("first", "NotebookEdit")}
	content := strings.Repeat(strings.Repeat("x", 190)+"\n", 40)
	for i := 0; i < 40; i++ {
		lines = append(lines, writeUse("w"+string(rune('A'+i)), content))
	}
	tr := feedAll(lines...)
	if tr.detailFrom == 0 {
		t.Fatal("setup: nothing trimmed")
	}
	before := tr.stepBytes
	tr.feed([]byte(resultLine("first", true, "boom")))
	if s := tr.Steps[0]; !s.Failed || s.Out != nil || tr.stepBytes != before {
		t.Fatalf("out %v, bytes %d then %d", s.Out, before, tr.stepBytes)
	}
}

func TestSuccessfulHeaderOnlyEditKeepsNoOutput(t *testing.T) {
	for _, tool := range []string{"NotebookEdit", "apply_patch"} {
		tr := feedAll(headerOnlyUse("t1", tool), resultLine("t1", false, "Updated cell 3"))
		if s := tr.Steps[0]; !s.Done || s.Failed || s.Out != nil || tr.stepBytes != 0 {
			t.Fatalf("%s: %+v", tool, s)
		}
	}
}

// Claude Code wraps a refused tool call in <tool_use_error> tags; the reader
// sees only the message inside.
func TestFailedEditDropsTheToolUseErrorTags(t *testing.T) {
	for _, c := range []struct{ content, first, last string }{
		{"<tool_use_error>File has not been read yet. Read it first before writing to it.</tool_use_error>",
			"File has not been read yet. Read it first before writing to it.", ""},
		{"<tool_use_error>String to replace not found in file.\nString: foo</tool_use_error>",
			"String to replace not found in file.", "String: foo"},
	} {
		tr := feedAll(stepRecord("assistant", "", editBlock("t1", "/a.go", "foo", "bar")), resultLine("t1", true, c.content))
		s := tr.Steps[0]
		if !s.Failed || len(s.Out) == 0 || s.Out[0] != c.first || c.last != "" && s.Out[len(s.Out)-1] != c.last {
			t.Fatalf("%q: out %q", c.content, s.Out)
		}
		if detailBytes(&s) != tr.stepBytes {
			t.Fatalf("tracked %d, held %d", tr.stepBytes, detailBytes(&s))
		}
	}
}

// Only an edit's toolUseResult is decoded: a Read's embeds the whole file, and
// nothing in it is used.
func TestEditResultDecodesOnlyEdits(t *testing.T) {
	tur := json.RawMessage(`{"type":"update","structuredPatch":[{"oldStart":1,"newStart":1,"lines":["-a","+b"]}]}`)
	tr := feedAll(
		useLine("r1", "Read", map[string]any{"file_path": "/a.go"}),
		stepRecord("assistant", "", editBlock("e1", "/a.go", "a", "b")),
	)
	if patch, _, _ := tr.editResult("r1", tur); patch != nil {
		t.Fatalf("Read result decoded: %+v", patch)
	}
	if patch, _, _ := tr.editResult("e1", tur); len(patch) != 1 {
		t.Fatalf("Edit result: %+v", patch)
	}
	if patch, _, _ := tr.editResult("missing", tur); patch != nil {
		t.Fatalf("unknown id decoded: %+v", patch)
	}
}
