package ui

import (
	"image/color"
	"testing"
	"time"

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
