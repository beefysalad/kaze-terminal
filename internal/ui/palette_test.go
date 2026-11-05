package ui

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func palDraw(t *testing.T, c *chrome, items []palItem, main image.Rectangle, scale float32) palHits {
	t.Helper()
	return palDrawSt(t, c, palState{items: items, hover: -1}, main, scale)
}

// palDrawSt draws st (its editor holding st.query) over main at scale.
func palDrawSt(t *testing.T, c *chrome, st palState, main image.Rectangle, scale float32) palHits {
	t.Helper()
	gtx := pageContext(main.Max.X+40, main.Max.Y+40)
	gtx.Metric = unit.Metric{PxPerDp: scale, PxPerSp: scale}
	var ed widget.Editor
	ed.SingleLine = true
	ed.SetText(st.query)
	st.ed = &ed
	st.skills = palSkillScope(st.query)
	return c.layoutPalette(gtx, st, main)
}

func skillItems(n int) []palItem {
	var out []palItem
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("/skill-%03d", i)
		out = append(out, palItem{Kind: palSkill, Group: palHeadSkills, Key: "skill:" + name, Name: name, Detail: "does a thing", Prompt: name})
	}
	return out
}

func TestPaletteBoxIsCentered(t *testing.T) {
	c := newChrome()
	for _, scale := range []float32{1, 2} {
		main := image.Rect(260, 38, 1460, 838)
		if scale == 2 {
			main = image.Rect(520, 76, 2920, 1676)
		}
		h := palDraw(t, c, skillItems(3), main, scale)
		w := int(600 * scale)
		if h.box.Dx() != w {
			t.Errorf("scale %v: box %d wide, want %d", scale, h.box.Dx(), w)
		}
		if mid, want := (h.box.Min.X+h.box.Max.X)/2, (main.Min.X+main.Max.X)/2; mid < want-1 || mid > want+1 {
			t.Errorf("scale %v: box centred at %d, want %d", scale, mid, want)
		}
		if top := main.Min.Y + int(60*scale); h.box.Min.Y != top {
			t.Errorf("scale %v: box top %d, want %d", scale, h.box.Min.Y, top)
		}
		if !h.field.In(h.box) || !h.list.In(h.box) {
			t.Errorf("field %v or list %v outside the box %v", h.field, h.list, h.box)
		}
		if h.field.Dy() != int(54*scale) {
			t.Errorf("scale %v: field %d high, want %d", scale, h.field.Dy(), int(54*scale))
		}
	}
}

// Review Focus 5: a narrow box hides the switch; a wide one shows both parts
// right of the editor, and the chip never runs into them.
func TestPaletteNarrowHidesSwitch(t *testing.T) {
	c := newChrome()
	narrow := palDrawSt(t, c, palState{items: skillItems(3), hover: -1, query: "/"}, image.Rect(0, 38, 470, 700), 1)
	if narrow.box.Dx() >= 440 {
		t.Fatalf("narrow box %d wide", narrow.box.Dx())
	}
	if !narrow.switchSkills.Empty() || !narrow.switchSessions.Empty() {
		t.Fatalf("switch drawn in a %d wide box: %v %v", narrow.box.Dx(), narrow.switchSessions, narrow.switchSkills)
	}
	if narrow.chip.Empty() || !narrow.chip.In(narrow.field) {
		t.Fatalf("chip %v not in the field %v", narrow.chip, narrow.field)
	}
	main := image.Rect(260, 38, 1460, 838)
	for _, q := range []string{"", "/"} {
		h := palDrawSt(t, c, palState{items: skillItems(3), hover: -1, query: q}, main, 1)
		if h.box.Dx() != 600 {
			t.Fatalf("box %d wide", h.box.Dx())
		}
		for _, r := range []image.Rectangle{h.switchSessions, h.switchSkills} {
			if r.Empty() || !r.In(h.field) {
				t.Fatalf("%q: switch part %v not inside the field %v", q, r, h.field)
			}
			if r.Min.X < h.editor.Max.X {
				t.Fatalf("%q: switch part %v left of the editor's end %v", q, r, h.editor)
			}
		}
		if h.switchSessions.Max.X > h.switchSkills.Min.X {
			t.Fatalf("Sessions %v not left of Skills %v", h.switchSessions, h.switchSkills)
		}
		if q == "" {
			if !h.chip.Empty() {
				t.Fatalf("chip %v drawn in Sessions", h.chip)
			}
			continue
		}
		if h.chip.Empty() || h.chip.Max.X > h.editor.Min.X || h.chip.Overlaps(h.switchSessions) {
			t.Fatalf("chip %v, editor %v, switch %v", h.chip, h.editor, h.switchSessions)
		}
	}
}

func TestPaletteEmptyLines(t *testing.T) {
	c := newChrome()
	main := image.Rect(260, 38, 1460, 838)
	h := palDrawSt(t, c, palState{hover: -1, query: "zzz"}, main, 1)
	if h.headers != nil || len(h.rows) != 0 {
		t.Fatalf("headers %v, rows %d", h.headers, len(h.rows))
	}
	if want := []string{`No results for "zzz"`, "Press / to search skills instead."}; !slices.Equal(h.lines, want) {
		t.Fatalf("lines %q, want %q", h.lines, want)
	}
	h = palDrawSt(t, c, palState{hover: -1, query: "/zzz"}, main, 1)
	if want := []string{`No results for "zzz"`, "Try part of what the skill does, like /review."}; !slices.Equal(h.lines, want) {
		t.Fatalf("lines %q, want %q", h.lines, want)
	}
	h = palDrawSt(t, c, palState{items: skillItems(2), hover: -1, query: "s"}, main, 1)
	if h.lines != nil {
		t.Fatalf("lines %q with rows shown", h.lines)
	}
}

// spec 8.10 and section 4: text 4.5:1 on its surfaces, marks and tiles 3:1.
func TestPaletteInkReadableInEveryTheme(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		k := c.palInk()
		need := func(what string, fg, bg color.NRGBA, min float64) {
			t.Helper()
			if r := contrast(fg, bg); r < min {
				t.Errorf("%s: %s %.2f:1, want %.1f", th.Key, what, r, min)
			}
		}
		for _, s := range []struct {
			name string
			bg   color.NRGBA
		}{{"bg", k.bg}, {"sel", k.sel}} {
			need("text on "+s.name, k.text, s.bg, 4.5)
			need("muted on "+s.name, k.muted, s.bg, 4.5)
			need("faint on "+s.name, k.faint, s.bg, 4.5)
			need("go on "+s.name, k.goInk, s.bg, 4.5)
			need("badge on "+s.name, k.badge, s.bg, 4.5)
		}
		need("muted on foot", k.muted, k.foot, 4.5)
		need("muted on switch", k.muted, k.switchBG, 4.5)
		need("faint on switch", k.faint, k.switchBG, 4.5)
		need("action icon on raised", k.muted, k.raised, 3)
		need("text on raised", k.text, k.raised, 4.5)
		need("go on raised", k.goInk, k.raised, 4.5)
		need("chip", k.chipFG, k.chipBG, 4.5)
		need("keycap", k.keyFG, k.keyBG, 4.5)
		need("needs pill", k.pillNeedsFG, k.pillNeedsBG, 4.5)
		need("running pill", k.pillRunFG, k.pillRunBG, 4.5)
		need("idle pill", k.pillIdleFG, k.pillIdleBG, 4.5)
		need("done pill", k.pillDoneFG, k.pillDoneBG, 4.5)
		need("browse tile", k.tileFG, k.tileBG, 4.5)
		need("selection bar", k.selBar, k.sel, 3)
		for i := range k.plugins {
			need(fmt.Sprintf("plugin tile %d", i), k.plugins[i], k.bg, 3)
			need(fmt.Sprintf("plugin initial %d", i), k.pluginInk[i], k.plugins[i], 4.5)
		}
	}
}

func TestPalPluginColorStable(t *testing.T) {
	a := palPluginColor("devbox")
	if a != palPluginColor("devbox") || a < 0 || a >= 6 {
		t.Fatalf("devbox: %d", a)
	}
	if a != 3 || palPluginColor("superpowers") != 4 {
		t.Fatalf("devbox %d (want 3), superpowers %d (want 4)", a, palPluginColor("superpowers"))
	}
}

func TestPalInkCachedPerTheme(t *testing.T) {
	c := newChrome()
	a := c.palInk()
	if b := c.palInk(); a != b {
		t.Fatal("palInk rebuilt for the same theme")
	}
	c.setTheme(theme.All()[3])
	if b := c.palInk(); a == b || b.bg != theme.All()[3].UI.ToastBG {
		t.Fatal("palInk not rebuilt after setTheme")
	}
}

func TestPaletteNarrowMainView(t *testing.T) {
	main := image.Rect(100, 38, 400, 300)
	h := palDraw(t, newChrome(), skillItems(40), main, 1)
	if !h.box.In(main) {
		t.Fatalf("box %v outside the main view %v", h.box, main)
	}
	tiny := image.Rect(0, 0, 20, 20)
	palDraw(t, newChrome(), skillItems(3), tiny, 1) // no panic
}

func TestPaletteRowsAndHeaders(t *testing.T) {
	items := []palItem{
		{Kind: palSession, Group: palHeadSessions, Key: "tab:1", Name: "api", State: "running", Running: true},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:2", Name: "misc", State: "shell"},
	}
	items = append(items, skillItems(3)...)
	items = append(items, palItem{Kind: palSkill, Group: "DEVBOX", GroupCount: "1", Plugin: "devbox", Key: "skill:devbox:run", Name: "/devbox:run", Dim: len("/devbox:"), Detail: "Run it"})
	for _, scale := range []float32{1, 2} {
		main := image.Rect(260, 38, 1460, 838)
		if scale == 2 {
			main = image.Rect(520, 76, 2920, 1676)
		}
		h := palDraw(t, newChrome(), items, main, scale)
		if len(h.rows) != 6 {
			t.Fatalf("%d rows", len(h.rows))
		}
		if !slices.Equal(h.headers, []string{"SESSIONS", "SKILLS", "DEVBOX"}) {
			t.Fatalf("headers %v", h.headers)
		}
		inset := int(6 * scale)
		for i, r := range h.rows {
			if r.index != i || !r.rect.In(h.list) {
				t.Errorf("row %d: %+v outside %v", i, r, h.list)
			}
			if r.rect.Dy() != int(38*scale) || r.rect.Min.X != h.box.Min.X+inset || r.rect.Max.X != h.box.Max.X-inset {
				t.Errorf("scale %v row %d: %v in box %v, want 38 dp tall, inset 6 dp", scale, i, r.rect, h.box)
			}
		}
	}
}

// The selected session row alone says "go to session".
func TestSelectedRowShowsGo(t *testing.T) {
	items := []palItem{{Kind: palSession, Group: palHeadSessions, Key: "tab:1", Name: "api", State: "idle", Agent: "claude"}}
	main := image.Rect(260, 38, 1460, 838)
	if h := palDrawSt(t, newChrome(), palState{items: items, hover: -1, sel: 0}, main, 1); h.goShown != 1 {
		t.Fatalf("selected: goShown %d", h.goShown)
	}
	if h := palDrawSt(t, newChrome(), palState{items: items, hover: -1, sel: -1}, main, 1); h.goShown != 0 {
		t.Fatalf("not selected: goShown %d", h.goShown)
	}
}

// A plugin group's header gets its tile; Yours has none.
func TestPluginHeaderHasTile(t *testing.T) {
	main := image.Rect(260, 38, 1460, 838)
	yours := []palItem{{Kind: palSkill, Group: "YOURS", GroupCount: "1", Key: "skill:a", Name: "/a"}}
	if h := palDraw(t, newChrome(), yours, main, 1); h.tiles != 0 {
		t.Fatalf("YOURS drew %d tiles", h.tiles)
	}
	devbox := []palItem{
		{Kind: palSkill, Group: "DEVBOX", GroupCount: "2", Plugin: "devbox", Key: "skill:devbox:a", Name: "/devbox:a", Dim: 8},
		{Kind: palMoreGroup, Group: "DEVBOX", GroupCount: "2", Plugin: "devbox", Key: "more:devbox", Name: "1 more in devbox", Count: 1},
	}
	if h := palDraw(t, newChrome(), devbox, main, 1); h.tiles != 1 {
		t.Fatalf("DEVBOX drew %d tiles, want 1", h.tiles)
	}
}

func TestPaletteListScrolls(t *testing.T) {
	c := newChrome()
	main := image.Rect(260, 38, 1460, 838)
	h := palDraw(t, c, skillItems(200), main, 1)
	if h.scrollMax <= 0 || h.list.Dy() > 420 {
		t.Fatalf("scrollMax %d, list %d high", h.scrollMax, h.list.Dy())
	}
	if len(h.rows) > 420/38+2 {
		t.Fatalf("%d rows laid out for a 420dp list", len(h.rows))
	}
	gtx := pageContext(1500, 900)
	var ed widget.Editor
	h = c.layoutPalette(gtx, palState{items: skillItems(200), ed: &ed, hover: -1, scroll: 1 << 20}, main)
	if len(h.rows) == 0 || h.rows[len(h.rows)-1].index != 199 {
		t.Fatalf("scrolled to the end: last row %+v", h.rows)
	}
	for _, r := range h.rows {
		if r.rect.Max.Y <= h.list.Min.Y || r.rect.Min.Y >= h.list.Max.Y {
			t.Fatalf("row %d laid out outside the list", r.index)
		}
	}
}

func TestPaletteLongTextStaysInside(t *testing.T) {
	items := []palItem{
		{Kind: palSkill, Group: palHeadSkills, Key: "skill:x", Name: "/" + strings.Repeat("長い名前", 75), Detail: strings.Repeat("d", 200)},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:1", Name: strings.Repeat("title", 60), State: "needs you", Mark: markNeeds, Agent: "claude"},
		{Kind: palKaze, Group: palHeadKaze, Key: "act:x", Name: strings.Repeat("Act", 40), Icon: "plus", Keys: strings.Split(strings.Repeat("Ctrl+", 11)+"T", "+")},
	}
	for _, main := range []image.Rectangle{image.Rect(0, 38, 700, 700), image.Rect(0, 38, 330, 700)} {
		for sel := range items {
			h := palDrawSt(t, newChrome(), palState{items: items, hover: -1, sel: sel}, main, 1)
			for _, r := range h.rows {
				if !r.rect.In(h.box) {
					t.Fatalf("row %v outside the box %v", r.rect, h.box)
				}
			}
			if len(h.badges) == 0 {
				t.Fatal("no badges recorded")
			}
			for _, b := range h.badges {
				in := false
				for _, r := range h.rows {
					in = in || b.In(r.rect)
				}
				if !in {
					t.Fatalf("badge %v outside every row %v", b, h.rows)
				}
			}
		}
	}
}

func TestPaletteDrawsInEveryTheme(t *testing.T) {
	win, _ := alertWindow(t, 1)
	items := []palItem{
		{Kind: palSession, Group: palHeadWaiting, Key: "tab:1", Name: "a", Mark: markNeeds, State: "needs you", Agent: "claude"},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:2", Name: "web", State: "idle", Agent: "codex"},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:3", Name: "run", State: "running", Running: true},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:4", Name: "fin", State: "done", Mark: markDone},
		{Kind: palSession, Group: palHeadSessions, Key: "tab:5", Name: "sh", State: "shell"},
		{Kind: palKaze, Group: palHeadKaze, Key: "act:new_tab", Name: "New session", Keys: []string{"Ctrl", "Shift", "T"}, Icon: "plus"},
		{Kind: palKaze, Group: palHeadKaze, Key: "act:x", Name: "No icon"},
		{Kind: palBrowse, Group: palHeadRecentSkills, Key: "browse", Name: "Browse all 2 skills", Count: 2},
		{Kind: palSkill, Group: "DEVBOX", GroupCount: "3", Plugin: "devbox", Key: "skill:devbox:a", Name: "/devbox:a", Dim: 8, Detail: "does a"},
		{Kind: palMoreGroup, Group: "DEVBOX", GroupCount: "3", Plugin: "devbox", Key: "more:devbox", Name: "2 more in devbox", Count: 2},
		{Kind: palMoreMatches, Group: palHeadSkills, Key: "more:matches", Name: "2 more matching skills", Count: 2, Query: "/x"},
	}
	items = append(items, skillItems(2)...)
	for _, th := range theme.All() {
		win.applyTheme(th)
		for sel := range items {
			h := palDrawSt(t, win.ui, palState{items: items, hover: -1, sel: sel, reveal: true, footSkill: items[sel].Kind == palSkill}, image.Rect(260, 38, 1460, 1238), 1)
			if !slices.ContainsFunc(h.rows, func(r palRowHit) bool { return r.index == sel }) {
				t.Fatalf("%s: selected row %d not drawn", th.Key, sel)
			}
			for _, r := range h.rows {
				if !r.rect.In(h.box) {
					t.Fatalf("%s: row %v outside the box %v", th.Key, r.rect, h.box)
				}
			}
		}
	}
}

// spec 8.8, Ruling 10: the switch parts set the scope they name, the chip
// leaves Skills; the palette stays open with the keyboard in the field.
func TestSwitchAndChipClicks(t *testing.T) {
	win, _, r, _ := palWindow(t)
	openPal(t, win, r)
	text := func(want string) {
		t.Helper()
		if !win.palette.open {
			t.Fatal("the palette closed")
		}
		if got := win.palette.ed.Text(); got != want {
			t.Fatalf("text %q, want %q", got, want)
		}
	}
	if win.palette.hits.switchSkills.Empty() {
		t.Fatalf("no switch in a %d wide box", win.palette.hits.box.Dx())
	}
	clickAt(t, win, r, centre(win.palette.hits.switchSkills))
	text("/")
	if win.palette.hits.chip.Empty() {
		t.Fatal("no chip in Skills")
	}
	clickAt(t, win, r, centre(win.palette.hits.chip))
	text("")
	palType(t, win, r, "/rev")
	clickAt(t, win, r, centre(win.palette.hits.switchSessions))
	text("rev")
	clickAt(t, win, r, centre(win.palette.hits.switchSessions)) // already there: no change
	text("rev")

	gtx := pageContext(1000, 600)
	win.wantFocus = false
	win.palettePointer(gtx, pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(centre(win.palette.hits.switchSkills).X), float32(centre(win.palette.hits.switchSkills).Y))})
	text("/rev")
	if !win.wantFocus {
		t.Fatal("a click on the switch did not keep the keyboard in the field")
	}
}
