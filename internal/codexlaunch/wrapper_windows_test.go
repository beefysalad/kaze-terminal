//go:build windows

package codexlaunch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnknownWrapperStillRunsCodex(t *testing.T) {
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
	wrapper := filepath.Join(real, "codex.cmd")
	if err := os.WriteFile(wrapper, []byte("@echo off\r\necho custom wrapper %*\r\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(l.dir, "codex.exe"))
	cmd.Env = l.Env([]string{"PATH=" + real, "PATHEXT=.CMD", "KAZE_TAB_ID=1", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "ComSpec=" + os.Getenv("ComSpec")})
	b, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(b), "custom wrapper --no-daemon") || !strings.Contains(string(b), "not recognized") {
		t.Fatalf("err=%v output=%s", err, b)
	}
}
