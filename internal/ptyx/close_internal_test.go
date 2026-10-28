package ptyx

import (
	"context"
	"errors"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

// stuckPty is a pty whose Close never returns until released.
type stuckPty struct {
	pty.Pty
	release chan struct{}
}

func (p *stuckPty) Close() error                       { <-p.release; return nil }
func (p *stuckPty) Name() string                       { return "stuck" }
func (p *stuckPty) Command(string, ...string) *pty.Cmd { return nil }
func (p *stuckPty) CommandContext(context.Context, string, ...string) *pty.Cmd {
	return nil
}

func TestCloseTimeoutIsReported(t *testing.T) {
	old := closeGrace
	closeGrace = 50 * time.Millisecond
	defer func() { closeGrace = old }()

	p := &stuckPty{release: make(chan struct{})}
	defer close(p.release)
	s := &session{p: p, cmd: &pty.Cmd{}, exited: make(chan struct{})}
	close(s.exited)
	if err := s.Close(); !errors.Is(err, ErrCloseTimeout) {
		t.Fatalf("Close = %v, want ErrCloseTimeout", err)
	}
}

func TestWaitSurfacesReapError(t *testing.T) {
	s := &session{exited: make(chan struct{}), waitErr: errors.New("boom")}
	close(s.exited)
	if _, err := s.Wait(); err == nil {
		t.Fatal("Wait swallowed the reap error")
	}
}
