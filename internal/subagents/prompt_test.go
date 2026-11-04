package subagents

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// userLine is a user record whose message.content is content.
func userLine(content any) string {
	b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}})
	return string(b) + "\n"
}

func writePrompt(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "agent-a.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFirstPromptString(t *testing.T) {
	p := writePrompt(t, userLine("Lane A: alert cards\nDo it.")+fixtureLine("assistant", textBlock("ok")))
	if got, ok := FirstPrompt(p); !ok || got != "Lane A: alert cards\nDo it." {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestFirstPromptTextBlocks(t *testing.T) {
	p := writePrompt(t, userLine([]map[string]any{textBlock("first"), {"type": "image"}, textBlock("second")}))
	if got, ok := FirstPrompt(p); !ok || got != "first\nsecond" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestFirstPromptSkipsLeadingRecords(t *testing.T) {
	summary := `{"type":"summary","summary":"earlier"}` + "\n"
	p := writePrompt(t, summary+fixtureLine("attachment")+userLine("the prompt"))
	if got, ok := FirstPrompt(p); !ok || got != "the prompt" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestFirstPromptSkipsToolResults(t *testing.T) {
	result := userLine([]map[string]any{{"type": "tool_result", "content": "not a prompt"}})
	p := writePrompt(t, result+userLine("the prompt"))
	if got, ok := FirstPrompt(p); !ok || got != "the prompt" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestFirstPromptIncompleteLine(t *testing.T) {
	line := userLine("the prompt")
	p := writePrompt(t, strings.TrimSuffix(line, "\n"))
	if got, ok := FirstPrompt(p); ok || got != "" {
		t.Fatalf("half a line read: %q %v", got, ok)
	}
	if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := FirstPrompt(p); !ok || got != "the prompt" {
		t.Fatalf("%q %v", got, ok)
	}
}

type countingReader struct {
	io.ReadCloser
	n *int
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	*c.n += n
	return n, err
}

func TestFirstPromptOversizedFirstLine(t *testing.T) {
	p := writePrompt(t, userLine(strings.Repeat("x", 1<<20))+userLine("later"))
	read, opens := 0, 0
	orig := openPrompt
	t.Cleanup(func() { openPrompt = orig })
	openPrompt = func(path string) (io.ReadCloser, error) {
		opens++
		f, err := orig(path)
		if err != nil {
			return nil, err
		}
		return countingReader{f, &read}, nil
	}
	if got, ok := FirstPrompt(p); ok || got != "" {
		t.Fatalf("got %d bytes, %v", len(got), ok)
	}
	if opens != 1 || read > maxPromptRead {
		t.Fatalf("opened %d times, read %d bytes", opens, read)
	}
}

func TestFirstPromptMissingOrDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{filepath.Join(dir, "missing.jsonl"), dir, ""} {
		if got, ok := FirstPrompt(p); ok || got != "" {
			t.Fatalf("%q: %q %v", p, got, ok)
		}
	}
}
