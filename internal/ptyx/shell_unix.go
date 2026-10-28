//go:build !windows

package ptyx

import "os"

// DefaultShell is the user's login shell, or zsh (the macOS default).
func DefaultShell() []string {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/zsh"
	}
	return []string{sh, "-l"}
}
