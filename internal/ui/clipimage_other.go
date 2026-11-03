//go:build (!darwin && !windows) || ios

package ui

// systemClipImage has no clipboard image check on this platform.
func systemClipImage() bool { return false }
