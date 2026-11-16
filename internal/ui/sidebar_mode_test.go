package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/input"
)

func TestEffectiveSideMode(t *testing.T) {
	for _, c := range []struct {
		saved sideMode
		width int
		want  sideMode
	}{
		{sideFull, 1000, sideFull},
		{sideFull, 500, sideRail},
		{sideRail, 500, sideRail},
		{sideHidden, 500, sideHidden},
		{sideFull, 250, sideHidden},
		{sideRail, 299, sideHidden},
		{sideRail, 1000, sideRail},
		{sideFull, 560, sideFull},
		{sideRail, 300, sideRail},
	} {
		if got := effectiveSideMode(c.saved, c.width, minRailWindow, minSideWindow); got != c.want {
			t.Errorf("effectiveSideMode(%v, %d) = %v, want %v", c.saved, c.width, got, c.want)
		}
	}
}

func TestSideModeString(t *testing.T) {
	if sideFull.String() != "full" || sideRail.String() != "rail" || sideHidden.String() != "hidden" {
		t.Fatalf("got %q %q %q", sideFull, sideRail, sideHidden)
	}
}

func TestSavedSideModeFallsBackToBool(t *testing.T) {
	win, _ := sideWindow(t, true)
	win.opt.Config.SidebarMode, win.opt.Config.Sidebar = "", true
	if m := win.savedSideMode(); m != sideFull {
		t.Fatalf("empty mode, sidebar on: %v, want full", m)
	}
	win.opt.Config.Sidebar = false
	if m := win.savedSideMode(); m != sideHidden {
		t.Fatalf("empty mode, sidebar off: %v, want hidden", m)
	}
	win.opt.Config.SidebarMode = "rail"
	if m := win.savedSideMode(); m != sideRail {
		t.Fatalf("mode rail, sidebar off: %v, want rail", m)
	}
}

func TestSideRight(t *testing.T) {
	win, _ := sideWindow(t, true)
	if win.sideRight() {
		t.Fatal("the default side is right")
	}
	win.opt.Config.SidebarSide = "right"
	if !win.sideRight() {
		t.Fatal(`SidebarSide "right" is not right`)
	}
}

func TestToggleCyclesAndSaves(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	for _, want := range []sideMode{sideRail, sideHidden, sideFull} {
		win.toggleSidebar()
		if got := win.savedSideMode(); got != want {
			t.Fatalf("after toggle: saved %v, want %v", got, want)
		}
		b, err := os.ReadFile(win.cfgPath)
		if err != nil || !strings.Contains(string(b), `sidebar_mode = "`+want.String()+`"`) {
			t.Fatalf("config file %q (%v), want sidebar_mode = %q", b, err, want)
		}
	}
}

func TestToggleWhileNarrowCyclesSavedMode(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	var r input.Router
	sizedFrame(t, win, &r, 500, 600)
	if win.side.mode != sideRail || win.side.hidden || win.gridMin.X != railW+8 {
		t.Fatalf("500px, saved full: mode %v, hidden %v, grid x %d", win.side.mode, win.side.hidden, win.gridMin.X)
	}
	win.toggleSidebar()
	sizedFrame(t, win, &r, 500, 600)
	if win.savedSideMode() != sideRail || win.side.mode != sideRail || win.gridMin.X != railW+8 {
		t.Fatalf("one toggle: saved %v, shown %v, grid x %d", win.savedSideMode(), win.side.mode, win.gridMin.X)
	}
	win.toggleSidebar()
	sizedFrame(t, win, &r, 500, 600)
	if win.savedSideMode() != sideHidden || win.side.mode != sideHidden || !win.side.hidden || win.gridMin.X != 8 {
		t.Fatalf("two toggles: saved %v, shown %v, grid x %d", win.savedSideMode(), win.side.mode, win.gridMin.X)
	}
}

func TestSideChangeDropsEditors(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, "")
	var r input.Router
	sideFrame(t, win, &r)
	win.side.filtering = true
	win.setSideRight(true)
	sideFrame(t, win, &r)
	if win.side.filtering {
		t.Fatal("moving the sidebar left the filter with the keyboard")
	}
	if !win.sideRight() {
		t.Fatal("setSideRight(true) did not move the sidebar")
	}
	win.startRename(a.ID)
	if win.side.renaming != a.ID {
		t.Fatal("the rename did not start")
	}
	win.setSideMode(sideRail)
	if win.side.renaming != 0 {
		t.Fatalf("changing the width left row %d renaming", win.side.renaming)
	}
	if win.savedSideMode() != sideRail {
		t.Fatalf("saved %v, want rail", win.savedSideMode())
	}
}

// The rail draws a tile per session and no filter field.
func TestRailFrameDrawsTiles(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	win.opt.Config.SidebarMode = "rail"
	var r input.Router
	sideFrame(t, win, &r)
	if win.side.mode != sideRail || win.side.w != railW || len(win.side.hits.rows) != 1 || !win.side.hits.filter.Empty() {
		t.Fatalf("rail: mode %v, width %d, %d rows, filter %v", win.side.mode, win.side.w, len(win.side.hits.rows), win.side.hits.filter)
	}
}

// A width change that switches full and rail drops the hover: the row under the
// pointer in one mode is not the tile under it in the other.
func TestNarrowModeChangeClearsHover(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, "")
	var r input.Router
	sizedFrame(t, win, &r, 1000, 600)
	win.side.st.hover = a.ID
	sizedFrame(t, win, &r, 500, 600)
	if win.side.mode != sideRail || win.side.st.hover != 0 {
		t.Fatalf("full to rail: mode %v, hover %d, want rail and no hover", win.side.mode, win.side.st.hover)
	}
	win.side.st.hover = a.ID
	sizedFrame(t, win, &r, 1000, 600)
	if win.side.mode != sideFull || win.side.st.hover != 0 {
		t.Fatalf("rail to full: mode %v, hover %d, want full and no hover", win.side.mode, win.side.st.hover)
	}
}

// A width change that switches full and rail scrolls the shown session into view.
func TestNarrowModeChangeRevealsShown(t *testing.T) {
	win, _ := sideWindow(t, true)
	var last *tabs.Tab
	for range 30 {
		last = openIn(t, win, "")
	}
	win.mgr.SetActive(last.ID)
	var r input.Router
	inside := func(w int, want sideMode) {
		t.Helper()
		win.side.st.scroll = 0 // the other mode's scroll, which does not show the last session
		sizedFrame(t, win, &r, w, 400)
		sizedFrame(t, win, &r, w, 400)
		h := rowHit(t, win, last.ID)
		if win.side.mode != want || !h.rect.In(win.side.hits.list) {
			t.Fatalf("%dpx: mode %v, shown %v outside the list %v", w, win.side.mode, h.rect, win.side.hits.list)
		}
	}
	sizedFrame(t, win, &r, 1000, 400)
	inside(500, sideRail)
	inside(1000, sideFull)
}
