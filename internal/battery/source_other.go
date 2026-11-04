//go:build !windows

package battery

// Every OS but Windows, macOS included for now, reports no battery, so the
// status bar never shows one there. The macOS IOKit source is a follow-up
// issue (#139): adding source_darwin.go means narrowing this file's build tag
// to !windows && !darwin.

type noSource struct{}

// Native reports no battery off Windows.
func Native() Source { return noSource{} }

func (noSource) Read() (Status, error)               { return Status{}, nil }
func (noSource) Watch(changed func()) (func(), bool) { return nil, false }
