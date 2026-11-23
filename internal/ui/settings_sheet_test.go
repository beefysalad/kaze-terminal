package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/unit"
)

func TestSheetRect(t *testing.T) {
	for _, tc := range []struct {
		window, want image.Rectangle
		scale        float32
	}{
		{image.Rect(0, 0, 1200, 800), image.Rect(160, 80, 1040, 720), 1},
		{image.Rect(20, 40, 920, 720), image.Rect(30, 60, 910, 700), 1},
		{image.Rect(0, 0, 899, 800), image.Rect(0, 0, 899, 800), 1},
		{image.Rect(0, 0, 1200, 679), image.Rect(0, 0, 1200, 679), 1},
		{image.Rect(0, 0, 2400, 1600), image.Rect(320, 160, 2080, 1440), 2},
		{image.Rectangle{}, image.Rectangle{}, 1},
	} {
		if got := sheetRect(tc.window, unit.Metric{PxPerDp: tc.scale}); got != tc.want {
			t.Errorf("sheetRect(%v, %v) = %v, want %v", tc.window, tc.scale, got, tc.want)
		}
	}
}

func TestSettingsSheetDismiss(t *testing.T) {
	win, _ := alertWindow(t, 1)
	var router input.Router
	sizedFrame(t, win, &router, 1200, 800)
	win.openSettings()
	sizedFrame(t, win, &router, 1200, 800)
	win.settingsPointer(pressAt(center(win.set.sheet)))
	if !win.set.shown {
		t.Fatal("inside click dismissed sheet")
	}
	win.settingsPointer(pressAt(center(image.Rect(0, 0, 10, 10))))
	if win.set.shown {
		t.Fatal("backdrop click left sheet shown")
	}
	if win.focusTarget() != win.termTag {
		t.Fatal("terminal focus not restored")
	}
	win.openSettings()
	win.settingsKey(pageContext(1200, 800), key.Event{Name: key.NameEscape, State: key.Press})
	if win.set.shown {
		t.Fatal("Escape left sheet shown")
	}
}

func TestSettingsSheetEscape(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/sheet"))
	var r input.Router
	sizedFrame(t, win, &r, 1200, 800)
	before := win.mgr.Active().ID
	win.openSettings()
	sizedFrame(t, win, &r, 1200, 800)
	sizedFrame(t, win, &r, 1200, 800)
	if !r.Source().Focused(&win.set.tag) {
		t.Fatal("sheet did not take focus")
	}
	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	sizedFrame(t, win, &r, 1200, 800)
	sizedFrame(t, win, &r, 1200, 800)
	if win.set.shown || win.mgr.Active().ID != before || !r.Source().Focused(win.termTag) {
		t.Fatalf("Escape restore: shown %v active %d want %d focused %v target %v", win.set.shown, win.mgr.Active().ID, before, r.Source().Focused(win.termTag), win.focusTarget())
	}
}

func TestSettingsSheetEditorEscape(t *testing.T) {
	win, _ := alertWindow(t, 1)
	f := fieldWindow(t, win, &fieldDraft{})
	f.clickField("name")
	f.typeText("sheet edit")
	f.send(modPress(key.NameEscape, 0))
	if win.set.shown {
		t.Fatal("Escape from editor left sheet open")
	}
}

func TestSettingsSheetKeepsDock(t *testing.T) {
	win, _, r, a, _, _ := dockOne(t)
	before := win.dock.hits.panel
	win.openSettings()
	twoFrames(t, win, r)
	if win.dockOwner() != a.ID || before.Empty() || win.dock.hits.panel != before {
		t.Fatalf("dock changed under sheet: owner %d panel %v, want %v", win.dockOwner(), win.dock.hits.panel, before)
	}
	win.closeSettings()
	twoFrames(t, win, r)
	if !r.Source().Focused(&win.dock.term) {
		t.Fatal("member terminal focus not restored")
	}
}

func TestSettingsSheetBackdropDoesNotScroll(t *testing.T) {
	win, _ := alertWindow(t, 1)
	var r input.Router
	win.openSettings()
	sizedFrame(t, win, &r, 1200, 800)
	if win.set.scrollMax == 0 {
		t.Fatal("fixture needs scrolling")
	}
	win.settingsPointer(pointer.Event{Kind: pointer.Scroll, Position: f32.Pt(5, 5), Scroll: f32.Pt(0, 100)})
	if win.set.scroll != 0 {
		t.Fatal("wheel on backdrop scrolled sheet")
	}
}

// Exercise the real Gio router: the click dismisses without selecting the
// covered session, and sheet typing never enters the original PTY.
func TestSettingsSheetRoutesInput(t *testing.T) {
	win, st := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/a"))
	openIn(t, win, osPath("/code/b"))
	win.mgr.SetActive(a.ID)
	var r input.Router
	sizedFrame(t, win, &r, 1200, 800)
	win.openSettings()
	for range 2 {
		sizedFrame(t, win, &r, 1200, 800)
	}
	r.Queue(key.EditEvent{Text: "sheet-only"}, key.Event{Name: "Q", State: key.Press})
	for range 2 {
		sizedFrame(t, win, &r, 1200, 800)
	}
	if got := ptyOf(st, 0).sent(); strings.Contains(got, "sheet-only") || strings.Contains(got, "q") {
		t.Fatalf("PTY got sheet input %q", got)
	}
	// The sidebar row lies outside the centred sheet at this width.
	if len(win.side.hits.spaces) < 2 {
		t.Fatal("fixture needs two spaces")
	}
	at := win.side.hits.spaces[1].rect.Add(win.side.at)
	if at.Min.In(win.set.sheet) {
		t.Fatal("fixture row must be on backdrop")
	}
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: atPoint(at.Min.Add(image.Pt(10, 10)))}, pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: atPoint(at.Min.Add(image.Pt(10, 10)))})
	for range 3 {
		sizedFrame(t, win, &r, 1200, 800)
	}
	if win.set.shown || win.mgr.Active().ID != a.ID {
		t.Fatal("backdrop click reached covered session")
	}
}

func atPoint(p image.Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }
