package agentdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0640); err != nil {
		t.Fatal(err)
	}
}
func get(t *testing.T, p string) string {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestRoundTrip(t *testing.T) {
	for _, s := range []string{"", "owner", "owner\n", "owner\n\n", "owner\r\n", "owner\r\n\r\n", "prefix <!-- kaze:peer-agents:begin --> suffix\n", "```\ninline " + begin + "\n```"} {
		t.Run(strings.ReplaceAll(s, "\n", "N"), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "AGENTS.md")
			put(t, p, s)
			if st, e := State(p); e != nil || st != NotInstalled {
				t.Fatalf("initial %v %v", st, e)
			}
			if e := Install(p); e != nil {
				t.Fatal(e)
			}
			installed := get(t, p)
			if st, e := State(p); e != nil || st != Installed {
				t.Fatalf("installed %v %v", st, e)
			}
			fi, _ := os.Stat(p)
			if fi.Mode().Perm() != 0640 {
				t.Fatal(fi.Mode())
			}
			if e := Install(p); e != nil {
				t.Fatal(e)
			}
			if get(t, p) != installed {
				t.Fatal("not idempotent")
			}
			if e := Remove(p); e != nil {
				t.Fatal(e)
			}
			if got := get(t, p); got != s {
				t.Fatalf("round trip %q want %q", got, s)
			}
			if e := Remove(p); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReplaceAndMarkers(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		p := filepath.Join(t.TempDir(), "doc")
		prefix := "before" + nl + nl
		suffix := nl + "after"
		put(t, p, prefix+begin+nl+"edited"+nl+end+suffix)
		if st, e := State(p); e != nil || st != Different {
			t.Fatalf("%v %v", st, e)
		}
		if e := Install(p); e != nil {
			t.Fatal(e)
		}
		if got := get(t, p); got != prefix+Block+suffix {
			t.Fatalf("replacement %q", got)
		}
		if e := Remove(p); e != nil {
			t.Fatal(e)
		}
		if got := get(t, p); got != "before"+nl+"after" {
			t.Fatalf("remove %q", got)
		}
		// Whole-line markers count even inside a code fence.
		put(t, p, "```\n"+begin+"\nold\n"+end+"\n```\n")
		if st, e := State(p); e != nil || st != Different {
			t.Fatalf("fence %v %v", st, e)
		}
		put(t, p, "before\n"+begin+"\nunfinished")
		for _, f := range []func(string) error{Install, Remove} {
			if e := f(p); e == nil {
				t.Fatal("lone begin accepted")
			}
			if get(t, p) != "before\n"+begin+"\nunfinished" {
				t.Fatal("changed malformed file")
			}
		}
		if _, e := State(p); e == nil {
			t.Fatal("state accepted malformed file")
		}
	}
}
func TestMissingSymlinksBackupsAndErrors(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "missing", "doc")
	if e := Remove(p); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Dir(p)); !os.IsNotExist(e) {
		t.Fatal("Remove created directory")
	}
	if e := Install(p); e != nil {
		t.Fatal(e)
	}
	real := filepath.Join(root, "real")
	put(t, real, "owner")
	link := filepath.Join(root, "link")
	if e := os.Symlink(real, link); e != nil {
		t.Fatal(e)
	}
	if e := Install(link); e != nil {
		t.Fatal(e)
	}
	if fi, e := os.Lstat(link); e != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink replaced")
	}
	if !strings.Contains(get(t, real), Block) {
		t.Fatal("target unchanged")
	}
	backups, _ := filepath.Glob(link + ".bak-*")
	if len(backups) != 1 || get(t, backups[0]) != "owner" {
		t.Fatal("backup", backups)
	}
	if e := Remove(link); e != nil {
		t.Fatal(e)
	}
	if get(t, real) != "owner" {
		t.Fatal("target not restored")
	}
	for _, f := range []func(string) error{Install, Remove} {
		if e := f(root); e == nil {
			t.Fatal("directory accepted")
		}
	}
	if _, e := State(root); e == nil {
		t.Fatal("directory state accepted")
	}
	put(t, real, "private")
	if e := os.Chmod(real, 0400); e != nil {
		t.Fatal(e)
	}
	if e := Install(real); e == nil {
		t.Fatal("read-only accepted")
	}
	if get(t, real) != "private" {
		t.Fatal("changed read-only")
	}
}
func TestPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", "")
	if p, e := ClaudePath(); e != nil || p != filepath.Join(home, ".claude", "CLAUDE.md") {
		t.Fatal(p, e)
	}
	if p, e := CodexPath(); e != nil || p != filepath.Join(home, ".codex", "AGENTS.md") {
		t.Fatal(p, e)
	}
	custom := t.TempDir()
	t.Setenv("CODEX_HOME", custom)
	if p, e := CodexPath(); e != nil || p != filepath.Join(custom, "AGENTS.md") {
		t.Fatal(p, e)
	}
}
