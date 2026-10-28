package ui

import (
	"image"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

func TestButtonsAreRightAlignedInOrder(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Buttons[2] != image.Rect(954, 0, 1000, 38) {
		t.Fatalf("close %v", b.Buttons[2])
	}
	if b.Buttons[1] != image.Rect(908, 0, 954, 38) {
		t.Fatalf("max %v", b.Buttons[1])
	}
	if b.Buttons[0] != image.Rect(862, 0, 908, 38) {
		t.Fatalf("min %v", b.Buttons[0])
	}
}

func TestDragFillsSpaceBetweenTabsAndButtons(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Drag != image.Rect(300, 0, 862, 38) {
		t.Fatalf("drag %v", b.Drag)
	}
}

func TestRegionsNeverOverlapButtons(t *testing.T) {
	b := layoutBar(image.Pt(400, 38), 900, 46, true) // tabs want more than fits
	if b.Tabs.Max.X > b.Buttons[0].Min.X {
		t.Fatalf("tabs %v overlap min %v", b.Tabs, b.Buttons[0])
	}
	if !b.Drag.Empty() {
		t.Fatalf("no room for drag, got %v", b.Drag)
	}
}

func TestRegionsScaleWithDp(t *testing.T) {
	b := layoutBar(image.Pt(1500, 57), 300, 69, true) // 150%
	if b.Buttons[2].Dx() != 69 || b.Buttons[2].Dy() != 57 {
		t.Fatalf("%v", b.Buttons[2])
	}
}

func TestDecoratedHasNoButtonsOrDrag(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, false)
	if !b.Buttons[0].Empty() || !b.Drag.Empty() || b.Tabs.Max.X != 1000 {
		t.Fatalf("%+v", b)
	}
}

func TestTabsAreNotMoveArea(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Drag.Overlaps(b.Tabs) {
		t.Fatalf("drag %v overlaps tabs %v", b.Drag, b.Tabs)
	}
}

func TestClickFiresOnReleaseInside(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	var s captionState
	p := b.Buttons[0].Min.Add(image.Pt(5, 5))
	if s.pointer(pointer.Press, p, b) != capNone {
		t.Fatal("fired on press")
	}
	if s.pointer(pointer.Release, p, b) != capMinimize {
		t.Fatal("no fire on release")
	}
}

func TestReleaseOutsideButtonDoesNothing(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	var s captionState
	s.pointer(pointer.Press, b.Buttons[2].Min.Add(image.Pt(5, 5)), b)
	if s.pointer(pointer.Release, image.Pt(500, 20), b) != capNone {
		t.Fatal("fired outside")
	}
}

func TestMaximizeTogglesWithRealState(t *testing.T) {
	if actionFor(capMaximize, false) != system.ActionMaximize {
		t.Fatal("max")
	}
	if actionFor(capMaximize, true) != system.ActionUnmaximize {
		t.Fatal("restore")
	}
	if actionFor(capMinimize, true) != system.ActionMinimize || actionFor(capClose, false) != system.ActionClose {
		t.Fatal("min/close")
	}
}
