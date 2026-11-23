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

// The backdrop intercepts the sidebar search click, dismissing Settings without
// opening the underlying search field.
func TestSidebarFilterClickStopsRecording(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	if win.set.keyRec != "find" {
		t.Fatalf("clicking find's keys did not start recording (keyRec %q)", win.set.keyRec)
	}
	clickRect(t, win, r, win.side.hits.search.Add(win.side.at))
	if win.side.filtering || win.set.shown {
		t.Fatal("backdrop click opened search or left Settings shown")
	}
	if win.set.keyRec != "" {
		t.Fatalf("still recording %q after a click on the sidebar filter", win.set.keyRec)
	}
}
