//go:build !windows

package main

import "time"

var clockStart = time.Now()

// nowMs returns a monotonic timestamp in milliseconds.
func nowMs() float64 { return float64(time.Since(clockStart).Nanoseconds()) / 1e6 }
