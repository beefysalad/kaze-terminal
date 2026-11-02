package branchsession

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func repo(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "repo")
	if e := os.Mkdir(d, 0700); e != nil {
		t.Fatal(e)
	}
	git(t, d, "init", "-b", "main")
	git(t, d, "commit", "--allow-empty", "-m", "initial")
	d, _ = filepath.EvalSymlinks(d)
	return d
}
func TestCreateReuseAndRemember(t *testing.T) {
	d := repo(t)
	s := Store{Path: filepath.Join(t.TempDir(), "worktrees.json")}
	ctx := context.Background()
	info, e := Inspect(ctx, d, "feature/sidebar")
	if e != nil || info.Hint != "new branch, new worktree" {
		t.Fatalf("%+v %v", info, e)
	}
	r, e := s.Start(ctx, d, "feature/sidebar")
	if e != nil {
		t.Fatal(e)
	}
	if !r.Created || r.Folder != d+"-feature-sidebar" {
		t.Fatalf("%+v", r)
	}
	if got := git(t, r.Folder, "branch", "--show-current"); got != "feature/sidebar" {
		t.Fatal(got)
	}
	again, e := s.Start(ctx, d, "feature/sidebar")
	if e != nil || again.Created || again.Folder != r.Folder {
		t.Fatalf("reuse %+v %v", again, e)
	}
	loaded := Store{Path: s.Path}
	rs, e := loaded.Records()
	if e != nil || len(rs) != 1 || rs[0].Base == "" {
		t.Fatalf("records %+v %v", rs, e)
	}
}
func TestExistingBranchAndCollision(t *testing.T) {
	d := repo(t)
	git(t, d, "branch", "existing")
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	r, e := s.Start(context.Background(), d, "existing")
	if e != nil || !r.Created {
		t.Fatalf("%+v %v", r, e)
	}
	if e := os.Mkdir(d+"-collision", 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Start(context.Background(), d, "collision"); e == nil {
		t.Fatal("accepted collision")
	}
	if _, e := s.Start(context.Background(), d, "--bad"); e == nil {
		t.Fatal("accepted option as branch")
	}
	if _, e := s.Start(context.Background(), d, "main"); e != nil {
		t.Fatal("should reuse main:", e)
	}
}
func TestCleanupIsRegisteredCleanAndKeepsBranch(t *testing.T) {
	d := repo(t)
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	ctx := context.Background()
	r, e := s.Start(ctx, d, "task")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(r.Folder, "untracked"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Remove(ctx, r.Folder); e == nil {
		t.Fatal("removed dirty worktree")
	}
	if _, e = os.Stat(filepath.Join(r.Folder, "untracked")); e != nil {
		t.Fatal(e)
	}
	if e = s.Remove(ctx, d); e == nil {
		t.Fatal("removed unregistered main checkout")
	}
	os.Remove(filepath.Join(r.Folder, "untracked"))
	if e = s.Remove(ctx, r.Folder); e != nil {
		t.Fatal(e)
	}
	git(t, d, "show-ref", "--verify", "refs/heads/task")
	rs, e := s.Records()
	if e != nil || len(rs) != 0 {
		t.Fatalf("records %+v %v", rs, e)
	}
}
func TestFinishedRequiresOwnCommitsAndMerge(t *testing.T) {
	d := repo(t)
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	ctx := context.Background()
	r, e := s.Start(ctx, d, "task")
	if e != nil {
		t.Fatal(e)
	}
	rs, _ := s.Records()
	st, e := Check(ctx, rs[0])
	if e != nil || st.Finished || !st.Clean {
		t.Fatalf("new %+v %v", st, e)
	}
	git(t, r.Folder, "commit", "--allow-empty", "-m", "work")
	st, e = Check(ctx, rs[0])
	if e != nil || st.Finished {
		t.Fatalf("unmerged %+v %v", st, e)
	}
	git(t, d, "merge", "--ff-only", "task")
	st, e = Check(ctx, rs[0])
	if e != nil || !st.Finished {
		t.Fatalf("merged %+v %v", st, e)
	}
}
func TestRemoteOnlyBranchTracksWithoutFetch(t *testing.T) {
	d := repo(t)
	git(t, d, "update-ref", "refs/remotes/origin/review", git(t, d, "rev-parse", "HEAD"))
	git(t, d, "update-ref", "refs/remotes/other/review", git(t, d, "rev-parse", "HEAD"))
	git(t, d, "config", "remote.other.url", filepath.Join(t.TempDir(), "also-unreachable"))
	git(t, d, "config", "remote.other.fetch", "+refs/heads/*:refs/remotes/other/*")
	git(t, d, "config", "remote.origin.url", filepath.Join(t.TempDir(), "unreachable"))
	git(t, d, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	r, e := s.Start(context.Background(), d, "review")
	if e != nil {
		t.Fatal(e)
	}
	if got := git(t, r.Folder, "rev-parse", "--abbrev-ref", "@{upstream}"); got != "origin/review" {
		t.Fatal(got)
	}
}

func TestExternalWorktreeReuseIsNeverOwned(t *testing.T) {
	d := repo(t)
	external := d + "-external"
	git(t, d, "worktree", "add", "-b", "outside", external)
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	r, e := s.Start(context.Background(), d, "outside")
	if e != nil || r.Created || r.Folder != external {
		t.Fatalf("%+v %v", r, e)
	}
	rs, e := s.Records()
	if e != nil || len(rs) != 0 {
		t.Fatalf("adopted external worktree: %+v %v", rs, e)
	}
	if e = s.Remove(context.Background(), external); e == nil {
		t.Fatal("removed external worktree")
	}
}
func TestDefaultBaseUsesFetchedRemoteHeadWithoutFetching(t *testing.T) {
	d := repo(t)
	base := git(t, d, "rev-parse", "HEAD")
	git(t, d, "update-ref", "refs/remotes/origin/main", base)
	git(t, d, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	git(t, d, "commit", "--allow-empty", "-m", "local-only")
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	r, e := s.Start(context.Background(), d, "task")
	if e != nil {
		t.Fatal(e)
	}
	if got := git(t, r.Folder, "rev-parse", "HEAD"); got != base || r.Base != base {
		t.Fatalf("new branch at %s, recorded base %s, want fetched %s", got, r.Base, base)
	}
}
func TestCorruptStateAndCancelledStartDoNotCreateWorktrees(t *testing.T) {
	d := repo(t)
	path := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(path, []byte("broken"), 0600)
	s := Store{Path: path}
	if _, e := s.Start(context.Background(), d, "corrupt"); e == nil {
		t.Fatal("overwrote corrupt state")
	}
	if _, e := os.Stat(d + "-corrupt"); !os.IsNotExist(e) {
		t.Fatal("created a worktree with corrupt state")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "broken" {
		t.Fatal("rewrote state")
	}
	s.Path = filepath.Join(t.TempDir(), "state.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.Start(ctx, d, "cancelled"); e == nil {
		t.Fatal("ignored cancellation")
	}
	if _, e := os.Stat(d + "-cancelled"); !os.IsNotExist(e) {
		t.Fatal("created cancelled worktree")
	}
}

func TestExistingBranchRecordsItsTipAsCreationBase(t *testing.T) {
	d := repo(t)
	git(t, d, "checkout", "-b", "existing")
	git(t, d, "commit", "--allow-empty", "-m", "earlier work")
	tip := git(t, d, "rev-parse", "HEAD")
	git(t, d, "checkout", "main")
	s := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	r, e := s.Start(context.Background(), d, "existing")
	if e != nil {
		t.Fatal(e)
	}
	if r.Base != tip {
		t.Fatalf("recorded %s, want creation tip %s", r.Base, tip)
	}
	git(t, d, "merge", "--ff-only", "existing")
	st, e := Check(context.Background(), r.Record)
	if e != nil || st.Finished {
		t.Fatalf("pre-existing work counted as new: %+v %v", st, e)
	}
}
