package ui

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// badgeInk is the dark letter on a space's coloured badge, the same in every theme.
var badgeInk = color.NRGBA{R: 0x0b, G: 0x0d, B: 0x12, A: 0xff}

// layoutSidebar paints the sidebar into size and returns where its parts are: the
// SPACES header with its search and + icons, the filter field while it is open, the
// pinned Settings row, the scrolling list of spaces and agents, and the workspaces
// section pinned to the bottom.
func (c *chrome) layoutSidebar(gtx layout.Context, m sideModel, st sideState, size image.Point) sideHits {
	ops, th := gtx.Ops, c.sideTheme()
	var h sideHits
	paint.FillShape(ops, th.Chrome, clip.Rect{Max: size}.Op())
	// The 1 px divider sits on the edge that faces the terminal; the list stays off it.
	edge, listL, listR := size.X-1, 0, size.X-1
	if st.right {
		edge, listL, listR = 0, 1, size.X
	}
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(edge, 0, edge+1, size.Y)).Op())
	root := clip.Rect{Max: size}.Push(ops)
	defer root.Pop()

	pad, gap := gtx.Dp(8), gtx.Dp(8)
	hitPad := gtx.Dp(4)
	y := 0

	c.spacesHeader(gtx, &h, st, listR, y)
	y += gtx.Dp(sideHeadH)

	// The filter field, opened by the search icon.
	if st.filterOpen {
		fieldH := gtx.Dp(30)
		h.filter = image.Rect(pad, y, size.X-pad, y+fieldH)
		frr := clip.UniformRRect(h.filter, gtx.Dp(7))
		paint.FillShape(ops, th.Border, clip.Stroke{Path: frr.Path(ops), Width: float32(max(gtx.Dp(1), 1))}.Op())
		ftxt, fcol := "Filter spaces and agents…", th.Muted
		if st.filter != "" {
			ftxt, fcol = st.filter, th.ActiveFG
		}
		if st.ed == nil {
			fCall, fSize := c.fitSide(gtx, ftxt, 12, h.filter.Dx()-2*gap, fcol)
			place(ops, fCall, image.Pt(h.filter.Min.X+gap, h.filter.Min.Y+(fieldH-fSize.Y)/2))
		} else {
			c.lineEditor(gtx, st.ed, h.filter, ftxt, fcol, gap)
		}
		y += fieldH + pad
	}

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
	h.list = image.Rect(listL, min(y, secTop), listR, secTop)

	c.sideList(gtx, &h, m, st, closeCall, closeSize)
	if len(m.Runs) > 0 {
		c.sideRuns(gtx, &h, m, st, secTop, size, closeCall, closeSize)
	}
	return h
}

// spacesHeader draws the SPACES label with the search and + icon buttons at its right.
func (c *chrome) spacesHeader(gtx layout.Context, h *sideHits, st sideState, right, y int) {
	ops, th := gtx.Ops, c.sideTheme()
	pad, gap := gtx.Dp(8), gtx.Dp(8)
	hdH, btn := gtx.Dp(sideHeadH), gtx.Dp(sideIconBtn)
	by := y + (hdH-btn)/2
	h.plus = image.Rect(right-pad-btn, by, right-pad, by+btn)
	h.search = image.Rect(h.plus.Min.X-btn, by, h.plus.Min.X, by+btn)
	call, sz := c.label(gtx, "SPACES", 11, th.Muted)
	place(ops, call, image.Pt(pad+gap, y+(hdH-sz.Y)/2))
	for _, b := range [...]struct {
		kind sideBtn
		r    image.Rectangle
		icon string
		on   bool
	}{{btnSearch, h.search, "search", st.filterOpen}, {btnPlus, h.plus, "plus", false}} {
		fg := th.Muted
		switch {
		case st.hoverBtn == b.kind:
			fg = th.ActiveFG
			paint.FillShape(ops, th.Active, clip.UniformRRect(b.r, gtx.Dp(7)).Op(ops))
		case b.on:
			fg = th.Accent
		}
		ic := min(gtx.Dp(14), b.r.Dx(), b.r.Dy())
		cx, cy := (b.r.Min.X+b.r.Max.X)/2, (b.r.Min.Y+b.r.Max.Y)/2
		c.icon(gtx, b.icon, image.Rect(cx-ic/2, cy-ic/2, cx-ic/2+ic, cy-ic/2+ic), fg)
	}
}

// spaceSpan is a space line's height with its gap and its documents.
func spaceSpan(gtx layout.Context, s spaceEntry) int {
	return gtx.Dp(sideSpaceH) + gtx.Dp(sideSpaceGap) + len(s.Docs)*gtx.Dp(sideDocH)
}

// agentCounts is how many agent rows each group has.
func agentCounts(rows []agentRow) (n [3]int) {
	for _, r := range rows {
		n[r.State]++
	}
	return n
}

// agentsSpan is the Agents section's height: its header, a label per group present
// and a row per agent; 0 for no agents.
func agentsSpan(gtx layout.Context, rows []agentRow) int {
	if len(rows) == 0 {
		return 0
	}
	h := gtx.Dp(sideHeadH) + len(rows)*gtx.Dp(sideAgentRowH) + gtx.Dp(6)
	for _, n := range agentCounts(rows) {
		if n > 0 {
			h += gtx.Dp(sideAgentLabelH)
		}
	}
	return h
}

// sideList draws the scrolled spaces and the Agents section, clipped to h.list, and
// records hits for the visible parts only. While everything fits, the Agents section
// sits at the bottom of the list; otherwise it follows the spaces and scrolls.
func (c *chrome) sideList(gtx layout.Context, h *sideHits, m sideModel, st sideState, closeCall op.CallOp, closeSize image.Point) {
	ops, th := gtx.Ops, c.sideTheme()
	pad := gtx.Dp(8)

	spacesH := 0
	for _, s := range m.Spaces {
		spacesH += spaceSpan(gtx, s)
	}
	agentsH := agentsSpan(gtx, m.Agents)
	total := spacesH + agentsH
	h.scrollMax = max(0, total-h.list.Dy())
	if total == 0 && st.filter != "" && !h.list.Empty() {
		h.noMatches = true
		call, sz := c.label(gtx, "No matches", 12, th.Muted)
		place(ops, call, image.Pt(h.list.Min.X+pad+pad, h.list.Min.Y+gtx.Dp(12)-sz.Y/2))
	}
	scroll := min(max(st.scroll, 0), h.scrollMax)

	if h.list.Empty() {
		return
	}
	cl := clip.Rect(h.list).Push(ops)
	defer cl.Pop()
	y := h.list.Min.Y - scroll
	for _, s := range m.Spaces {
		c.spaceLine(gtx, h, s, st, y, closeCall, closeSize)
		y += spaceSpan(gtx, s)
	}
	if agentsH > 0 {
		if total <= h.list.Dy() {
			y = h.list.Max.Y - agentsH
		}
		c.agentsSection(gtx, h, m.Agents, st, y)
	}
}

// spaceLine draws one space: a badge, its name and its count, and under it the
// documents opened from its sessions.
func (c *chrome) spaceLine(gtx layout.Context, h *sideHits, s spaceEntry, st sideState, y int, closeCall op.CallOp, closeSize image.Point) {
	ops, th := gtx.Ops, c.sideTheme()
	pad, gap := gtx.Dp(8), gtx.Dp(8)
	rect := image.Rect(pad, y, h.list.Max.X-pad, y+gtx.Dp(sideSpaceH))
	bx := rect.Min.X + gap
	c.docLines(gtx, h, sideRow{Docs: s.Docs}, st, bx, rect.Max.X-gap, rect.Max.Y+gtx.Dp(sideSpaceGap), closeCall, closeSize)
	if !rect.Overlaps(h.list) {
		return
	}
	on := s.Key == st.space && !st.settingsOn && st.doc == 0
	fg := th.Folder
	switch {
	case on:
		paint.FillShape(ops, th.Active, clip.UniformRRect(rect, gtx.Dp(8)).Op(ops))
		ax := rect.Min.X // the accent sits on the line's outer side, mirrored with the sidebar
		if st.right {
			ax = rect.Max.X - gtx.Dp(2)
		}
		paint.FillShape(ops, th.Accent, clip.UniformRRect(image.Rect(ax, rect.Min.Y+gtx.Dp(8), ax+gtx.Dp(2), rect.Max.Y-gtx.Dp(8)), gtx.Dp(1)).Op(ops))
		fg = th.ActiveFG
	case st.hoverSpace == s.Key:
		paint.FillShape(ops, tint(th.Active, 0x90), clip.UniformRRect(rect, gtx.Dp(8)).Op(ops))
	}
	mid := rect.Min.Y + rect.Dy()/2
	side := gtx.Dp(20)
	badge := image.Rect(bx, mid-side/2, bx+side, mid-side/2+side)
	paint.FillShape(ops, s.Color, clip.UniformRRect(badge, gtx.Dp(6)).Op(ops))
	bCall, bSize := c.labelW(gtx, string(s.Badge), 11, badgeInk, font.Bold)
	place(ops, bCall, image.Pt(badge.Min.X+(side-bSize.X)/2, mid-bSize.Y/2))

	count, ccol := strconv.Itoa(s.Count)+" tabs", th.Muted
	switch {
	case s.Waiting > 0:
		count, ccol = strconv.Itoa(s.Waiting)+" waiting", th.Needs
	case s.Count == 1:
		count = "1 tab"
	}
	cCall, cSize := c.label(gtx, count, 11, ccol)
	cx := rect.Max.X - gap - cSize.X
	place(ops, cCall, image.Pt(cx, mid-cSize.Y/2))
	nx := badge.Max.X + gap + gtx.Dp(1)
	nCall, nSize := c.fitSideWeight(gtx, s.Name, 12, cx-gap-nx, fg, font.Bold)
	place(ops, nCall, image.Pt(nx, mid-nSize.Y/2))
	h.spaces = append(h.spaces, sideKeyHit{key: s.Key, rect: rect.Intersect(h.list)})
}

// agentAge is the time since an agent's turn began: "now", "5m" or "2h"; "" for none.
func agentAge(now, since time.Time) string {
	if since.IsZero() {
		return ""
	}
	switch d := now.Sub(since); {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return strconv.Itoa(int(d/time.Minute)) + "m"
	default:
		return strconv.Itoa(int(d/time.Hour)) + "h"
	}
}

// agentAges folds every agent's age text into one number, so the retained ops are
// rebuilt when a shown age changes and not otherwise.
func agentAges(now time.Time, rows []agentRow) (key uint64) {
	for _, r := range rows {
		if r.Since.IsZero() {
			continue
		}
		d := now.Sub(r.Since)
		bucket := uint64(max(d, 0) / time.Minute)
		key = key*1099511628211 + bucket + 1
	}
	return key
}

// agentsSection draws the AGENTS header and its rows from y down, grouped Needs
// you, Working and Idle; a click on a row shows that session.
func (c *chrome) agentsSection(gtx layout.Context, h *sideHits, rows []agentRow, st sideState, y int) {
	ops, th := gtx.Ops, c.sideTheme()
	pad, gap := gtx.Dp(8), gtx.Dp(8)
	hdH, labelH, rowH := gtx.Dp(sideHeadH), gtx.Dp(sideAgentLabelH), gtx.Dp(sideAgentRowH)
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(h.list.Min.X, y, h.list.Max.X, y+max(gtx.Dp(1), 1))).Op())
	call, sz := c.label(gtx, "AGENTS", 11, th.Muted)
	place(ops, call, image.Pt(pad+gap, y+(hdH-sz.Y)/2))
	y += hdH
	counts := agentCounts(rows)
	names := [3]string{"Needs you ", "Working ", "Idle "}
	last := agentGroup(-1)
	for _, r := range rows {
		if r.State != last {
			last = r.State
			lcol := th.Muted
			if r.State == agentNeeds {
				lcol = th.Needs
			}
			lCall, lSize := c.label(gtx, names[r.State]+strconv.Itoa(counts[r.State]), 11, lcol)
			place(ops, lCall, image.Pt(pad+gap, y+(labelH-lSize.Y)/2))
			y += labelH
		}
		rect := image.Rect(pad, y, h.list.Max.X-pad, y+rowH)
		y = rect.Max.Y
		if !rect.Overlaps(h.list) {
			continue
		}
		c.agentLine(gtx, r, st, rect)
		h.rows = append(h.rows, sideRowHit{id: r.ID, rect: rect.Intersect(h.list)})
	}
}

// agentLine draws one agent row in rect: logo, space name, subtitle, age, state mark.
func (c *chrome) agentLine(gtx layout.Context, r agentRow, st sideState, rect image.Rectangle) {
	ops, th := gtx.Ops, c.sideTheme()
	gap := gtx.Dp(8)
	fg := th.Folder
	switch {
	case r.ID == st.shown && !st.settingsOn && st.doc == 0:
		fg = th.ActiveFG
		paint.FillShape(ops, th.Active, clip.UniformRRect(rect, gtx.Dp(7)).Op(ops))
	case r.ID == st.hover:
		paint.FillShape(ops, tint(th.Active, 0x90), clip.UniformRRect(rect, gtx.Dp(7)).Op(ops))
	}
	mid := rect.Min.Y + rect.Dy()/2
	x := rect.Min.X + gap
	logoPx := gtx.Dp(13)
	if logo, img := agentLogo(r.Name, logoPx, agentLogoColor(r.Name, fg)); img != nil {
		off := op.Offset(image.Pt(x, mid-logoPx/2)).Push(ops)
		logo.Add(ops)
		paint.PaintOp{}.Add(ops)
		off.Pop()
	} else {
		c.icon(gtx, "terminal", image.Rect(x, mid-logoPx/2, x+logoPx, mid+logoPx/2), fg)
	}
	x += logoPx + gap

	dot := gtx.Dp(7)
	right := rect.Max.X - gap
	c.sideDot(gtx, image.Rect(right-dot, mid-dot/2, right, mid-dot/2+dot), r.Mark, r.Running)
	right -= dot + gap
	if age := agentAge(gtx.Now, r.Since); age != "" {
		aCall, aSize := c.label(gtx, age, 11, th.Muted)
		right -= aSize.X
		place(ops, aCall, image.Pt(right, mid-aSize.Y/2))
		right -= gap
	}
	nCall, nSize := c.fitSideWeight(gtx, r.Space, 12, min(max(right-x, 0), gtx.Dp(110)), th.ActiveFG, font.Bold)
	place(ops, nCall, image.Pt(x, mid-nSize.Y/2))
	x += nSize.X + gap
	if room := right - x; room > 0 && r.Subtitle != "" {
		sCall, sSize := c.fitSide(gtx, r.Subtitle, 11, room, th.Muted)
		place(ops, sCall, image.Pt(x, mid-sSize.Y/2))
	}
}
