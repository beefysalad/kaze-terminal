//go:build windows

package bench

import (
	"syscall"
	"time"
)

// processStart is when Windows created this process, so launch time includes
// loading the binary and the Go runtime starting.
func processStart() time.Time {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return initAt
	}
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return initAt
	}
	return time.Unix(0, created.Nanoseconds())
}
