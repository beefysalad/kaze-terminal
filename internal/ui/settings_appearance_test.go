package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// appearanceWindow is a window with default settings, saving to path, and the
// Appearance section shown and laid out.
func appearanceWindow(t *testing.T, path string) *window {
	t.Helper()
	win, _ := alertWindow(t, 1)
	win.opt.Config = config.Default()
	win.cfgPath = path
	win.openSettings()
	win.drawSettings(pageContext(800, 600), pageRect)
	return win
}

// press clicks the control with id and arg, then lays the page out again.
func press(t *testing.T, win *window, id string, arg int) {
	t.Helper()
	pressAny(t, win, id, arg)
}

func themeIndex(t *testing.T, key string) int {
	t.Helper()
	i := slices.IndexFunc(theme.All(), func(th theme.Theme) bool { return th.Key == key })
	if i < 0 {
		t.Fatalf("no theme %q", key)
	}
	return i
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAppearanceRows(t *testing.T) {
	win := appearanceWindow(t, "")
	all := strings.Join(win.set.texts, "|")
	for _, want := range []string{
		"Theme", "Midnight", "Sumi", "Tide", "Washi", "Sakura", "Matcha", "Kōyō", "Hai", "Fuji", "Kiri",
		"Font size", "14",
		"Line spacing", "Tight", "Normal", "Roomy",
		"CURSOR", "Shape", "Block", "Bar", "Underline",
		"Blinking cursor", "Off keeps idle CPU at zero",
	} {
		if !slices.Contains(win.set.texts, want) {
			t.Fatalf("page lacks %q: %s", want, all)
		}
	}
	if !strings.Contains(strings.Join(win.set.texts, " "), "JetBrains Mono, with Japanese and symbol fallback.") {
		t.Fatalf("page lacks the font line: %s", all)
	}
	if n := len(controlsOf(win, "theme")); n != 10 {
		t.Fatalf("%d theme swatches, want 10", n)
	}
	for id, n := range map[string]int{"font": 2, "spacing": 3, "cursor": 3, "blink": 1} {
		if got := len(controlsOf(win, id)); got != n {
			t.Fatalf("%d %s hit areas, want %d", got, id, n)
		}
	}
	// The rows come in the spec's order.
	order := []string{"theme", "font", "spacing", "cursor", "blink"}
	var seen []string
	for _, c := range win.set.controls {
		if len(seen) == 0 || seen[len(seen)-1] != c.id {
			seen = append(seen, c.id)
		}
	}
	if !slices.Equal(seen, order) {
		t.Fatalf("controls in order %v, want %v", seen, order)
	}
}

func TestThemeSwatchAppliesAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kaze", "config.toml")
	win := appearanceWindow(t, path)
	press(t, win, "theme", themeIndex(t, "tide"))
	if got, want := win.ui.th.Accent, (color.NRGBA{0x40, 0xe0, 0xd0, 0xff}); got != want {
		t.Fatalf("accent %v after Tide, want %v", got, want)
	}
	if win.opt.Config.Theme != "tide" {
		t.Fatalf("config theme %q", win.opt.Config.Theme)
	}
	tide, _ := theme.ByKey("tide")
	if win.pal.BG != tide.Term.BG {
		t.Fatalf("terminal background %v, want Tide's %v", win.pal.BG, tide.Term.BG)
	}
	if got := readFile(t, path); !strings.Contains(got, `theme = "tide"`) {
		t.Fatalf("saved file %q lacks the theme", got)
	}
	if win.notice != "" {
		t.Fatalf("notice %q after a good save", win.notice)
	}
}

// Review focus 3: a commented CRLF file with [colors] keeps everything, theme above the table.
func TestThemeSaveKeepsTheOwnersFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	const in = "# kaze settings\r\nfont_size = 14  # x\r\n\r\n# colors\r\n[colors]\r\nbg = \"#ffffff\"  # c\r\n"
	if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	win := appearanceWindow(t, path)
	win.opt.Config.Colors = map[string]string{"bg": "#ffffff"}
	press(t, win, "theme", themeIndex(t, "sumi"))
	want := "# kaze settings\r\nfont_size = 14  # x\r\ntheme = \"sumi\"\r\n\r\n# colors\r\n[colors]\r\nbg = \"#ffffff\"  # c\r\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("saved\n%q\nwant\n%q", got, want)
	}
	if win.pal.BG != (color.NRGBA{0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("[colors] bg lost to the theme: %v", win.pal.BG)
	}
}

func TestFontStepperClamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	win := appearanceWindow(t, path)
	win.opt.Config.FontSize = 31
	win.drawSettings(pageContext(800, 600), pageRect)
	press(t, win, "font", 32)
	if win.opt.Config.FontSize != 32 {
		t.Fatalf("font size %v after +, want 32", win.opt.Config.FontSize)
	}
	plus := controlsOf(win, "font")[1]
	if !plus.dead {
		t.Fatal("+ is live at 32")
	}
	win.settingsPointer(pressAt(center(plus.rect)))
	win.setFontSize(33)
	if win.opt.Config.FontSize != 32 {
		t.Fatalf("font size %v past the top, want 32", win.opt.Config.FontSize)
	}
	if got := readFile(t, path); !strings.Contains(got, "font_size = 32") {
		t.Fatalf("saved file %q, want font_size = 32", got)
	}

	// The file allows 4 to 72; the stepper shows that value and steps into 8 to 32.
	for _, c := range []struct {
		from     float32
		end, got int
	}{{40, 0, 32}, {4, 1, 8}} {
		win.cfgPath = ""
		win.opt.Config.FontSize = c.from
		win.drawSettings(pageContext(800, 600), pageRect)
		if !slices.Contains(win.set.texts, strconv.Itoa(int(c.from))) {
			t.Fatalf("stepper does not show %v: %q", c.from, win.set.texts)
		}
		end := controlsOf(win, "font")[c.end]
		pressAny(t, win, "font", end.arg)
		if win.opt.Config.FontSize != float32(c.got) {
			t.Fatalf("from %v: font size %v, want %d", c.from, win.opt.Config.FontSize, c.got)
		}
	}
}

func TestAppearanceChoicesApplyAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	win := appearanceWindow(t, path)
	fresh := func() {
		win.rend = render.NewRenderer(14, win.pal, rendOptions(win.opt.Config))
		win.rendStale = false
	}

	fresh()
	press(t, win, "spacing", 2)
	if win.opt.Config.LineSpacing != "roomy" || !win.rendStale {
		t.Fatalf("Roomy: spacing %q, renderer kept %v", win.opt.Config.LineSpacing, !win.rendStale)
	}
	fresh()
	press(t, win, "cursor", 1)
	if win.opt.Config.CursorStyle != "bar" || !win.rendStale {
		t.Fatalf("Bar: cursor %q, renderer kept %v", win.opt.Config.CursorStyle, !win.rendStale)
	}
	press(t, win, "blink", 1)
	if !win.opt.Config.CursorBlink {
		t.Fatal("the toggle did not turn blinking on")
	}
	got := readFile(t, path)
	for _, want := range []string{`line_spacing = "roomy"`, `cursor_style = "bar"`, "cursor_blink = true"} {
		if !strings.Contains(got, want) {
			t.Fatalf("saved file %q lacks %s", got, want)
		}
	}
	press(t, win, "blink", 0)
	if win.opt.Config.CursorBlink || !strings.Contains(readFile(t, path), "cursor_blink = false") {
		t.Fatal("the toggle did not turn blinking off and save it")
	}
}

// Review focus 2: a font size or line spacing change resizes every tab to the new grid.
func TestFontChangeResizesAllTabs(t *testing.T) {
	st := &recStarter{}
	win := testWindow(t, st)
	win.ui = newChrome()
	win.focused = true
	win.opt.Config = config.Default()
	win.opt.Config.Sidebar, win.custom = false, true // the full-width grid under the 42dp bar the sizes below assume
	for range 2 {
		if _, err := win.mgr.New(); err != nil {
			t.Fatal(err)
		}
	}
	var r input.Router
	at := time.Unix(1000, 0)
	frameWith(t, win, &r, at)
	grid := func() [2]int {
		cw, ch := win.rend.CellSize()
		cols, rows := gridSize(784, 518, cw, ch) // the grid inside an 800x600 window: 600 - 42 bar - 32 status - 8 padding
		return [2]int{cols, rows}
	}
	check := func(what string, before [2]int) [2]int {
		t.Helper()
		want := grid()
		if want == before {
			t.Fatalf("%s: grid stayed %v", what, want)
		}
		for i, p := range st.ptys {
			if got := p.lastSize(); got != want {
				t.Fatalf("%s: tab %d resized to %v, want %v", what, i+1, got, want)
			}
		}
		return want
	}
	start := grid()
	for i, p := range st.ptys {
		if got := p.lastSize(); got != start {
			t.Fatalf("tab %d at %v before any change, want %v", i+1, got, start)
		}
	}

	win.openSettings()
	frameWith(t, win, &r, at)
	press(t, win, "font", 15)
	frameWith(t, win, &r, at)
	after := check("font +1", start)

	press(t, win, "spacing", 2)
	frameWith(t, win, &r, at)
	check("Roomy", after)
}

func TestSaveErrorShowsNotice(t *testing.T) {
	dir := t.TempDir() // a directory where the file should be: every save fails
	win := appearanceWindow(t, dir)
	press(t, win, "theme", themeIndex(t, "tide"))
	if win.notice == "" || !strings.Contains(win.notice, "not saved") {
		t.Fatalf("notice %q after a failed save", win.notice)
	}
	if !win.noticeUntil.After(time.Now()) {
		t.Fatal("the notice is already expired")
	}
	if win.ui.th.Accent != (color.NRGBA{0x40, 0xe0, 0xd0, 0xff}) || win.opt.Config.Theme != "tide" {
		t.Fatal("the live theme change was undone by the failed save")
	}
}

// Ten themes wrap into rows of at most five, so each swatch stays wide enough
// to show its little terminal and name; none overlap.
func TestThemeSwatchesWrapInRowsOfFive(t *testing.T) {
	win := appearanceWindow(t, "")
	win.set.controls = win.set.controls[:0]
	win.drawSettings(pageContext(1000, 640), image.Rect(0, 38, 1000, 616))
	sw := controlsOf(win, "theme")
	rows := map[int]int{}
	for i, a := range sw {
		rows[a.rect.Min.Y]++
		if a.rect.Dx() != sw[0].rect.Dx() {
			t.Errorf("swatch %d is %d wide, swatch 0 is %d", i, a.rect.Dx(), sw[0].rect.Dx())
		}
		for _, b := range sw[i+1:] {
			if a.rect.Overlaps(b.rect) {
				t.Errorf("swatches overlap: %v %v", a.rect, b.rect)
			}
		}
	}
	for y, n := range rows {
		if n > 5 {
			t.Errorf("row at y=%d holds %d swatches, want at most 5", y, n)
		}
	}
	if len(rows) != 2 {
		t.Errorf("%d rows of swatches, want 2", len(rows))
	}
}

// In a narrow window the swatches wrap into fewer columns instead of shrinking
// until their names collide, and the row's hint wraps inside the card.
func TestThemeSwatchesKeepTheirWidthWhenNarrow(t *testing.T) {
	win := appearanceWindow(t, "")
	gtx := pageContext(560, 600)
	win.set.controls = win.set.controls[:0]
	win.set.texts = win.set.texts[:0]
	win.drawSettings(gtx, image.Rect(0, 38, 560, 576))
	sw := controlsOf(win, "theme")
	if len(sw) != 10 {
		t.Fatalf("%d swatches", len(sw))
	}
	for i, a := range sw {
		if a.rect.Dx() < gtx.Dp(swatchMinW) {
			t.Errorf("swatch %d is %d wide, want at least %d", i, a.rect.Dx(), gtx.Dp(swatchMinW))
		}
		if a.rect.Max.X > 560 {
			t.Errorf("swatch %d at %v leaves the window", i, a.rect)
		}
		for _, b := range sw[i+1:] {
			if a.rect.Overlaps(b.rect) {
				t.Errorf("swatches overlap: %v %v", a.rect, b.rect)
			}
		}
	}
	if slices.Contains(win.set.texts, "Sets the terminal colors and kaze's own chrome.") {
		t.Errorf("the hint is one line in a narrow window: %q", win.set.texts)
	}
}
