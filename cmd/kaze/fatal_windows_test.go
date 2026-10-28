package main

import "testing"

// Resolving is enough: calling MessageBoxW would block the test on a dialog.
func TestMessageBoxIsAvailable(t *testing.T) {
	if err := procMessageBoxW.Find(); err != nil {
		t.Fatalf("user32 MessageBoxW: %v", err)
	}
}
