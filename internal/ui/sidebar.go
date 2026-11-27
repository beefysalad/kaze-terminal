package ui

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// sidebarW is the sidebar's width in dp at 100% scale.
const sidebarW = 260

// The sidebar's line heights in dp: a space, the SPACES and AGENTS headers, an agent
// group's label and an agent row.
const sideSpaceH, sideHeadH, sideAgentLabelH, sideAgentRowH = 34, 36, 22, 30

// sideSpaceGap is the gap in dp between space lines.
const sideSpaceGap = 2

// sideIconBtn is a header icon button's side in dp.
const sideIconBtn = 28

// maxSideText bounds how much of one title is shaped; the rest never fits 260dp anyway.
const maxSideText = 160

// sideState is what the window knows that the model does not.
type sideState struct {
	trayFolded    map[tabs.TabID]bool
	openAgent     subagents.Key
	activeRepo    string // common repository of the selected main session
	doc, hoverDoc docID
	shown         tabs.TabID      // highlighted row or member
	space         string          // the key of the selected space ("" for none)
	hoverSpace    string          // the space line under the pointer
	hoverBtn      sideBtn         // the header icon under the pointer
	settings      bool            // the pinned "Settings" row is open
	settingsOn    bool            // and shown
	hover         tabs.TabID      // agent row or rail tile under the pointer
	scroll        int             // px the list is scrolled
	filter        string          // as typed, for the placeholder/clear state
	filterOpen    bool            // the filter field is drawn: it has the keyboard or text
	ed            *widget.Editor  // the filter's editor; nil draws the text without a caret
	static        bool            // editors are drawn live over the retained chrome
	runFolded     map[uint64]bool // workspace runs whose header is folded
	docked        tabs.TabID      // the member tab the shown panel draws, highlighted like shown
	right         bool            // the sidebar sits at the window's right edge: its divider and accent mirror
	tipRoom       int             // px of window beside the rail on the terminal's side, for its tooltip; 0 for no limit
}

// sideBtn is one of the SPACES header's icon buttons.
type sideBtn uint8

const (
	btnNone sideBtn = iota
	btnSearch
	btnPlus
)

type sideRowHit struct {
	id          tabs.TabID
	rect, close image.Rectangle
	mark        mark // the alert dot a member line drew
	on          bool // a rail tile drawn highlighted
}

type sideKeyHit struct {
	key  string
	rect image.Rectangle
}

type sideRunHit struct {
	run  uint64
	rect image.Rectangle
	mark mark // the alert dot a folded header drew
}

// sideHits is where the last sidebar frame drew things, for clicks and the wheel.
type sideAgentHit struct {
	key  subagents.Key
	tab  tabs.TabID
	rect image.Rectangle
	more bool
}

type sideHits struct {
	agents []sideAgentHit
	folds  []sideRowHit

	docs                    []sideDocHit
	rows                    []sideRowHit // agent rows, or the rail's tiles
	spaces                  []sideKeyHit // space lines, by space key
	members                 []sideRowHit
	runs                    []sideRunHit
	settings, settingsClose image.Rectangle
	filter                  image.Rectangle // the filter field; empty while it is not drawn
	search, plus            image.Rectangle // the SPACES header's icon buttons
	list                    image.Rectangle // the scrollable area, for wheel events
	tip                     image.Rectangle // the rail's tile tooltip, empty when none shows
	tipCall                 op.CallOp       // the rail's tooltip, drawn over the terminal by drawSideTip
	scrollMax               int
	noMatches               bool // the filter hides every session
}

// fitSide draws s at size within room px, cut with a trailing "…" when it is longer.
func (c *chrome) fitSide(gtx layout.Context, s string, size unit.Sp, room int, col color.NRGBA) (op.CallOp, image.Point) {
	return c.fitSideWeight(gtx, s, size, room, col, font.Normal)
}

// fitSideWeight is fitSide in the chrome font at weight w.
func (c *chrome) fitSideWeight(gtx layout.Context, s string, size unit.Sp, room int, col color.NRGBA, w font.Weight) (op.CallOp, image.Point) {
	call, sz, _ := c.fitSideText(gtx, s, size, room, col, w)
	return call, sz
}

// fitSideText is fitSideWeight that also returns the text it drew.
func (c *chrome) fitSideText(gtx layout.Context, s string, size unit.Sp, room int, col color.NRGBA, w font.Weight) (op.CallOp, image.Point, string) {
	s = cutRunes(c.fit(s), maxSideText)
	call, sz := c.labelW(gtx, s, size, col, w)
	for i := 0; i < 6 && sz.X > room && s != ""; i++ {
		n := utf8.RuneCountInString(s)
		keep := min(n*max(room, 0)/max(sz.X, 1), n-1)
		if keep < 2 {
			s = ""
		} else {
			s = cutRunes(s, keep)
		}
		call, sz = c.labelW(gtx, s, size, col, w)
	}
	if sz.X > room {
		call, sz = c.labelW(gtx, "", size, col, w)
		sz.X, s = 0, ""
	}
	return call, sz, s
}

// sideDot draws a state mark: an amber dot with a soft halo for needs you, green for
// done, an accent ring for running, muted otherwise.
func (c *chrome) sideDot(gtx layout.Context, d image.Rectangle, mk mark, running bool) {
	c.stateDot(gtx, d, mk, running, true)
}

// stateDot is sideDot; glow false leaves out the halo of a session that needs you.
func (c *chrome) stateDot(gtx layout.Context, d image.Rectangle, mk mark, running, glow bool) {
	ops := gtx.Ops
	if running && mk == markNone {
		c.runningMark(gtx, centeredMarkBox(d, gtx.Dp(14)))
		return
	}
	col, ok := c.dotColor(false, mk)
	if !ok {
		col = c.th.Muted
	}
	if mk == markNeeds && glow {
		h := gtx.Dp(3)
		halo := col
		halo.A = 0x40
		paint.FillShape(ops, halo, clip.Ellipse(image.Rect(d.Min.X-h, d.Min.Y-h, d.Max.X+h, d.Max.Y+h)).Op(ops))
	}
	paint.FillShape(ops, col, clip.Ellipse(d).Op(ops))
}

// centeredMarkBox gives the running symbol a bounded slot around the existing
// idle-dot center, keeping neighboring text and all non-running dots in place.
func centeredMarkBox(dot image.Rectangle, size int) image.Rectangle {
	at := dot.Min.Sub(image.Pt((size-dot.Dx())/2, (size-dot.Dy())/2))
	return image.Rectangle{Min: at, Max: at.Add(image.Pt(size, size))}
}

// runningMark is the approved still closed ring with a bright core. At 14px,
// the ring radius is 5.6px, its stroke 1.3px and the core radius 2.6px. All
// geometry stays inside slot; painting the still mark never requests a frame.
func (c *chrome) runningMark(gtx layout.Context, slot image.Rectangle) {
	if slot.Empty() {
		return
	}
	ops := gtx.Ops
	size := float32(min(slot.Dx(), slot.Dy()))
	cx, cy := float32(slot.Min.X)+float32(slot.Dx())/2, float32(slot.Min.Y)+float32(slot.Dy())/2
	// Fractional paths retain the same proportions at 1x and 2x, including
	// the smaller mark in the agents pill.
	circle := func(radius float32) clip.PathSpec {
		var p clip.Path
		p.Begin(ops)
		const k = float32(0.5522847498)
		p.MoveTo(f32.Pt(cx+radius, cy))
		p.CubeTo(f32.Pt(cx+radius, cy+k*radius), f32.Pt(cx+k*radius, cy+radius), f32.Pt(cx, cy+radius))
		p.CubeTo(f32.Pt(cx-k*radius, cy+radius), f32.Pt(cx-radius, cy+k*radius), f32.Pt(cx-radius, cy))
		p.CubeTo(f32.Pt(cx-radius, cy-k*radius), f32.Pt(cx-k*radius, cy-radius), f32.Pt(cx, cy-radius))
		p.CubeTo(f32.Pt(cx+k*radius, cy-radius), f32.Pt(cx+radius, cy-k*radius), f32.Pt(cx+radius, cy))
		p.Close()
		return p.End()
	}
	ring := c.th.Accent
	ring.A = 140 // approximately 55%, as approved
	paint.FillShape(ops, ring, clip.Stroke{Path: circle(size * 5.6 / 14), Width: size * 1.3 / 14}.Op())
	core := c.th.ToastGo
	if core == c.th.Accent {
		core = mix(c.th.Accent, c.th.ActiveFG, 0.2)
	}
	if !c.animate {
		paint.FillShape(ops, core, clip.Outline{Path: circle(size * 2.6 / 14)}.Op())
		return
	}
	// Animated (Settings, Appearance, Animated status): a bright arc runs around the ring
	// and the core breathes, once per statusAnimPeriod. The window wakes for each phase.
	c.animDrawn = true
	phase := float64(gtx.Now.UnixNano()%int64(statusAnimPeriod)) / float64(statusAnimPeriod)
	radius := size * 5.6 / 14
	start := phase*2*math.Pi - math.Pi/2
	var arc clip.Path
	arc.Begin(ops)
	arc.MoveTo(f32.Pt(cx+radius*float32(math.Cos(start)), cy+radius*float32(math.Sin(start))))
	arc.ArcTo(f32.Pt(cx, cy), f32.Pt(cx, cy), 1.9)
	paint.FillShape(ops, c.th.Accent, clip.Stroke{Path: arc.End(), Width: size * 1.6 / 14}.Op())
	core.A = uint8(float64(core.A) * (0.65 + 0.35*(0.5+0.5*math.Sin(phase*2*math.Pi-math.Pi/2))))
	paint.FillShape(ops, core, clip.Outline{Path: circle(size * 2.6 / 14)}.Op())
}

// lineEditor draws a sidebar text field's editor in r, gap in from its left, and ph
// while it is empty.
func (c *chrome) lineEditor(gtx layout.Context, ed *widget.Editor, r image.Rectangle, ph string, phCol color.NRGBA, gap int) {
	ops, th := gtx.Ops, c.sideTheme()
	_, line := c.label(gtx, "Ag", 12, th.ActiveFG)
	at := image.Pt(r.Min.X+gap, r.Min.Y+(r.Dy()-line.Y)/2)
	if ed.Len() == 0 && ph != "" {
		call, _ := c.fitSide(gtx, ph, 12, r.Dx()-2*gap, phCol)
		place(ops, call, at)
	}
	rec := op.Record(ops)
	paint.ColorOp{Color: th.ActiveFG}.Add(ops)
	fg := rec.Stop()
	rec = op.Record(ops)
	paint.ColorOp{Color: tint(th.Accent, 0x55)}.Add(ops)
	sel := rec.Stop()
	eg := gtx
	eg.Constraints = layout.Exact(image.Pt(max(r.Dx()-2*gap, 1), line.Y))
	t := op.Offset(at).Push(ops)
	ed.Layout(eg, c.shaper, font.Font{Typeface: chromeFace}, 12, fg, sel)
	t.Pop()
}

func fillGlyph(ops *op.Ops, col color.NRGBA, r image.Rectangle, glyph clip.Op) {
	cl := clip.Rect(r).Push(ops)
	paint.FillShape(ops, col, glyph)
	cl.Pop()
}

// sideRuns draws the workspaces section from secTop to the bottom: a header, then per
// run its name with all up / N ready / N down and one line per member, with its alert dot and,
// while hovered, its ×.
func (c *chrome) sideRuns(gtx layout.Context, h *sideHits, m sideModel, st sideState, secTop int, size image.Point, closeCall op.CallOp, closeSize image.Point) {
	ops, th := gtx.Ops, c.sideTheme()
	pad, gap, hitPad := gtx.Dp(8), gtx.Dp(8), gtx.Dp(4)
	hdH, memH := gtx.Dp(26), gtx.Dp(24)
	left, right := 0, size.X-1 // off the 1 px divider, whichever edge it is on
	if st.right {
		left, right = 1, size.X
	}
	area := image.Rect(left, secTop, right, size.Y)
	paint.FillShape(ops, th.Chrome, clip.Rect(area).Op())
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(left, secTop, right, secTop+1)).Op())
	cl := clip.Rect(area).Push(ops)
	defer cl.Pop()

	y := secTop
	call, sz := c.label(gtx, "WORKSPACES", 10, th.Muted)
	place(ops, call, image.Pt(pad+gap, y+(hdH-sz.Y)/2))
	y += hdH
	whole := func(r image.Rectangle) bool { return r.Max.Y <= size.Y }

	remaining := 0 // members still to place, so a capped section can say how many are left
	for _, run := range m.Runs {
		if !st.runFolded[run.Run] {
			remaining += len(run.Members)
		}
	}
	hidden, moreY := 0, 0
	for _, run := range m.Runs {
		folded := st.runFolded[run.Run]
		hd := image.Rect(pad, y, right-pad, y+hdH)
		y += hdH
		if whole(hd) {
			state, scol := runHeaderState(run, th)
			sCall, sSize := c.label(gtx, state, 11, scol)
			sx := hd.Max.X - gap - sSize.X
			place(ops, sCall, image.Pt(sx, hd.Min.Y+(hdH-sSize.Y)/2))
			hit := sideRunHit{run: run.Run, rect: hd}
			if folded && run.Mark != markNone {
				// Folded, the members' lines are gone, so the header carries their strongest mark.
				sx = c.alertDot(gtx, sx, hd.Min.Y+hdH/2, run.Mark)
				hit.mark = run.Mark
			}
			chev := "▾"
			if folded {
				chev = "›"
			}
			cCall, cSize := c.label(gtx, chev, 10, th.Muted)
			place(ops, cCall, image.Pt(hd.Min.X+gap, hd.Min.Y+(hdH-cSize.Y)/2))
			nx := hd.Min.X + gap + cSize.X + gap
			nCall, nSize := c.fitSide(gtx, run.Name, 12, sx-gap-nx, th.ActiveFG)
			place(ops, nCall, image.Pt(nx, hd.Min.Y+(hdH-nSize.Y)/2))
			h.runs = append(h.runs, hit)
		}
		if folded {
			continue
		}
		for _, mem := range run.Members {
			r := image.Rect(pad, y, right-pad, y+memH)
			y += memH
			remaining--
			// Keep the last slot for "+N more" while members remain after this one.
			limit := size.Y
			if remaining > 0 {
				limit -= memH
			}
			if r.Max.Y > limit {
				if hidden == 0 {
					moreY = r.Min.Y
				}
				hidden++
				continue
			}
			on := mem.ID != 0 && (mem.ID == st.shown || mem.ID == st.docked) && !st.settingsOn
			if on {
				paint.FillShape(ops, th.Active, clip.UniformRRect(r, gtx.Dp(6)).Op(ops))
			}
			bar := image.Rect(r.Min.X+gap, r.Min.Y+gtx.Dp(5), r.Min.X+gap+gtx.Dp(3), r.Max.Y-gtx.Dp(5))
			paint.FillShape(ops, c.memberColor(mem.Color), clip.UniformRRect(bar, gtx.Dp(1)).Op(ops))
			scol := th.Muted
			if mem.Down {
				scol = th.UsageHigh
			} else if mem.Mark == markNeeds {
				scol = th.Needs
			}
			hit := sideRowHit{id: mem.ID, rect: r}
			end, mid := r.Max.X-gap, r.Min.Y+memH/2
			if st.hover == mem.ID && mem.ID != 0 {
				cx := end - closeSize.X
				place(ops, closeCall, image.Pt(cx, mid-closeSize.Y/2))
				hit.close = image.Rect(cx-hitPad, r.Min.Y, r.Max.X, r.Max.Y)
				end = cx - gap
			}
			sCall, sSize := c.label(gtx, mem.Status, 11, scol)
			sx := end - sSize.X
			place(ops, sCall, image.Pt(sx, mid-sSize.Y/2))
			if mem.Mark != markNone {
				sx = c.alertDot(gtx, sx, mid, mem.Mark)
				hit.mark = mem.Mark
			}
			tx := bar.Max.X + gap
			fg := th.Folder
			if on {
				fg = th.ActiveFG
			}
			tCall, tSize := c.fitSide(gtx, mem.Title, 12, sx-gap-tx, fg)
			place(ops, tCall, image.Pt(tx, mid-tSize.Y/2))
			if mem.ID != 0 {
				h.members = append(h.members, hit)
			}
		}
	}
	if hidden > 0 {
		mCall, mSize := c.label(gtx, "+"+strconv.Itoa(hidden)+" more", 11, th.Muted)
		place(ops, mCall, image.Pt(pad+gap, moreY+(memH-mSize.Y)/2))
	}
}

const (
	// maxMemberLabel bounds the "› title" a folded group header adds for the shown session.
	maxMemberLabel = 16
	// maxStackDots bounds a group header's dots, so a big group cannot widen it.
	maxStackDots = 4
)

// cutRunes shortens s to n runes with a trailing "…"; short text is not copied.
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// shortFolder is the last two parts of a member's folder, enough to tell members apart.
func shortFolder(dir string) string {
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	parent := filepath.Dir(dir)
	if parent == dir || filepath.Base(parent) == parent || parent == filepath.VolumeName(parent)+string(filepath.Separator) {
		return filepath.Base(dir)
	}
	return filepath.Join(filepath.Base(parent), filepath.Base(dir))
}

// memberColor maps a recipe color through the theme's ANSI colors.
func (c *chrome) memberColor(name string) color.NRGBA {
	switch name {
	case "red":
		return c.ansi[1]
	case "green":
		return c.ansi[2]
	case "yellow":
		return c.ansi[3]
	case "blue":
		return c.ansi[4]
	case "magenta":
		return c.ansi[5]
	case "cyan":
		return c.ansi[6]
	}
	return c.th.Muted
}

// badgeColor is an alert mark's color: amber while it needs you, green when done.
func (c *chrome) badgeColor(m mark) color.NRGBA {
	switch m {
	case markNeeds:
		return c.th.Needs
	case markDone:
		return c.th.Done
	}
	return c.th.Muted
}

// alertDot draws a mark's dot centred on mid, ending a gap left of right, and
// returns its left edge.
func (c *chrome) alertDot(gtx layout.Context, right, mid int, mk mark) int {
	d, gap := gtx.Dp(6), gtx.Dp(6)
	x := right - gap - d
	paint.FillShape(gtx.Ops, c.badgeColor(mk), clip.Ellipse(image.Rect(x, mid-d/2, x+d, mid-d/2+d)).Op(gtx.Ops))
	return x
}

// sideInk is the sidebar's text colors, nudged just far enough to read at 4.5:1 on
// the sidebar, the tray chip and the tooltip in every theme (spec 5).
type sideInk struct{ muted, needs, kind, chipLead, chipBG, tipFG, tipBG color.NRGBA }

func (c *chrome) sideInk() sideInk {
	if c.inkFor == c.th && c.inkMag == c.ansi[5] {
		return c.ink
	}
	th := c.th
	var k sideInk
	k.chipBG = mix(th.Chrome, th.Accent, 0.08)
	// Muted and amber text also sit on the chip, a touch nearer the accent than the sidebar.
	k.muted = render.Readable(render.Readable(th.Muted, k.chipBG), th.Chrome)
	k.needs = render.Readable(render.Readable(th.Needs, k.chipBG), th.Chrome)
	k.kind = render.Readable(c.memberColor("magenta"), th.Chrome)
	k.chipLead = render.Readable(mix(th.Accent, th.ActiveFG, 0.35), k.chipBG)
	k.tipBG = th.ToastBG
	k.tipFG = render.Readable(th.ToastFG, th.ToastBG)
	c.ink, c.inkFor, c.inkMag = k, th, c.ansi[5]
	return k
}

// sideTheme is the theme with the sidebar's muted and amber text nudged by sideInk.
func (c *chrome) sideTheme() theme.Chrome {
	th, k := c.th, c.sideInk()
	th.Muted, th.Needs = k.muted, k.needs
	return th
}

// runHeaderState is a workspace header's state text and color: "N down" in red
// when any member is down, else "N ready" muted when any member's commands
// finished (no server may be running there, #118), else "all up".
func runHeaderState(run sideRun, th theme.Chrome) (string, color.NRGBA) {
	switch {
	case run.Down > 0:
		return strconv.Itoa(run.Down) + " down", th.UsageHigh
	case run.NotStarted > 0:
		return strconv.Itoa(run.NotStarted) + " not started", th.Muted
	case run.Ready > 0:
		return strconv.Itoa(run.Ready) + " ready", th.Muted
	}
	return "all up", th.Done
}

func (c *chrome) fitMonoText(gtx layout.Context, s string, size unit.Sp, room int, col color.NRGBA, w font.Weight) (op.CallOp, image.Point, string) {
	s = cutRunes(c.fit(s), maxSideText)
	call, sz := c.labelMonoW(gtx, s, size, col, w)
	for i := 0; i < 6 && sz.X > room && s != ""; i++ {
		n := utf8.RuneCountInString(s)
		keep := min(n*max(room, 0)/max(sz.X, 1), n-1)
		if keep < 2 {
			s = ""
		} else {
			s = cutRunes(s, keep)
		}
		call, sz = c.labelMonoW(gtx, s, size, col, w)
	}
	if sz.X > room {
		call, sz = c.labelMonoW(gtx, "", size, col, w)
		sz.X, s = 0, ""
	}
	return call, sz, s
}

func (c *chrome) fitMono(g layout.Context, s string, z unit.Sp, room int, col color.NRGBA) (op.CallOp, image.Point) {
	a, b, _ := c.fitMonoText(g, s, z, room, col, font.Normal)
	return a, b
}
