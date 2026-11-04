package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
	"github.com/patrick-gogo/kaze-terminal/internal/vt"
)

// shellTitle sets the title the tab's shell reports, as OSC 0 would.
func shellTitle(tab *tabs.Tab, s string) {
	tab.Session.Lock()
	vt.NewParser(tab.Session.Applier()).Feed([]byte("\x1b]0;" + s + "\x07"))
	tab.Session.Unlock()
}

// cardTab is a background tab of a two-tab live window, in a folder on branch.
func cardTab(t *testing.T, branch string) (*window, *tabs.Tab) {
	t.Helper()
	win, _, _ := liveWindow(t, 2)
	win.branchFn = func(string) string { return branch }
	win.repoFn = func(dir string) (string, string) { return dir, "repo" }
	tab := win.mgr.Tabs()[1]
	report(t, tab, tmp(t))
	shellTitle(tab, "zsh")
	win.mgr.Tabs() // refresh titles
	return win, tab
}

func toastOf(t *testing.T, win *window, id tabs.TabID) toast {
	t.Helper()
	for _, o := range win.alerts.toasts {
		if o.Tab == id && o.Run == 0 {
			return o
		}
	}
	t.Fatalf("no toast for tab %d", id)
	return toast{}
}

func TestCardTitleIsSidebarRow(t *testing.T) {
	win, tab := cardTab(t, "feature/V3-3149")
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindPrompt, Command: "/review-pr", Args: "auth-tokens"}, t0)
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindNeeds, Reason: "Claude needs your permission to use Bash"}, t0)
	c := win.cardOf(toastOf(t, win, tab.ID))
	if c.title != "auth-tokens" || c.title == "zsh" {
		t.Fatalf("title %q, want the sidebar row's auth-tokens", c.title)
	}
	if c.ctx != "⎇ feature/V3-3149 · /review-pr" {
		t.Fatalf("ctx %q", c.ctx)
	}
	if c.mark != "claude" || c.kind != notify.KindNeeds || c.tab != tab.ID {
		t.Fatalf("card %+v", c)
	}
	if c.body != "Claude needs your permission to use Bash" || c.status != "Needs your permission" {
		t.Fatalf("body %q status %q", c.body, c.status)
	}
}

func TestCardContextDropsMissingParts(t *testing.T) {
	win, tab := cardTab(t, "")
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindPrompt, Command: "/ship", Args: "x"}, t0)
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindDone}, t0)
	if c := win.cardOf(toastOf(t, win, tab.ID)); c.ctx != "/ship" {
		t.Fatalf("no branch: ctx %q, want /ship", c.ctx)
	}
	win2, tab2 := cardTab(t, "")
	deliverNow(win2, notify.Msg{Tab: int(tab2.ID), Kind: notify.KindDone}, t0)
	if c := win2.cardOf(toastOf(t, win2, tab2.ID)); c.ctx != "" {
		t.Fatalf("neither: ctx %q, want empty", c.ctx)
	}
}

func TestCardStatus(t *testing.T) {
	for _, c := range []struct {
		kind   notify.Kind
		reason string
		ran    time.Duration
		run    bool
		want   string
	}{
		{notify.KindNeeds, "Claude needs your PERMISSION to use Bash", 0, false, "Needs your permission"},
		{notify.KindNeeds, "Please approve the plan", 0, false, "Needs your permission"},
		{notify.KindNeeds, "Which branch should I use?", 0, false, "Waiting for your answer"},
		{notify.KindNeeds, "", 0, false, "Waiting for your answer"},
		{notify.KindDone, "", 400 * time.Second, false, "Done · ran 6m 40s"},
		{notify.KindDone, "", 45 * time.Second, false, "Done · ran 45s"},
		{notify.KindDone, "", time.Hour + 2*time.Minute + 5*time.Second, false, "Done · ran 1h 2m"},
		{notify.KindDone, "", 0, false, "Done"},
		{notify.KindDone, "web stack started", 0, true, "Run finished"},
		{notify.KindNeeds, "1 failed", 0, true, "Run finished"},
	} {
		if got := cardStatus(c.kind, c.reason, c.ran, c.run); got != c.want {
			t.Errorf("cardStatus(%s, %q, %v, %v) = %q, want %q", c.kind, c.reason, c.ran, c.run, got, c.want)
		}
	}
}

func TestDoneCardRanSincePrompt(t *testing.T) {
	win, tab := cardTab(t, "")
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindPrompt}, t0)
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindDone, Reason: "Fixed it"}, t0.Add(400*time.Second))
	got := toastOf(t, win, tab.ID)
	if got.Ran != 400*time.Second {
		t.Fatalf("Ran %v, want 6m40s", got.Ran)
	}
	if c := win.cardOf(got); c.status != "Done · ran 6m 40s" || c.body != "Fixed it" {
		t.Fatalf("status %q body %q", c.status, c.body)
	}
	// A second done without a new prompt does not know when its turn began.
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindDone}, t0.Add(500*time.Second))
	if got := toastOf(t, win, tab.ID); got.Ran != 0 {
		t.Fatalf("second done: Ran %v, want 0", got.Ran)
	}
}

func TestRunCardUsesWorkspaceMark(t *testing.T) {
	win, tab := cardTab(t, "main")
	win.alerts.addRun(toast{Tab: tab.ID, Run: 7, Title: "web stack started", Agent: "workspace", Kind: notify.KindDone})
	var run toast
	for _, o := range win.alerts.toasts {
		if o.Run == 7 {
			run = o
		}
	}
	c := win.cardOf(run)
	if c.mark != "workspace" || c.ctx != "workspace" || c.title != "web stack started" || c.run != 7 {
		t.Fatalf("run card %+v", c)
	}
	if c.status != "Run finished" {
		t.Fatalf("status %q", c.status)
	}
}

func TestCardContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		k := c.cardInk()
		low := 99.0
		check := func(label string, fg, bg color.NRGBA) {
			got := contrast(fg, bg)
			low = min(low, got)
			if got < 4.5 {
				t.Errorf("%s: %s contrast %.2f, want at least 4.5", th.Key, label, got)
			}
		}
		for label, fg := range map[string]color.NRGBA{
			"title": k.title, "body": k.body, "muted": k.muted, "branch": k.branch,
			"cmd": k.cmd, "go": k.go_, "needs": k.needs, "done": k.done,
		} {
			check(label+" on bg", fg, k.bg)
		}
		check("key cap text on capBG", k.body, k.capBG)
		check("badge on needs", k.badgeFG, k.needs)
		check("badge on done", k.badgeFG, k.done)
		if k.bg != th.UI.ToastBG || k.line != th.UI.ToastLine {
			t.Errorf("%s: card surface is not the theme's toast tokens", th.Key)
		}
		t.Logf("%s: lowest card contrast %.2f", th.Key, low)
	}
}

// cardTexts is every text the last drawToasts drew.
func cardTexts(win *window) []string {
	var out []string
	for _, x := range win.alerts.texts {
		out = append(out, x.s)
	}
	return out
}

func TestCardBodyWrapsOnceAndCuts(t *testing.T) {
	win, _ := alertWindow(t, 2)
	body := strings.Repeat("fixed the link split ", 8)[:160]
	win.alerts.apply(notify.Msg{Tab: int(win.mgr.Tabs()[1].ID), Kind: notify.KindDone, Reason: body}, win.mgr.Active().ID, true, only(2), t0)
	win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	w := &win.alerts.toasts[0].wrap
	if w.lines[0] == "" || w.lines[1] == "" || !strings.HasSuffix(w.lines[1], "…") {
		t.Fatalf("lines %q, want two with the second cut", w.lines)
	}
	if !strings.HasPrefix(body, w.lines[0]) || strings.HasSuffix(w.lines[0], " ") || body[len(w.lines[0])] != ' ' {
		t.Fatalf("line 1 %q does not end on a word of %q", w.lines[0], body)
	}
	if w.measures == 0 || w.measures > 40 {
		t.Fatalf("%d measures for one wrap", w.measures)
	}
	before := w.measures
	for range 10 {
		win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	}
	if w := &win.alerts.toasts[0].wrap; w.measures != before {
		t.Fatalf("10 more frames measured %d more times, want 0", w.measures-before)
	}
	win.drawToasts(barContext(800), toastArea(800, 600, 24)) // still 380 wide: no re-wrap
	if w := &win.alerts.toasts[0].wrap; w.measures != before {
		t.Fatal("a window resize that keeps the card width re-wrapped")
	}
	win.drawToasts(barContext(340), toastArea(340, 600, 24))
	if w := &win.alerts.toasts[0].wrap; w.measures == before {
		t.Fatal("a narrower card kept the old wrap")
	}
}

func TestCardKeyCapsOnNewestOnly(t *testing.T) {
	win, _ := alertWindow(t, 3)
	win.binds, _ = resolveBindings("windows", nil)
	for _, tb := range win.mgr.Tabs()[1:] {
		win.alerts.apply(notify.Msg{Tab: int(tb.ID), Kind: notify.KindNeeds}, win.mgr.Active().ID, true, only(3), t0)
	}
	newest := win.mgr.Tabs()[2].ID
	win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	var newRect image.Rectangle
	for _, h := range win.ui.toastHits {
		if h.id == newest {
			newRect = h.rect
		}
	}
	for _, k := range []string{"Ctrl", "Shift", "J"} {
		n := 0
		for _, x := range win.alerts.texts {
			if x.s == k {
				n++
				if !x.r.In(newRect) {
					t.Fatalf("key cap %q at %v is not on the newest card %v", k, x.r, newRect)
				}
			}
		}
		if n != 1 {
			t.Fatalf("key cap %q drawn %d times, want once (texts %q)", k, n, cardTexts(win))
		}
	}
	if got := strings.Count(strings.Join(cardTexts(win), "\n"), "Open session"); got != 2 {
		t.Fatalf("%d cards say Open session, want 2", got)
	}
	delete(win.binds, "next_waiting")
	win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	for _, s := range cardTexts(win) {
		if s == "Ctrl" || s == "Shift" || s == "J" {
			t.Fatalf("unbound next_waiting still drew key cap %q", s)
		}
	}
}

func TestDoneCardShowsReply(t *testing.T) {
	win, tab := cardTab(t, "main")
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindPrompt, Command: "/ship", Args: "links"}, t0)
	deliverNow(win, notify.Msg{Tab: int(tab.ID), Kind: notify.KindDone, Reason: "Fixed the link split. 214 tests pass"}, t0.Add(400*time.Second))
	win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	all := strings.Join(cardTexts(win), "\n")
	for _, want := range []string{"Fixed the link split", "Done · ran 6m 40s", "links", "⎇ main", "/ship", "Open session", "×"} {
		if !strings.Contains(all, want) {
			t.Fatalf("texts %q lack %q", cardTexts(win), want)
		}
	}
}

func TestNextWaitingOpensNewestCard(t *testing.T) {
	win, ids := needsWindow(t, 4)
	win.alerts.apply(notify.Msg{Tab: int(ids[1]), Kind: notify.KindNeeds}, ids[0], true, win.mgr.Has, t0)
	win.alerts.apply(notify.Msg{Tab: int(ids[2]), Kind: notify.KindDone}, ids[0], true, win.mgr.Has, t0)
	win.nextWaiting()
	if got := win.mgr.Active().ID; got != ids[2] {
		t.Fatalf("active %d, want the newest card's session %d", got, ids[2])
	}
	if !win.side.reveal {
		t.Fatal("the sidebar was not told to show the row")
	}
	var r input.Router
	sizedFrame(t, win, &r, 1000, 700)
	for _, o := range win.alerts.toasts {
		if o.Tab == ids[2] {
			t.Fatal("the opened session's card stayed after a frame")
		}
	}
	if len(win.alerts.toasts) != 1 || win.alerts.toasts[0].Tab != ids[1] {
		t.Fatalf("cards %+v, want only the needs card left", win.alerts.toasts)
	}
}

func TestNextWaitingWithoutCardsWalksMarks(t *testing.T) {
	win, ids := needsWindow(t, 3, 1)
	win.nextWaiting()
	if got := win.mgr.Active().ID; got != ids[1] {
		t.Fatalf("active %d, want the marked %d", got, ids[1])
	}
}

// A run summary card stays when its tab is visited, so the key skips a card
// on the shown tab and goes on to the sessions that wait.
func TestNextWaitingSkipsCardOnShownTab(t *testing.T) {
	win, ids := needsWindow(t, 3, 2)
	win.alerts.addRun(toast{Tab: ids[0], Kind: notify.KindNeeds, Text: "1 failed", Run: 5, Title: "web"})
	win.nextWaiting()
	if got := win.mgr.Active().ID; got != ids[2] {
		t.Fatalf("active %d, want the waiting session %d", got, ids[2])
	}
	if len(win.alerts.toasts) != 1 {
		t.Fatalf("cards %+v, want the run card kept", win.alerts.toasts)
	}
}
