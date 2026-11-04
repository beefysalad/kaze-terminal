package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func jq(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestDocPathsClaudeTools(t *testing.T) {
	d := t.TempDir()
	plan, goFile := filepath.Join(d, "plan.md"), filepath.Join(d, "main.go")
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "Read"} {
		got := docPaths(tool, raw(jq(map[string]string{"file_path": plan})), "")
		if !reflect.DeepEqual(got, []string{plan}) {
			t.Errorf("%s: got %v", tool, got)
		}
	}
	if got := docPaths("Write", raw(jq(map[string]string{"file_path": goFile})), ""); got != nil {
		t.Errorf("go file: got %v", got)
	}
	if got := docPaths("Grep", raw(jq(map[string]string{"pattern": "x", "path": d})), ""); got != nil {
		t.Errorf("grep: got %v", got)
	}
	notes := filepath.Join(d, "notes.md")
	if got := docPaths("mcp__x__save", raw(jq(map[string]string{"path": notes})), ""); !reflect.DeepEqual(got, []string{notes}) {
		t.Errorf("mcp: got %v", got)
	}
	if got := docPaths("Read", raw(`{"file_path":"docs/a.md"}`), d); !reflect.DeepEqual(got, []string{filepath.Join(d, "docs/a.md")}) {
		t.Errorf("relative: got %v", got)
	}
	if got := docPaths("Read", raw(`{"file_path":"docs/a.md"}`), ""); got != nil {
		t.Errorf("relative, no cwd: got %v", got)
	}
}

func TestDocPathsCodexPatch(t *testing.T) {
	c := t.TempDir()
	abs := filepath.Join(c, "elsewhere", "b.md")
	patch := "*** Begin Patch\n*** Add File: docs/a.md\n+# A\n*** Update File: " + abs +
		"\n*** Move to: docs/c.md\n*** Delete File: docs/d.md\n*** Update File: x.go\n*** End Patch"
	got := docPaths("apply_patch", raw(jq(map[string]string{"command": patch})), c)
	want := []string{filepath.Join(c, "docs/a.md"), abs, filepath.Join(c, "docs/c.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDocPathsCapped(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 12; i++ {
		b.WriteString("*** Add File: d" + string(rune('a'+i)) + ".md\n")
	}
	got := docPaths("apply_patch", raw(jq(map[string]string{"command": b.String()})), t.TempDir())
	if len(got) != notify.MaxDocs {
		t.Fatalf("got %d", len(got))
	}
}

func claudeWrite(path, cwd, agentID string) *strings.Reader {
	m := map[string]any{"tool_name": "Write", "tool_input": map[string]string{"file_path": path}, "cwd": cwd}
	if agentID != "" {
		m["agent_id"] = agentID
		m["agent_type"] = "Explore"
	}
	return strings.NewReader(jq(m))
}

func TestMainCallSendsDoc(t *testing.T) {
	srv, got := startServer(t)
	d := t.TempDir()
	p := filepath.Join(d, "plan.md")
	runNotify([]string{"tool"}, claudeWrite(p, d, ""), envOf(srv, "3"))
	m := wait(t, got)
	if m.Kind != notify.KindDoc || m.Tab != 3 || !reflect.DeepEqual(m.Docs, []string{p}) {
		t.Fatalf("got %+v", m)
	}
}

func TestMainCallWithoutDocSendsNothing(t *testing.T) {
	srv, got := startServer(t)
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	runNotify([]string{"tool"}, in, envOf(srv, "3"))
	select {
	case m := <-got:
		t.Fatalf("unexpected %+v", m)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSubagentToolCarriesDocs(t *testing.T) {
	srv, got := startServer(t)
	d := t.TempDir()
	p := filepath.Join(d, "plan.md")
	runNotify([]string{"tool"}, claudeWrite(p, d, "a1"), envOf(srv, "3"))
	m := wait(t, got)
	if m.Kind != notify.KindTool || m.AgentID != "a1" || m.Summary == "" || !reflect.DeepEqual(m.Docs, []string{p}) {
		t.Fatalf("got %+v", m)
	}
}

func TestMainAgentCallUnchanged(t *testing.T) {
	srv, got := startServer(t)
	in := strings.NewReader(`{"tool_name":"Agent","tool_input":{"description":"d","prompt":"p","subagent_type":"Explore"}}`)
	runNotify([]string{"tool"}, in, envOf(srv, "3"))
	m := wait(t, got)
	if m.Kind != notify.KindTool || m.Tool != "Agent" || m.Docs != nil {
		t.Fatalf("got %+v", m)
	}
}
