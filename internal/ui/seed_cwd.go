package ui

import (
	"net/url"
	"path/filepath"
	"runtime"

	"github.com/beefysalad/kaze-terminal/internal/term"
)

// seedCwd reports dir as the shell's folder before its first output, as an OSC 7
// would, so a new tab is in its space at once instead of in the shell space until
// the prompt draws. The shell's own report replaces it. "" seeds nothing.
func seedCwd(dir string) func(*term.Session) {
	if dir == "" {
		return nil
	}
	p := filepath.ToSlash(dir)
	if runtime.GOOS == "windows" {
		p = "/" + p
	}
	report := "7;file://" + (&url.URL{Path: p}).EscapedPath()
	return func(s *term.Session) {
		s.Lock()
		defer s.Unlock()
		s.Applier().OSC([]byte(report))
	}
}
