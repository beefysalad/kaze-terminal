package ui

import (
	"image"
	"testing"

	"gioui.org/text"
	"gioui.org/unit"
)

// sideTitle lays out the bar with the sidebar on the right (or left) at the given width.
func sideTitle(w, h int, mac bool, side int, right, noPlus bool) titleLayout {
	in := titleIn{Size: image.Pt(w, h), Mac: mac, SideW: side, Right: right, NoPlus: noPlus, Crumb: fullCrumb,
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
	for name, r := range map[string]image.Rectangle{"gear": l.Gear, "agents": l.Agents, "toggle": l.Toggle, "plus": l.Plus} {
		if r.Empty() || !r.In(l.Lead) {
			t.Fatalf("%s %v not inside the lead %v", name, r, l.Lead)
		}
	}
	if !(l.Plus.Max.X <= l.Agents.Min.X && l.Agents.Max.X <= l.Gear.Min.X && l.Gear.Max.X <= l.Toggle.Min.X) {
		t.Fatalf("want plus %v, agents %v, gear %v, toggle %v left to right", l.Plus, l.Agents, l.Gear, l.Toggle)
	}
	if l.Plus.Min.X != 940+titleTestDims.LeadPad {
		t.Fatalf("plus %v, want it at the strip's left end", l.Plus)
	}
	if l.Mark.Min.X != 70+titleTestDims.Gap {
		t.Fatalf("mark %v, want it after the lights", l.Mark)
	}
	if l.Search.Empty() || l.Search.Max.X > 940 {
		t.Fatalf("search %v must end before the lead", l.Search)
	}
}

func TestTitleRightRailWindows(t *testing.T) {
	l := sideTitle(1200, 38, false, 52, true, true)
	if !l.Plus.Empty() {
		t.Fatalf("rail shows no plus, got %v", l.Plus)
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

func TestTitleLeftRailHasNoPlus(t *testing.T) {
	l := sideTitle(1200, 38, false, 52, false, true)
	if !l.Plus.Empty() {
		t.Fatalf("rail shows no plus, got %v", l.Plus)
	}
	if l.Lead.Max.X < l.Toggle.Max.X {
		t.Fatalf("lead %v does not cover the toggle %v", l.Lead, l.Toggle)
	}
}

func TestTitleRightHiddenKeepsButtons(t *testing.T) {
	for _, mac := range []bool{true, false} {
		l := sideTitle(1200, 38, mac, 0, true, true)
		if !l.Plus.Empty() || l.Toggle.Empty() || l.Gear.Empty() || l.Agents.Empty() {
			t.Fatalf("mac=%v: toggle %v gear %v agents %v plus %v", mac, l.Toggle, l.Gear, l.Agents, l.Plus)
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
		buttons := []image.Rectangle{l.Toggle, l.Plus, l.Search, l.Agents, l.Gear, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
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
	in.NoPlus = true
	if l := cache.layout(in, metric, shape); !l.Plus.Empty() {
		t.Fatalf("no-plus change reused a layout with plus %v", l.Plus)
	}
}

func TestPanelRightIconKnown(t *testing.T) {
	if !iconNames["panelRight"] {
		t.Fatal("panelRight is not a known icon")
	}
}

// In full mode on Windows the right strip is exactly the sidebar's width, so its
// divider lines up with the sidebar's; the new session button that does not fit
// sits just left of it.
func TestTitleRightFullWindowsMatchesSidebar(t *testing.T) {
	for _, w := range []int{560, 800, 1200} {
		l := sideTitle(w, 38, false, 260, true, false)
		if l.Lead.Min.X != w-260 || l.Lead.Max.X != w {
			t.Fatalf("%dpx: lead %v, want exactly the sidebar's 260 px", w, l.Lead)
		}
		if l.Plus.Empty() || (l.Plus.Max.X > l.Lead.Min.X && !l.Plus.In(l.Lead)) {
			t.Fatalf("%dpx: plus %v must sit inside or just left of the lead %v", w, l.Plus, l.Lead)
		}
		rs := []image.Rectangle{l.Crumb, l.Search, l.Plus, l.Agents, l.Gear, l.Toggle, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
		for i, a := range rs {
			for _, b := range rs[i+1:] {
				if !a.Empty() && !b.Empty() && a.Overlaps(b) {
					t.Fatalf("%dpx: %v overlaps %v", w, a, b)
				}
			}
		}
		if l.Crumb.Max.X > l.Plus.Min.X || l.Search.Max.X > l.Plus.Min.X {
			t.Fatalf("%dpx: crumb %v or search %v past the plus %v", w, l.Crumb, l.Search, l.Plus)
		}
	}
}

func TestPaletteSidebarRightIcon(t *testing.T) {
	if got := palActionIcons["sidebar_right"]; got != "panelRight" {
		t.Fatalf("Sidebar on the right shows icon %q, want panelRight", got)
	}
}
