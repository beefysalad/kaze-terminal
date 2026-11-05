package ui

import (
	"image"
	"testing"
)

// A click on the empty sidebar, outside the Settings pane, ends recording
// (review finding 2).
func TestSidebarClickStopsRecording(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	if win.set.keyRec != "find" {
		t.Fatalf("clicking find's keys did not start recording (keyRec %q)", win.set.keyRec)
	}
	clickRect(t, win, r, image.Rectangle{Min: win.side.at.Add(image.Pt(120, 700)), Max: win.side.at.Add(image.Pt(121, 701))})
	if win.set.keyRec != "" {
		t.Fatalf("still recording %q after a click on the empty sidebar", win.set.keyRec)
	}
}

// A click on the sidebar's filter field ends recording too: the field has the
// keyboard now.
func TestSidebarFilterClickStopsRecording(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	if win.set.keyRec != "find" {
		t.Fatalf("clicking find's keys did not start recording (keyRec %q)", win.set.keyRec)
	}
	clickRect(t, win, r, win.side.hits.filter.Add(win.side.at))
	if !win.side.filtering {
		t.Fatal("the click missed the sidebar filter field")
	}
	if win.set.keyRec != "" {
		t.Fatalf("still recording %q after a click on the sidebar filter", win.set.keyRec)
	}
}
