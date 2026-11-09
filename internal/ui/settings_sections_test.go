package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSettingsIgnoresFormerDevboxPairing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".config", "devbox")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials")
	data := []byte("DEVBOX_URL=https://box.example\nDEVBOX_API_KEY=unused-test-key\nDEVBOX_NAME=test\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	win, _ := alertWindow(t, 1)
	win.openSettings()
	var titles []string
	for _, section := range win.sections() {
		titles = append(titles, section.title)
	}
	want := []string{"Appearance", "Workspaces", "Agents", "Shortcuts", "Updates", "About"}
	if !slices.Equal(titles, want) {
		t.Fatalf("settings sections with an old pairing: %v; want %v", titles, want)
	}
	// Pairing belongs to the external tool; removing the Kaze integration must
	// leave that file intact.
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("external pairing changed: %q, %v", got, err)
	}
}
