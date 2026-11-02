// Package branchsession owns local branch worktrees created by Kaze. Callers
// run its bounded filesystem/git operations on a background worker.
package branchsession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const Timeout = 15 * time.Second
const maxState = 1 << 20
const maxRecords = 256

type Record struct {
	Folder  string    `json:"folder"`
	Repo    string    `json:"repo"`
	Branch  string    `json:"branch"`
	Base    string    `json:"base"`
	Created time.Time `json:"created"`
}
type Result struct {
	Record
	Created bool
}
type Info struct {
	Main, Repo, Folder, Branch, Base, Hint, RemoteRef string
	Existing, Remote                                  bool
}
type Status struct {
	Record
	Clean, Finished bool
}
type state struct {
	Version   int      `json:"version"`
	Worktrees []Record `json:"worktrees"`
}

// Store serializes state mutations, including multiple requests from one window.
type Store struct {
	Path string
	mu   sync.Mutex
}

// cappedOutput still reports success to the child after filling its limit, so
// pathological git output cannot grow memory or leave a blocked pipe.
type cappedOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if n > maxState-b.Len() {
		b.overflow = true
	}
	if room := maxState - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(len(p), room)])
	}
	return n, nil
}
func run(ctx context.Context, dir string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	hideWindow(c)
	c.WaitDelay = 500 * time.Millisecond
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		switch strings.ToUpper(key) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE":
			continue
		}
		c.Env = append(c.Env, v)
	}
	var out, stderr cappedOutput
	c.Stdout = &out
	c.Stderr = &stderr
	if e := c.Run(); e != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = e.Error()
		}
		return "", fmt.Errorf("git: %s", msg)
	}
	if out.overflow {
		return "", errors.New("Git output exceeded the 1 MB limit.")
	}
	return strings.TrimSpace(out.String()), nil
}
func exists(ctx context.Context, dir, ref string) bool {
	_, e := run(ctx, dir, "show-ref", "--verify", "--quiet", ref)
	return e == nil
}
func Slug(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
func defaultRef(ctx context.Context, main string) string {
	if r, e := run(ctx, main, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); e == nil && exists(ctx, main, r) {
		return r
	}
	for _, b := range []string{"main", "master"} {
		if exists(ctx, main, "refs/heads/"+b) {
			return "refs/heads/" + b
		}
	}
	return "HEAD"
}

type tree struct{ path, branch string }

func trees(ctx context.Context, dir string) ([]tree, error) {
	raw, e := run(ctx, dir, "worktree", "list", "--porcelain", "-z")
	if e != nil {
		return nil, e
	}
	var out []tree
	var cur tree
	for _, f := range strings.Split(raw, "\x00") {
		switch {
		case strings.HasPrefix(f, "worktree "):
			if cur.path != "" {
				out = append(out, cur)
			}
			cur = tree{path: worktreePath(strings.TrimPrefix(f, "worktree "))}
		case strings.HasPrefix(f, "branch refs/heads/"):
			cur.branch = strings.TrimPrefix(f, "branch refs/heads/")
		}
	}
	if cur.path != "" {
		out = append(out, cur)
	}
	return out, nil
}
func Inspect(ctx context.Context, dir, branch string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	branch = strings.TrimSpace(branch)
	if len(branch) > 200 || branch == "" {
		return Info{}, errors.New("Enter a branch name (up to 200 characters).")
	}
	if _, e := run(ctx, dir, "check-ref-format", "--branch", branch); e != nil {
		return Info{}, e
	}
	// check-ref-format accepts @{-1}; only literal local branch names are allowed.
	if strings.HasPrefix(branch, "-") || strings.Contains(branch, "@{") {
		return Info{}, errors.New("Enter a literal branch name.")
	}
	ts, e := trees(ctx, dir)
	if e != nil {
		return Info{}, e
	}
	if len(ts) == 0 {
		return Info{}, errors.New("No main checkout found.")
	}
	common, e := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return Info{}, e
	}
	in := Info{Main: ts[0].path, Repo: filepath.Clean(common), Branch: branch, Hint: "new branch, new worktree"}
	in.Base, e = run(ctx, in.Main, "rev-parse", defaultRef(ctx, in.Main)+"^{commit}")
	if e != nil {
		return Info{}, e
	}
	for _, t := range ts {
		if t.branch == branch {
			in.Folder = t.path
			in.Existing = true
			in.Hint = "opens its worktree"
			return in, nil
		}
	}
	in.Folder = filepath.Join(filepath.Dir(in.Main), filepath.Base(in.Main)+"-"+Slug(branch))
	in.Existing = exists(ctx, in.Main, "refs/heads/"+branch)
	if !in.Existing {
		// Prefer origin; git's normal DWIM chooses a sole other remote too.
		refs, e := run(ctx, in.Main, "for-each-ref", "--format=%(refname)", "refs/remotes/")
		if e != nil {
			return Info{}, e
		}
		var matches []string
		for _, r := range strings.Split(refs, "\n") {
			_, b, ok := strings.Cut(strings.TrimPrefix(r, "refs/remotes/"), "/")
			if ok && b == branch {
				matches = append(matches, r)
			}
		}
		switch {
		case exists(ctx, in.Main, "refs/remotes/origin/"+branch):
			in.Existing, in.Remote = true, true
			in.RemoteRef = "refs/remotes/origin/" + branch
		case len(matches) == 1:
			in.Existing, in.Remote = true, true
			in.RemoteRef = matches[0]
		case len(matches) > 1:
			return Info{}, errors.New("Branch exists on several remotes; create a local tracking branch first.")
		}
	}
	if in.Existing {
		in.Hint = "existing branch, new worktree"
		ref := "refs/heads/" + branch
		if in.Remote {
			ref = in.RemoteRef
		}
		in.Base, e = run(ctx, in.Main, "rev-parse", ref+"^{commit}")
		if e != nil {
			return Info{}, e
		}
	}
	return in, nil
}

func (s *Store) read() (state, error) {
	st := state{Version: 1}
	if s.Path == "" {
		return st, errors.New("Worktree state path is unavailable.")
	}
	f, e := os.Open(s.Path)
	if os.IsNotExist(e) {
		return st, nil
	}
	if e != nil {
		return st, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxState+1))
	if e != nil {
		return st, e
	}
	if len(b) > maxState {
		return st, errors.New("Worktree state file is too large.")
	}
	if e = json.Unmarshal(b, &st); e != nil {
		return st, e
	}
	if st.Version != 1 || len(st.Worktrees) > maxRecords {
		return st, errors.New("Unsupported worktree state file.")
	}
	for _, r := range st.Worktrees {
		if !filepath.IsAbs(r.Folder) || !filepath.IsAbs(r.Repo) || r.Branch == "" || r.Base == "" {
			return st, errors.New("Invalid worktree record.")
		}
	}
	return st, nil
}
func (s *Store) write(st state) error {
	b, e := json.MarshalIndent(st, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.Path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.Path), ".worktrees-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, s.Path)
}
func prune(st *state) bool {
	changed := false
	out := st.Worktrees[:0]
	for _, r := range st.Worktrees {
		if _, e := os.Stat(r.Folder); os.IsNotExist(e) {
			changed = true
			continue
		}
		out = append(out, r)
	}
	st.Worktrees = out
	return changed
}
func (s *Store) Records() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, e := s.read()
	if e != nil {
		return nil, e
	}
	if prune(&st) {
		if e = s.write(st); e != nil {
			return nil, e
		}
	}
	return st.Worktrees, nil
}
func (s *Store) Start(ctx context.Context, dir, branch string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	st, e := s.read()
	if e != nil {
		return Result{}, e
	}
	prune(&st)
	in, e := Inspect(ctx, dir, branch)
	if e != nil {
		return Result{}, e
	}
	r := Result{Record: Record{Folder: in.Folder, Repo: in.Repo, Branch: in.Branch, Base: in.Base, Created: time.Now()}}
	if in.Hint == "opens its worktree" {
		return r, nil
	}
	if len(st.Worktrees) >= maxRecords {
		return Result{}, errors.New("Worktree memory is full; clean up a worktree first.")
	}
	if _, e = os.Lstat(in.Folder); e == nil {
		return Result{}, fmt.Errorf("Folder already exists: %s", in.Folder)
	} else if !os.IsNotExist(e) {
		return Result{}, e
	}
	args := []string{"worktree", "add", in.Folder, in.Branch}
	if in.Remote {
		args = []string{"worktree", "add", "--track", "-b", in.Branch, in.Folder, in.RemoteRef}
	}
	if !in.Existing {
		args = []string{"worktree", "add", "-b", in.Branch, in.Folder, in.Base}
	}
	if _, e = run(ctx, in.Main, args...); e != nil {
		return Result{}, e
	}
	r.Created = true
	st.Worktrees = append(st.Worktrees, r.Record)
	if e = s.write(st); e != nil {
		return Result{}, fmt.Errorf("Worktree created at %s, but could not record it: %w", in.Folder, e)
	}
	return r, nil
}
func registered(ctx context.Context, r Record) (string, error) {
	ts, e := trees(ctx, r.Folder)
	if e != nil {
		return "", e
	}
	common, e := run(ctx, r.Folder, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return "", e
	}
	if filepath.Clean(common) != filepath.Clean(r.Repo) {
		return "", errors.New("Worktree repository changed; leaving it alone.")
	}
	for _, t := range ts {
		if filepath.Clean(t.path) == filepath.Clean(r.Folder) && t.branch == r.Branch {
			if filepath.Clean(ts[0].path) == filepath.Clean(r.Folder) {
				return "", errors.New("Refusing to remove a main checkout.")
			}
			return ts[0].path, nil
		}
	}
	return "", errors.New("Worktree identity changed; leaving it alone.")
}
func Check(ctx context.Context, r Record) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	st := Status{Record: r}
	main, e := registered(ctx, r)
	if e != nil {
		return st, e
	}
	raw, e := run(ctx, r.Folder, "status", "--porcelain", "--untracked-files=normal")
	if e != nil {
		return st, e
	}
	st.Clean = raw == ""
	own, e := run(ctx, main, "rev-list", "--count", r.Base+"..refs/heads/"+r.Branch)
	if e != nil {
		return st, e
	}
	if own != "0" {
		_, e = run(ctx, main, "merge-base", "--is-ancestor", "refs/heads/"+r.Branch, defaultRef(ctx, main))
		st.Finished = e == nil
	}
	return st, nil
}
func (s *Store) Remove(ctx context.Context, folder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	st, e := s.read()
	if e != nil {
		return e
	}
	for i, r := range st.Worktrees {
		if filepath.Clean(r.Folder) != filepath.Clean(folder) {
			continue
		}
		main, e := registered(ctx, r)
		if e != nil {
			return e
		}
		if _, e = run(ctx, main, "worktree", "remove", r.Folder); e != nil {
			return e
		}
		st.Worktrees = append(st.Worktrees[:i], st.Worktrees[i+1:]...)
		if e = s.write(st); e != nil {
			return fmt.Errorf("Folder removed, but could not update worktree memory: %w", e)
		}
		return nil
	}
	return errors.New("Kaze did not create this worktree.")
}

// Git emits slash-separated worktree paths on Windows too. Normalize before
// comparing a reused checkout with a folder Kaze created using filepath.Join.
func worktreePath(path string) string { return filepath.Clean(filepath.FromSlash(path)) }
