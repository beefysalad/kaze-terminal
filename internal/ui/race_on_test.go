//go:build race

package ui

// raceDetector is true in -race builds, whose instrumentation adds allocations
// that a frame's real count does not have.
const raceDetector = true
