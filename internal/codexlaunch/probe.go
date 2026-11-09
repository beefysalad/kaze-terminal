package codexlaunch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const probeEnv = "KAZE_CODEX_SHIM_PROBE"
const probeTimeout = 5 * time.Second

// probeShim starts the private launcher once, so a copy the OS refuses to run
// is left off PATH instead of shadowing a working Codex install.
func probeShim(path, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Env = append(os.Environ(), probeEnv+"="+token)
	hideConsole(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("the private launcher could not start: %w %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

// removeOwned deletes a launcher directory, shim first: Windows refuses while
// the shim is running, which keeps the ownership marker in place.
func removeOwned(dir string) error {
	if err := os.Remove(filepath.Join(dir, shimName(runtime.GOOS))); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dir, zshDirName)); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, markerName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Remove(dir)
}

// SweepStale removes launchers left by Kaze runs that ended without cleanup.
func SweepStale() { sweep(os.TempDir(), processAlive) }

func sweep(root string, alive func(int) bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), directoryPrefix) {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if m, err := loadManifest(dir); err == nil && m.PID > 0 && !alive(m.PID) {
			_ = removeOwned(dir)
		}
	}
}

// supportsNoDaemon asks the real Codex, because older versions exit on the
// unknown flag. Anything but a clean exit means launch it unchanged.
func supportsNoDaemon(command Command, env []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	args := append(append([]string{}, command.Prefix...), "--no-daemon", "--help")
	cmd := exec.CommandContext(ctx, command.Path, args...)
	cmd.Env = env
	hideConsole(cmd)
	return cmd.Run() == nil
}
