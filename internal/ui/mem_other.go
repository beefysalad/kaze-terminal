//go:build !windows

package ui

// processMB is not measured outside Windows yet; the caller falls back to Go's own stats.
func processMB() (float64, bool) { return 0, false }
