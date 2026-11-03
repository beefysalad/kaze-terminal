package ui

import (
	"image"
	"image/color"
	"strconv"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// The board's list pane (mockup .bhead, .seg, .search, .card, .chead, .meter,
// .brow, .donefold, .empty). Sizes in dp.
const (
	boardRowDp   = 44
	boardFoldDp  = 30
	boardFieldDp = 28
)

// boardSegments are the segment labels, by boardFilter.
var boardSegments = [4]string{"All", "Needs you", "Running", "Done"}

// boardRowHit is where a row was drawn; close is its × (a hovered or selected done row).
type boardRowHit struct {
	row         boardRow
	rect, close image.Rectangle
}

// boardCardHit is where a session card and its controls were drawn.
type boardCardHit struct {
	tab                            tabs.TabID
	rect, goTo, clear, fold, meter image.Rectangle
}

// fitSans is sans cut with a trailing "…" to fit room px.
func (c *chrome) fitSans(gtx layout.Context, s string, size unit.Sp, room int, col color.NRGBA, w font.Weight) (op.CallOp, image.Point) {
	call, sz := c.sans(gtx, s, size, col, w)
	for i := 0; i < 6 && sz.X > room && s != ""; i++ {
		n := utf8.RuneCountInString(s)
		if keep := min(n*max(room, 0)/max(sz.X, 1), n-1); keep < 2 {
			s = ""
		} else {
			s = cutRunes(s, keep)
		}
		call, sz = c.sans(gtx, s, size, col, w)
	}
	if sz.X > room {
		call, sz = c.sans(gtx, "", size, col, w)
		sz.X = 0
	}
	return call, sz
}

// spaced is s in caps-label style: SemiBold, each letter track px apart.
func (c *chrome) spaced(gtx layout.Context, s string, size unit.Sp, col color.NRGBA, track int) (op.CallOp, image.Point) {
	m := op.Record(gtx.Ops)
	x, h := 0, 0
	for _, r := range s {
		call, sz := c.sans(gtx, string(r), size, col, font.SemiBold)
		place(gtx.Ops, call, image.Pt(x, 0))
		x += sz.X + track
		h = max(h, sz.Y)
	}
	return m.Stop(), image.Pt(max(x-track, 0), h)
}

// pill draws a raised rounded box with a hairline around it.
func pill(gtx layout.Context, r image.Rectangle, fill, line color.NRGBA, radius int) {
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(r, radius).Op(gtx.Ops))
	stroke(gtx.Ops, r, radius, line)
}

// drawBoardList draws the list pane in r: the header, the segments and search,
// then the Needs you card and the session cards, scrolled by b.scroll.
func (win *window) drawBoardList(gtx layout.Context, r image.Rectangle, v *boardView, ink boardInk) {
	b, c, ops, dp := &win.board, win.ui, gtx.Ops, gtx.Dp
	pg := ink.page
	rec := func(s string) { b.texts = append(b.texts, s) }
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()

	// Header: the title and summary, "Back to <session>" on the right.
	x0, x1 := r.Min.X+dp(22), r.Max.X-dp(22)
	y := r.Min.Y + dp(20)
	rec("Agents")
	tc, ts := c.sans(gtx, "Agents", 22, pg.text, font.SemiBold)
	place(ops, tc, image.Pt(x0, y))
	right := x1
	if back, _ := win.sessionName(win.boardBack()); back != "" {
		label := "Back to " + back
		rec(label)
		bc, bs := c.fitSans(gtx, label, 12, max((x1-x0)/2, 0), pg.text, font.Medium)
		h := bs.Y + 2*dp(4)
		b.back = image.Rect(x1-bs.X-2*dp(10), y+(ts.Y-h)/2, x1, y+(ts.Y-h)/2+h)
		pill(gtx, b.back, ink.raised, ink.line, dp(7))
		place(ops, bc, image.Pt(b.back.Min.X+dp(10), b.back.Min.Y+dp(4)))
		right = b.back.Min.X - dp(12)
	}
	if sum := boardSummary(v.agents, v.sessions); sum != "" {
		rec(sum)
		sx := x0 + ts.X + dp(12)
		sc, ss := c.fitSans(gtx, sum, 12.5, max(right-sx, 0), pg.muted, font.Normal)
		place(ops, sc, image.Pt(sx, y+ts.Y-ss.Y-dp(3)))
	}
	y += ts.Y + dp(14)

	// Tools: the segments, the search field and the session token.
	fh := dp(boardFieldDp)
	sx := x0
	box := image.Rect(sx, y, sx, y+fh)
	type segDraw struct {
		lab, cnt     op.CallOp
		labSz, cntSz image.Point
	}
	var segs [4]segDraw
	w := dp(2)
	for i, name := range boardSegments {
		on := boardFilter(i) == b.filter
		lc := pg.muted
		if on {
			lc = pg.text
		}
		cc := pg.muted
		if boardFilter(i) == boardNeeds && v.counts[i] > 0 {
			cc = pg.needs
		}
		rec(name)
		s := &segs[i]
		s.lab, s.labSz = c.sans(gtx, name, 12, lc, font.Medium)
		s.cnt, s.cntSz = c.label(gtx, strconv.Itoa(v.counts[i]), 11, cc)
		w += s.labSz.X + dp(6) + s.cntSz.X + 2*dp(10) + dp(2)
	}
	box.Max.X = sx + w
	pill(gtx, box, ink.field, ink.line, dp(8))
	x := box.Min.X + dp(2)
	for i := range segs {
		s := &segs[i]
		sr := image.Rect(x, box.Min.Y+dp(2), x+s.labSz.X+dp(6)+s.cntSz.X+2*dp(10), box.Max.Y-dp(2))
		if boardFilter(i) == b.filter {
			paint.FillShape(ops, ink.raised, clip.UniformRRect(sr, dp(6)).Op(ops))
		}
		mid := sr.Min.Y + sr.Dy()/2
		place(ops, s.lab, image.Pt(sr.Min.X+dp(10), mid-s.labSz.Y/2))
		place(ops, s.cnt, image.Pt(sr.Min.X+dp(10)+s.labSz.X+dp(6), mid-s.cntSz.Y/2))
		b.seg[i] = sr
		x = sr.Max.X + dp(2)
	}
	end := x1
	if b.tab != 0 {
		if title, _ := win.sessionName(b.tab); title != "" {
			label := title + " ×"
			rec(label)
			tc, tsz := c.fitSans(gtx, label, 12, max((x1-x0)/3, 0), pg.text, font.Medium)
			b.token = image.Rect(x1-tsz.X-2*dp(10), y+dp(2), x1, y+fh-dp(2))
			pill(gtx, b.token, ink.raised, ink.line, dp(7))
			place(ops, tc, image.Pt(b.token.Min.X+dp(10), b.token.Min.Y+(b.token.Dy()-tsz.Y)/2))
			end = b.token.Min.X - dp(8)
		}
	}
	sy := y
	if end-(box.Max.X+dp(10)) < dp(120) { // no room beside the segments: the search goes under them
		sy = y + fh + dp(8)
		b.search = image.Rect(x0, sy, x1, sy+fh)
	} else {
		b.search = image.Rect(box.Max.X+dp(10), sy, end, sy+fh)
	}
	edge := ink.line
	if b.searching {
		edge = ink.priBG
	}
	pill(gtx, b.search, ink.field, edge, dp(7))
	field := b.search
	if !b.searching && b.ed.Len() == 0 {
		kc, ks := c.label(gtx, "/", 11, pg.muted)
		k := image.Rect(field.Max.X-dp(7)-ks.X-2*dp(5), field.Min.Y+dp(5), field.Max.X-dp(7), field.Max.Y-dp(5))
		stroke(ops, k, dp(4), ink.line)
		place(ops, kc, image.Pt(k.Min.X+dp(5), k.Min.Y+(k.Dy()-ks.Y)/2))
		field.Max.X = k.Min.X - dp(4)
	}
	c.lineEditor(gtx, &b.ed, field, "Search agents", pg.muted, dp(10))
	y = sy + fh + dp(12)

	// The hooks hint.
	if b.missing {
		hint := "kaze needs three more Claude Code hooks to see subagents."
		rec(hint)
		hc, hs := c.sans(gtx, hint, 13, pg.needs, font.Medium)
		place(ops, hc, image.Pt(x0, y+dp(6)))
		rec("Set up in Settings")
		bc, bs := c.sans(gtx, "Set up in Settings", 12, pg.text, font.Medium)
		bx := x0 + hs.X + dp(12)
		b.setup = image.Rect(bx, y, bx+bs.X+2*dp(12), y+bs.Y+2*dp(6))
		pill(gtx, b.setup, ink.raised, ink.line, dp(7))
		place(ops, bc, image.Pt(bx+dp(12), y+dp(6)))
		y += b.setup.Dy() + dp(12)
	}
	paint.FillShape(ops, ink.line, clip.Rect(image.Rect(r.Min.X, y, r.Max.X, y+1)).Op())
	y++

	// Body: the cards, scrolled.
	body := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y)
	bc := clip.Rect(body).Push(ops)
	defer bc.Pop()
	if len(v.needs) == 0 && len(v.cards) == 0 {
		b.scroll = 0
		head, line := "No agents match", "Try another filter, or clear the search."
		if v.agents == 0 {
			head, line = "No subagents yet", "They show here when Claude Code or Codex starts helpers."
		}
		rec(head)
		rec(line)
		hc, hs := c.sans(gtx, head, 14, pg.text, font.SemiBold)
		lc, ls := c.fitSans(gtx, line, 13, body.Dx()-2*dp(24), pg.muted, font.Normal)
		cy := body.Min.Y + dp(56)
		place(ops, hc, image.Pt(body.Min.X+(body.Dx()-hs.X)/2, cy))
		place(ops, lc, image.Pt(body.Min.X+(body.Dx()-ls.X)/2, cy+hs.Y+dp(4)))
		return
	}
	cx0, cx1 := r.Min.X+dp(16), r.Max.X-dp(16)
	cy := y + dp(14) - b.scroll
	if len(v.needs) > 0 {
		cy = win.drawBoardCard(gtx, cx0, cx1, cy, body, nil, v.needs, ink) + dp(12)
	}
	for i := range v.cards {
		g := &v.cards[i]
		cy = win.drawBoardCard(gtx, cx0, cx1, cy, body, g, g.rows, ink) + dp(12)
	}
	end = cy - dp(12) + dp(24) + b.scroll
	b.scroll = min(max(b.scroll, 0), max(0, end-body.Max.Y))
}

// drawBoardCard draws one card from y between x0 and x1: the Needs you card
// when g is nil, else g's session card. It returns the card's bottom.
func (win *window) drawBoardCard(gtx layout.Context, x0, x1, y int, body image.Rectangle, g *boardGroup, rows []boardRow, ink boardInk) int {
	b, c, ops, dp := &win.board, win.ui, gtx.Ops, gtx.Dp
	th := c.th
	needs := g == nil
	pal, fill, line, hover := ink.onCard, ink.card, ink.line, ink.raised
	if needs {
		pal, fill, line, hover = ink.onNeeds, ink.needsBG, ink.needsLine, ink.needsRaised
	}
	_, lh := c.sans(gtx, "Ag", 12.5, pal.text, font.SemiBold)
	headH := dp(10) + lh.Y + dp(8)
	meterH := 0
	if !needs {
		meterH = dp(3) + dp(6)
	}
	foldH, unfolded := 0, false
	if !needs && b.filter != boardDone && g.done > 0 {
		foldH, unfolded = dp(boardFoldDp), !g.folded
	}
	total := headH + meterH + len(rows)*dp(boardRowDp) + foldH
	rect := image.Rect(x0, y, x1, y+total)
	if !rect.Overlaps(body) {
		return rect.Max.Y // off screen: nothing drawn, nothing to hit
	}
	rad := dp(10)
	paint.FillShape(ops, fill, clip.UniformRRect(rect, rad).Op(ops))
	inside := clip.UniformRRect(rect, rad).Push(ops)
	hit := boardCardHit{rect: rect.Intersect(body)}

	// Head.
	hx, hr := x0+dp(14), x1-dp(14)
	mid := y + dp(10) + lh.Y/2
	if needs {
		b.texts = append(b.texts, "NEEDS YOU")
		call, sz := c.spaced(gtx, "NEEDS YOU", 11, pal.needs, dp(1))
		place(ops, call, image.Pt(hx, mid-sz.Y/2))
	} else {
		hit.tab = g.tab
		btn := func(label string) image.Rectangle {
			b.texts = append(b.texts, label)
			call, sz := c.sans(gtx, label, 11.5, pal.muted, font.Medium)
			r := image.Rect(hr-sz.X-2*dp(6), mid-sz.Y/2-dp(3), hr, mid+sz.Y/2+dp(3))
			place(ops, call, image.Pt(r.Min.X+dp(6), mid-sz.Y/2))
			hr = r.Min.X - dp(4)
			return r.Intersect(body)
		}
		hit.goTo = btn("Go to session ›")
		if g.meter.Done > 0 {
			hit.clear = btn("Clear")
		}
		logoPx := dp(12)
		if logo, img := agentLogo(g.agent, logoPx, agentLogoColor(g.agent, pal.text)); img != nil {
			off := op.Offset(image.Pt(hx, mid-logoPx/2)).Push(ops)
			logo.Add(ops)
			paint.PaintOp{}.Add(ops)
			off.Pop()
			hx += logoPx + dp(8)
		}
		b.texts = append(b.texts, g.title)
		tc, ts := c.fitSans(gtx, g.title, 12.5, max(hr-dp(8)-hx, 0), pal.text, font.SemiBold)
		place(ops, tc, image.Pt(hx, mid-ts.Y/2))
		hx += ts.X + dp(10)
		if g.meta != "" && hr-dp(8)-hx > 0 {
			b.texts = append(b.texts, g.meta)
			mc, ms := c.fitSide(gtx, g.meta, 11, hr-dp(8)-hx, pal.muted)
			place(ops, mc, image.Pt(hx, mid-ms.Y/2))
		}
		// The state meter: every agent of the session, needs, running and done.
		my := y + headH
		m := image.Rect(x0+dp(14), my, x1-dp(14), my+dp(3))
		n, run, done := meterSplit(g.meter, m.Dx(), dp(2))
		mx := m.Min.X
		for _, part := range [...]struct {
			w   int
			col color.NRGBA
		}{{n, th.Needs}, {run, th.Accent}, {done, ink.meterDone}} {
			if part.w > 0 {
				paint.FillShape(ops, part.col, clip.UniformRRect(image.Rect(mx, m.Min.Y, mx+part.w, m.Max.Y), dp(1)).Op(ops))
				mx += part.w + dp(2)
			}
		}
		hit.meter = m.Intersect(body)
	}

	// Rows.
	ry := y + headH + meterH
	for i, row := range rows {
		rr := image.Rect(x0, ry, x1, ry+dp(boardRowDp))
		ry = rr.Max.Y
		if !rr.Overlaps(body) {
			continue
		}
		x := win.drawBoardRow(gtx, rr, row, i == 0, pal, line, hover)
		h := boardRowHit{row: row, rect: rr.Intersect(body)}
		if !x.Empty() {
			h.close = x.Intersect(body)
		}
		b.rows = append(b.rows, h)
	}

	// The fold: "✓ N finished ›" or "Hide finished".
	if foldH > 0 {
		fr := image.Rect(x0, ry, x1, ry+foldH)
		paint.FillShape(ops, line, clip.Rect(image.Rect(x0, ry, x1, ry+1)).Op())
		fx := x0 + dp(40)
		if unfolded {
			b.texts = append(b.texts, "Hide finished")
			call, sz := c.sans(gtx, "Hide finished", 11.5, pal.muted, font.Normal)
			place(ops, call, image.Pt(fx, fr.Min.Y+(fr.Dy()-sz.Y)/2))
		} else {
			rest := strconv.Itoa(g.done) + " finished ›"
			b.texts = append(b.texts, "✓ "+rest)
			kc, ks := c.sans(gtx, "✓", 11.5, pal.done, font.Normal)
			place(ops, kc, image.Pt(fx, fr.Min.Y+(fr.Dy()-ks.Y)/2))
			call, sz := c.sans(gtx, rest, 11.5, pal.muted, font.Normal)
			place(ops, call, image.Pt(fx+ks.X+dp(6), fr.Min.Y+(fr.Dy()-sz.Y)/2))
		}
		hit.fold = fr.Intersect(body)
	}
	inside.Pop()
	stroke(ops, rect, rad, line)
	if !needs {
		b.cardHits = append(b.cardHits, hit)
	}
	return rect.Max.Y
}

// drawBoardRow draws one agent row in rr and returns its × box, empty unless
// it is a done row that is hovered or selected.
func (win *window) drawBoardRow(gtx layout.Context, rr image.Rectangle, row boardRow, first bool, pal dockPalette, line, hover color.NRGBA) (close image.Rectangle) {
	b, c, ops, dp := &win.board, win.ui, gtx.Ops, gtx.Dp
	is := func(k subagents.Key) bool { return !row.session && k != (subagents.Key{}) && k == row.key }
	sel, hov := is(b.sel), is(b.hover)
	if sel || hov {
		paint.FillShape(ops, hover, clip.Rect(rr).Op())
	}
	if sel {
		paint.FillShape(ops, win.ui.th.Accent, clip.Rect(image.Rect(rr.Min.X, rr.Min.Y, rr.Min.X+dp(2), rr.Max.Y)).Op())
	}
	if !first {
		paint.FillShape(ops, line, clip.Rect(image.Rect(rr.Min.X, rr.Min.Y, rr.Max.X, rr.Min.Y+1)).Op())
	}
	x, right := rr.Min.X+dp(14), rr.Max.X-dp(14)
	taskCol := pal.text
	if row.state == subagents.Done {
		taskCol = pal.muted
	}
	tk := row.task
	if tk == "" {
		tk = row.typ
	}
	_, line1 := c.label(gtx, "Ag", 12.5, taskCol)
	ty := rr.Min.Y + dp(6)
	mid := ty + line1.Y/2

	mk := markNone
	switch row.state {
	case subagents.Waiting:
		mk = markNeeds
	case subagents.Done:
		mk = markDone
	}
	d := dp(7)
	dx := x + (dp(16)-d)/2
	c.stateDot(gtx, image.Rect(dx, mid-d/2, dx+d, mid-d/2+d), mk, row.state == subagents.Running, false)
	x += dp(16) + dp(10)

	b.texts = append(b.texts, row.typ)
	yc, ys := c.fitSide(gtx, row.typ, 12, dp(84), pal.kind)
	place(ops, yc, image.Pt(x, mid-ys.Y/2))
	x += dp(84) + dp(10)

	if row.state == subagents.Done && !row.session && (sel || is(b.hover)) {
		xc, xs := c.sans(gtx, "×", 14, pal.muted, font.Normal)
		place(ops, xc, image.Pt(right-xs.X, mid-xs.Y/2))
		close = image.Rect(right-xs.X-dp(8), rr.Min.Y, rr.Max.X, rr.Max.Y)
		right -= xs.X + dp(10)
	}
	if row.time != "" {
		b.texts = append(b.texts, row.time)
		ec, es := c.label(gtx, row.time, 11, pal.muted)
		place(ops, ec, image.Pt(right-es.X, mid-es.Y/2))
	}
	b.texts = append(b.texts, tk)
	kc, ks := c.fitSide(gtx, tk, 12.5, max(right-dp(54)-dp(10)-x, 0), taskCol)
	place(ops, kc, image.Pt(x, mid-ks.Y/2))
	if row.step != "" {
		b.texts = append(b.texts, row.step)
		sc, _ := c.fitSide(gtx, row.step, 11, max(right-x, 0), pal.muted)
		place(ops, sc, image.Pt(x, ty+line1.Y+dp(2)))
	}
	return close
}
