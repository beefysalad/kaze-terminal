package sysnotify

import (
	"errors"
	"runtime"
	"sync"
)

// A sleeping locked thread retains the apartment for the complete COM object
// lifetime. The initialization boundary is injectable; tests never call WinRT.
type windowsApartment struct {
	mu     sync.RWMutex
	work   chan windowsCOMWork
	done   chan struct{}
	closed bool
}
type windowsCOMWork struct {
	call func() error
	done chan error
}

func newWindowsApartmentWith(init func() error, uninit func()) (*windowsApartment, error) {
	a := &windowsApartment{work: make(chan windowsCOMWork), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(a.done)
		if err := init(); err != nil {
			ready <- err
			return
		}
		defer uninit()
		ready <- nil
		for job := range a.work {
			job.done <- job.call()
		}
	}()
	if err := <-ready; err != nil {
		<-a.done
		return nil, err
	}
	return a, nil
}
func (a *windowsApartment) run(f func() error) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return errors.New("Windows notification apartment is closed")
	}
	job := windowsCOMWork{call: f, done: make(chan error, 1)}
	a.work <- job
	return <-job.done
}
func (a *windowsApartment) close() {
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		close(a.work)
	}
	a.mu.Unlock()
	<-a.done
}
