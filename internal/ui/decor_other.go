//go:build !windows

package ui

// customTitleBar: elsewhere the system title bar stays. On macOS an undecorated
// Gio window loses the traffic lights, so that waits for the Mac milestone.
const customTitleBar = false
