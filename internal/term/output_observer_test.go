package term_test

import (
	"github.com/beefysalad/kaze-terminal/internal/term"
	"testing"
	"time"
)

func TestOutputObservers(t *testing.T) {
	p := newFake()
	s := term.NewSession(p, 40, 3, nil)
	defer s.Close()
	if n := s.OutputObserverCount(); n != 0 {
		t.Fatalf("initial %d", n)
	}
	wake := make(chan struct{}, 1)
	remove := s.AddOutputObserver(func() {
		s.Lock()
		s.Unlock()
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	if n := s.OutputObserverCount(); n != 1 {
		t.Fatalf("registered %d", n)
	}
	written := make(chan error, 1)
	go func() { _, err := p.outW.Write([]byte("hello")); written <- err }()
	select {
	case err := <-written:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("write blocked")
	}
	select {
	case <-wake:
	case <-time.After(time.Second):
		t.Fatal("observer blocked or absent")
	}
	if s.LastOutputTime().IsZero() {
		t.Fatal("missing output timestamp")
	}
	remove()
	remove()
	if n := s.OutputObserverCount(); n != 0 {
		t.Fatalf("removed %d", n)
	}
}
