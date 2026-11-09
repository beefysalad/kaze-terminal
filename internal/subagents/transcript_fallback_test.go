package subagents

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// numbered is n lines "<prefix>0", "<prefix>1", ... joined by newlines.
func numbered(prefix string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return strings.Join(lines, "\n")
}

// wideLines is n lines of 400 bytes: 20 of them fill a step's detail.
func wideLines(n int) string {
	return strings.TrimSuffix(strings.Repeat(strings.Repeat("é", 200)+"\n", n), "\n")
}

func fallbackUse(tool string, in map[string]any) *Transcript {
	return feedAll(useLine("t1", tool, in))
}

func kinds(d []DiffLine) string {
	var b strings.Builder
	for _, l := range d {
		b.WriteByte(l.Kind)
	}
	return b.String()
}

func TestFallbackEditReportsLinesPastTheLineCap(t *testing.T) {
	s := fallbackUse("Edit", map[string]any{"file_path": "/w/a", "old_string": numbered("o", 30), "new_string": numbered("n", 30)}).Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 20 || s.Added != 30 || s.Removed != 30 {
		t.Fatalf("kept %d more %d +%d -%d", len(s.Diff), s.DiffMore, s.Added, s.Removed)
	}
}

// A byte cap inside the removed group must not let a short added line through.
func TestFallbackEditByteCapLeavesNoHole(t *testing.T) {
	s := fallbackUse("Edit", map[string]any{"file_path": "/w/a", "old_string": wideLines(25), "new_string": numbered("n", 5)}).Steps[0]
	if len(s.Diff) != 20 || s.DiffMore != 10 {
		t.Fatalf("kept %d more %d", len(s.Diff), s.DiffMore)
	}
	if k := kinds(s.Diff); strings.Contains(k, "+") {
		t.Fatalf("a hole in the diff: %s", k)
	}
}

func TestFallbackEditLeftOutContextCounts(t *testing.T) {
	old := wideLines(21) + "\nz1\nz2"
	nw := "n\nz1\nz2"
	s := fallbackUse("Edit", map[string]any{"file_path": "/w/a", "old_string": old, "new_string": nw}).Steps[0]
	// The 21st removed row is over the bytes, so it, the added row and both context rows are left out.
	if len(s.Diff) != 20 || s.DiffMore != 4 {
		t.Fatalf("kept %d more %d", len(s.Diff), s.DiffMore)
	}
}

func multi(edits ...[2]string) map[string]any {
	var list []map[string]any
	for _, e := range edits {
		list = append(list, map[string]any{"old_string": e[0], "new_string": e[1]})
	}
	return map[string]any{"file_path": "/w/a", "edits": list}
}

func TestFallbackMultiEditKeepsCountingPastTheLineCap(t *testing.T) {
	s := fallbackUse("MultiEdit", multi([2]string{numbered("o", 30), numbered("n", 30)}, [2]string{"a\nb", "c\nd\ne"})).Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 25 || s.Added != 33 || s.Removed != 32 {
		t.Fatalf("kept %d more %d +%d -%d", len(s.Diff), s.DiffMore, s.Added, s.Removed)
	}
}

// Once one edit runs out of bytes, a later edit's short lines stay out too.
func TestFallbackMultiEditByteCapLeavesNoHole(t *testing.T) {
	s := fallbackUse("MultiEdit", multi([2]string{wideLines(21), ""}, [2]string{"a", "b"})).Steps[0]
	if len(s.Diff) != 20 || s.DiffMore != 3 {
		t.Fatalf("kept %d more %d: %s", len(s.Diff), s.DiffMore, kinds(s.Diff))
	}
}

func TestFallbackMultiEditExactlyFullHasNothingMore(t *testing.T) {
	s := fallbackUse("MultiEdit", multi([2]string{numbered("o", 20), numbered("n", 20)})).Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 0 {
		t.Fatalf("kept %d more %d", len(s.Diff), s.DiffMore)
	}
}

func TestFallbackWriteReportsLinesPastTheCaps(t *testing.T) {
	s := fallbackUse("Write", map[string]any{"file_path": "/w/a", "content": numbered("l", 60) + "\n"}).Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 20 || s.Added != 60 {
		t.Fatalf("line cap: kept %d more %d +%d", len(s.Diff), s.DiffMore, s.Added)
	}
	s = fallbackUse("Write", map[string]any{"file_path": "/w/a", "content": wideLines(25) + "\nshort\nshort"}).Steps[0]
	if len(s.Diff) != 20 || s.DiffMore != 7 || s.Added != 27 {
		t.Fatalf("byte cap: kept %d more %d +%d", len(s.Diff), s.DiffMore, s.Added)
	}
}

func TestFallbackWriteCountsAgreeWithRows(t *testing.T) {
	s := fallbackUse("Write", map[string]any{"file_path": "/w/a", "content": "a\n\n"}).Steps[0]
	if len(s.Diff)+s.DiffMore != s.Added || s.DiffMore != 0 {
		t.Fatalf("kept %d more %d +%d", len(s.Diff), s.DiffMore, s.Added)
	}
}

func TestFallbackWithinTheCapsHasNothingMore(t *testing.T) {
	s := fallbackUse("Edit", map[string]any{"file_path": "/w/a", "old_string": "a", "new_string": "b"}).Steps[0]
	if s.DiffMore != 0 {
		t.Fatalf("more %d", s.DiffMore)
	}
}

// writeResult is the record Claude Code writes after a Write: kind is "create"
// for a new file, and a new file's structuredPatch is empty.
func writeResult(id, kind string, isErr bool) string {
	b, _ := json.Marshal(map[string]any{"type": "user",
		"toolUseResult": map[string]any{"type": kind, "structuredPatch": []any{}, "originalFile": nil},
		"message":       map[string]any{"content": []map[string]any{{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": "ok"}}}})
	return string(b) + "\n"
}

func writeUse(id, content string) string {
	return useLine(id, "Write", map[string]any{"file_path": "/w/a", "content": content})
}

func TestNewFileWriteIsNumberedFromOne(t *testing.T) {
	tr := feedAll(writeUse("t1", numbered("l", 60)))
	running := tr.Steps[0].Diff
	for _, d := range running {
		if d.New != 0 {
			t.Fatalf("numbers before the result: %+v", d)
		}
	}
	tr.feed([]byte(writeResult("t1", "create", false)))
	s := tr.Steps[0]
	if len(s.Diff) != 40 || s.DiffMore != 20 {
		t.Fatalf("kept %d more %d", len(s.Diff), s.DiffMore)
	}
	for i, d := range s.Diff {
		if d.New != i+1 || d.Old != 0 || d.Kind != '+' {
			t.Fatalf("row %d: %+v", i, d)
		}
	}
	// A new slice, so a drawn recording of the unnumbered rows is not kept.
	if &s.Diff[0] == &running[0] || running[0].New != 0 {
		t.Fatal("numbered the shown rows in place")
	}
	if detailBytes(&s) != tr.stepBytes {
		t.Fatalf("tracked %d, held %d", tr.stepBytes, detailBytes(&s))
	}
}

// Only a create proves the numbers: anything else keeps the rule against inventing them.
func TestOnlyACreatedFileIsNumberedWithoutAPatch(t *testing.T) {
	for name, res := range map[string]string{
		"update":     writeResult("t1", "update", false),
		"no details": resultLine("t1", false, "ok"),
	} {
		tr := feedAll(writeUse("t1", "a\nb"), res)
		for _, d := range tr.Steps[0].Diff {
			if d.New != 0 || d.Old != 0 {
				t.Fatalf("%s: numbers %+v", name, d)
			}
		}
	}
	tr := feedAll(editUse("t1"), writeResult("t1", "create", false))
	for _, d := range tr.Steps[0].Diff {
		if d.New != 0 || d.Old != 0 {
			t.Fatalf("edit: numbers %+v", d)
		}
	}
	if s := feedAll(writeUse("t1", "a"), writeResult("t1", "create", true)).Steps[0]; s.Diff != nil {
		t.Fatalf("failed write kept %+v", s.Diff)
	}
}
