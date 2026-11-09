package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/codexlaunch"
)

func TestMain(m *testing.M) {
	if codexlaunch.IsInvocation(os.Args[0]) {
		main()
		return
	}
	os.Exit(m.Run())
}

func TestPrivateCodexVersionDispatchesBeforeKazeVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture; native Windows argv integration is in codexlaunch")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := codexlaunch.New(self)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(real, "codex"), []byte("#!/bin/sh\nprintf 'fake Codex version'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	env := l.Env([]string{"PATH=" + real, "KAZE_TAB_ID=4", "KAZE_NOTIFY=127.0.0.1:3", "KAZE_NOTIFY_TOKEN=token"})
	dir := ""
	for _, e := range env {
		if len(e) > len("KAZE_CODEX_SHIM=") && e[:len("KAZE_CODEX_SHIM=")] == "KAZE_CODEX_SHIM=" {
			dir = e[len("KAZE_CODEX_SHIM="):]
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "codex"), "--version")
	cmd.Env = env
	b, err := cmd.CombinedOutput()
	if err != nil || string(b) != "fake Codex version" {
		t.Fatalf("dispatch output=%s err=%v", b, err)
	}
}
