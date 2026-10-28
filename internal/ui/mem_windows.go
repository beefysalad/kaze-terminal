//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var procGetProcessMemoryInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// processMB is the working set, which includes the GPU driver and fonts that Go's own stats miss.
func processMB() (float64, bool) {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, false
	}
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	if r, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); r == 0 {
		return 0, false
	}
	return float64(c.workingSetSize) / (1 << 20), true
}
