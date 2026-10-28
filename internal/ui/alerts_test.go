package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

var t0 = time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)

// only reports tabs 1..n as open.
func only(n int) func(tabs.TabID) bool {
	return func(id tabs.TabID) bool { return id >= 1 && int(id) <= n }
}

func TestNeedsMarksTabAndToastsSticky(t *testing.T) {
	var a alerts
	chime, redraw := a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds, Reason: "permission"}, 1, true, only(3), t0)
	if chime != notify.KindNeeds || !redraw {
		t.Fatalf("apply = %q, %v; want needs, true", chime, redraw)
	}
	if a.markOf(2) != markNeeds {
		t.Fatalf("mark %v, want needs", a.markOf(2))
	}
	l := a.list()
	if len(l) != 1 || l[0].Tab != 2 || l[0].Kind != notify.KindNeeds || l[0].Text != "permission" || !l[0].Until.IsZero() {
		t.Fatalf("toasts %+v, want one sticky needs toast for tab 2", l)
	}
	if _, changed := a.expire(t0.Add(time.Hour)); changed || len(a.list()) != 1 {
		t.Fatal("a needs toast expired")
	}
}

func TestDoneToastExpiresAfter6s(t *testing.T) {
	var a alerts
	chime, _ := a.apply(notify.Msg{Tab: 2, Kind: notify.KindDone}, 1, true, only(2), t0)
	if chime != notify.KindDone {
		t.Fatalf("chime %q, want done (the window decides whether it sounds)", chime)
	}
	next, changed := a.expire(t0.Add(5 * time.Second))
	if changed || !next.Equal(t0.Add(6*time.Second)) || len(a.list()) != 1 {
		t.Fatalf("at 5s: next %v changed %v toasts %d; want t0+6s, false, 1", next, changed, len(a.list()))
	}
	next, changed = a.expire(t0.Add(6 * time.Second))
	if !changed || !next.IsZero() || len(a.list()) != 0 {
		t.Fatalf("at 6s: next %v changed %v toasts %d; want zero, true, 0", next, changed, len(a.list()))
	}
	if a.markOf(2) != markDone {
		t.Fatal("the done mark went with the toast; it stays until the tab is visited")
	}
}

func TestActiveFocusedTabGetsNothing(t *testing.T) {
	var a alerts
	chime, redraw := a.apply(notify.Msg{Tab: 1, Kind: notify.KindNeeds}, 1, true, only(2), t0)
	if chime != "" || redraw || a.markOf(1) != markNone || len(a.list()) != 0 {
		t.Fatalf("active focused tab got chime %q redraw %v mark %v toasts %d", chime, redraw, a.markOf(1), len(a.list()))
	}
}

func TestActiveButUnfocusedStillAlerts(t *testing.T) {
	var a alerts
	chime, redraw := a.apply(notify.Msg{Tab: 1, Kind: notify.KindNeeds}, 1, false, only(2), t0)
	if chime != notify.KindNeeds || !redraw || a.markOf(1) != markNeeds || len(a.list()) != 1 {
		t.Fatalf("unfocused active tab got chime %q redraw %v mark %v toasts %d", chime, redraw, a.markOf(1), len(a.list()))
	}
}

func TestVisitClearsMarkAndToasts(t *testing.T) {
	var a alerts
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds}, 1, true, only(3), t0)
	a.apply(notify.Msg{Tab: 3, Kind: notify.KindDone}, 1, true, only(3), t0)
	a.visited(2)
	if a.markOf(2) != markNone {
		t.Fatal("visit kept the mark")
	}
	l := a.list()
	if len(l) != 1 || l[0].Tab != 3 || a.markOf(3) != markDone {
		t.Fatalf("visiting tab 2 touched tab 3: %+v", l)
	}
}

func TestAlertForUnknownTabIgnored(t *testing.T) {
	var a alerts
	for _, id := range []int{0, -1, 4, 1 << 30} {
		chime, redraw := a.apply(notify.Msg{Tab: id, Kind: notify.KindNeeds}, 1, true, only(3), t0)
		if chime != "" || redraw {
			t.Fatalf("tab %d: chime %q redraw %v", id, chime, redraw)
		}
	}
	if len(a.list()) != 0 || len(a.marks) != 0 {
		t.Fatalf("unknown tabs left state: %+v %v", a.list(), a.marks)
	}
}

func TestMaxThreeToastsNewestFirst(t *testing.T) {
	var a alerts
	for id := 2; id <= 6; id++ {
		a.apply(notify.Msg{Tab: id, Kind: notify.KindNeeds}, 1, true, only(6), t0.Add(time.Duration(id)*time.Second))
	}
	l := a.list()
	if len(l) != 3 || l[0].Tab != 6 || l[1].Tab != 5 || l[2].Tab != 4 {
		t.Fatalf("toasts %+v, want tabs 6, 5, 4", l)
	}
	if len(a.toasts) > maxToasts {
		t.Fatalf("%d toasts kept, want at most %d", len(a.toasts), maxToasts)
	}
	for id := tabs.TabID(2); id <= 6; id++ {
		if a.markOf(id) != markNeeds {
			t.Fatalf("tab %d lost its mark when its toast was pushed out", id)
		}
	}
}

func TestNeedsOverridesDone(t *testing.T) {
	var a alerts
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindDone}, 1, true, only(2), t0)
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds}, 1, true, only(2), t0)
	if a.markOf(2) != markNeeds {
		t.Fatalf("done then needs: mark %v, want needs", a.markOf(2))
	}
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindDone}, 1, true, only(2), t0)
	if a.markOf(2) != markNeeds {
		t.Fatalf("needs then done: mark %v, want needs (still unseen)", a.markOf(2))
	}
	if l := a.list(); len(l) != 1 {
		t.Fatalf("one tab has %d toasts, want 1", len(l))
	}
}

func TestReasonIsCleanedAndCapped(t *testing.T) {
	var a alerts
	reason := "a\x1b[31mb\x07‮" + strings.Repeat("x", 300)
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds, Reason: reason}, 1, true, only(2), t0)
	got := a.list()[0].Text
	if strings.ContainsAny(got, "\x1b\x07‮") {
		t.Fatalf("control chars kept: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > maxReason {
		t.Fatalf("reason has %d runes, want at most %d", n, maxReason)
	}
	if !strings.HasPrefix(got, "a[31mb") {
		t.Fatalf("reason %q", got[:10])
	}
}

func TestDismissDropsToastKeepsMark(t *testing.T) {
	var a alerts
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds}, 1, true, only(2), t0)
	a.dismiss(2)
	if len(a.list()) != 0 || a.markOf(2) != markNeeds {
		t.Fatalf("after dismiss: toasts %d mark %v", len(a.list()), a.markOf(2))
	}
}

func TestPruneForgetsClosedTabs(t *testing.T) {
	var a alerts
	a.apply(notify.Msg{Tab: 2, Kind: notify.KindNeeds}, 1, true, only(3), t0)
	a.apply(notify.Msg{Tab: 3, Kind: notify.KindNeeds}, 1, true, only(3), t0)
	a.prune(only(2))
	if a.markOf(3) != markNone || len(a.list()) != 1 || a.list()[0].Tab != 2 {
		t.Fatalf("closed tab 3 kept: mark %v toasts %+v", a.markOf(3), a.list())
	}
}

// alertWindow is a focused test window with n tabs, the first active, and a recorded chime.
func alertWindow(t *testing.T, n int) (*window, *[]time.Time) {
	t.Helper()
	win := testWindow(t, &recStarter{})
	win.ui = newChrome()
	win.focused = true
	for range n {
		if _, err := win.mgr.New(); err != nil {
			t.Fatal(err)
		}
	}
	win.mgr.SetActive(win.mgr.Tabs()[0].ID)
	var played []time.Time
	win.chime = func(at time.Time) { played = append(played, at) }
	win.opt.Config.ChimeNeeds = true
	return win, &played
}

func TestGoSwitchesTab(t *testing.T) {
	win, _ := alertWindow(t, 3)
	target := win.mgr.Tabs()[2].ID
	win.alerts.apply(notify.Msg{Tab: int(target), Kind: notify.KindNeeds}, win.mgr.Active().ID, true, only(3), t0)
	win.drawToasts(barContext(1000), 600, 24)
	if len(win.ui.toastHits) != 1 {
		t.Fatalf("%d toast hit areas, want 1", len(win.ui.toastHits))
	}
	g := win.ui.toastHits[0].goRect
	win.toastEvent(pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(g.Min.X+2), float32(g.Min.Y+2))})
	if got := win.mgr.Active().ID; got != target {
		t.Fatalf("active %d after Go, want %d", got, target)
	}
	if !win.wantFocus {
		t.Fatal("Go left the terminal without keyboard focus")
	}
}

func TestTabClickAndGoShareTheSwitch(t *testing.T) {
	win, _ := alertWindow(t, 2)
	target := win.mgr.Tabs()[1].ID
	win.alerts.apply(notify.Msg{Tab: int(target), Kind: notify.KindNeeds}, win.mgr.Active().ID, true, only(2), t0)
	win.selectTab(target)
	win.visit(win.mgr.Active())
	if win.mgr.Active().ID != target || win.alerts.markOf(target) != markNone || len(win.alerts.list()) != 0 {
		t.Fatalf("selectTab+visit: active %d mark %v toasts %d", win.mgr.Active().ID, win.alerts.markOf(target), len(win.alerts.list()))
	}
}

func TestToastCloseDismisses(t *testing.T) {
	win, _ := alertWindow(t, 2)
	first := win.mgr.Active().ID
	target := win.mgr.Tabs()[1].ID
	win.alerts.apply(notify.Msg{Tab: int(target), Kind: notify.KindNeeds}, first, true, only(2), t0)
	win.drawToasts(barContext(1000), 600, 24)
	c := win.ui.toastHits[0].closeRect
	win.toastEvent(pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(c.Min.X+2), float32(c.Min.Y+2))})
	if win.mgr.Active().ID != first || len(win.alerts.list()) != 0 || win.alerts.markOf(target) != markNeeds {
		t.Fatalf("× : active %d toasts %d mark %v", win.mgr.Active().ID, len(win.alerts.list()), win.alerts.markOf(target))
	}
}

func TestVisitNeedsFocus(t *testing.T) {
	win, _ := alertWindow(t, 2)
	target := win.mgr.Tabs()[1].ID
	win.alerts.apply(notify.Msg{Tab: int(target), Kind: notify.KindNeeds}, win.mgr.Active().ID, true, only(2), t0)
	win.focused = false
	win.selectTab(target)
	win.visit(win.mgr.Active())
	if win.alerts.markOf(target) != markNeeds {
		t.Fatal("an unfocused window cleared the mark")
	}
	win.focused = true
	win.visit(win.mgr.Active())
	if win.alerts.markOf(target) != markNone {
		t.Fatal("focusing the window on the tab kept the mark")
	}
}

func TestDeliverChimesPerConfig(t *testing.T) {
	win, played := alertWindow(t, 3)
	ids := win.mgr.Tabs()
	win.opt.Config.ChimeNeeds, win.opt.Config.ChimeDone = true, false
	win.deliver(notify.Msg{Tab: int(ids[1].ID), Kind: notify.KindDone}, t0)
	if len(*played) != 0 {
		t.Fatal("done chimed with chime_done off")
	}
	win.deliver(notify.Msg{Tab: int(ids[1].ID), Kind: notify.KindNeeds}, t0)
	if len(*played) != 1 {
		t.Fatalf("needs played %d times, want 1", len(*played))
	}
	win.deliver(notify.Msg{Tab: int(ids[0].ID), Kind: notify.KindNeeds}, t0)
	if len(*played) != 1 {
		t.Fatal("the active tab of a focused window chimed")
	}
	win.opt.Config.ChimeNeeds = false
	win.deliver(notify.Msg{Tab: int(ids[2].ID), Kind: notify.KindNeeds}, t0)
	if len(*played) != 1 {
		t.Fatal("needs chimed with chime_needs off")
	}
	win.opt.Config.ChimeDone = true
	win.deliver(notify.Msg{Tab: int(ids[2].ID), Kind: notify.KindDone}, t0)
	if len(*played) != 2 {
		t.Fatal("done stayed silent with chime_done on")
	}
}

func TestOnNotifyNeverBlocks(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.alertCh = make(chan notify.Msg, 2)
	done := make(chan struct{})
	go func() {
		for range 50 {
			win.onNotify(notify.Msg{Tab: 1, Kind: notify.KindNeeds})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onNotify blocked on a full channel")
	}
	if len(win.alertCh) != 2 {
		t.Fatalf("channel holds %d, want 2 (the rest dropped)", len(win.alertCh))
	}
}

func TestDrainAppliesQueuedAlerts(t *testing.T) {
	win, _ := alertWindow(t, 2)
	target := win.mgr.Tabs()[1].ID
	win.alertCh = make(chan notify.Msg, 4)
	win.onNotify(notify.Msg{Tab: int(target), Kind: notify.KindNeeds})
	win.drainAlerts(t0)
	if win.alerts.markOf(target) != markNeeds || len(win.alertCh) != 0 {
		t.Fatalf("after drain: mark %v queued %d", win.alerts.markOf(target), len(win.alertCh))
	}
}

func TestToastsSitAboveStatusBarBottomRight(t *testing.T) {
	win, _ := alertWindow(t, 4)
	for _, tb := range win.mgr.Tabs()[1:] {
		win.alerts.apply(notify.Msg{Tab: int(tb.ID), Kind: notify.KindNeeds, Reason: strings.Repeat("long reason ", 20)}, win.mgr.Active().ID, true, only(4), t0)
	}
	const w, h, statusH = 400, 600, 24
	win.drawToasts(barContext(w), h, statusH)
	hits := win.ui.toastHits
	if len(hits) != 3 {
		t.Fatalf("%d toasts drawn, want 3", len(hits))
	}
	for i, th := range hits {
		if th.rect.Max.Y > h-statusH || th.rect.Max.X > w || th.rect.Min.X < 0 {
			t.Fatalf("toast %d at %v overlaps the status bar or leaves a %dx%d window", i, th.rect, w, h)
		}
		if !th.goRect.In(th.rect) || !th.closeRect.In(th.rect) {
			t.Fatalf("toast %d: Go %v / × %v outside %v", i, th.goRect, th.closeRect, th.rect)
		}
		if i > 0 && th.rect.Overlaps(hits[i-1].rect) {
			t.Fatalf("toasts %d and %d overlap", i-1, i)
		}
	}
	if hits[0].id != win.mgr.Tabs()[3].ID || hits[0].rect.Min.Y > hits[1].rect.Min.Y {
		t.Fatal("the newest toast is not on top")
	}
}

func TestTabDotsFollowMarks(t *testing.T) {
	win, _ := alertWindow(t, 3)
	list := win.mgr.Tabs()
	win.alerts.apply(notify.Msg{Tab: int(list[1].ID), Kind: notify.KindNeeds}, list[0].ID, true, only(3), t0)
	win.alerts.apply(notify.Msg{Tab: int(list[2].ID), Kind: notify.KindDone}, list[0].ID, true, only(3), t0)
	cases := []struct {
		id   tabs.TabID
		on   bool
		want bool
		col  uint32
	}{
		{list[0].ID, true, true, 0x8b7bff},
		{list[1].ID, false, true, 0xf2b84b},
		{list[2].ID, false, true, 0x62d39a},
	}
	for _, c := range cases {
		col, ok := dotColor(c.on, win.alerts.markOf(c.id))
		if ok != c.want || col != rgb(c.col) {
			t.Fatalf("tab %d: dot %v %v, want %06x", c.id, col, ok, c.col)
		}
	}
	if _, ok := dotColor(false, markNone); ok {
		t.Fatal("an unmarked inactive tab has a dot")
	}
}
