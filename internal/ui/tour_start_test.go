package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTourStartsOnFreshInstall(t *testing.T) {
	w := &window{cfgPath: filepath.Join(t.TempDir(), "config.toml")}
	w.prepareTour()
	if !w.tour.pending {
		t.Fatal("not pending")
	}
	os.WriteFile(w.cfgPath, []byte(""), 0600)
	w.prepareTour()
	if w.tour.pending {
		t.Fatal("upgrade")
	}
}
func TestPaletteHasShowTheTour(t *testing.T) {
	for _, c := range kazeCommands {
		if c.Label == "Show the tour" {
			return
		}
	}
	t.Fatal("missing")
}
