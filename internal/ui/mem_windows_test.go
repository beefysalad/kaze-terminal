//go:build windows

package ui

import (
	"syscall"
	"testing"
)

// Committed but untouched memory is private bytes, not working set. The status bar
// must count it, since it tracks whether kaze's memory grows.
func TestProcessMBCountsPrivateBytes(t *testing.T) {
	k := syscall.NewLazyDLL("kernel32.dll")
	alloc, free := k.NewProc("VirtualAlloc"), k.NewProc("VirtualFree")
	before, ok := processMB()
	if !ok {
		t.Fatal("processMB failed")
	}
	const size = 256 << 20
	addr, _, err := alloc.Call(0, size, 0x1000 /* MEM_COMMIT */, 0x04 /* PAGE_READWRITE */)
	if addr == 0 {
		t.Skipf("VirtualAlloc: %v", err)
	}
	defer free.Call(addr, 0, 0x8000 /* MEM_RELEASE */)
	after, _ := processMB()
	if after-before < 200 {
		t.Fatalf("committing 256 MB moved the reading from %.0f to %.0f MB; it is not private bytes", before, after)
	}
}
