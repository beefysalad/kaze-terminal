package subagents

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// useLine is an assistant record with one tool_use block.
func useLine(id, name string, input map[string]any) string {
	return stepRecord("assistant", "", map[string]any{"type": "tool_use", "id": id, "name": name, "input": input})
}

// resultLine is a user record with one tool_result block.
func resultLine(id string, isErr bool, content any) string {
	return stepRecord("user", "", map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": content})
}

func userText(s string) string {
	b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": s}})
	return string(b) + "\n"
}

func stepRecord(kind, cwd string, blocks ...map[string]any) string {
	rec := map[string]any{"type": kind, "message": map[string]any{"content": blocks}}
	if cwd != "" {
		rec["cwd"] = cwd
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func feedAll(lines ...string) *Transcript {
	tr := &Transcript{Available: true}
	tr.feed([]byte(strings.Join(lines, "")))
	return tr
}

func TestTranscriptStepsInRecordOrder(t *testing.T) {
	tr := feedAll(
		userText("the prompt"),
		fixtureLine("assistant", textBlock("First I look.")),
		useLine("t1", "Bash", map[string]any{"command": "ls", "description": "List files"}),
		fixtureLine("assistant", textBlock("Now I edit.")),
		useLine("t2", "Edit", map[string]any{"file_path": "/w/a.go", "old_string": "a", "new_string": "b"}),
	)
	if len(tr.Steps) != 2 || len(tr.Messages) < 2 {
		t.Fatalf("steps %d messages %d", len(tr.Steps), len(tr.Messages))
	}
	m0, s0, m1, s1 := tr.Messages[0].Seq, tr.Steps[0].Seq, -1, tr.Steps[1].Seq
	for _, m := range tr.Messages {
		if m.Text == "Now I edit." {
			m1 = int(m.Seq)
		}
	}
	if !(m0 < s0 && s0 < uint64(m1) && uint64(m1) < s1) {
		t.Fatalf("order m0 %d s0 %d m1 %d s1 %d", m0, s0, m1, s1)
	}
	if tr.Steps[0].ID != "t1" || tr.Steps[0].Tool != "Bash" || tr.Steps[1].Tool != "Edit" {
		t.Fatalf("steps %+v", tr.Steps)
	}
	// Today's tool counts are kept.
	if tr.Messages[1].Tools.Commands != 1 {
		t.Fatalf("tool counts lost: %+v", tr.Messages)
	}
}

func TestTranscriptBashResult(t *testing.T) {
	out := "Exit code 1\nok  \tgithub.com/x/a\t0.1s\n--- FAIL: TestX (0.01s)\n    x_test.go:9: boom\n    --- FAIL: TestX/sub (0.00s)\nFAIL\tgithub.com/x/ui\t4.8s\n\n"
	tr := feedAll(
		useLine("t1", "Bash", map[string]any{"command": "go vet ./... && go test ./internal/ui/", "description": "Run go vet and the ui tests"}),
	)
	v := tr.Version
	if tr.Steps[0].Done {
		t.Fatal("done before its result")
	}
	tr.feed([]byte(resultLine("t1", true, out)))
	s := tr.Steps[0]
	if s.Desc != "Run go vet and the ui tests" || s.Command != "go vet ./... && go test ./internal/ui/" {
		t.Fatalf("desc %q command %q", s.Desc, s.Command)
	}
	if !s.Done || !s.Failed || s.Fails != 1 {
		t.Fatalf("done %v failed %v fails %d", s.Done, s.Failed, s.Fails)
	}
	if len(s.Out) == 0 || s.Out[len(s.Out)-1] != "FAIL    github.com/x/ui    4.8s" {
		t.Fatalf("out %q", s.Out)
	}
	if tr.Version == v {
		t.Fatal("a result did not bump Version")
	}
	// A passing result.
	tr = feedAll(useLine("t2", "Bash", map[string]any{"command": "go test ./..."}), resultLine("t2", false, "ok  \tx\t0.1s"))
	if s := tr.Steps[0]; !s.Done || s.Failed || s.Fails != 0 {
		t.Fatalf("passing %+v", s)
	}
	// Background runs.
	tr = feedAll(useLine("t3", "Bash", map[string]any{"command": "sleep 9", "run_in_background": true}))
	if !tr.Steps[0].Background {
		t.Fatal("background lost")
	}
}

func TestTranscriptEditCounts(t *testing.T) {
	old := "ctx1\nctx0\no1\no2\no3\no4\no5\nend0\nend1"
	nw := "ctx1\nctx0\nn1\nn2\nn3\nn4\nn5\nn6\nn7\nn8\nend0\nend1"
	tr := feedAll(useLine("e", "Edit", map[string]any{"file_path": "/w/a.go", "old_string": old, "new_string": nw}))
	s := tr.Steps[0]
	if s.Added != 8 || s.Removed != 5 {
		t.Fatalf("+%d −%d", s.Added, s.Removed)
	}
	want := []DiffLine{{Kind: ' ', Text: "ctx1"}, {Kind: ' ', Text: "ctx0"}, {Kind: '-', Text: "o1"}}
	for i, w := range want {
		if s.Diff[i] != w {
			t.Fatalf("diff[%d] %+v, want %+v (%+v)", i, s.Diff[i], w, s.Diff)
		}
	}
	if last := s.Diff[len(s.Diff)-1]; last != (DiffLine{Kind: ' ', Text: "end1"}) || len(s.Diff) != 2+5+8+2 {
		t.Fatalf("diff %+v", s.Diff)
	}
	// Write: every line added.
	tr = feedAll(useLine("w", "Write", map[string]any{"file_path": "/w/b.go", "content": "a\nb\n\tc\n"}))
	if s := tr.Steps[0]; s.Added != 3 || s.Removed != 0 || len(s.Diff) != 3 || s.Diff[2] != (DiffLine{Kind: '+', Text: "    c"}) {
		t.Fatalf("write %+v", s)
	}
	// MultiEdit: summed.
	tr = feedAll(useLine("m", "MultiEdit", map[string]any{"file_path": "/w/c.go", "edits": []map[string]any{
		{"old_string": "a", "new_string": "b\nc"}, {"old_string": "x\ny", "new_string": ""}}}))
	if s := tr.Steps[0]; s.Added != 2 || s.Removed != 3 || len(s.Diff) != 5 {
		t.Fatalf("multiedit %+v", s)
	}
}

func TestTranscriptCommitSubject(t *testing.T) {
	tr := feedAll(
		useLine("c", "Bash", map[string]any{"command": `git add -A && git commit -q -m "refactor(ui): Remove the scratch shell key"`}),
		resultLine("c", false, "[feat/x c3f9a12] refactor(ui): Remove the scratch shell key\n 2 files changed"),
	)
	if s := tr.Steps[0]; s.Commit != "c3f9a12" || s.Subject != "refactor(ui): Remove the scratch shell key" {
		t.Fatalf("commit %q subject %q", s.Commit, s.Subject)
	}
	// Not a commit command: no commit read from look-alike output.
	tr = feedAll(useLine("d", "Bash", map[string]any{"command": "cat log"}), resultLine("d", false, "[feat/x c3f9a12] x"))
	if tr.Steps[0].Commit != "" {
		t.Fatal("commit from a non-commit command")
	}
}

func TestTranscriptResultContentBlocks(t *testing.T) {
	tr := feedAll(
		useLine("t", "Bash", map[string]any{"command": "ls"}),
		resultLine("t", false, []map[string]any{{"type": "text", "text": "one\ntwo"}, {"type": "image"}, {"type": "text", "text": "three"}}),
	)
	if s := tr.Steps[0]; !s.Done || strings.Join(s.Out, "|") != "one|two|three" {
		t.Fatalf("out %q", s.Out)
	}
}

func TestTranscriptPathRelativeToCwd(t *testing.T) {
	tr := feedAll(
		stepRecord("assistant", `C:\w\kaze`, map[string]any{"type": "tool_use", "id": "r", "name": "Read", "input": map[string]any{"file_path": `C:\w\kaze\internal\ui\agent.go`}}),
		stepRecord("assistant", `C:\w\kaze`, map[string]any{"type": "tool_use", "id": "s", "name": "Read", "input": map[string]any{"file_path": `C:\other\deep\x\y\z.go`}}),
		stepRecord("assistant", `C:\w\kaze`, map[string]any{"type": "tool_use", "id": "u", "name": "Read", "input": map[string]any{"file_path": `C:\w\kazeX\a.go`}}),
		stepRecord("assistant", "/home/me/kaze", map[string]any{"type": "tool_use", "id": "g", "name": "Grep", "input": map[string]any{"pattern": "scratch", "path": "/home/me/kaze/internal"}}),
	)
	got := []string{tr.Steps[0].Path, tr.Steps[1].Path, tr.Steps[2].Path, tr.Steps[3].Path}
	want := []string{"internal/ui/agent.go", "x/y/z.go", "w/kazeX/a.go", "internal"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths %q, want %q", got, want)
		}
	}
	if tr.Steps[3].Pattern != "scratch" {
		t.Fatalf("pattern %q", tr.Steps[3].Pattern)
	}
}

func detailBytes(s *TranscriptStep) int {
	n := 0
	for _, d := range s.Diff {
		n += len(d.Text)
	}
	for _, o := range s.Out {
		n += len(o)
	}
	return n
}

func TestTranscriptStepCaps(t *testing.T) {
	var ob, nb strings.Builder
	for i := range 10000 {
		ob.WriteString("o" + string(rune('a'+i%26)) + "\n")
		nb.WriteString("n" + string(rune('a'+i%26)) + "\n")
	}
	line := useLine("big", "Edit", map[string]any{"file_path": "/w/a.go", "old_string": ob.String(), "new_string": nb.String()})
	if len(line) >= MaxTranscriptLine {
		t.Fatalf("fixture line %d too long", len(line))
	}
	tr := feedAll(line)
	s := tr.Steps[0]
	if len(s.Diff) > maxDiffLines || s.Added != 10000 || s.Removed != 10000 {
		t.Fatalf("diff %d +%d −%d", len(s.Diff), s.Added, s.Removed)
	}
	var out strings.Builder
	for range 400 {
		out.WriteString(strings.Repeat("é", 300) + "\n")
	}
	tr = feedAll(useLine("b", "Bash", map[string]any{"command": "x"}), resultLine("b", false, out.String()))
	s = tr.Steps[0]
	if len(s.Out) > maxOutLines || len(s.Out) == 0 {
		t.Fatalf("out %d lines", len(s.Out))
	}
	for _, o := range s.Out {
		if utf8.RuneCountInString(o) > maxDetailLine {
			t.Fatalf("out line %d runes", utf8.RuneCountInString(o))
		}
	}
	if detailBytes(&s) > maxStepDetail {
		t.Fatalf("step detail %d bytes", detailBytes(&s))
	}
	// 250 steps, each with a large detail: 200 kept, all detail within the cap.
	tr = &Transcript{Available: true}
	big := strings.Repeat(strings.Repeat("x", 150)+"\n", 40)
	for i := range 250 {
		id := "s" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		tr.feed([]byte(useLine(id, "Bash", map[string]any{"command": "x"}) + resultLine(id, false, big)))
		tr.feed([]byte(useLine(id+"e", "Write", map[string]any{"file_path": "/w/a.go", "content": big})))
	}
	if len(tr.Steps) != MaxTranscriptSteps {
		t.Fatalf("%d steps kept", len(tr.Steps))
	}
	total := 0
	for i := range tr.Steps {
		total += detailBytes(&tr.Steps[i])
	}
	if total > maxStepsDetail || total != tr.stepBytes {
		t.Fatalf("detail %d bytes (tracked %d), cap %d", total, tr.stepBytes, maxStepsDetail)
	}
	if !tr.Dropped {
		t.Fatal("dropped steps not flagged")
	}
	if last := tr.Steps[len(tr.Steps)-1]; len(last.Diff) == 0 {
		t.Fatal("the newest step lost its detail")
	}
}

func TestTranscriptUnmatchedResultIgnored(t *testing.T) {
	tr := feedAll(useLine("t", "Bash", map[string]any{"command": "ls"}))
	v := tr.Version
	tr.feed([]byte(resultLine("nope", true, "x") + userText("a plain prompt")))
	if tr.Steps[0].Done || tr.Version != v {
		t.Fatalf("unmatched result applied: %+v", tr.Steps[0])
	}
}

func TestTranscriptHandbackIsNoStep(t *testing.T) {
	tr := feedAll(useLine("h", "SubagentHandback", map[string]any{"message": "report"}))
	if len(tr.Steps) != 0 || tr.Report("") != "report" {
		t.Fatalf("steps %+v", tr.Steps)
	}
}

func BenchmarkTranscriptRead(b *testing.B) {
	var sb strings.Builder
	for i := range 50 {
		id := "t" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		sb.WriteString(fixtureLine("assistant", textBlock("Now I look at the next file and explain what I found there.")))
		sb.WriteString(useLine(id, "Bash", map[string]any{"command": "go test ./internal/ui/", "description": "Run the ui tests"}))
		sb.WriteString(resultLine(id, false, "ok  \tgithub.com/x/ui\t4.1s\n"))
		sb.WriteString(useLine(id+"e", "Edit", map[string]any{"file_path": "/w/a.go", "old_string": "a\nb\nc", "new_string": "a\nB\nc\nd"}))
	}
	data := []byte(sb.String())
	b.ReportAllocs()
	for b.Loop() {
		tr := Transcript{}
		tr.feed(data)
	}
}

func TestTranscriptOddToolInputKeepsText(t *testing.T) {
	tr := feedAll(fixtureLine("assistant", textBlock("still here"),
		map[string]any{"type": "tool_use", "id": "x", "name": "mcp__thing", "input": map[string]any{"path": 5, "content": []int{1}}}))
	if len(tr.Messages) == 0 || tr.Messages[0].Text != "still here" || len(tr.Steps) != 1 {
		t.Fatalf("messages %+v steps %+v", tr.Messages, tr.Steps)
	}
}
