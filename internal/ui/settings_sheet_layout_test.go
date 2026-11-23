package ui

import (
	"fmt"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func sheetWindow(t *testing.T, themeKey string) *window {
	t.Helper()
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/kaze"))
	win.applyKeys(nil)
	root := t.TempDir()
	win.cfgPath = filepath.Join(root, "config.toml")
	win.claudePath = filepath.Join(root, "claude", "settings.json")
	win.codexDir = filepath.Join(root, "codex")
	win.exePath = filepath.Join(root, "kaze")
	th, _ := theme.ByKey(themeKey)
	win.applyTheme(th)
	win.opt.Config.CursorBlink = false
	win.upd.checked = true
	win.upd.checkedAt = time.Now()
	win.openSettings()
	return win
}

func TestSettingsSheetSectionsFit(t *testing.T) {
	for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
		for section, sec := range defaultSections() {
			t.Run(fmt.Sprintf("%dx%d/%s", size.X, size.Y, sec.title), func(t *testing.T) {
				win := sheetWindow(t, "midnight")
				var r input.Router
				win.selectSection(section)
				sizedFrame(t, win, &r, size.X, size.Y)
				for _, c := range win.set.controls {
					if c.rect.Min.X < win.set.pane.Min.X || c.rect.Max.X > win.set.pane.Max.X {
						t.Errorf("%s control %v outside pane %v", c.id, c.rect, win.set.pane)
					}
				}
				if n := len(win.set.controls); n > 0 {
					win.set.focus = n - 1
					sizedFrame(t, win, &r, size.X, size.Y)
					if got := win.set.controls[n-1].rect; got.Min.Y < win.set.pane.Min.Y || got.Max.Y > win.set.pane.Max.Y {
						t.Errorf("last control %v cannot be reached in %v", got, win.set.pane)
					}
				}
			})
		}
	}
}

// Fixed work bounds keep this probe safe even before any allocation fix.
func TestSettingsSheetFrameCost(t *testing.T) {
	win := sheetWindow(t, "midnight")
	var ops op.Ops
	g := layout.Context{Ops: &ops, Now: time.Unix(1000, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1200, 800))}
	frame := func() {
		ops.Reset()
		if err := win.frame(g); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		frame()
	}
	start := time.Now()
	for range 100 {
		frame()
	}
	elapsed := time.Since(start) / 100
	allocs := testing.AllocsPerRun(20, frame)
	bounds := image.Rect(0, 0, 1200, 800)
	draw := func(sheet bool) {
		ops.Reset()
		if sheet {
			win.drawSettingsSheet(g, bounds)
		} else {
			win.drawSettings(g, sheetRect(bounds, g.Metric))
		}
	}
	for range 5 {
		draw(false)
		draw(true)
	}
	page := testing.AllocsPerRun(20, func() { draw(false) })
	sheet := testing.AllocsPerRun(20, func() { draw(true) })
	if page > 60 {
		t.Errorf("wide page allocations %.0f exceed pre-wrap budget 60", page)
	}
	if sheet > page {
		t.Errorf("overlay adds %.0f allocations", sheet-page)
	}
	t.Logf("1200x800 unchanged frame %s, %.0f allocs; page %.0f, sheet %.0f, overlay delta %.0f", elapsed, allocs, page, sheet, sheet-page)
}

func TestSheetHeadingWrapsHint(t *testing.T) {
	win := sheetWindow(t, "washi")
	g := pageContext(560, 640)
	p := &page{gtx: g, c: win.ui, win: win, st: &win.set, col: win.ui.pageColors()}
	p.heading(image.Rect(256, 30, 524, 640), "Agents", claudeSection().hint)
	for _, text := range win.set.texts {
		_, sz := win.ui.label(g, text, 13, p.col.muted)
		if sz.X > 268 {
			t.Errorf("heading line %q is %dpx wide, room 268", text, sz.X)
		}
	}
}

func TestSheetRowWrapsCopy(t *testing.T) {
	win := sheetWindow(t, "washi")
	g := pageContext(560, 640)
	p := &page{gtx: g, c: win.ui, win: win, st: &win.set, col: win.ui.pageColors()}
	p.row(image.Rect(0, 0, 232, 600), 0, settingRow{label: "Run a workspace when kaze starts", hint: "There are no saved workspaces yet.", ctrls: []ctrl{{kind: ctlButton, id: "none", text: "None"}}})
	for _, text := range win.set.texts {
		_, sz := win.ui.label(g, text, 12, p.col.title)
		if sz.X > 232 {
			t.Errorf("row line %q is %dpx wide, room 232", text, sz.X)
		}
	}
}

func TestSheetShortcutSearchClearsHeading(t *testing.T) {
	win := sheetWindow(t, "midnight")
	for i, s := range win.sections() {
		if s.title == "Shortcuts" {
			win.selectSection(i)
		}
	}
	var r input.Router
	sizedFrame(t, win, &r, 560, 640)
	cs := controlsOf(win, "cmd_filter")
	if len(cs) != 1 {
		t.Fatal("missing search")
	}
	if cs[0].rect.Min.Y < win.set.pane.Min.Y+60 {
		t.Fatalf("search %v overlaps heading", cs[0].rect)
	}
}

func TestSheetUpdateHeroDoesNotOverlap(t *testing.T) {
	for _, width := range []int{560, 1200} {
		for _, state := range []string{"current", "newer", "error", "progress", "confirm"} {
			t.Run(fmt.Sprintf("%d/%s", width, state), func(t *testing.T) {
				win := sheetWindow(t, "midnight")
				for i, s := range win.sections() {
					if s.title == "Updates" {
						win.selectSection(i)
					}
				}
				switch state {
				case "newer", "confirm":
					win.upd.newer = true
					win.upd.rel.Version = "0.44"
					win.upd.confirm = state == "confirm"
				case "error":
					win.upd.err = "A very long server error while checking for updates"
				case "progress":
					win.upd.busy = "Downloading 100%"
					win.upd.pct = 100
				}
				var r input.Router
				sizedFrame(t, win, &r, width, 800)
				g := win.set.updateHero
				if g.text.Overlaps(g.buttons) || g.text.Min.X < win.set.pane.Min.X || g.text.Max.X > win.set.pane.Max.X {
					t.Errorf("hero text %v overlaps buttons %v or exceeds %v", g.text, g.buttons, win.set.pane)
				}
				for _, c := range win.set.controls {
					if c.rect.Min.X < win.set.pane.Min.X+36 || c.rect.Max.X > win.set.pane.Max.X-36 {
						t.Errorf("update control %s exceeds pane: %v", c.id, c.rect)
					}
				}
			})
		}
	}
}

func TestSheetShortcutRowsKeepColumns(t *testing.T) {
	win := sheetWindow(t, "midnight")
	for i, s := range win.sections() {
		if s.title == "Shortcuts" {
			win.selectSection(i)
		}
	}
	var r input.Router
	sizedFrame(t, win, &r, 560, 640)
	for _, row := range win.set.keyRows {
		if row.action == "" {
			continue
		}
		_, sz := win.ui.label(pageContext(560, 640), row.action, 11.5, win.ui.pageColors().fg)
		name := image.Rectangle{Min: row.name, Max: row.name.Add(sz)}
		if name.Overlaps(row.keys) {
			t.Errorf("%s name %v overlaps keys %v", row.action, name, row.keys)
		}
	}
}

func TestSheetWorkspaceEditorKeepsFocus(t *testing.T) {
	for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
		win := sheetWindow(t, "midnight")
		win.selectSection(wsIndex(t))
		win.activate("ws.new", 0)
		var r input.Router
		for range 3 {
			sizedFrame(t, win, &r, size.X, size.Y)
		}
		for range 32 {
			for _, c := range win.set.controls {
				if c.rect.Min.X < win.set.pane.Min.X+36 || c.rect.Max.X > win.set.pane.Max.X-36 {
					t.Fatalf("workspace control %s %v outside %v", c.id, c.rect, win.set.pane)
				}
			}
			if f := win.set.fields[win.set.editing]; f != nil {
				if !r.Source().Focused(&f.ed) {
					t.Fatalf("editor %s lost focus", win.set.editing)
				}
				for _, c := range win.set.controls {
					if c.id == win.set.editing && !c.rect.In(win.set.pane) {
						t.Fatalf("editor %s not revealed: %v in %v", c.id, c.rect, win.set.pane)
					}
				}
			}
			r.Queue(key.Event{Name: key.NameTab, State: key.Press})
			for range 2 {
				sizedFrame(t, win, &r, size.X, size.Y)
			}
		}
	}
}

func TestSheetNoIdleWakeups(t *testing.T) {
	win := sheetWindow(t, "midnight")
	var r input.Router
	for range 8 {
		sizedFrame(t, win, &r, 1200, 800)
		r.WakeupTime()
	}
	sizedFrame(t, win, &r, 1200, 800)
	if at, ok := r.WakeupTime(); ok {
		t.Fatalf("unchanged sheet asks for another frame at %v", at)
	}
}

func TestSheetWrapsLongWords(t *testing.T) {
	win := sheetWindow(t, "midnight")
	g := pageContext(560, 640)
	p := &page{gtx: g, c: win.ui, win: win, st: &win.set, col: win.ui.pageColors()}
	word := strings.Repeat("long-path-", 12)
	lines := p.wrap(word, 12, 200)
	for _, line := range lines {
		_, sz := win.ui.label(g, line, 12, p.col.fg)
		if sz.X > 200 {
			t.Fatalf("word line is %dpx wide, room 200", sz.X)
		}
	}
	if strings.Join(lines, "") != word {
		t.Fatal("wrapping lost text")
	}
}

func TestSheetSavedWorkspaceFits(t *testing.T) {
	for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
		f := newWSPage(t)
		f.win.drawSettingsSheet(pageContext(size.X, size.Y), image.Rectangle{Max: size})
		if !slices.Contains(f.win.set.texts, "samurai") {
			t.Errorf("saved workspace name is unreadable at %v", size)
		}
		for _, c := range f.win.set.controls {
			if c.rect.Min.X < f.win.set.pane.Min.X+36 || c.rect.Max.X > f.win.set.pane.Max.X-36 {
				t.Errorf("saved workspace %s %v exceeds content %v", c.id, c.rect, f.win.set.pane)
			}
		}
	}
}

func TestSheetSavedWorkspaceKeepsMemberDetails(t *testing.T) {
	f := newWSPage(t)
	f.win.set.texts = nil
	g := pageContext(560, 640)
	p := &page{gtx: g, c: f.win.ui, win: f.win, st: &f.win.set, col: f.win.ui.pageColors()}
	p.memberRow(image.Rect(0, 0, 232, 640), 0, f.win.listRows()[0])
	for _, text := range []string{"samurai", "api", "admin", "storefront", "api logs"} {
		if !slices.Contains(f.win.set.texts, text) {
			t.Errorf("member list lost %q: %q", text, f.win.set.texts)
		}
	}
}
