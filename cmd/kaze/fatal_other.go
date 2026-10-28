//go:build !windows

package main

// showErrorNative adds nothing elsewhere: reportFatal's stderr log is the message.
func showErrorNative(title, text string) {}
