// Package tour records first-run onboarding independently of user settings.
package tour

import (
	"os"
	"path/filepath"
)

type Decision int

const (
	Show Decision = iota
	Skip
)

// Check is read-only. Unknown file states fail closed.
func Check(dir string) Decision {
	if dir == "" {
		return Skip
	}
	for _, n := range []string{"tour.json", "config.toml", "sessions.json"} {
		if _, e := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(e) {
			return Skip
		}
	}
	return Show
}
func MarkSeen(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".tour-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.WriteString("{\"seen\": true}\n"); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(dir, "tour.json"))
}
