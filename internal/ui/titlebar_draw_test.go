package ui

import (
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func TestCrumbUsesSidebarLabel(t *testing.T) {
	win, _ := sideWindow(t, true)
	tab := openIn(t, win, osPath("/code/alpha/a"))
	win.mgr.SetActive(tab.ID)
	deliverNow(win, promptMsg(int(tab.ID), "/ship-ticket", "TODO-142 login"), t0)
	c := win.crumb()
	if c.Title != "TODO-142" || c.Cmd != "/ship-ticket" || c.Repo != "alpha" || c.State != "running" {
		t.Fatalf("crumb %+v", c)
	}
}

func TestCrumbOnSettingsAndBoard(t *testing.T) {
	win, _ := sideWindow(t, true)
	tab := openIn(t, win, osPath("/code/alpha/a"))
	win.mgr.SetActive(tab.ID)
	win.openSettings()
	if c := win.crumb(); c != (crumbParts{Title: "Settings"}) {
		t.Fatalf("settings crumb %+v", c)
	}
	win.openBoard(0)
	if c := win.crumb(); c != (crumbParts{Title: "Agents"}) {
		t.Fatalf("board crumb %+v", c)
	}
}

func TestTitleClicksReachActions(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	r := new(input.Router)
	sideFrame(t, win, r)
	win.setSideMode(sideRail) // a full sidebar's header holds the search
	sideFrame(t, win, r)
	click(win, win.bar.Agents)
	if !win.board.open {
		t.Fatal("Agents did not open the board")
	}
	click(win, win.bar.Agents)
	if win.board.open {
		t.Fatal("Agents did not close the board")
	}
	click(win, win.bar.Search)
	if !win.palette.open {
		t.Fatal("search did not open the palette")
	}
	win.closePalette()
	click(win, win.bar.Gear)
	if !win.set.open {
		t.Fatal("the gear did not open Settings")
	}
}

func TestSidebarHasNoIconRow(t *testing.T) {
	m := sideModel{Loose: sideRows(1, 2, "s")}
	h, size := sideDraw(t, m, sideState{}, 500) // the icon row and + are gone from sideHits altogether
	if !h.filter.Empty() || h.plus.Max.X < size.X-9 || h.plus.Min.Y > 12 || h.search.Max.X > h.plus.Min.X {
		t.Fatalf("filter %v, search %v, plus %v: the icons should sit at the top right and the field stay closed", h.filter, h.search, h.plus)
	}
}

func TestTitleBarDrawsInEveryTheme(t *testing.T) {
	for _, th := range theme.All() {
		for _, custom := range []bool{true, false} {
			win, _ := sideWindow(t, custom)
			openIn(t, win, "")
			win.applyTheme(th)
			win.side.w = sidebarW // what the frame sets before it draws the bar
			win.drawBar(barContext(1200), 42)
			b := win.bar
			for name, r := range map[string]image.Rectangle{"toggle": b.Toggle, "agents": b.Agents, "gear": b.Gear,
				"mark": b.Mark, "name": b.Word} {
				if r.Empty() {
					t.Fatalf("%s custom=%v: %s empty", th.Key, custom, name)
				}
			}
			if custom == b.Buttons[0].Empty() {
				t.Fatalf("%s custom=%v: caption buttons %v", th.Key, custom, b.Buttons)
			}
		}
	}
}

func TestMacDragSnapshotForDoubleClick(t *testing.T) {
	win, _ := sideWindow(t, false)
	openIn(t, win, "")
	win.lights = newBridge(&fakeLights{}, func(f func()) { f() })
	win.drawBar(barContext(1200), 42)
	c := win.bar.Crumb
	if !win.lights.dragAt(float64(c.Min.X+2), 20) {
		t.Fatal("the crumb should count as empty bar for a double click")
	}
	g := win.bar.Gear
	if win.lights.dragAt(float64(g.Min.X+5), 20) {
		t.Fatal("a double click on the gear must not zoom")
	}
}

func TestTitleBarIconsExist(t *testing.T) {
	for _, n := range []string{"panel", "plus", "terminal", "agents", "cog", "search"} {
		if !iconNames[n] {
			t.Errorf("icon %q is not drawn", n)
		}
	}
}

func TestCrumbTitleNotCutWhenItFits(t *testing.T) {
	win, _ := sideWindow(t, false)
	tab := openIn(t, win, "")
	win.mgr.SetActive(tab.ID)
	win.drawBar(barContext(1200), 42)
	c := win.crumb()
	_, need := win.ui.labelW(barContext(1200), c.Title, 12.5, win.ui.th.ActiveFG, font.SemiBold)
	if win.bar.Crumb.Dx() < need.X {
		t.Fatalf("crumb %d px for %q, which needs %d", win.bar.Crumb.Dx(), c.Title, need.X)
	}
}
