//go:build !windows

package codexlaunch

import (
	"os/exec"
	"runtime"
	"syscall"
)

func platformName() string { return runtime.GOOS }
func execute(path string, args, env []string) (int, error) {
	return 126, syscall.Exec(path, append([]string{path}, args...), env)
}

func hideConsole(*exec.Cmd) {}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
