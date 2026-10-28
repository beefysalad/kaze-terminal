// Package ptyx runs a shell inside a pseudo-console (ConPTY on Windows, a
// Unix pty on macOS). It is the only package that imports go-pty.
package ptyx

import (
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

// Session is a running process attached to a pseudo-console.
type Session interface {
	io.ReadWriter
	Resize(cols, rows int) error
	Close() error
	// Wait blocks until the process exits and returns its exit code.
	Wait() (exitCode int, err error)
}

// closeGrace bounds how long Close waits for the pty to release; ConPTY can
// block on close while output is still pending.
var closeGrace = time.Second

var (
	// ErrCloseTimeout means the process was killed but the pty did not release in time.
	ErrCloseTimeout = errors.New("ptyx: close timed out")
	// ErrInvalidSize means cols or rows is outside 1..65535.
	ErrInvalidSize = errors.New("ptyx: invalid size")
	// ErrClosed means the session was already closed.
	ErrClosed = errors.New("ptyx: session closed")
)

func validSize(cols, rows int) error {
	if cols < 1 || cols > 65535 || rows < 1 || rows > 65535 {
		return ErrInvalidSize
	}
	return nil
}

type session struct {
	p   pty.Pty
	cmd *pty.Cmd

	exited  chan struct{} // closed once the process has been reaped
	code    int
	waitErr error

	closed    atomic.Bool
	closeOnce sync.Once
}

// Start launches argv in a new pseudo-console of cols x rows.
func Start(argv []string, cols, rows int, env []string, dir string) (Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("ptyx: empty argv")
	}
	if err := validSize(cols, rows); err != nil {
		return nil, err
	}
	// go-pty resolves a bare name against dir, not PATH, once dir is set.
	if dir != "" && filepath.Base(argv[0]) == argv[0] {
		lp, err := exec.LookPath(argv[0])
		if err != nil {
			return nil, err
		}
		argv = append([]string{lp}, argv[1:]...)
	}
	p, err := pty.New()
	if err != nil {
		return nil, err
	}
	if err := p.Resize(cols, rows); err != nil {
		p.Close()
		return nil, err
	}
	cmd := p.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		p.Close()
		return nil, err
	}
	s := &session{p: p, cmd: cmd, exited: make(chan struct{})}
	go func() {
		// A non-zero exit is reported through the code, not as an error.
		err := cmd.Wait()
		s.code = -1
		if cmd.ProcessState != nil {
			s.code = cmd.ProcessState.ExitCode()
		} else {
			s.waitErr = err
		}
		close(s.exited)
	}()
	return s, nil
}

func (s *session) Read(b []byte) (int, error)  { return s.p.Read(b) }
func (s *session) Write(b []byte) (int, error) { return s.p.Write(b) }

func (s *session) Resize(cols, rows int) error {
	if err := validSize(cols, rows); err != nil {
		return err
	}
	if s.closed.Load() {
		return ErrClosed
	}
	return s.p.Resize(cols, rows)
}

func (s *session) Wait() (int, error) {
	<-s.exited
	return s.code, s.waitErr
}

// Close kills the process if it is still running and releases the pty. It
// never blocks longer than closeGrace.
func (s *session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		select {
		case <-s.exited:
		default:
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
		}
		done := make(chan error, 1)
		go func() { done <- s.p.Close() }()
		select {
		case err = <-done:
		case <-time.After(closeGrace):
			err = ErrCloseTimeout
		}
	})
	return err
}
