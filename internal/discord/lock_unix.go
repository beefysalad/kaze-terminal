//go:build !windows

package discord

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

func lockFile(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			unix.Flock(int(f.Fd()), unix.LOCK_UN)
			f.Close()
		})
	}, nil
}
