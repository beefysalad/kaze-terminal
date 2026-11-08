//go:build windows

package codexlaunch

import (
	"os"
	"os/exec"
	"os/signal"
)

func platformName() string { return "windows" }
func execute(path string, args, env []string) (int, error) {
	// Codex receives the same console control event. Keep the proxy alive while
	// Codex cancels an interactive turn instead of exiting on Go's default Ctrl+C.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode(), nil
	}
	return 126, err
}
