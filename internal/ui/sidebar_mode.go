package ui

// sideMode is how much of the sidebar shows: the full list, the narrow rail of
// tiles, or nothing.
type sideMode uint8

const (
	sideFull sideMode = iota
	sideRail
	sideHidden
)

const (
	// railW is the rail's width in dp.
	railW = 52
	// minRailWindow is the narrowest window, in dp, that still shows the rail.
	minRailWindow = 300
)

// String is the mode's config value.
func (m sideMode) String() string {
	switch m {
	case sideRail:
		return "rail"
	case sideHidden:
		return "hidden"
	}
	return "full"
}

// next is the mode Ctrl+Shift+B and the title bar toggle go to: full, rail, hidden, full.
func (m sideMode) next() sideMode {
	switch m {
	case sideFull:
		return sideRail
	case sideRail:
		return sideHidden
	}
	return sideFull
}

// savedSideMode is the mode the settings ask for: sidebar_mode when set, else the
// old sidebar bool (true is full, false hidden).
func (win *window) savedSideMode() sideMode {
	switch win.opt.Config.SidebarMode {
	case "full":
		return sideFull
	case "rail":
		return sideRail
	case "hidden":
		return sideHidden
	}
	if win.opt.Config.Sidebar {
		return sideFull
	}
	return sideHidden
}

// sideRight reports whether the sidebar sits at the window's right edge.
func (win *window) sideRight() bool { return win.opt.Config.SidebarSide == "right" }

// effectiveSideMode is the mode a window width px wide shows for the saved one: under
// railMin px (gtx.Dp(minRailWindow)) nothing, under fullMin px (gtx.Dp(minSideWindow))
// full narrows to the rail. Comparing in px keeps it in step with the rest of the
// layout at a fractional scale. Never saved.
func effectiveSideMode(saved sideMode, width, railMin, fullMin int) sideMode {
	switch {
	case width < railMin:
		return sideHidden
	case width < fullMin && saved == sideFull:
		return sideRail
	}
	return saved
}

// setSideMode saves m as the sidebar's mode. The sidebar's editors give the keyboard
// back first, since the field or row they sit over may go away.
func (win *window) setSideMode(m sideMode) {
	win.dropSideEdit()
	win.opt.Config.SidebarMode = m.String()
	win.side.st.hover = 0
	win.saveSetting("sidebar_mode", m.String())
}

// setSideRight saves which edge the sidebar sits at.
func (win *window) setSideRight(right bool) {
	win.dropSideEdit()
	side := "left"
	if right {
		side = "right"
	}
	win.opt.Config.SidebarSide = side
	win.side.st.hover = 0
	win.saveSetting("sidebar_side", side)
}
