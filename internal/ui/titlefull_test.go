package ui

import (
	"image"
	"testing"
)

// fullModeTitle lays the bar out for one sidebar mode and side. side is the
// sidebar's width (0 hidden); full says the sidebar header owns + and search.
func fullModeTitle(w int, mac bool, side int, right, full bool) titleLayout {
	in := titleIn{Size: image.Pt(w, 38), Mac: mac, SideW: side, Right: right, SideFull: full, Crumb: fullCrumb,
		Key: "⌘K", Dims: titleTestDims, Measure: sevenPx, WordW: titleWordW}
	if mac {
		in.LightsW = 70
	}
	return layoutTitle(in)
}

// The sidebar header holds the search field and +, so a full sidebar leaves
// them out of the bar; a rail or hidden sidebar keeps the search field.
func TestTitleBarDropsSearchOnlyWhileTheSidebarIsFull(t *testing.T) {
	modes := []struct {
		name string
		side int
		full bool
	}{{"full", 260, true}, {"rail", 52, false}, {"hidden", 0, false}}
	for _, m := range modes {
		for _, right := range []bool{false, true} {
			for _, mac := range []bool{false, true} {
				l := fullModeTitle(1200, mac, m.side, right, m.full)
				tag := m.name
				if right {
					tag += "/right"
				}
				if mac {
					tag += "/mac"
				}
				if m.full != l.Search.Empty() {
					t.Errorf("%s: search %v, want empty=%v", tag, l.Search, m.full)
				}
				if !m.full && l.Search.Empty() {
					t.Errorf("%s: search missing", tag)
				}
				for name, r := range map[string]image.Rectangle{"toggle": l.Toggle, "agents": l.Agents, "gear": l.Gear, "crumb": l.Crumb} {
					if r.Empty() {
						t.Errorf("%s: %s missing", tag, name)
					}
				}
				rs := []image.Rectangle{l.Crumb, l.Search, l.Agents, l.Gear, l.Toggle, l.Buttons[0], l.Buttons[1], l.Buttons[2]}
				for i, a := range rs {
					for j, b := range rs {
						if i < j && !a.Empty() && !b.Empty() && a.Overlaps(b) {
							t.Errorf("%s: %v overlaps %v", tag, a, b)
						}
					}
				}
				// Without the search field its space is window drag space.
				if m.full {
					mid := image.Pt((l.Crumb.Max.X+l.Agents.Min.X)/2, 19)
					if l.at(mid) != capNone || !inAny(l.Drag, mid) {
						t.Errorf("%s: gap at %v is not drag space (drag %v)", tag, mid, l.Drag)
					}
				}
				for _, b := range []image.Rectangle{l.Toggle, l.Agents, l.Gear, l.Search} {
					if !b.Empty() && inAny(l.Drag, b.Min) {
						t.Errorf("%s: drag covers the button %v", tag, b)
					}
				}
			}
		}
	}
}

func inAny(rs []image.Rectangle, p image.Point) bool {
	for _, r := range rs {
		if p.In(r) {
			return true
		}
	}
	return false
}
