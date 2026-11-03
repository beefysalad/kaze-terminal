package ui

import (
	"image"
	"image/color"
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

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// sidebarW is the sidebar's width in dp at 100% scale.
const sidebarW = 260

// sideGroupH and sideRowH are a group header's and a session row's height in dp.
const sideGroupH, sideRowH = 28, 46

// maxSideText bounds how much of one title is shaped; the rest never fits 260dp anyway.
const maxSideText = 160

// sideState is what the window knows that the model does not.
type sideState struct {
	activeRepo    string // common repository of the selected main session
	launch        *branchLauncher
	doc, hoverDoc docID
	shown         tabs.TabID      // highlighted row or member
	openAgent     subagents.Key   // the subagent whose docked panel is shown
	hoverAgent    subagents.Key   // the tray line under the pointer, whose step shows as a tooltip
	settings      bool            // the pinned "Settings" row is open
	settingsOn    bool            // and shown
	hover         tabs.TabID      // row showing its ×
	scroll        int             // px the session list is scrolled
	filter        string          // as typed, for the placeholder/clear state
	ed            *widget.Editor  // the filter's editor; nil draws the text without a caret
	renaming      tabs.TabID      // the row whose title is ren, 0 for none
	ren           *widget.Editor  // the rename editor
	static        bool            // editors are drawn live over the retained chrome
	runFolded     map[uint64]bool // workspace runs whose header is folded
	floating      tabs.TabID      // the front member pane over the shown session, highlighted like shown
}

type sideRowHit struct {
	id          tabs.TabID
	rect, close image.Rectangle
	title       image.Rectangle // a session row's title line, where a double click renames
	fold        image.Rectangle // its agent tray's fold control, empty when it has no tray
	mark        mark            // the alert dot a member line drew
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
type sideHits struct {
	cleanups                                                                        []sideKeyHit
	launchToggle, launchButton, launchRepo, launchBranch, launchStart, launchCancel image.Rectangle
	launchAgents                                                                    [3]image.Rectangle
	launchRepos                                                                     []image.Rectangle
	docs                                                                            []sideDocHit
	rows                                                                            []sideRowHit
	groups                                                                          []sideKeyHit
	members                                                                         []sideRowHit
	agents                                                                          []sideAgentHit // subagent lines and "+N more" lines
	runs                                                                            []sideRunHit
	settings, settingsClose                                                         image.Rectangle
	filter                                                                          image.Rectangle
	next                                                                            image.Rectangle // the "Next waiting" button
	list                                                                            image.Rectangle // the scrollable area, for wheel events
	rail                                                                            image.Rectangle // the active project's accent rail, empty when none shows
	tip                                                                             image.Rectangle // the hovered agent's step tooltip, empty when none shows
	rename                                                                          image.Rectangle // the rename editor, empty while none is drawn
	scrollMax                                                                       int
	noMatches                                                                       bool      // the filter hides every session
	chip                                                                            [3]string // the tray chip or footer parts as drawn, for tests
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
// geometry stays inside slot; painting this mark never requests a frame.
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
	paint.FillShape(ops, core, clip.Outline{Path: circle(size * 2.6 / 14)}.Op())
}

// layoutSidebar paints the sidebar into size and returns where its parts are. The
// session list scrolls inside hits.list; the workspaces section is pinned to the bottom.
func (c *chrome) layoutSidebar(gtx layout.Context, m sideModel, st sideState, size image.Point) sideHits {
	ops, th := gtx.Ops, c.sideTheme()
	var h sideHits
	paint.FillShape(ops, th.Chrome, clip.Rect{Max: size}.Op())
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(size.X-1, 0, size.X, size.Y)).Op())
	root := clip.Rect{Max: size}.Push(ops)
	defer root.Pop()

	pad, gap := gtx.Dp(8), gtx.Dp(8)
	hitPad := gtx.Dp(4)
	y := 0

	// The filter field; new sessions start from the title bar's +.
	fieldH := gtx.Dp(30)
	h.filter = image.Rect(pad, y+pad, size.X-pad, y+pad+fieldH)
	frr := clip.UniformRRect(h.filter, gtx.Dp(7))
	paint.FillShape(ops, th.Border, clip.Stroke{Path: frr.Path(ops), Width: float32(max(gtx.Dp(1), 1))}.Op())
	ftxt, fcol := "Filter sessions…", th.Muted
	if st.filter != "" {
		ftxt, fcol = st.filter, th.ActiveFG
	}
	if st.ed == nil {
		fCall, fSize := c.fitSide(gtx, ftxt, 12, h.filter.Dx()-2*gap, fcol)
		place(ops, fCall, image.Pt(h.filter.Min.X+gap, h.filter.Min.Y+(fieldH-fSize.Y)/2))
	} else {
		c.lineEditor(gtx, st.ed, h.filter, ftxt, fcol, gap)
	}
	y += pad + fieldH + pad

	// The pinned Settings row.
	closeCall, closeSize := c.label(gtx, "×", 12, th.Muted)
	if st.settings {
		rowH := gtx.Dp(28)
		r := image.Rect(pad, y, size.X-pad, y+rowH)
		fg := th.Muted
		if st.settingsOn {
			fg = th.ActiveFG
			paint.FillShape(ops, th.Active, clip.UniformRRect(r, gtx.Dp(7)).Op(ops))
		}
		call, sz := c.label(gtx, "Settings", 12, fg)
		place(ops, call, image.Pt(r.Min.X+gap, r.Min.Y+(rowH-sz.Y)/2))
		cx := r.Max.X - gap - closeSize.X
		place(ops, closeCall, image.Pt(cx, r.Min.Y+(rowH-closeSize.Y)/2))
		h.settings = r
		h.settingsClose = image.Rect(cx-hitPad, r.Min.Y, r.Max.X, r.Max.Y)
		y += rowH + gtx.Dp(4)
	}

	// The workspaces section takes at most half the sidebar, from the bottom.
	secTop := size.Y
	if len(m.Runs) > 0 {
		secH := gtx.Dp(26)
		for _, run := range m.Runs {
			secH += gtx.Dp(26)
			if !st.runFolded[run.Run] {
				secH += len(run.Members) * gtx.Dp(24)
			}
		}
		secH = min(secH+gtx.Dp(6), max(size.Y/2, 0))
		secTop = size.Y - secH
	}
	launcherH := c.launcherHeight(gtx, st.launch)
	listBot := max(secTop-launcherH, 0)
	h.list = image.Rect(0, min(y, listBot), size.X-1, listBot)
	c.drawBranchLauncher(gtx, &h, st.launch, st.static, image.Rect(pad, listBot, size.X-1-pad, secTop))

	c.sideList(gtx, &h, m, st, closeCall, closeSize)
	if len(m.Runs) > 0 {
		c.sideRuns(gtx, &h, m, st, secTop, size, closeCall, closeSize)
	}
	c.trayTip(gtx, &h, st.hoverAgent)
	return h
}

// trayTip draws the hovered tray line's current step as a one-line tooltip under the
// line, or above it when the list has no room below. It is part of the retained
// sidebar, keyed by the hovered agent, so it needs no timer and no extra frames.
func (c *chrome) trayTip(gtx layout.Context, h *sideHits, k subagents.Key) {
	if k == (subagents.Key{}) {
		return
	}
	for _, a := range h.agents {
		if a.kind != hitAgentLine || a.key != k || a.tip == "" {
			continue
		}
		ops, ink := gtx.Ops, c.sideInk()
		pad, rad := gtx.Dp(6), gtx.Dp(6)
		box := image.Rect(a.rect.Min.X, 0, h.list.Max.X-gtx.Dp(8), 0)
		call, sz := c.fitSide(gtx, a.tip, 11, max(box.Dx()-2*pad, 0), ink.tipFG)
		ht := sz.Y + 2*pad
		box.Min.Y, box.Max.Y = a.rect.Max.Y, a.rect.Max.Y+ht
		if box.Max.Y > h.list.Max.Y {
			box.Min.Y, box.Max.Y = a.rect.Min.Y-ht, a.rect.Min.Y
		}
		if box.Empty() || sz.X == 0 {
			return
		}
		cl := clip.Rect(h.list).Push(ops)
		defer cl.Pop()
		rr := clip.UniformRRect(box, rad)
		paint.FillShape(ops, ink.tipBG, rr.Op(ops))
		stroke(ops, box, rad, c.th.ToastLine)
		place(ops, call, image.Pt(box.Min.X+pad, box.Min.Y+pad))
		h.tip = box.Intersect(h.list)
		return
	}
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

// sideList draws the scrolled groups and loose rows, clipped to h.list, and records hits
// for the visible parts only.
func (c *chrome) sideList(gtx layout.Context, h *sideHits, m sideModel, st sideState, closeCall op.CallOp, closeSize image.Point) {
	ops, th, ink := gtx.Ops, c.sideTheme(), c.sideInk()
	pad, gap, hitPad := gtx.Dp(8), gtx.Dp(8), gtx.Dp(4)
	groupH, rowH := gtx.Dp(sideGroupH), gtx.Dp(sideRowH)
	dot := gtx.Dp(7)

	total := 0
	for _, g := range m.Groups {
		total += groupH
		if g.Finished > 0 {
			total += gtx.Dp(sideMoreH)
		}
		if !g.Folded {
			for _, r := range g.Rows {
				total += rowSpan(gtx, r)
			}
		}
	}
	for _, r := range m.Loose {
		total += rowSpan(gtx, r)
	}
	h.scrollMax = max(0, total-h.list.Dy())
	if total == 0 && st.filter != "" && !h.list.Empty() {
		h.noMatches = true
		call, sz := c.label(gtx, "No matches", 12, th.Muted)
		place(ops, call, image.Pt(h.list.Min.X+pad+gap, h.list.Min.Y+gtx.Dp(12)-sz.Y/2))
	}
	scroll := min(max(st.scroll, 0), h.scrollMax)

	if h.list.Empty() {
		return
	}
	cl := clip.Rect(h.list).Push(ops)
	defer cl.Pop()
	y := h.list.Min.Y - scroll

	row := func(r sideRow) {
		rect := image.Rect(pad, y, h.list.Max.X-pad, y+rowH)
		y += rowSpan(gtx, r)
		c.docLines(gtx, h, r, st, rect.Min.X+gap+dot+gap, rect.Max.X-gap, rect.Max.Y, closeCall, closeSize)
		c.trayLines(gtx, h, r, rect, rect.Max.Y+len(r.Docs)*gtx.Dp(sideDocH), st.openAgent, ink)
		if !rect.Overlaps(h.list) {
			return
		}
		on := r.ID == st.shown && !st.settingsOn && st.doc == 0
		fg := th.Folder
		if on {
			paint.FillShape(ops, th.Active, clip.UniformRRect(rect, gtx.Dp(7)).Op(ops))
			paint.FillShape(ops, th.Accent, clip.UniformRRect(image.Rect(rect.Min.X, rect.Min.Y+gtx.Dp(8), rect.Min.X+gtx.Dp(2), rect.Max.Y-gtx.Dp(8)), gtx.Dp(1)).Op(ops))
			fg = th.ActiveFG
		}
		tx := rect.Min.X + gap + dot + gap
		c.sideDot(gtx, image.Rect(rect.Min.X+gap, rect.Min.Y+gtx.Dp(14)-dot/2, rect.Min.X+gap+dot, rect.Min.Y+gtx.Dp(14)-dot/2+dot), r.Mark, r.Running)
		room := rect.Max.X - gap - closeSize.X - gap - tx
		titleBand := image.Rect(tx, rect.Min.Y+gtx.Dp(4), tx+max(room, 0), rect.Min.Y+gtx.Dp(24))
		word := r.stateWord()
		wcol := th.Accent
		switch {
		case r.TrayTotal > 0:
			// The tray's fold control: amber when the session or a folded agent waits.
			if r.Mark == markNeeds || r.TrayFolded && r.TrayWaiting > 0 {
				wcol = ink.needs
			}
		case r.Mark == markNeeds:
			wcol = th.Needs
		case r.Mark == markDone:
			wcol = th.Done
		}
		var wCall op.CallOp
		var wSize image.Point
		wordW := 0
		if word != "" {
			wCall, wSize = c.label(gtx, word, 11, wcol)
			wordW = wSize.X
		}
		var addCall, delCall op.CallOp
		var addSize, delSize image.Point
		countsW := 0 // the +N -M block, drawn left of the state word
		if r.Add > 0 {
			addCall, addSize = c.label(gtx, "+"+strconv.Itoa(r.Add), 11, th.Done)
			countsW += addSize.X
		}
		if r.Del > 0 {
			delCall, delSize = c.label(gtx, "−"+strconv.Itoa(r.Del), 11, th.UsageHigh)
			if countsW > 0 {
				countsW += gap
			}
			countsW += delSize.X
		}
		right := rect.Max.X - gap
		if st.hover == r.ID {
			right -= closeSize.X + gap
		}
		if word != "" {
			place(ops, wCall, image.Pt(right-wordW, rect.Min.Y+gtx.Dp(14)-wSize.Y/2))
			titleBand.Max.X = max(titleBand.Min.X, right-wordW-gap)
		} else {
			titleBand.Max.X = right
		}
		c.rowTitle(gtx, r, st, h, titleBand, fg)
		detail := r.Detail
		if detail == "" {
			detail = r.Branch
			if detail == "" {
				detail = r.Folder
			}
		}
		detailCol := th.Branch
		if r.Branch == "" {
			detailCol = th.Muted
		}
		room = rect.Max.X - gap - tx
		if countsW > 0 {
			room -= countsW + gap
		}
		call, sz := c.fitSide(gtx, detail, 11, max(room, 0), detailCol)
		mid := rect.Min.Y + gtx.Dp(33)
		place(ops, call, image.Pt(tx, mid-sz.Y/2))
		if countsW > 0 {
			x := rect.Max.X - gap - countsW
			if r.Add > 0 {
				place(ops, addCall, image.Pt(x, mid-addSize.Y/2))
				x += addSize.X + gap
			}
			if r.Del > 0 {
				place(ops, delCall, image.Pt(x, mid-delSize.Y/2))
			}
		}
		hit := sideRowHit{id: r.ID, rect: rect.Intersect(h.list), title: titleBand.Intersect(h.list)}
		if r.TrayTotal > 0 && word != "" {
			wmid := rect.Min.Y + gtx.Dp(14)
			hit.fold = image.Rect(right-wordW-hitPad, wmid-wSize.Y/2-hitPad, right+hitPad, wmid+wSize.Y/2+hitPad).Intersect(h.list)
		}
		if st.hover == r.ID && r.ID != 0 {
			cx := rect.Max.X - gap - closeSize.X
			cmid := rect.Min.Y + gtx.Dp(14)
			place(ops, closeCall, image.Pt(cx, cmid-closeSize.Y/2))
			hit.close = image.Rect(cx-hitPad, cmid-closeSize.Y/2-hitPad, rect.Max.X, cmid+closeSize.Y/2+hitPad).Intersect(h.list)
		}
		h.rows = append(h.rows, hit)
	}

	for _, g := range m.Groups {
		hd := image.Rect(pad, y, h.list.Max.X-pad, y+groupH)
		y += groupH
		if hd.Overlaps(h.list) {
			c.sideGroupHeader(gtx, g, st, hd)
			h.groups = append(h.groups, sideKeyHit{key: g.Key, rect: hd.Intersect(h.list)})
		}
		if !g.Folded {
			for _, r := range g.Rows {
				row(r)
			}
		}
		if g.Key != "" && g.Key == st.activeRepo {
			// The project you are in: a thin accent rail from its header to its last row.
			x := h.list.Min.X + gtx.Dp(2)
			h.rail = image.Rect(x, hd.Min.Y, x+gtx.Dp(2), y).Intersect(h.list)
			if !h.rail.Empty() {
				paint.FillShape(ops, th.Accent, clip.UniformRRect(h.rail, gtx.Dp(1)).Op(ops))
			}
		}
		if g.Finished > 0 {
			band := image.Rect(pad, y, h.list.Max.X-pad, y+gtx.Dp(sideMoreH))
			y = band.Max.Y
			if band.Overlaps(h.list) {
				call, sz := c.fitSide(gtx, strconv.Itoa(g.Finished)+" finished worktrees · clean up", 10, band.Dx()-gap*2, th.Muted)
				place(ops, call, image.Pt(band.Min.X+gap, band.Min.Y+(band.Dy()-sz.Y)/2))
				h.cleanups = append(h.cleanups, sideKeyHit{g.Key, band.Intersect(h.list)})
			}
		}
	}
	for _, r := range m.Loose {
		row(r)
	}
}

// rowTitle draws the agent logo and name, or the rename field.
func (c *chrome) rowTitle(gtx layout.Context, r sideRow, st sideState, h *sideHits, band image.Rectangle, fg color.NRGBA) {
	ops, th := gtx.Ops, c.sideTheme()
	if r.ID == st.renaming {
		field := image.Rect(band.Min.X-gtx.Dp(4), band.Min.Y, band.Max.X, band.Max.Y)
		rr := clip.UniformRRect(field, gtx.Dp(4))
		paint.FillShape(ops, th.Chrome, rr.Op(ops))
		paint.FillShape(ops, th.Accent, clip.Stroke{Path: rr.Path(ops), Width: float32(max(gtx.Dp(1), 1))}.Op())
		if st.ren != nil {
			c.lineEditor(gtx, st.ren, field, "", th.Muted, gtx.Dp(4))
		}
		h.rename = field.Intersect(h.list)
		return
	}
	mid := band.Min.Y + band.Dy()/2
	x := band.Min.X
	logoPx, gap := gtx.Dp(12), gtx.Dp(5)
	logo, img := agentLogo(r.Agent, logoPx, agentLogoColor(r.Agent, fg))
	if img != nil && band.Dx() >= logoPx+gap {
		off := op.Offset(image.Pt(x, mid-logoPx/2)).Push(ops)
		logo.Add(ops)
		paint.PaintOp{}.Add(ops)
		off.Pop()
		x += logoPx + gap
	}
	call, sz := c.fitSide(gtx, r.Title, 12, max(band.Max.X-x, 0), fg)
	place(ops, call, image.Pt(x, mid-sz.Y/2))
}

// sideGroupHeader draws one repo header: the name and the count, or the
// amber "N waiting"; a folded group also names the shown session inside it.
func (c *chrome) sideGroupHeader(gtx layout.Context, g sideGroup, st sideState, hd image.Rectangle) {
	ops, th := gtx.Ops, c.sideTheme()
	gap := gtx.Dp(8)
	mid := hd.Min.Y + hd.Dy()/2
	nameColor, weight := c.groupNameStyle(g.Key != "" && g.Key == st.activeRepo)
	arrowColor := th.Muted

	cx := hd.Min.X + gap
	chev := "▾"
	if g.Folded {
		chev = "›"
	}
	cCall, cSize := c.label(gtx, chev, 10, arrowColor)
	place(ops, cCall, image.Pt(cx, mid-cSize.Y/2))
	cx += cSize.X + gap

	right, rcol := strconv.Itoa(g.Count), th.Muted
	if g.Waiting > 0 {
		right, rcol = strconv.Itoa(g.Waiting)+" waiting", th.Needs
	}
	rCall, rSize := c.label(gtx, right, 11, rcol)
	rx := hd.Max.X - gap - rSize.X
	place(ops, rCall, image.Pt(rx, mid-rSize.Y/2))

	name := g.Name
	if g.Folded {
		for _, r := range g.Rows {
			if r.ID == st.shown && !st.settingsOn && st.doc == 0 {
				name += " › " + cutRunes(r.Title, maxMemberLabel)
				break
			}
		}
	}
	nCall, nSize := c.fitSideWeight(gtx, name, 12, rx-gap-cx, nameColor, weight)
	place(ops, nCall, image.Pt(cx, mid-nSize.Y/2))
}

// sideRuns draws the workspaces section from secTop to the bottom: a header, then per
// run its name with all up / N ready / N down and one line per member, with its alert dot and,
// while hovered, its ×.
func (c *chrome) sideRuns(gtx layout.Context, h *sideHits, m sideModel, st sideState, secTop int, size image.Point, closeCall op.CallOp, closeSize image.Point) {
	ops, th := gtx.Ops, c.sideTheme()
	pad, gap, hitPad := gtx.Dp(8), gtx.Dp(8), gtx.Dp(4)
	hdH, memH := gtx.Dp(26), gtx.Dp(24)
	right := size.X - 1
	area := image.Rect(0, secTop, right, size.Y)
	paint.FillShape(ops, th.Chrome, clip.Rect(area).Op())
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(0, secTop, right, secTop+1)).Op())
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
			on := mem.ID != 0 && (mem.ID == st.shown || mem.ID == st.floating) && !st.settingsOn
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

// groupNameStyle is a project header's name color and weight: the project you are
// in is bold, the rail beside it does the rest.
func (c *chrome) groupNameStyle(active bool) (color.NRGBA, font.Weight) {
	if active {
		return c.th.ActiveFG, font.Bold
	}
	return c.th.ActiveFG, font.Normal
}

// runHeaderState is a workspace header's state text and color: "N down" in red
// when any member is down, else "N ready" muted when any member's commands
// finished (no server may be running there, #118), else "all up".
func runHeaderState(run sideRun, th theme.Chrome) (string, color.NRGBA) {
	switch {
	case run.Down > 0:
		return strconv.Itoa(run.Down) + " down", th.UsageHigh
	case run.Ready > 0:
		return strconv.Itoa(run.Ready) + " ready", th.Muted
	}
	return "all up", th.Done
}
