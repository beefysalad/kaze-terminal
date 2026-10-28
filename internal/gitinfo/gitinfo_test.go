package gitinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBranchFromHead(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/feature/x\n")
	if got := Branch(root); got != "feature/x" {
		t.Fatalf("got %q", got)
	}
}

func TestBranchFromSubdir(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Branch(sub); got != "main" {
		t.Fatalf("got %q", got)
	}
}

func TestDetachedHead(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git", "HEAD"), strings.Repeat("ab12cd3", 6)[:40]+"\n")
	if got := Branch(root); got != "ab12cd3" {
		t.Fatalf("got %q", got)
	}
}

func TestWorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", ".git", "worktrees", "w", "HEAD"), "ref: refs/heads/wt-branch\n")
	wt := filepath.Join(root, "wt")
	write(t, filepath.Join(wt, ".git"), "gitdir: ../real/.git/worktrees/w\n")
	if got := Branch(wt); got != "wt-branch" {
		t.Fatalf("relative: got %q", got)
	}
	// Absolute gitdir, forward slashes.
	abs := filepath.ToSlash(filepath.Join(root, "real", ".git", "worktrees", "w"))
	write(t, filepath.Join(wt, ".git"), "gitdir: "+abs+"\n")
	if got := Branch(wt); got != "wt-branch" {
		t.Fatalf("absolute: got %q", got)
	}
}

func TestNotARepo(t *testing.T) {
	if got := Branch(t.TempDir()); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestGitdirLoopStops(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git"), "gitdir: .git\n")
	done := make(chan string, 1)
	go func() { done <- Branch(root) }()
	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not stop")
	}
}

func TestDepthLimit(t *testing.T) {
	root := t.TempDir()
	dir := root
	for i := 0; i < 65; i++ {
		dir = filepath.Join(dir, "d")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Skip("cannot create deep dir:", err)
	}
	// A repo above the 64-level limit must not be found.
	write(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/far\n")
	start := time.Now()
	if got := Branch(dir); got != "" {
		t.Fatalf("got %q", got)
	}
	if time.Since(start) > time.Second {
		t.Fatal("too slow")
	}
}

func TestUnreadableHead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "HEAD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Branch(root); got != "" {
		t.Fatalf("got %q", got)
	}
}
