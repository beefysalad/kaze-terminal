//go:build !windows

package codexlaunch

import (
	"runtime"
	"syscall"
)

func platformName() string { return runtime.GOOS }
func execute(path string, args, env []string) (int, error) {
	return 126, syscall.Exec(path, append([]string{path}, args...), env)
}
