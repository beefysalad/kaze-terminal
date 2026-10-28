// Package ptyx runs a shell inside a pseudo-console (ConPTY on Windows, a
// Unix pty on macOS). It is the only package that imports go-pty.
package ptyx

import (
	"errors"
	"io"
	"sync"
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
const closeGrace = time.Second

type session struct {
	p   pty.Pty
	cmd *pty.Cmd

	exited chan struct{} // closed once the process has been reaped
	code   int

	closeOnce sync.Once
}

// Start launches argv in a new pseudo-console of cols x rows.
func Start(argv []string, cols, rows int, env []string, dir string) (Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("ptyx: empty argv")
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
		_ = cmd.Wait()
		s.code = -1
		if cmd.ProcessState != nil {
			s.code = cmd.ProcessState.ExitCode()
		}
		close(s.exited)
	}()
	return s, nil
}

func (s *session) Read(b []byte) (int, error)  { return s.p.Read(b) }
func (s *session) Write(b []byte) (int, error) { return s.p.Write(b) }

func (s *session) Resize(cols, rows int) error { return s.p.Resize(cols, rows) }

func (s *session) Wait() (int, error) {
	<-s.exited
	return s.code, nil
}

// Close kills the process if it is still running and releases the pty. It
// never blocks longer than closeGrace.
func (s *session) Close() error {
	var err error
	s.closeOnce.Do(func() {
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
		}
	})
	return err
}
