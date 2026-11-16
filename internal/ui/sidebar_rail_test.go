package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/vt"
	"image"
	"slices"
	"strings"
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
	var router input.Router
	sideFrame(t, win, &router)
	win.side.st.hover = win.mgr.Tabs()[0].ID // after the first frame, whose switch to the rail drops any hover
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

// A filter typed in full mode never hides tiles in the rail, and the keyboard
// never stays in the filter field the rail does not draw.
func TestRailIgnoresFilterAndDropsItsFocus(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		win, _ := sideWindow(t, true)
		a := openIn(t, win, osPath("/code/alpha/api"))
		b := openIn(t, win, osPath("/code/beta/web"))
		var r input.Router
		sizedFrame(t, win, &r, 1000, 600)
		win.side.ed.SetText("api")
		win.side.filtering = true
		sizedFrame(t, win, &r, 1000, 600)
		if len(win.side.hits.rows) != 1 {
			t.Fatalf("narrow %v: the filter shows %d rows in full, want 1", narrow, len(win.side.hits.rows))
		}
		w := 1000
		if narrow {
			w = 500
		} else {
			win.opt.Config.SidebarMode = "rail" // the setting, without setSideMode's own drop
		}
		sizedFrame(t, win, &r, w, 600)
		sizedFrame(t, win, &r, w, 600)
		var ids []tabs.TabID
		for _, h := range win.side.hits.rows {
			ids = append(ids, h.id)
		}
		if win.side.mode != sideRail || !slices.Contains(ids, a.ID) || !slices.Contains(ids, b.ID) {
			t.Fatalf("narrow %v: mode %v, tiles %v, want %d and %d", narrow, win.side.mode, ids, a.ID, b.ID)
		}
		if win.side.filtering || win.focusTarget() == &win.side.ed {
			t.Fatalf("narrow %v: the keyboard stays in the hidden filter field", narrow)
		}
	}
}

// A shown doc or agent panel keeps its session's tile highlighted; Settings does not.
func TestRailHighlightsSessionOfShownDoc(t *testing.T) {
	onOf := func(st sideState) map[tabs.TabID]bool {
		h, _ := railDraw(t, twoGroupsOneLoose(), st, 600)
		on := map[tabs.TabID]bool{}
		for _, r := range h.rows {
			if r.on {
				on[r.id] = true
			}
		}
		return on
	}
	if on := onOf(sideState{shown: 2, doc: 7}); !on[2] || len(on) != 1 {
		t.Fatalf("with a doc of 2 shown, highlighted %v, want only 2", on)
	}
	if on := onOf(sideState{shown: 2, settingsOn: true}); len(on) != 0 {
		t.Fatalf("with Settings shown, highlighted %v, want none", on)
	}
}

// The rail's tooltip stays inside the window: on a right rail in a narrow window
// a long title is cut to the room on the terminal's side.
func TestRailTipStaysInWindow(t *testing.T) {
	for _, right := range []bool{false, true} {
		win := railWindow(t)
		win.setSideRight(right)
		a := openIn(t, win, "")
		s := a.Session
		s.Lock()
		vt.NewParser(s.Applier()).Feed([]byte("\x1b]2;" + strings.Repeat("a very long session title ", 6) + "\x07"))
		s.Unlock()
		openIn(t, win, "") // shown, so a's needs mark stays
		deliverNow(win, notify.Msg{Tab: int(a.ID), Kind: notify.KindNeeds}, t0)
		if win.alerts.markOf(a.ID) != markNeeds {
			t.Fatal("setup: needs did not mark")
		}
		var r input.Router
		sizedFrame(t, win, &r, 320, 600)
		win.side.st.hover = a.ID
		sizedFrame(t, win, &r, 320, 600)
		tip := win.side.hits.tip.Add(win.side.at)
		if win.side.mode != sideRail || win.side.hits.tip.Empty() {
			t.Fatalf("right %v: mode %v, tip %v, want the rail with a tooltip", right, win.side.mode, win.side.hits.tip)
		}
		if tip.Min.X < 0 || tip.Max.X > 320 {
			t.Fatalf("right %v: tip %v leaves the 320 px window", right, tip)
		}
	}
}
