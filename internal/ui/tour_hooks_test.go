package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTourHookStatesFromFixtures(t *testing.T) {
	d := t.TempDir()
	w := &window{claudePath: filepath.Join(d, "settings.json"), codexDir: filepath.Join(d, "codex")}
	hs := w.tourHookStates()
	if hs[0].State != hookNotInstalled || hs[1].State != hookNotInstalled {
		t.Fatal(hs)
	}
	os.WriteFile(w.claudePath, []byte("{}"), 0600)
	hs = w.tourHookStates()
	if hs[0].State != hookNeedsFix {
		t.Fatal(hs)
	}
}
func TestTourFinishAndLaterChangeNoFiles(t *testing.T) {
	w := &window{cfgPath: filepath.Join(t.TempDir(), "config.toml")}
	w.openTour(true)
	w.tourMove(5)
	w.tourNext()
	if _, e := os.Stat(filepath.Join(w.tourDir(), "tour.json")); !os.IsNotExist(e) {
		t.Fatal("replay wrote flag")
	}
}
