package ui

import (
	"image"
	"testing"

	"gioui.org/text"
	"gioui.org/unit"
)

// sideTitle lays out the bar with the sidebar on the right (or left) at the given width.
func sideTitle(w, h int, mac bool, side int, right, noPlus bool) titleLayout {
	in := titleIn{Size: image.Pt(w, h), Mac: mac, SideW: side, Right: right, SideFull: !noPlus, Crumb: fullCrumb,
		Key: "⌘K", Dims: titleTestDims, Measure: sevenPx, WordW: titleWordW}
	if mac {
		in.LightsW = 70
	}
	return layoutTitle(in)
}

func TestTitleRightFullMac(t *testing.T) {
	l := sideTitle(1200, 38, true, 260, true, false)
	if l.Lead != image.Rect(940, 0, 1200, 38) {
		t.Fatalf("lead %v, want the strip over the right sidebar", l.Lead)
	}
	for name, r := range map[string]image.Rectangle{"gear": l.Gear, "agents": l.Agents, "toggle": l.Toggle} {
		if r.Empty() || !r.In(l.Lead) {
			t.Fatalf("%s %v not inside the lead %v", name, r, l.Lead)
		}
	}
	if !(l.Agents.Max.X <= l.Gear.Min.X && l.Gear.Max.X <= l.Toggle.Min.X) {
		t.Fatalf("want agents %v, gear %v, toggle %v left to right", l.Agents, l.Gear, l.Toggle)
	}
	if l.Mark.Min.X != 70+titleTestDims.Gap {
		t.Fatalf("mark %v, want it after the lights", l.Mark)
	}
	if !l.Search.Empty() {
		t.Fatalf("search %v: the full sidebar's header holds it", l.Search)
	}
	if l.Crumb.Empty() || l.Crumb.Max.X > l.Lead.Min.X {
		t.Fatalf("crumb %v must sit before the lead %v", l.Crumb, l.Lead)
	}
}

func TestTitleRightRailWindows(t *testing.T) {
	l := sideTitle(1200, 38, false, 52, true, true)
	if l.Search.Empty() {
		t.Fatal("a rail keeps the search field in the bar")
	}
	if l.Lead.Min.X > 1200-52 {
		t.Fatalf("lead %v narrower than the rail", l.Lead)
	}
	if l.Buttons[2].Max.X != 1200 {
		t.Fatalf("close %v not at the right edge", l.Buttons[2])
	}
	if l.Toggle.Max.X > l.Buttons[0].Min.X {
		t.Fatalf("toggle %v over the caption buttons %v", l.Toggle, l.Buttons[0])
	}
}

func TestTitleLeftRailKeepsSearch(t *testing.T) {
	l := sideTitle(1200, 38, false, 52, false, true)
	if l.Search.Empty() {
		t.Fatal("a rail keeps the search field in the bar")
	}
	if l.Lead.Max.X < l.Toggle.Max.X {
		t.Fatalf("lead %v does not cover the toggle %v", l.Lead, l.Toggle)
	}
}

func TestTitleRightHiddenKeepsButtons(t *testing.T) {
	for _, mac := range []bool{true, false} {
		l := sideTitle(1200, 38, mac, 0, true, true)
		if l.Toggle.Empty() || l.Gear.Empty() || l.Agents.Empty() {
			t.Fatalf("mac=%v: toggle %v gear %v agents %v", mac, l.Toggle, l.Gear, l.Agents)
		}
		if l.Toggle.Min.X < 600 || l.Lead.Max.X != 1200 || !l.Toggle.In(l.Lead) {
			t.Fatalf("mac=%v: toggle %v should sit at the top right in the lead %v", mac, l.Toggle, l.Lead)
		}
		if !mac && l.Toggle.Max.X > l.Buttons[0].Min.X {
			t.Fatalf("toggle %v over the caption buttons", l.Toggle)
		}
	}
}

func TestTitleRightNarrowNoOverlap(t *testing.T) {
	for _, w := range []int{320, 420, 560} {
		l := sideTitle(w, 38, false, 52, true, true)
		rs := []image.Rectangle{l.Crumb, l.Search, l.Agents, l.Gear, l.Toggle, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
		for i, a := range rs {
			for _, b := range rs[i+1:] {
				if !a.Empty() && !b.Empty() && a.Overlaps(b) {
					t.Fatalf("%dpx: %v overlaps %v", w, a, b)
				}
			}
		}
		if l.Toggle.Empty() || l.Gear.Empty() || l.Agents.Empty() {
			t.Fatalf("%dpx: a button is gone", w)
		}
	}
}

func TestTitleRightDragAvoidsButtons(t *testing.T) {
	for _, mac := range []bool{true, false} {
		l := sideTitle(1200, 38, mac, 260, true, false)
		buttons := []image.Rectangle{l.Toggle, l.Search, l.Agents, l.Gear, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
		for _, d := range l.Drag {
			for _, b := range buttons {
				if !b.Empty() && d.Overlaps(b) {
					t.Fatalf("mac=%v: drag %v over button %v", mac, d, b)
				}
			}
		}
		if len(l.Drag) == 0 {
			t.Fatalf("mac=%v: no drag space", mac)
		}
	}
}

func TestTitleCacheSeesSide(t *testing.T) {
	var cache titleLayoutCache
	in := titleIn{Size: image.Pt(1200, 38), SideW: 260, Crumb: fullCrumb, Dims: titleTestDims, Measure: sevenPx}
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	shape := new(text.Shaper)
	left := cache.layout(in, metric, shape).Lead
	in.Right = true
	if right := cache.layout(in, metric, shape).Lead; right == left {
		t.Fatalf("right side reused the left lead %v", left)
	}
	in.SideFull = true
	if l := cache.layout(in, metric, shape); !l.Search.Empty() {
		t.Fatalf("full-sidebar change reused a layout with search %v", l.Search)
	}
}

func TestPanelRightIconKnown(t *testing.T) {
	if !iconNames["panelRight"] {
		t.Fatal("panelRight is not a known icon")
	}
}

// In full mode on Windows the right strip is exactly the sidebar's width, so its
// divider lines up with the sidebar's.
func TestTitleRightFullWindowsMatchesSidebar(t *testing.T) {
	for _, w := range []int{560, 800, 1200} {
		l := sideTitle(w, 38, false, 260, true, false)
		if l.Lead.Min.X != w-260 || l.Lead.Max.X != w {
			t.Fatalf("%dpx: lead %v, want exactly the sidebar's 260 px", w, l.Lead)
		}
		rs := []image.Rectangle{l.Crumb, l.Search, l.Agents, l.Gear, l.Toggle, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
		for i, a := range rs {
			for _, b := range rs[i+1:] {
				if !a.Empty() && !b.Empty() && a.Overlaps(b) {
					t.Fatalf("%dpx: %v overlaps %v", w, a, b)
				}
			}
		}
		if l.Crumb.Max.X > l.Lead.Min.X {
			t.Fatalf("%dpx: crumb %v past the lead %v", w, l.Crumb, l.Lead)
		}
	}
}

func TestPaletteSidebarRightIcon(t *testing.T) {
	if got := palActionIcons["sidebar_right"]; got != "panelRight" {
		t.Fatalf("Sidebar on the right shows icon %q, want panelRight", got)
	}
}
