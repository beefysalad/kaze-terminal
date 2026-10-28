package ui

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

type captionButton int

const (
	capNone captionButton = iota
	capMinimize
	capMaximize // shows "restore" when maximized
	capClose
)

// barLayout is where things go in the top bar, in pixels.
type barLayout struct {
	Tabs    image.Rectangle    // where the tab strip may draw (ends before Drag)
	Drag    image.Rectangle    // empty space that moves the window; may be empty
	Buttons [3]image.Rectangle // indexed capMinimize-1, capMaximize-1, capClose-1
}

// layoutBar splits a bar of the given size. Without custom buttons (macOS,
// decorated) the tabs take the whole bar and there is no drag area.
func layoutBar(size image.Point, tabsWidth, buttonW int, custom bool) barLayout {
	var b barLayout
	if !custom {
		b.Tabs = image.Rect(0, 0, size.X, size.Y)
		return b
	}
	left := size.X - 3*buttonW
	if left < 0 {
		left = 0
	}
	for i := range b.Buttons {
		x := left + i*buttonW
		b.Buttons[i] = image.Rect(x, 0, x+buttonW, size.Y)
	}
	tabsEnd := tabsWidth
	if tabsEnd > left {
		tabsEnd = left
	}
	b.Tabs = image.Rect(0, 0, tabsEnd, size.Y)
	if tabsEnd < left {
		b.Drag = image.Rect(tabsEnd, 0, left, size.Y)
	}
	return b
}

// captionAt returns which caption button contains p.
func (b barLayout) captionAt(p image.Point) captionButton {
	for i, r := range b.Buttons {
		if p.In(r) {
			return captionButton(i + 1)
		}
	}
	return capNone
}

// captionState tracks hover/press for the three buttons across pointer events.
type captionState struct{ hover, pressed captionButton }

// pointer feeds one pointer event and returns the button to fire now. A click
// fires on release inside the button that was pressed.
func (s *captionState) pointer(kind pointer.Kind, p image.Point, b barLayout) captionButton {
	switch kind {
	case pointer.Press:
		s.hover = b.captionAt(p)
		s.pressed = s.hover
	case pointer.Release:
		fired := capNone
		if s.pressed != capNone && b.captionAt(p) == s.pressed {
			fired = s.pressed
		}
		s.pressed = capNone
		s.hover = b.captionAt(p)
		return fired
	case pointer.Move, pointer.Enter, pointer.Drag:
		s.hover = b.captionAt(p)
	case pointer.Leave:
		s.hover = capNone
	case pointer.Cancel:
		s.hover, s.pressed = capNone, capNone
	}
	return capNone
}

// actionFor maps a fired button to the window action, given the real maximized state.
func actionFor(c captionButton, maximized bool) system.Action {
	switch c {
	case capMinimize:
		return system.ActionMinimize
	case capMaximize:
		if maximized {
			return system.ActionUnmaximize
		}
		return system.ActionMaximize
	case capClose:
		return system.ActionClose
	}
	return 0
}
