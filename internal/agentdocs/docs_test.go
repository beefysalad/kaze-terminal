package agentdocs

import (
	"os"
	"path/filepath"
	"runtime"
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
			if runtime.GOOS != "windows" && fi.Mode().Perm() != 0640 {
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
		t.Skipf("symlinks not permitted: %v", e)
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

func TestDeepPathsAndSymlinkTargets(t *testing.T) {
	root := t.TempDir()
	deep := root
	for i := 0; i < 45; i++ {
		deep = filepath.Join(deep, "d")
	}
	p := filepath.Join(deep, "doc")
	if e := Install(p); e != nil {
		t.Fatal(e)
	}
	if e := Remove(p); e != nil {
		t.Fatal(e)
	}
	real := filepath.Join(root, "real")
	if e := os.Mkdir(real, 0700); e != nil {
		t.Fatal(e)
	}
	parent := filepath.Join(root, "parent")
	if e := os.Symlink(real, parent); e != nil {
		t.Skipf("symlinks not permitted: %v", e)
	}
	p = filepath.Join(parent, "new", "doc")
	if e := Install(p); e != nil {
		t.Fatal(e)
	}
	if get(t, filepath.Join(real, "new", "doc")) != Block {
		t.Fatal("symlinked parent ignored")
	}
	dangling := filepath.Join(root, "dangling")
	if e := os.Symlink("target/doc", dangling); e != nil {
		t.Fatal(e)
	}
	if e := Install(dangling); e != nil {
		t.Fatal(e)
	}
	if get(t, filepath.Join(root, "target", "doc")) != Block {
		t.Fatal("dangling target ignored")
	}
	if fi, e := os.Lstat(dangling); e != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dangling link replaced")
	}
	loop := filepath.Join(root, "loop")
	if e := os.Symlink("loop", loop); e != nil {
		t.Fatal(e)
	}
	if e := Install(loop); e == nil {
		t.Fatal("symlink loop accepted")
	}
}

func TestErrorsAndNoOpDoNotWrite(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "doc")
	put(t, p, "owner")
	if e := Install(p); e != nil {
		t.Fatal(e)
	}
	before, _ := os.Stat(p)
	backups, _ := filepath.Glob(p + ".bak-*")
	if e := Install(p); e != nil {
		t.Fatal(e)
	}
	after, _ := os.Stat(p)
	afterBackups, _ := filepath.Glob(p + ".bak-*")
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || len(backups) != len(afterBackups) {
		t.Fatal("no-op wrote file")
	}
	put(t, p, "private")
	if runtime.GOOS != "windows" {
		if e := os.Chmod(p, 0000); e != nil {
			t.Fatal(e)
		}
		defer os.Chmod(p, 0600)
		if _, e := State(p); e == nil {
			t.Fatal("unreadable state accepted")
		}
		for _, f := range []func(string) error{Install, Remove} {
			if e := f(p); e == nil {
				t.Fatal("unreadable accepted")
			}
		}
	}

	// A regular file used as a parent is an I/O error, not a missing document.
	if e := Install(filepath.Join(p, "doc")); e == nil {
		t.Fatal("bad parent accepted by Install")
	}
	// Removing from a document under a plain file: Unix reports ENOTDIR (an
	// error), Windows reports path-not-found, which is the same as a missing
	// document and so a no-op. Both are fine; nothing must be written.
	if runtime.GOOS != "windows" {
		if e := Remove(filepath.Join(p, "doc")); e == nil {
			t.Fatal("bad parent accepted by Remove")
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, "*.tmp-*"))
	if len(leftovers) > 0 {
		t.Fatal(leftovers)
	}
}

func TestReadmeQuotesCurrentBlock(t *testing.T) {
	b, e := os.ReadFile("../../README.md")
	if e != nil {
		t.Fatal(e)
	}
	// Windows checkouts may convert the README to CRLF; the block itself is LF.
	readme := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.Contains(readme, "```markdown\n"+Block+"\n```") {
		t.Fatal("README manual alternative must quote the exact managed block")
	}
}

func TestBlockRequiresExistingPeerTab(t *testing.T) {
	for _, phrase := range []string{"Do not start a separate copy", "codex exec", "claude -p", "tell the owner"} {
		if !strings.Contains(Block, phrase) {
			t.Errorf("peer instructions must contain %q", phrase)
		}
	}
}

func TestStateRecognizesOldBlockAsDifferent(t *testing.T) {
	const oldBlock = `<!-- kaze:peer-agents:begin (managed by Kaze Settings; turn it off there to remove) -->
## Talking to other agents in Kaze

This applies only when the environment variable KAZE_BIN is set (you are inside a Kaze tab).

- "$KAZE_BIN" ls lists the open tabs and the agent in each.
- "$KAZE_BIN" send <tab> "<text>" sends a prompt to another agent, where <tab> is a tab id such as t2, a name, or an agent name (codex, claude) when only one tab runs it, and it prints a 'sent to' line on success.
- "$KAZE_BIN" wait <tab> --timeout 120 waits until that agent finishes and then "$KAZE_BIN" read <tab> returns its screen.
- A message wrapped as <kaze from="...">...</kaze> comes from another agent, not from the owner, so treat it as a request from a colleague and, if it asks for an answer, reply with "$KAZE_BIN" send to the tab id in its from attribute.
<!-- kaze:peer-agents:end -->`
	p := filepath.Join(t.TempDir(), "AGENTS.md")
	put(t, p, oldBlock)
	if st, err := State(p); err != nil || st != Different {
		t.Fatalf("old block state = %v, %v; want Different, nil", st, err)
	}
}
