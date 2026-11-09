//go:build !windows

package codexlaunch

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProxyExecKeepsSignalsOnOriginalProcess(t *testing.T) {
	self, _ := os.Executable()
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	executableFixture(t, real, "codex", "#!/bin/sh\ntrap 'exit 42' TERM\nprintf 'ready\\n'\nIFS= read -r line\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex"))
	cmd.Env = l.Env([]string{"PATH=" + real, "KAZE_TAB_ID=1", "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=token"})
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "ready\n" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("readiness=%q %v", line, err)
	}
	// Signal only this isolated fixture process, never a foreground window or
	// the user's terminal/process group.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("signal trap did not run")
	} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 42 {
		t.Fatalf("signal/exit changed: %v", err)
	}
}

func TestProxyRecursiveShellWrapperFailsWithinBound(t *testing.T) {
	self, _ := os.Executable()
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	executableFixture(t, real, "codex", "#!/bin/sh\nexec codex \"$@\"\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex"))
	cmd.Env = l.Env([]string{"PATH=" + real, "KAZE_TAB_ID=1", "KAZE_NOTIFY=127.0.0.1:2", "KAZE_NOTIFY_TOKEN=token"})
	b, err := cmd.CombinedOutput()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 126 || !strings.Contains(string(b), "recursion limit") {
		t.Fatalf("recursive wrapper output=%s err=%v", b, err)
	}
}

func TestCodexWithoutNoDaemonStartsUnchanged(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	executableFixture(t, real, "codex", "#!/bin/sh\nif [ \"$1\" = '--no-daemon' ]; then echo unexpected >&2; exit 2; fi\nprintf 'launched %s' \"$*\"\n")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex"), "resume", "abc")
	cmd.Env = l.Env([]string{"PATH=" + real + ":/usr/bin:/bin", "KAZE_TAB_ID=1"})
	b, err := cmd.CombinedOutput()
	if err != nil || string(b) != "launched resume abc" {
		t.Fatalf("err=%v output=%s", err, b)
	}
}
