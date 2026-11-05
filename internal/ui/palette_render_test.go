package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/skills"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// palMockupSkills are the mockup's skills (docs/design/palette-mockup.html).
func palMockupSkills() []skills.Skill {
	var list []skills.Skill
	add := func(scope skills.Scope, desc map[string]string, names ...string) {
		for _, n := range names {
			list = append(list, skills.Skill{Name: n, Desc: desc[n], Scope: scope})
		}
	}
	d := map[string]string{
		"chat-digest": "Morning V3 chat brief from saved Chatwork messages", "ci-watch": "Babysit CI on my open PRs until each is green",
		"fix-conflicts": "Find your open PRs that conflict with master and fix them", "green-ci": "Get your open PRs' red CI to green",
		"handoff": "Capture a per-ticket end-of-session handoff", "my-prs": "A triaged dashboard of your open PRs", "open-pr": "Open the draft PR",
		"review-mine": "Deep pre-merge review of your own branch", "review-teammate-pr": "Shallow review of a teammate's PR",
		"ship-ticket": "Execute the plan, review, open the draft PR", "start-ticket": "Scaffold a new Jira ticket",
		"devbox:dashboard": "Open your devbox board in the browser", "devbox:devbox": "Send work that can run without you",
		"devbox:jobs": "Your devbox jobs, one line each", "devbox:run": "Send a task to a worker agent",
		"superpowers:brainstorming": "Use before any creative work", "superpowers:writing-plans": "Write a plan from a spec",
		"superpowers:requesting-code-review": "Review before merging", "v3-gauntlet:v3-review": "Builder and critic review loop",
		"v3-gauntlet:v3-ticket": "Plan a ticket without stopping", "impeccable:impeccable": "Design and polish a frontend",
	}
	add(skills.User, d, "chat-digest", "ci-watch", "fix-conflicts", "green-ci", "handoff", "my-prs", "open-pr", "review-mine", "review-teammate-pr", "ship-ticket", "start-ticket")
	add(skills.Plugin, d, "devbox:dashboard", "devbox:devbox", "devbox:jobs", "devbox:run", "superpowers:brainstorming", "superpowers:writing-plans",
		"superpowers:requesting-code-review", "v3-gauntlet:v3-review", "v3-gauntlet:v3-ticket", "impeccable:impeccable")
	return list
}

// palRenderWindow is the mockup's scene: five sessions (one needing you, one
// running, one codex) and its skills, the palette open.
func palRenderWindow(t *testing.T, th theme.Theme) (*window, *input.Router) {
	t.Helper()
	win, _ := sideWindow(t, true)
	tmp := t.TempDir()
	win.cfgPath = filepath.Join(tmp, "kaze", "config.toml")
	win.claudePath = filepath.Join(tmp, ".claude", "settings.json")
	win.opt.Shell = []string{"/bin/zsh"}
	win.skillsFn = func(skills.Sources) []skills.Skill { return palMockupSkills() }
	win.applyTheme(th)
	var made []*tabs.Tab
	for _, title := range []string{"Review PR 835 V3-425", "Epic V3-3347 tickets", "Lap questions V3-3550 and V3-3551", "Issues 143 and 112", "kaze-browser-p1"} {
		tab, err := win.mgr.NewOpts(tabs.StartOpts{Title: title, Color: "cyan"})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, tab)
	}
	win.mgr.SetActive(made[1].ID)
	win.wantFocus = true
	r := new(input.Router)
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	for i, tab := range made {
		agent := "claude"
		if i == 4 {
			agent = "codex"
		}
		win.onNotify(notify.Msg{Tab: int(tab.ID), Kind: notify.KindSession, Agent: agent})
	}
	win.alerts.apply(notify.Msg{Tab: int(made[0].ID), Kind: notify.KindNeeds}, 0, false, func(tabs.TabID) bool { return true }, time.Now())
	sideFrame(t, win, r)
	openPal(t, win, r)
	return win, r
}

// palTypeAtCaret types s the way the platform does (Gio's EditorInsert): an
// EditEvent over the current selection, then a SelectionEvent that puts the
// caret after the text. palType sends only the first, so in a headless run the
// caret stays at 0: that, not the palette, is why captures showed the caret at
// the start of the text.
func palTypeAtCaret(t *testing.T, win *window, r *input.Router, s string) {
	t.Helper()
	a, b := win.palette.ed.Selection()
	a, b = min(a, b), max(a, b)
	end := a + utf8.RuneCountInString(s)
	r.Queue(key.EditEvent{Range: key.Range{Start: a, End: b}, Text: s}, key.SelectionEvent{Start: end, End: end})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

// TestPaletteRenders writes development captures of the palette (labelled as
// such, not product screenshots) when KAZE_PALETTE_RENDER_DIR is set.
func TestPaletteRenders(t *testing.T) {
	dir := os.Getenv("KAZE_PALETTE_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_PALETTE_RENDER_DIR for headless captures of the palette")
	}
	os.MkdirAll(dir, 0o700)
	for _, th := range theme.All() {
		for _, state := range []struct {
			name, text string
			w          int
		}{{"open", "", 1000}, {"typing", "v3", 1000}, {"slash", "/", 1000}, {"review", "/review", 1000}, {"none", "zzz", 1000}, {"narrow", "/review", 520}} {
			if state.name == "narrow" && th.Key != "midnight" && th.Key != "washi" {
				continue
			}
			win, r := palRenderWindow(t, th)
			if state.text != "" {
				palTypeAtCaret(t, win, r, state.text)
			}
			size := image.Pt(state.w, 700)
			g := pageContext(size.X, size.Y)
			g.Source = r.Source()
			g.Now = time.Unix(1000, 0)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			gpu, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Fatal(err)
			}
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "palette-"+th.Key+"-"+state.name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, img)
			f.Close()
		}
	}
}

// Every colour the palette drew, on the surface it drew it on, meets its
// minimum: text 4.5:1, marks 3:1. The states cover Sessions (rows, pills,
// badges, a selected row), the Skills scope (groups, tiles, descriptions),
// a narrow box and the empty list.
func TestPaletteTextReadableAsDrawn(t *testing.T) {
	for _, th := range theme.All() {
		lowest, what := 99.0, ""
		for _, state := range []struct {
			text string
			w    int
		}{{"", 1000}, {"v3", 1000}, {"/", 1000}, {"/review", 1000}, {"zzz", 1000}, {"/review", 520}} {
			win, r := palRenderWindow(t, th)
			if state.text != "" {
				palType(t, win, r, state.text)
			}
			var pairs []palPair
			win.ui.palInk().audit = &pairs
			g := pageContext(state.w, 700)
			g.Source = r.Source()
			g.Now = time.Unix(1000, 0)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			win.ui.palInk().audit = nil
			if len(pairs) < 3 {
				t.Fatalf("%s %q: only %d pairs recorded", th.Key, state.text, len(pairs))
			}
			for _, p := range pairs {
				c := contrast(p.fg, p.bg)
				if c-p.min+4.5 < lowest {
					lowest, what = c-p.min+4.5, p.what
				}
				if c < p.min {
					t.Errorf("%s %q (%d px): %s %.2f:1, want %.1f", th.Key, state.text, state.w, p.what, c, p.min)
				}
			}
		}
		t.Logf("%s: lowest margin %.2f over the minimum (4.5 scale), on %s", th.Key, lowest, what)
	}
}

// Typing the way the platform does leaves the caret after the text.
func TestPaletteCaretFollowsTyping(t *testing.T) {
	win, r := palRenderWindow(t, theme.Default())
	palTypeAtCaret(t, win, r, "/rev")
	palTypeAtCaret(t, win, r, "iew")
	if a, b := win.palette.ed.Selection(); a != 7 || b != 7 {
		t.Fatalf("caret %d..%d after \"/review\", want 7..7", a, b)
	}
}
