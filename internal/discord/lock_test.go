package discord

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLockIsExclusive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "discord.lock")
	release, err := lockFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	other, err := lockFile(p)
	if other != nil {
		other()
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock: %v", err)
	}
	release()
	release() // Release must remain safe when Close and cleanup both call it.
	third, err := lockFile(p)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	third()
}
