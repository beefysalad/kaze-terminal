package ui

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Exercise the actual PTY boundary with an isolated noninteractive shell.
// This emits only fake fixture values and never injects input into a window.
func TestShellEnvironmentTransformKeepsTabIdentity(t *testing.T) {
	argv := []string{"/bin/sh", "-c", `printf 'shim=%s tab=%s notify=%s\n' "$KAZE_SHIM_FIXTURE" "$KAZE_TAB_ID" "$KAZE_NOTIFY"`}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/d", "/c", "echo shim=%KAZE_SHIM_FIXTURE% tab=%KAZE_TAB_ID% notify=%KAZE_NOTIFY%"}
	}
	called := false
	w := &window{opt: Options{Shell: argv, ShellEnv: func(env []string) []string {
		called = true
		return append(append([]string(nil), env...), "KAZE_SHIM_FIXTURE=kaze-test-shim")
	}}}
	w.mgr = tabs.NewManager(nil, 80, 24, nil)
	p, err := w.start(append(os.Environ(), "KAZE_TAB_ID=7", "KAZE_NOTIFY=test-listener"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !called {
		t.Fatal("shell environment transform was not applied")
	}
	want := "shim=kaze-test-shim tab=7 notify=test-listener"
	done := make(chan string, 1)
	go func() {
		var out strings.Builder
		buf := make([]byte, 1024)
		for out.Len() < 16384 {
			n, err := p.Read(buf)
			out.Write(buf[:n])
			if strings.Contains(out.String(), want) || err != nil {
				break
			}
		}
		done <- out.String()
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, want) {
			t.Fatalf("missing scoped env/identity: %q", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading fixture PTY")
	}
}

type gatedShellPTY struct {
	*pipePTY
	gate <-chan struct{}
}

func (p *gatedShellPTY) Close() error { <-p.gate; return p.pipePTY.Close() }

func TestShellCleanupReceivesReleaseResult(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(map[bool]string{false: "released", true: "still closing"}[blocked], func(t *testing.T) {
			gate := make(chan struct{})
			if blocked {
				defer close(gate)
			} else {
				close(gate)
			}
			p := &gatedShellPTY{pipePTY: newPipePTY(), gate: gate}
			called, released := false, false
			win := &window{opt: Options{ShellsClosed: func(done bool) { called = true; released = done }}}
			win.mgr = tabs.NewManager(func([]string, string) (ptyx.Session, error) { return p, nil }, 80, 24, nil)
			if _, err := win.mgr.New(); err != nil {
				t.Fatal(err)
			}
			win.shutdown()
			if !called || released == blocked {
				t.Fatalf("cleanup called=%v released=%v blocked=%v", called, released, blocked)
			}
		})
	}
}
