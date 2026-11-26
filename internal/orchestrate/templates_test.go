package orchestrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemplate(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestTemplateParsing(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		valid      bool
		warnings   int
	}{
		{"empty-body", "---\nagent: claude\n---", true, 0},
		{"valid", "---\nagent: claude\ndescription: 'Review changes'\nmodel: sonnet-4.5\nmode: read-only\n---\nFollow the tests.\n", true, 0},
		{"unknown", "---\nagent: codex\nflags: --yolo\n---\nReview", true, 1},
		{"missing", "---\ndescription: review\n---\nBody", false, 0},
		{"body", "Body only", false, 0},
		{"bad-agent", "---\nagent: shell\n---\nBody", false, 0},
		{"bad-mode", "---\nagent: claude\nmode: bypass\n---\nBody", false, 0},
		{"bad-model", "---\nagent: codex\nmodel: --yolo\n---\nBody", false, 0},
		{"injection", "---\nagent: codex\nmodel: x'; touch /tmp/no\n---\nBody", false, 0},
		{"duplicate", "---\nagent: codex\nagent: claude\n---\nBody", false, 0},
		{"unclosed", "---\nagent: codex\nBody", false, 0},
		{"utf8", "---\nagent: codex\n---\n\xff", false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTemplate(t, dir, "review", tt.text)
			got, warnings, err := LoadTemplate(dir, "review")
			if (err == nil) != tt.valid {
				t.Fatalf("template=%+v error=%v", got, err)
			}
			if tt.valid && (got.Agent == "" || len(warnings) != tt.warnings) {
				t.Fatalf("%+v %v", got, warnings)
			}
			if tt.name == "valid" && (got.Body != "Follow the tests.\n" || got.Description != "Review changes" || got.Model != "sonnet-4.5") {
				t.Fatal(got)
			}
		})
	}
}
func TestTemplateFilesAndNames(t *testing.T) {
	dir := t.TempDir()
	valid := "---\nagent: codex\n---\nReview"
	for _, name := range []string{"", "../escape", "UPPER", strings.Repeat("a", 33), "claude", "codex", "bad_name"} {
		_, _, err := LoadTemplate(dir, name)
		if err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	writeTemplate(t, dir, "review", valid)
	got, _, err := LoadTemplate(dir, "review")
	if err != nil || got.Agent != "codex" {
		t.Fatal(got, err)
	}
	writeTemplate(t, dir, "review", strings.Replace(valid, "codex", "claude", 1))
	got, _, err = LoadTemplate(dir, "review")
	if err != nil || got.Agent != "claude" {
		t.Fatal("cached", got, err)
	}
	writeTemplate(t, dir, "big", strings.Repeat("x", (32<<10)+1))
	if _, _, err := LoadTemplate(dir, "big"); err == nil {
		t.Fatal("oversize accepted")
	}
	if err := os.Mkdir(filepath.Join(dir, "directory.md"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadTemplate(dir, "directory"); err == nil {
		t.Fatal("directory accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadTemplate(dir, "link"); err == nil {
		t.Fatal("symlink accepted")
	}
	catalog := ListTemplates(dir)
	if len(catalog.Templates) != 1 || len(catalog.Warnings) < 3 {
		t.Fatalf("%+v", catalog)
	}
}
func TestTemplateLimitAndResolution(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 70; i++ {
		writeTemplate(t, dir, "review-"+strings.Repeat("a", i/26)+string(rune('a'+i%26)), "---\nagent: codex\n---\nBody")
	}
	if c := ListTemplates(dir); len(c.Templates) != 64 || len(c.Warnings) == 0 {
		t.Fatalf("%+v", c)
	}
	writeTemplate(t, dir, "review", "---\nagent: codex\nmodel: gpt-5.1\nmode: read-only\n---\nKeep tests.")
	for _, mode := range []string{"", "edit", "read-only"} {
		r, _, err := ResolveTemplate(dir, Request{Agent: "review", Task: "Task", Mode: mode})
		if err != nil || r.Agent != "codex" || r.Mode != "read-only" || r.Model != "gpt-5.1" || r.Instructions != "Keep tests." {
			t.Fatal(r, err)
		}
	}
	writeTemplate(t, dir, "editor", "---\nagent: claude\nmode: edit\n---\nEdit.")
	r, _, err := ResolveTemplate(dir, Request{Agent: "editor", Task: "Task", Mode: "read-only"})
	if err != nil || r.Mode != "read-only" {
		t.Fatal(r, err)
	}
	_, _, err = ResolveTemplate(dir, Request{Agent: "missing"})
	wantCode(t, err, CodeUnknownTemplate)
	r, _, err = ResolveTemplate(dir, Request{Agent: "claude"})
	if err != nil || r.Agent != "claude" {
		t.Fatal(r, err)
	}
}
func TestTemplatePlannerGolden(t *testing.T) {
	r, o := launchFixture()
	r.Model = "sonnet-4.5"
	r.Instructions = "Template body ' & --yolo"
	r.Mode = "read-only"
	wantMessage := strings.Replace(goldenBrief, "Task:\n", r.Instructions+"\nTask:\n", 1) + r.Task
	for _, agent := range []string{"claude", "codex"} {
		r.Agent = agent
		for _, goos := range []string{"darwin", "windows"} {
			o.GOOS = goos
			plan, err := Plan(r, o)
			if err != nil {
				t.Fatal(err)
			}
			if goos == "darwin" {
				flags := `claude --session-id '` + session + `' -n 'review' --permission-mode plan`
				if agent == "codex" {
					flags = "codex --no-daemon --sandbox read-only"
				}
				want := `t="$KAZE_TASK"; unset KAZE_TASK; exec ` + flags + ` --model 'sonnet-4.5' -- "$t"`
				if plan.Argv[4] != want || plan.Env[0] != "KAZE_TASK="+wantMessage {
					t.Fatal(plan)
				}
			} else {
				n := len(plan.Argv)
				if plan.Argv[n-4] != "--model" || plan.Argv[n-3] != r.Model || plan.Argv[n-2] != "--" || plan.Argv[n-1] != wantMessage {
					t.Fatal(plan)
				}
			}
		}
	}
	r.Model = "x'; echo bad"
	if _, err := Plan(r, o); err == nil {
		t.Fatal("model injection accepted")
	}
}
