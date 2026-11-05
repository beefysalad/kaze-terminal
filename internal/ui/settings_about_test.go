package ui

import (
	"golang.org/x/image/font/sfnt"

	"github.com/beefysalad/kaze-terminal/assets/fonts"
	"github.com/beefysalad/kaze-terminal/internal/theme"
	"github.com/beefysalad/kaze-terminal/internal/version"
	"image"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestAboutSectionShowsBuildAcrossThemes(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.openSettings()
	i := slices.IndexFunc(win.sections(), func(s settingsSection) bool { return s.title == "About" })
	if i < 0 {
		t.Fatal("missing About section")
	}
	win.selectSection(i)
	win.set.build = "build abc1234 · Oct 29, 4:30 PM + local changes"
	for _, th := range theme.All() {
		win.applyTheme(th)
		win.drawSettings(pageContext(800, 600), image.Rect(0, 38, 800, 576))
		text := strings.Join(win.set.texts, "\n")
		for _, want := range []string{"by Patrick", aboutKanji, aboutMeaning, aboutTagline, "MIT", platformName(runtime.GOOS), runtime.GOARCH,
			"Copy build info", "GitHub", "Report issue"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q", th.Key, want)
			}
		}
		// Neither the About page nor the sidebar footer shows the build line.
		if strings.Contains(text, "abc1234") || strings.Contains(text, "Oct 29") {
			t.Errorf("%s still shows the build line", th.Key)
		}
	}
}

// The version shows once in the page body (the sidebar footer's "kaze <v>" is
// shared chrome), with the update state beside it.
func TestAboutShowsTheVersionOnce(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.openSettings()
	for _, c := range []struct {
		name  string
		setup func()
		state string
	}{
		{"unchecked", func() {}, ""},
		{"up to date", func() { win.upd.checked = true }, "Up to date"},
		{"newer", func() { win.upd.newer, win.upd.rel.Version = true, "9.9" }, "9.9 available"},
	} {
		win.upd.checked, win.upd.newer, win.upd.err = false, false, ""
		c.setup()
		win.set.texts = win.set.texts[:0]
		drawAbout(&page{gtx: pageContext(800, 600), c: win.ui, st: &win.set, col: win.ui.pageColors()}, image.Rect(218, 60, 772, 576), win)
		n := 0
		for _, s := range win.set.texts {
			if strings.Contains(s, version.Number) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: version shown %d times in %q", c.name, n, win.set.texts)
		}
		text := strings.Join(win.set.texts, "\n")
		if c.state != "" && !strings.Contains(text, c.state) {
			t.Errorf("%s: missing state %q in %q", c.name, c.state, text)
		}
		if c.state == "" && (strings.Contains(text, "Up to date") || strings.Contains(text, "available")) {
			t.Errorf("%s: shows a state before any check: %q", c.name, text)
		}
	}
}

func aboutAt(t *testing.T, r image.Rectangle) (*window, aboutGeom) {
	t.Helper()
	win, _ := alertWindow(t, 1)
	win.openSettings()
	win.set.controls = win.set.controls[:0]
	drawAbout(&page{gtx: pageContext(800, 600), c: win.ui, st: &win.set, col: win.ui.pageColors()}, r, win)
	return win, win.set.about
}

func aboutButton(t *testing.T, win *window, id string) image.Rectangle {
	t.Helper()
	for _, c := range win.set.controls {
		if c.id == id {
			return c.rect
		}
	}
	t.Fatalf("no %s button", id)
	return image.Rectangle{}
}

func near(a, b, tol int) bool { return a-b <= tol && b-a <= tol }

// "by Patrick" starts under the wordmark and sits on the bottom of the mark;
// the lockup is centred in the pane.
func TestAboutBylineSitsUnderTheWordmark(t *testing.T) {
	for _, r := range []image.Rectangle{image.Rect(218, 60, 772, 576), image.Rect(160, 60, 520, 576)} {
		_, g := aboutAt(t, r)
		if g.lockup.Empty() || g.by.Empty() {
			t.Fatalf("%v: nothing laid out: %+v", r, g)
		}
		if !near(g.by.Min.X, g.wordX, 1) {
			t.Errorf("%v: byline starts at %d, wordmark at %d", r, g.by.Min.X, g.wordX)
		}
		if !near(g.byBase, g.markBottom, 1) {
			t.Errorf("%v: byline baseline %d, mark bottom %d", r, g.byBase, g.markBottom)
		}
		mid := (r.Min.X + r.Max.X) / 2
		if !near((g.lockup.Min.X+g.lockup.Max.X)/2, mid, 2) {
			t.Errorf("%v: lockup %v is not centred on %d", r, g.lockup, mid)
		}
	}
}

func TestAboutLockupIsSmallerWhenNarrow(t *testing.T) {
	_, wide := aboutAt(t, image.Rect(218, 60, 772, 576))
	_, narrow := aboutAt(t, image.Rect(160, 60, 520, 576))
	if wide.lockup.Dx() != 208 || narrow.lockup.Dx() != 168 {
		t.Fatalf("lockup widths %d and %d, want 208 and 168", wide.lockup.Dx(), narrow.lockup.Dx())
	}
}

// The buttons are the card's footer: Copy at the left, Report then GitHub at the right.
func TestAboutButtonsSitInTheCardFooter(t *testing.T) {
	r := image.Rect(218, 60, 772, 576)
	win, g := aboutAt(t, r)
	if g.card.Dx() > 440 || !near((g.card.Min.X+g.card.Max.X)/2, (r.Min.X+r.Max.X)/2, 1) {
		t.Fatalf("card %v is not a centred column of at most 440 in %v", g.card, r)
	}
	cp, is, gh := aboutButton(t, win, "about_copy"), aboutButton(t, win, "about_issue"), aboutButton(t, win, "about_github")
	for _, b := range []image.Rectangle{cp, is, gh} {
		if !b.In(g.card) {
			t.Errorf("button %v is outside the card %v", b, g.card)
		}
		if b.Min.Y != cp.Min.Y {
			t.Errorf("buttons are not on one row: %v %v %v", cp, is, gh)
		}
	}
	if !(cp.Max.X < is.Min.X && is.Max.X < gh.Min.X) {
		t.Errorf("order is not Copy, Report, GitHub: %v %v %v", cp, is, gh)
	}
	if !near(cp.Min.X-g.card.Min.X, g.card.Max.X-gh.Max.X, 1) {
		t.Errorf("footer insets differ: left %d right %d", cp.Min.X-g.card.Min.X, g.card.Max.X-gh.Max.X)
	}
}

// At the narrowest width that still puts the buttons on one row, in
// JetBrains Mono (wider than the old sans), they fit the card without overlapping.
func TestAboutButtonsFitOneRowAtItsNarrowest(t *testing.T) {
	r := image.Rect(100, 60, 520, 576) // 420 wide: the first width that is not narrow
	win, g := aboutAt(t, r)
	cp, is, gh := aboutButton(t, win, "about_copy"), aboutButton(t, win, "about_issue"), aboutButton(t, win, "about_github")
	if cp.Min.Y != is.Min.Y || is.Min.Y != gh.Min.Y {
		t.Fatalf("not one row: %v %v %v", cp, is, gh)
	}
	if !(cp.Max.X < is.Min.X && is.Max.X < gh.Min.X) || !cp.In(g.card) || !gh.In(g.card) {
		t.Fatalf("buttons overlap or leave the card %v: %v %v %v", g.card, cp, is, gh)
	}
}

// In a narrow pane the buttons stack full width, GitHub first.
func TestAboutNarrowStacksButtons(t *testing.T) {
	r := image.Rect(160, 60, 520, 576)
	win, g := aboutAt(t, r)
	if !g.card.In(r) {
		t.Fatalf("card %v overflows the pane %v", g.card, r)
	}
	gh, cp, is := aboutButton(t, win, "about_github"), aboutButton(t, win, "about_copy"), aboutButton(t, win, "about_issue")
	if !(gh.Max.Y <= cp.Min.Y && cp.Max.Y <= is.Min.Y) {
		t.Errorf("not stacked GitHub, Copy, Report: %v %v %v", gh, cp, is)
	}
	if gh.Dx() != cp.Dx() || cp.Dx() != is.Dx() || !gh.In(g.card) {
		t.Errorf("buttons are not one full width inside the card: %v %v %v in %v", gh, cp, is, g.card)
	}
}

// The kanji must come from a bundled UI font, or the tagline would show a box.
func TestAboutNameKanjiIsInAUIFont(t *testing.T) {
	var b sfnt.Buffer
	cjk, err := sfnt.Parse(fonts.CJK)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := cjk.GlyphIndex(&b, '風'); i == 0 {
		t.Fatal("the bundled Japanese font has no 風")
	}
	if aboutKanji != "風" {
		t.Fatalf("aboutKanji is %q", aboutKanji)
	}
}

func TestAboutCopyUsesDisplayedVersionAndBuild(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.openSettings()
	win.set.build = "build test123"
	var copied string
	win.toClipboard = func(s string) { copied = s }
	aboutAct(win, "about_copy", 0)
	drawAbout(&page{gtx: pageContext(800, 600), c: win.ui, st: &win.set, col: win.ui.pageColors()}, image.Rect(218, 60, 772, 576), win)
	if !strings.Contains(copied, "kaze "+version.Number) || !strings.Contains(copied, "build test123") {
		t.Fatalf("copy %q", copied)
	}
}

// The kanji is set apart by a small fixed gap: drawn in one run with the
// sentence, the Japanese font's wide space opened a hole after it.
func TestAboutKanjiSitsCloseToTheMeaning(t *testing.T) {
	win, g := aboutAt(t, image.Rect(218, 60, 772, 576))
	p := &page{gtx: pageContext(800, 600), c: win.ui, st: &win.set, col: win.ui.pageColors()}
	_, ks := p.text(aboutKanji, 12.5, p.col.muted)
	_, ms := p.text(aboutMeaning, 12.5, p.col.muted)
	if g.meaning.Dx() > ks.X+ms.X+6 {
		t.Fatalf("meaning line %d wide, want at most %d", g.meaning.Dx(), ks.X+ms.X+6)
	}
}
