package discord

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

func lockFile(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	h := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	if err = windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &overlapped); err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrLocked
		}
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			windows.UnlockFileEx(h, 0, 1, 0, &overlapped)
			f.Close()
		})
	}, nil
}
