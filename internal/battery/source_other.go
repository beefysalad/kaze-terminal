//go:build !windows && !darwin

package battery

// Every OS but Windows and macOS reports no battery, so the status bar never
// shows one there.

type noSource struct{}

// Native reports no battery off Windows and macOS.
func Native() Source { return noSource{} }

func (noSource) Read() (Status, error)               { return Status{}, nil }
func (noSource) Watch(changed func()) (func(), bool) { return nil, false }
