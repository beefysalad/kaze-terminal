package main

import (
	"syscall"
	"unsafe"
)

// time.Now on Windows is quantized to the system timer tick, too coarse for
// sub-millisecond frame times, so read QueryPerformanceCounter directly.
var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	qpc      = kernel32.NewProc("QueryPerformanceCounter")
	qpf      = kernel32.NewProc("QueryPerformanceFrequency")
	qpcFreq  = func() float64 {
		var f int64
		qpf.Call(uintptr(unsafe.Pointer(&f)))
		return float64(f)
	}()
)

// nowMs returns a high-resolution monotonic timestamp in milliseconds.
func nowMs() float64 {
	var c int64
	qpc.Call(uintptr(unsafe.Pointer(&c)))
	return float64(c) * 1000 / qpcFreq
}
