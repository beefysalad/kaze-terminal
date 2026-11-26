// Package orchestrate contains worker policy and launch planning. It starts no
// processes, owns no goroutines or timers, and reads only an injected filesystem.
package orchestrate

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxTaskBytes = 16 << 10
const (
	CodeInvalidRequest  = "invalid_request"
	CodeUnknownAgent    = "unknown_agent"
	CodeUnknownTemplate = "unknown_template"
	CodeBadCWD          = "bad_cwd"
	CodeNotAllowed      = "not_allowed"
	CodeLimit           = "limit"
	CodeRateLimited     = "rate_limited"
	CodeTextTooLarge    = "text_too_large"
	CodeSpawnFailed     = "spawn_failed"
)

// Error exposes stable codes for the control transport and CLI.
type Error struct{ Code, Message string }

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{Code: code, Message: message} }

// Request has no arbitrary flags. Templates are reserved for the template loader;
// until resolved to a concrete agent they fail closed with unknown_template.
type Request struct{ Agent, Template, Name, Task, CWD, Mode string }

// Parent is supplied by the authenticated host, never from request claims.
type Parent struct {
	Folder, RepoRoot string
	Depth            int
	AnyFolder        bool
}

// FileSystem resolves symlinks before containment checks. Implementations must
// return absolute canonical paths from EvalSymlinks for absolute input paths.
type FileSystem interface {
	EvalSymlinks(string) (string, error)
	Stat(string) (fs.FileInfo, error)
}

func validateText(r Request) error {
	if r.Template != "" {
		return failure(CodeUnknownTemplate, "template must be resolved before spawning")
	}
	if r.Agent != "claude" && r.Agent != "codex" {
		return failure(CodeUnknownAgent, "agent must be claude or codex")
	}
	if len(r.Task) > MaxTaskBytes {
		return failure(CodeTextTooLarge, "task exceeds 16 KiB")
	}
	if strings.TrimSpace(r.Task) == "" || !utf8.ValidString(r.Task) || strings.ContainsRune(r.Task, 0) {
		return failure(CodeInvalidRequest, "task must be nonempty UTF-8 text without NUL")
	}
	name := strings.TrimSpace(r.Name)
	if !utf8.ValidString(r.Name) || utf8.RuneCountInString(name) > 64 || (r.Name != "" && name == "") || strings.HasPrefix(name, "-") || tabID.MatchString(name) || strings.IndexFunc(r.Name, unicode.IsControl) >= 0 {
		return failure(CodeInvalidRequest, "name must be 1 to 64 runes without controls, a leading hyphen or a tab ID")
	}
	if r.Mode != "" && r.Mode != "edit" && r.Mode != "read-only" {
		return failure(CodeInvalidRequest, "mode must be edit or read-only")
	}
	return nil
}

// Validate returns a normalized copy, checking the cwd through the supplied fs.
// Relative cwd paths are relative to the parent's folder, never the host cwd.
func Validate(r Request, p Parent, f FileSystem) (Request, error) {
	if p.Depth != 0 {
		return Request{}, failure(CodeNotAllowed, "workers cannot start workers")
	}
	if err := validateText(r); err != nil {
		return Request{}, err
	}
	if f == nil || !filepath.IsAbs(p.Folder) {
		return Request{}, failure(CodeBadCWD, "parent folder must be absolute and filesystem must be supplied")
	}
	cwd := r.CWD
	if cwd == "" {
		cwd = p.Folder
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(p.Folder, cwd)
	}
	cwd, err := f.EvalSymlinks(filepath.Clean(cwd))
	if err != nil || !filepath.IsAbs(cwd) {
		return Request{}, failure(CodeBadCWD, "worker folder does not exist")
	}
	st, err := f.Stat(cwd)
	if err != nil || st == nil || !st.IsDir() {
		return Request{}, failure(CodeBadCWD, "worker folder must be a directory")
	}
	if !p.AnyFolder {
		root := p.RepoRoot
		if root == "" {
			root = p.Folder
		}
		if !filepath.IsAbs(root) {
			return Request{}, failure(CodeBadCWD, "parent root must be absolute")
		}
		root, err = f.EvalSymlinks(root)
		if err != nil || !filepath.IsAbs(root) {
			return Request{}, failure(CodeBadCWD, "cannot resolve parent root")
		}
		rel, err := filepath.Rel(root, cwd)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return Request{}, failure(CodeBadCWD, "worker folder is outside the parent root")
		}
	}
	r.Name = strings.TrimSpace(r.Name)
	r.CWD = cwd
	if r.Mode == "" {
		r.Mode = "edit"
	}
	return r, nil
}

// Limits uses defaults for nonpositive values. Worker caps have hard ceilings.
type Limits struct{ MaxPerParent, MaxTotal, PerMinute int }

// Usage is an immutable snapshot from the host. Starts is this parent's spawn
// history, including workers which have since exited. Only successful starts
// should be recorded. The host serializes admission and records a start before
// admitting another request; this function does not reserve capacity.
type Usage struct {
	ParentWorkers, TotalWorkers int
	Starts                      []time.Time
}

// CheckLimits computes a sliding window on demand. The host supplies its clock
// and prunes history on requests, so there is no idle work or retained state here.
func CheckLimits(l Limits, u Usage, clock func() time.Time) error {
	if l.MaxPerParent <= 0 {
		l.MaxPerParent = 4
	}
	if l.MaxPerParent > 8 {
		l.MaxPerParent = 8
	}
	if l.MaxTotal <= 0 {
		l.MaxTotal = 8
	}
	if l.MaxTotal > 16 {
		l.MaxTotal = 16
	}
	if l.PerMinute <= 0 {
		l.PerMinute = 6
	}
	if u.ParentWorkers >= l.MaxPerParent || u.TotalWorkers >= l.MaxTotal {
		return failure(CodeLimit, "worker limit reached")
	}
	if clock == nil {
		return failure(CodeInvalidRequest, "clock must be supplied")
	}
	cutoff := clock().Add(-time.Minute)
	count := 0
	for _, start := range u.Starts {
		if start.After(cutoff) {
			count++
			if count >= l.PerMinute {
				return failure(CodeRateLimited, "parent spawn rate exceeded")
			}
		}
	}
	return nil
}
