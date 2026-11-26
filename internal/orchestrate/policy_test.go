package orchestrate

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

type fakeFS struct {
	dirs  map[string]bool
	links map[string]string
}

func (f fakeFS) EvalSymlinks(p string) (string, error) {
	if target, ok := f.links[p]; ok {
		return target, nil
	}
	if _, ok := f.dirs[p]; !ok {
		return "", fs.ErrNotExist
	}
	return p, nil
}
func (f fakeFS) Stat(p string) (fs.FileInfo, error) {
	dir, ok := f.dirs[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	mode := fs.FileMode(0600)
	if dir {
		mode |= fs.ModeDir
	}
	return fs.Stat(fstest.MapFS{"entry": &fstest.MapFile{Mode: mode}}, "entry")
}
func fixture() (Request, Parent, fakeFS) {
	root := filepath.Join(string(filepath.Separator), "repo")
	if filepath.Separator == '\\' {
		root = `C:\repo`
	}
	return Request{Agent: "claude", Name: "review", Task: "review changes"}, Parent{Folder: root, RepoRoot: root}, fakeFS{dirs: map[string]bool{root: true, filepath.Join(root, "sub"): true, root + "-other": true, filepath.Join(root, "file"): false}, links: map[string]string{filepath.Join(root, "escape"): root + "-other"}}
}
func TestValidate(t *testing.T) {
	tests := []struct {
		name, code string
		change     func(*Request, *Parent)
	}{
		{"defaults", "", func(r *Request, p *Parent) {}},
		{"codex", "", func(r *Request, p *Parent) { r.Agent = "codex" }},
		{"unknown agent", CodeUnknownAgent, func(r *Request, p *Parent) { r.Agent = "sh" }},
		{"flags as agent", CodeUnknownAgent, func(r *Request, p *Parent) { r.Agent = "claude --dangerously-skip-permissions" }},
		{"template deferred", CodeUnknownTemplate, func(r *Request, p *Parent) { r.Agent = ""; r.Template = "review" }},
		{"empty task", CodeInvalidRequest, func(r *Request, p *Parent) { r.Task = " \n\t" }},
		{"large task", CodeTextTooLarge, func(r *Request, p *Parent) { r.Task = strings.Repeat("a", MaxTaskBytes+1) }},
		{"limit task", "", func(r *Request, p *Parent) { r.Task = strings.Repeat("a", MaxTaskBytes) }},
		{"nul task", CodeInvalidRequest, func(r *Request, p *Parent) { r.Task = "a\x00b" }},
		{"bad utf8", CodeInvalidRequest, func(r *Request, p *Parent) { r.Task = "\xff" }},
		{"empty name", "", func(r *Request, p *Parent) { r.Name = "" }},
		{"name leading dash", CodeInvalidRequest, func(r *Request, p *Parent) { r.Name = "-review" }},
		{"name newline", CodeInvalidRequest, func(r *Request, p *Parent) { r.Name = "a\nb" }},
		{"name long", CodeInvalidRequest, func(r *Request, p *Parent) { r.Name = strings.Repeat("x", 65) }},
		{"name quotes", "", func(r *Request, p *Parent) { r.Name = "John's review" }},
		{"read only", "", func(r *Request, p *Parent) { r.Mode = "read-only" }},
		{"bypass mode", CodeInvalidRequest, func(r *Request, p *Parent) { r.Mode = "bypassPermissions" }},
		{"worker depth", CodeNotAllowed, func(r *Request, p *Parent) { p.Depth = 1 }},
		{"missing", CodeBadCWD, func(r *Request, p *Parent) { r.CWD = filepath.Join(p.Folder, "missing") }},
		{"file", CodeBadCWD, func(r *Request, p *Parent) { r.CWD = filepath.Join(p.Folder, "file") }},
		{"relative child", "", func(r *Request, p *Parent) { r.CWD = "sub" }},
		{"symlink escape", CodeBadCWD, func(r *Request, p *Parent) { r.CWD = filepath.Join(p.Folder, "escape") }},
		{"prefix sibling", CodeBadCWD, func(r *Request, p *Parent) { r.CWD = p.Folder + "-other" }},
		{"any folder", "", func(r *Request, p *Parent) { r.CWD = p.Folder + "-other"; p.AnyFolder = true }},
		{"no repo fallback", "", func(r *Request, p *Parent) { p.RepoRoot = ""; r.CWD = "sub" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, p, f := fixture()
			tt.change(&r, &p)
			got, err := Validate(r, p, f)
			wantCode(t, err, tt.code)
			if err == nil && (!filepath.IsAbs(got.CWD) || got.Mode == "") {
				t.Fatalf("not normalized: %+v", got)
			}
		})
	}
	r, p, _ := fixture()
	_, err := Validate(r, p, nil)
	wantCode(t, err, CodeBadCWD)
}
func TestLimitsSlidingWindow(t *testing.T) {
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	for _, tt := range []struct {
		name string
		s    Usage
		code string
	}{
		{"available", Usage{}, ""},
		{"parent", Usage{ParentWorkers: 4}, CodeLimit},
		{"total", Usage{TotalWorkers: 8}, CodeLimit},
		{"rate", Usage{Starts: []time.Time{now, now, now, now, now, now}}, CodeRateLimited},
		{"boundary expires", Usage{Starts: []time.Time{now.Add(-time.Minute), now, now, now, now, now}}, ""},
		{"unsorted", Usage{Starts: []time.Time{now, now.Add(-time.Hour), now, now, now, now, now}}, CodeRateLimited},
	} {
		t.Run(tt.name, func(t *testing.T) { wantCode(t, CheckLimits(Limits{}, tt.s, clock), tt.code) })
	}
	wantCode(t, CheckLimits(Limits{MaxPerParent: 999, MaxTotal: 999}, Usage{ParentWorkers: 8}, clock), CodeLimit)
	wantCode(t, CheckLimits(Limits{MaxPerParent: 999, MaxTotal: 999}, Usage{TotalWorkers: 16}, clock), CodeLimit)
	wantCode(t, CheckLimits(Limits{MaxPerParent: 1}, Usage{ParentWorkers: 1}, clock), CodeLimit)
	wantCode(t, CheckLimits(Limits{PerMinute: 1}, Usage{Starts: []time.Time{now}}, clock), CodeRateLimited)
}

func TestNameMatchesControlConventions(t *testing.T) {
	for _, tt := range []struct{ name, code string }{
		{"t7", CodeInvalidRequest}, {"   ", CodeInvalidRequest},
		{strings.Repeat("日", 64), ""}, {strings.Repeat("日", 65), CodeInvalidRequest},
	} {
		r, p, f := fixture()
		r.Name = tt.name
		_, err := Validate(r, p, f)
		wantCode(t, err, tt.code)
	}
	r, p, f := fixture()
	r.Name = "  review  "
	got, err := Validate(r, p, f)
	wantCode(t, err, "")
	if got.Name != "review" {
		t.Fatalf("name = %q", got.Name)
	}
}
