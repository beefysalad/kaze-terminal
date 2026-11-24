// Package agentdocs manages the optional peer instructions in global agent files.
package agentdocs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const begin = "<!-- kaze:peer-agents:begin (managed by Kaze Settings; turn it off there to remove) -->"
const end = "<!-- kaze:peer-agents:end -->"

// Block is the single source for the managed instructions, including markers.
// It has no final newline so appending can preserve the owner's final newline.
const Block = begin + `
## Talking to other agents in Kaze

This applies only when the environment variable KAZE_BIN is set (you are inside a Kaze tab).

- "$KAZE_BIN" ls lists the open tabs and the agent in each.
- "$KAZE_BIN" send <tab> "<text>" sends a prompt to another agent, where <tab> is a tab id such as t2, a name, or an agent name (codex, claude) when only one tab runs it, and it prints a 'sent to' line on success.
- "$KAZE_BIN" wait <tab> --timeout 120 waits until that agent finishes and then "$KAZE_BIN" read <tab> returns its screen.
- A message wrapped as <kaze from="...">...</kaze> comes from another agent, not from the owner, so treat it as a request from a colleague and, if it asks for an answer, reply with "$KAZE_BIN" send to the tab id in its from attribute.
` + end

type Status int

const (
	NotInstalled Status = iota
	Installed
	Different
)

func ClaudePath() (string, error) {
	h, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(h, ".claude", "CLAUDE.md"), nil
}
func CodexPath() (string, error) {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return filepath.Join(h, "AGENTS.md"), nil
	}
	h, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(h, ".codex", "AGENTS.md"), nil
}

// span recognizes only exact whole-line markers, including in fenced Markdown.
// Reject ambiguous/malformed markers rather than risking unrelated text.
func span(b []byte) (start, stop int, err error) {
	start, stop = -1, -1
	for off := 0; off < len(b); {
		n := bytes.IndexByte(b[off:], '\n')
		next := len(b)
		if n >= 0 {
			next = off + n + 1
		}
		line := strings.TrimSuffix(strings.TrimSuffix(string(b[off:next]), "\n"), "\r")
		switch line {
		case begin:
			if start >= 0 {
				return -1, -1, errors.New("multiple peer instruction begin markers")
			}
			start = off
		case end:
			if start < 0 || stop >= 0 {
				return -1, -1, errors.New("unmatched peer instruction end marker")
			}
			stop = next
			if n >= 0 {
				stop--
			}
			if stop > off && b[stop-1] == '\r' {
				stop--
			}
		}
		off = next
	}
	if start >= 0 && stop < 0 {
		return -1, -1, errors.New("peer instruction begin marker has no end marker")
	}
	return
}

func read(path string) ([]byte, error) {
	st, e := os.Stat(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if st.Mode().Perm()&0444 == 0 {
		return nil, fmt.Errorf("%s is not readable", path)
	}
	return os.ReadFile(path)
}

func State(path string) (Status, error) {
	b, e := read(path)
	if e != nil {
		return NotInstalled, e
	}
	s, z, e := span(b)
	if e != nil {
		return NotInstalled, e
	}
	if s < 0 {
		return NotInstalled, nil
	}
	if string(b[s:z]) == Block {
		return Installed, nil
	}
	return Different, nil
}

func Install(path string) error { return change(path, true) }
func Remove(path string) error  { return change(path, false) }

func change(path string, install bool) error {
	b, e := read(path)
	if e != nil {
		return e
	}
	s, z, e := span(b)
	if e != nil {
		return e
	}
	var out []byte
	switch {
	case s >= 0 && install:
		out = append(out, b[:s]...)
		out = append(out, Block...)
		out = append(out, b[z:]...)
	case s >= 0:
		// Install retains whether the original ended in a newline. At EOF a
		// block without a final newline owns two preceding line breaks; otherwise
		// it owns just the blank separator line. Interior blocks own one blank line.
		cut := s
		if z == len(b) {
			cut = trimBreak(b, cut)
			cut = trimBreak(b, cut)
		} else {
			prior := trimBreak(b, cut)
			if prior > 0 && b[prior-1] == '\n' {
				cut = prior
			}
			if z < len(b) && b[z] == '\r' {
				z++
			}
			if z < len(b) && b[z] == '\n' {
				z++
			}
		}
		out = append(out, b[:cut]...)
		out = append(out, b[z:]...)
	case !install:
		return nil
	default:
		out = append(out, b...)
		if len(b) > 0 {
			if b[len(b)-1] == '\n' {
				out = append(out, '\n')
			} else {
				out = append(out, '\n', '\n')
			}
		}
		out = append(out, Block...)
		if len(b) > 0 && b[len(b)-1] == '\n' {
			out = append(out, '\n')
		}
	}
	if bytes.Equal(out, b) {
		return nil
	}
	return write(path, b, out, install)
}
func trimBreak(b []byte, n int) int {
	if n > 0 && b[n-1] == '\n' {
		n--
		if n > 0 && b[n-1] == '\r' {
			n--
		}
	}
	return n
}

// resolve also follows symlinked parents and dangling file symlinks. Missing
// paths are allowed so Install can create the target without replacing links.
func resolve(path string, depth int) (string, error) {
	if depth > 40 {
		return "", errors.New("too many symlinks")
	}
	path = filepath.Clean(path)
	st, e := os.Lstat(path)
	if e == nil && st.Mode()&os.ModeSymlink != 0 {
		target, e := os.Readlink(path)
		if e != nil {
			return "", e
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		return resolve(target, depth+1)
	}
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}
	real, e := resolve(parent, depth)
	if e != nil {
		return "", e
	}
	return filepath.Join(real, filepath.Base(path)), nil
}
func write(path string, old, data []byte, create bool) error {
	real, e := resolve(path, 0)
	if e != nil {
		return e
	}
	mode := os.FileMode(0600)
	st, e := os.Stat(real)
	if e == nil {
		mode = st.Mode().Perm()
		if mode&0222 == 0 {
			return fmt.Errorf("%s is not writable", path)
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if create {
		if e = os.MkdirAll(filepath.Dir(real), 0700); e != nil {
			return e
		}
	}
	if st != nil {
		if e = backup(path, old); e != nil {
			return fmt.Errorf("backup: %w", e)
		}
	}
	f, e := os.CreateTemp(filepath.Dir(real), filepath.Base(real)+".tmp-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, real)
}

// Match setup's timestamped, exclusive backups beside the user-facing path.
func backup(path string, b []byte) error {
	base := path + ".bak-" + time.Now().Format("20060102-150405")
	for i := 1; i <= 100; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		f, e := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if errors.Is(e, os.ErrExist) {
			continue
		}
		if e != nil {
			return e
		}
		_, e = f.Write(b)
		if ce := f.Close(); e == nil {
			e = ce
		}
		return e
	}
	return errors.New("too many backups this second")
}
