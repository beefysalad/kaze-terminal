package ui

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/theme"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

func TestFindBarTopRight(t *testing.T) {
	win, _, r, _, _, m1, _ := memberWindow(t)
	findKey(t, win, r)
	gtx := pageContext(1000, 600)
	bar, pane := win.find.hits.bar, win.find.pane
	if pane.Empty() || bar.Max.X != pane.Max.X-gtx.Dp(18) || bar.Min.Y != pane.Min.Y+gtx.Dp(8) || bar.Dy() != gtx.Dp(34) {
		t.Fatalf("bar %v in pane %v, want top right, 8 dp down, 18 dp in, 34 dp high", bar, pane)
	}
	// A workspace banner on the shown session: the bar sits under it.
	win.closeFind(false)
	win.mgr.SetActive(m1.ID)
	win.runs[1].members[0].run.State = workspaces.Waiting
	twoFrames(t, win, r)
	if win.wsr.rect.Empty() {
		t.Fatal("no banner drawn")
	}
	findKey(t, win, r)
	if bar := win.find.hits.bar; bar.Min.Y != win.wsr.rect.Max.Y+gtx.Dp(8) {
		t.Fatalf("bar %v under banner %v, want 8 dp below it", bar, win.wsr.rect)
	}
}

func TestFindBarOnMemberPane(t *testing.T) {
	win, _, r, _, _, _ := dockOne(t)
	findKey(t, win, r)
	body := win.dock.hits.body
	if bar := win.find.hits.bar; !bar.In(body) || bar.Min.Y < body.Min.Y {
		t.Fatalf("bar %v, want inside the dock body %v", bar, body)
	}
	if win.find.placeholder != "Find in api" {
		t.Fatalf("placeholder %q, want Find in api", win.find.placeholder)
	}
}

func TestFindCountText(t *testing.T) {
	for _, c := range []struct {
		cur, total   int
		capped, none bool
		want         string
	}{
		{3, 12, false, false, "3 of 12"},
		{0, 0, false, true, "No results"},
		{1, 1000, true, false, "1 of 1000+"},
		{0, 0, false, false, ""},
	} {
		if got := findCountText(c.cur, c.total, c.capped, c.none); got != c.want {
			t.Errorf("%+v: %q, want %q", c, got, c.want)
		}
	}
}

func TestFindBarTexts(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn", "warn", "warn")
	findKey(t, win, r)
	if slices.Contains(win.find.texts, "") || len(win.find.texts) != 1 || win.find.texts[0] != "Find in terminal" {
		t.Fatalf("empty field: texts %q, want only the placeholder", win.find.texts)
	}
	setFind(t, win, r, "warn")
	if !slices.Contains(win.find.texts, "1 of 3") {
		t.Fatalf("texts %q, want 1 of 3", win.find.texts)
	}
	setFind(t, win, r, "zzz")
	if !slices.Contains(win.find.texts, "No results") {
		t.Fatalf("texts %q, want No results", win.find.texts)
	}
}

func TestFindBarNarrow(t *testing.T) {
	win, _, r, _, _, _, _ := memberWindow(t)
	findKey(t, win, r)
	gtx := pageContext(1000, 600)
	pane := func(w int) image.Rectangle { return image.Rect(100, 50, 100+w, 400) }
	draw := func(w int) findHits {
		g := pageContext(1000, 600)
		win.drawFind(g, pane(w))
		return win.find.hits
	}
	h := draw(300)
	if h.field.Dx() != gtx.Dp(96) || !h.count.Empty() || !h.bar.In(pane(300)) {
		t.Fatalf("300 px: field %d, count %v, bar %v", h.field.Dx(), h.count, h.bar)
	}
	h = draw(160)
	if !h.prev.Empty() || !h.caseBtn.Empty() || h.close.Empty() || h.field.Empty() || !h.bar.In(pane(160)) {
		t.Fatalf("160 px: %+v, want only the field and close inside the pane", h)
	}
	if h.field.Max.X > h.close.Min.X {
		t.Fatalf("160 px: field %v overlaps close %v", h.field, h.close)
	}
}

func TestFindButtons(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "Warn", "warn", "warn")
	findFor(t, win, r, "warn")
	clickAt(t, win, r, centre(win.find.hits.caseBtn))
	settleFind(t, win, r)
	if _, total, _, _ := win.findCount(); !win.find.matchCase || total != 2 {
		t.Fatalf("Aa: matchCase %v, %d matches, want exact case with 2", win.find.matchCase, total)
	}
	cur, _, _, _ := win.findCount()
	clickAt(t, win, r, centre(win.find.hits.next))
	if got, _, _, _ := win.findCount(); got != cur%2+1 {
		t.Fatalf("next: %d, want %d", got, cur%2+1)
	}
	clickAt(t, win, r, centre(win.find.hits.prev))
	if got, _, _, _ := win.findCount(); got != cur {
		t.Fatalf("prev: %d, want %d", got, cur)
	}
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("clicks on the bar reached the shell: %q", got)
	}
	clickAt(t, win, r, centre(win.find.hits.close))
	if win.find.open {
		t.Fatal("× did not close the bar")
	}
}

func TestFindBarReadableInEveryTheme(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		ink := c.findInk()
		for _, p := range []struct {
			name   string
			fg, bg color.NRGBA
			min    float64
		}{
			{"field", ink.text, ink.bg, 4.5}, {"placeholder", ink.placeholder, ink.bg, 4.5},
			{"count", ink.count, ink.bg, 4.5}, {"none", ink.none, ink.bg, 4.5},
			{"Aa off", ink.icon, ink.bg, 4.5}, {"Aa hover", ink.iconHover, ink.hover, 4.5},
			{"Aa on", ink.btnOn, ink.on, 4.5}, {"icon", ink.icon, ink.bg, 3},
		} {
			if got := contrast(p.fg, p.bg); got < p.min {
				t.Errorf("%s %s: %.2f:1, want %.1f", th.Key, p.name, got, p.min)
			}
		}
	}
}

func TestFindCountTextBuiltOnce(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn")
	findFor(t, win, r, "warn")
	n := win.find.countBuilds
	for range 20 {
		sideFrame(t, win, r)
	}
	if got := win.find.countBuilds - n; got != 0 {
		t.Fatalf("the count text was built %d times over 20 still frames", got)
	}
}

func TestFindIconsKnown(t *testing.T) {
	for _, name := range []string{"down", "close", "chevup"} {
		if !iconNames[name] {
			t.Fatalf("icon %q unknown", name)
		}
	}
}
