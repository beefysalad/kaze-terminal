package ui

import (
	"image"
	"slices"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// railDraw lays the rail out in a railW x h box with the default theme.
func railDraw(t *testing.T, m sideModel, st sideState, h int) (sideHits, image.Point) {
	t.Helper()
	size := image.Pt(railW, h)
	gtx := pageContext(size.X, size.Y)
	return newChrome().layoutRail(gtx, m, st, size), size
}

// railWindow is a sideWindow showing the rail.
func railWindow(t *testing.T) *window {
	t.Helper()
	win, _ := sideWindow(t, true)
	win.setSideMode(sideRail)
	return win
}

func TestRailTilesInSidebarOrder(t *testing.T) {
	m := twoGroupsOneLoose()
	m.Groups[0].Folded = true // the rail ignores folds
	h, size := railDraw(t, m, sideState{}, 600)
	var got []tabs.TabID
	for _, r := range h.rows {
		got = append(got, r.id)
		if r.rect.Empty() || !r.rect.In(image.Rectangle{Max: size}) {
			t.Fatalf("tile %d rect %v outside %v", r.id, r.rect, size)
		}
		if !r.title.Empty() || !r.fold.Empty() || !r.close.Empty() {
			t.Fatalf("tile %d has title %v fold %v close %v", r.id, r.title, r.fold, r.close)
		}
	}
	if want := []tabs.TabID{1, 2, 10, 20}; !slices.Equal(got, want) {
		t.Fatalf("tiles %v, want %v", got, want)
	}
	for i := 1; i < len(h.rows); i++ {
		if h.rows[i].rect.Min.Y <= h.rows[i-1].rect.Min.Y {
			t.Fatalf("tile %d at %v is not below %v", h.rows[i].id, h.rows[i].rect, h.rows[i-1].rect)
		}
	}
}

func TestRailTileClickShowsAndMiddleCloses(t *testing.T) {
	win := railWindow(t)
	a := openIn(t, win, "")
	b := openIn(t, win, "")
	win.mgr.SetActive(a.ID)
	var r input.Router
	sideFrame(t, win, &r)
	if win.side.mode != sideRail || win.side.w != railW {
		t.Fatalf("mode %v width %d, want the rail", win.side.mode, win.side.w)
	}
	sideClick(win, &r, rowHit(t, win, b.ID).rect, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	if got := win.mgr.Active(); got == nil || got.ID != b.ID {
		t.Fatalf("a click on b's tile shows %v", got)
	}
	sideClick(win, &r, rowHit(t, win, a.ID).rect, pointer.ButtonTertiary)
	sideFrame(t, win, &r)
	if win.mgr.Lookup(a.ID) != nil {
		t.Fatal("a middle click on a's tile left it open")
	}
}

func TestRailBarsClickMember(t *testing.T) {
	win, _, r, a, _, m1, m2 := memberWindow(t)
	win.setSideMode(sideRail)
	twoFrames(t, win, r)
	var bar image.Rectangle
	ids := []tabs.TabID{}
	for _, h := range win.side.hits.members {
		ids = append(ids, h.id)
		if h.id == m2.ID {
			bar = h.rect
		}
	}
	if !slices.Equal(ids, []tabs.TabID{m1.ID, m2.ID}) || bar.Empty() {
		t.Fatalf("bars %v, want %d and %d", ids, m1.ID, m2.ID)
	}
	sideClick(win, r, bar, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	if win.mgr.Active().ID != a.ID || win.memberDeck(m2.ID) != a.ID {
		t.Fatalf("bar click: active %d, docked in %d; want it docked in %d", win.mgr.Active().ID, win.memberDeck(m2.ID), a.ID)
	}
}

func TestRailSettingsTile(t *testing.T) {
	win := railWindow(t)
	a := openIn(t, win, "")
	var r input.Router
	sideFrame(t, win, &r)
	win.openSettings()
	sideFrame(t, win, &r)
	h := win.side.hits
	if h.settings.Empty() || len(h.rows) == 0 || h.settings.Max.Y > h.rows[0].rect.Min.Y {
		t.Fatalf("settings tile %v, first tile %v", h.settings, h.rows)
	}
	sideClick(win, &r, rowHit(t, win, a.ID).rect, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	if win.set.shown {
		t.Fatal("a session tile click left Settings shown")
	}
}

func TestRailScrollsAndPinsBars(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	win.setSideMode(sideRail)
	var extra []*tabs.Tab
	for range 28 {
		extra = append(extra, openIn(t, win, ""))
	}
	frame := func() { sizedFrame(t, win, r, 1000, 400) }
	frame()
	frame()
	h := win.side.hits
	if h.scrollMax <= 0 || len(h.members) != 2 {
		t.Fatalf("scrollMax %d, %d bars", h.scrollMax, len(h.members))
	}
	bars := slices.Clone(h.members)
	win.side.st.scroll = h.scrollMax
	frame()
	if got := win.side.hits.members; len(got) != len(bars) || got[0].rect != bars[0].rect || got[1].rect != bars[1].rect {
		t.Fatalf("bars moved on scroll: %v, were %v", got, bars)
	}
	for _, tab := range win.side.hits.rows {
		if tab.rect.Max.Y > bars[0].rect.Min.Y {
			t.Fatalf("tile %v runs under the bars at %v", tab.rect, bars[0].rect)
		}
	}
	for _, tab := range extra[:20] {
		win.closeTab(tab.ID)
	}
	frame()
	if win.side.st.scroll > win.side.hits.scrollMax {
		t.Fatalf("scroll %d past max %d", win.side.st.scroll, win.side.hits.scrollMax)
	}
}

func TestRailHoverTooltip(t *testing.T) {
	for _, right := range []bool{false, true} {
		h, size := railDraw(t, twoGroupsOneLoose(), sideState{hover: 2, right: right}, 600)
		tile := image.Rectangle{}
		for _, r := range h.rows {
			if r.id == 2 {
				tile = r.rect
			}
		}
		if h.tip.Empty() || tile.Empty() {
			t.Fatalf("right %v: tip %v, tile %v", right, h.tip, tile)
		}
		if h.tip.Overlaps(tile) || (!right && h.tip.Min.X < size.X) || (right && h.tip.Max.X > 0) {
			t.Fatalf("right %v: tip %v is not beside the rail (%v) toward the terminal", right, h.tip, size)
		}
		if h.tip.Max.Y < tile.Min.Y || h.tip.Min.Y > tile.Max.Y {
			t.Fatalf("right %v: tip %v is not level with tile %v", right, h.tip, tile)
		}
	}
	if h, _ := railDraw(t, twoGroupsOneLoose(), sideState{}, 600); !h.tip.Empty() {
		t.Fatalf("tip %v with nothing hovered", h.tip)
	}
}

func TestRailNoAllocsWhenUnchanged(t *testing.T) {
	win := railWindow(t)
	openIn(t, win, osPath("/code/repo/api"))
	openMember(t, win, "api")
	win.side.st.hover = win.mgr.Tabs()[0].ID
	var router input.Router
	sideFrame(t, win, &router)
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() {
		ops.Reset()
		win.drawSide(g, image.Rect(0, 42, railW, 568))
		win.drawSideTip(g)
	}
	for range 10 {
		draw()
	}
	if win.side.mode != sideRail || win.side.hits.tip.Empty() {
		t.Fatalf("mode %v tip %v, want the rail with a tooltip", win.side.mode, win.side.hits.tip)
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged rail allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged rail rebuilt or reshaped")
	}
}
