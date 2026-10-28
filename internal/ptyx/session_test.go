package ptyx_test

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze/internal/ptyx"
)

// readUntil accumulates output until want appears or the timeout hits.
// ConPTY wraps and decorates output, so callers match with Contains.
func readUntil(t *testing.T, s ptyx.Session, want string, d time.Duration) string {
	t.Helper()
	var mu sync.Mutex
	var sb strings.Builder
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := s.Read(buf)
			if n > 0 {
				mu.Lock()
				sb.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := sb.String()
		mu.Unlock()
		if strings.Contains(got, want) {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("timed out waiting for %q; got %q", want, sb.String())
	return ""
}

func TestEchoThroughPTY(t *testing.T) {
	argv := []string{"sh", "-c", "echo kaze-ok"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/c", "echo kaze-ok"}
	}
	s, err := ptyx.Start(argv, 80, 24, os.Environ(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	out := readUntil(t, s, "kaze-ok", 5*time.Second)
	if !strings.Contains(out, "kaze-ok") {
		t.Fatalf("%q", out)
	}
}

func TestEnvIsPassedThrough(t *testing.T) {
	argv := []string{"sh", "-c", "echo tab=$KAZE_TAB_ID"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/c", "echo tab=%KAZE_TAB_ID%"}
	}
	s, err := ptyx.Start(argv, 80, 24, append(os.Environ(), "KAZE_TAB_ID=7"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	readUntil(t, s, "tab=7", 5*time.Second)
}

func TestWaitReturnsAfterExit(t *testing.T) {
	argv := []string{"sh", "-c", "exit 3"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/c", "exit 3"}
	}
	s, err := ptyx.Start(argv, 80, 24, os.Environ(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	type res struct {
		code int
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := s.Wait()
		ch <- res{c, err}
	}()
	select {
	case r := <-ch:
		if r.code != 3 {
			t.Fatalf("exit code = %d, err = %v; want 3", r.code, r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return within 5s")
	}
}

func TestCloseDoesNotHangWhileChildRuns(t *testing.T) {
	argv := []string{"sh", "-c", "sleep 30"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/k"}
	}
	s, err := ptyx.Start(argv, 80, 24, os.Environ(), "")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return within 2s")
	}
}

func TestResize(t *testing.T) {
	argv := []string{"sh", "-c", "sleep 30"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/k"}
	}
	s, err := ptyx.Start(argv, 80, 24, os.Environ(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultShellIsNotEmpty(t *testing.T) {
	if len(ptyx.DefaultShell()) == 0 {
		t.Fatal("empty")
	}
}
