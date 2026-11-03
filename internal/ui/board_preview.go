package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// boardPreviewAgent is the agent the preview shows, nil when none is selected
// or it is gone.
func (win *window) boardPreviewAgent() *subagents.Agent {
	if win.board.sel == (subagents.Key{}) {
		return nil
	}
	return win.subs.Get(win.board.sel)
}

// drawBoardPreview draws the selected agent in r (mockup .ph, .acts, .pbody): its
// state, type and time, task, session, actions, then its Activity or Report
// with the docked panel's timeline and report layout. The transcript is read
// only here, with the panel's polling rules.
func (win *window) drawBoardPreview(gtx layout.Context, r image.Rectangle, ink boardInk) {
	b, c, ops, dp := &win.board, win.ui, gtx.Ops, gtx.Dp
	b.openIn, b.copy, b.reader = image.Rectangle{}, image.Rectangle{}, image.Rectangle{}
	b.prevHits = dockHits{}
	pg := ink.page
	rec := func(s string) { b.texts = append(b.texts, s) }
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()
	a := win.boardPreviewAgent()
	if a == nil {
		head, line := "Pick an agent", "Its steps, messages and report show here."
		rec(head)
		rec(line)
		hc, hs := c.sans(gtx, head, 14, pg.text, font.SemiBold)
		lc, ls := c.fitSans(gtx, line, 13, r.Dx()-2*dp(24), pg.muted, font.Normal)
		y := r.Min.Y + dp(56)
		place(ops, hc, image.Pt(r.Min.X+(r.Dx()-hs.X)/2, y))
		place(ops, lc, image.Pt(r.Min.X+(r.Dx()-ls.X)/2, y+hs.Y+dp(4)))
		return
	}
	if b.prev.agent != a.Key {
		b.prev = dockSession{agent: a.Key, follow: true}
	}
	if at, ok := win.dockTick(&b.prev, a, gtx.Now); ok {
		gtx.Execute(op.InvalidateCmd{At: at})
	}

	x0, x1 := r.Min.X+dp(24), r.Max.X-dp(24)
	y := r.Min.Y + dp(20)

	// Eyebrow: the state, the type in a pill, the elapsed time.
	state, stCol := "RUNNING", pg.accent
	switch a.State {
	case subagents.Waiting:
		state, stCol = "NEEDS YOU", pg.needs
	case subagents.Done:
		state, stCol = "DONE", pg.done
	}
	rec(state)
	sc, ss := c.spaced(gtx, state, 11, stCol, dp(1))
	pillH := ss.Y + 2*dp(2)
	mid := y + pillH/2
	place(ops, sc, image.Pt(x0, mid-ss.Y/2))
	x := x0 + ss.X + dp(8)
	dc, ds := c.label(gtx, "·", 11, pg.muted)
	place(ops, dc, image.Pt(x, mid-ds.Y/2))
	x += ds.X + dp(8)
	rec(a.Type)
	tc, ts := c.fitSide(gtx, a.Type, 11, max((x1-x)/2, 0), pg.kind)
	pr := image.Rect(x, mid-pillH/2, x+ts.X+2*dp(7), mid-pillH/2+pillH)
	pill(gtx, pr, ink.raised, ink.line, dp(6))
	place(ops, tc, image.Pt(pr.Min.X+dp(7), mid-ts.Y/2))
	x = pr.Max.X + dp(8)
	el := agentElapsed(a, time.Now())
	rec(el)
	ec, es := c.label(gtx, el, 11, pg.muted)
	place(ops, ec, image.Pt(x, mid-es.Y/2))
	y += pillH + dp(10)

	// The task as a title, then where it runs.
	task := a.Task
	if task == "" {
		task = a.Type
	}
	rec(task)
	kc, ks := c.fitSans(gtx, task, 17, x1-x0, pg.text, font.SemiBold)
	place(ops, kc, image.Pt(x0, y))
	y += ks.Y + dp(4)
	title, repo := win.sessionName(tabs.TabID(a.Tab))
	x = x0
	for _, part := range [...]struct {
		s   string
		col color.NRGBA
		w   font.Weight
	}{{"In ", pg.muted, font.Normal}, {title, pg.text, font.Medium}, {" · " + repo, pg.muted, font.Normal}} {
		if part.s == "" || part.s == " · " || x >= x1 {
			continue
		}
		rec(part.s)
		call, sz := c.fitSans(gtx, part.s, 12, x1-x, part.col, part.w)
		place(ops, call, image.Pt(x, y))
		x += sz.X
		mid = sz.Y
	}
	y += mid + dp(14)

	// Actions.
	bx := x0
	button := func(label string, fill, line, fg color.NRGBA) image.Rectangle {
		rec(label)
		call, sz := c.sans(gtx, label, 12, fg, font.Medium)
		br := image.Rect(bx, y, bx+sz.X+2*dp(12), y+dp(boardFieldDp))
		if br.Max.X > x1 {
			return image.Rectangle{}
		}
		pill(gtx, br, fill, line, dp(7))
		place(ops, call, image.Pt(br.Min.X+dp(12), br.Min.Y+(br.Dy()-sz.Y)/2))
		bx = br.Max.X + dp(8)
		return br
	}
	b.openIn = button("Open in session", ink.priBG, ink.priBG, ink.priFG)
	if a.State == subagents.Done {
		b.copy = button("Copy report", ink.raised, ink.line, pg.text)
		b.reader = button("Open in reader", ink.raised, ink.line, pg.text)
	}
	y += dp(boardFieldDp) + dp(16)
	paint.FillShape(ops, ink.line, clip.Rect(image.Rect(r.Min.X, y, r.Max.X, y+1)).Op())
	y++

	// Body on the terminal's background, as the docked panel draws it.
	body := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y)
	paint.FillShape(ops, win.pal.BG, clip.Rect(body).Op())
	bc := c.dockColors(win.pal.BG)
	sect := "ACTIVITY"
	if a.State == subagents.Done {
		sect = "REPORT"
	}
	rec(sect)
	lc, ls := c.spaced(gtx, sect, 11, bc.muted, dp(1))
	place(ops, lc, image.Pt(x0, body.Min.Y+dp(16)))
	inner := image.Rect(x0, body.Min.Y+dp(16)+ls.Y+dp(10), x1, body.Max.Y-dp(16))
	h := dockHits{body: body}
	if inner.Dx() > 0 && inner.Dy() > 0 {
		if a.State == subagents.Done {
			win.drawDockReport(gtx, &b.prev, a, inner, bc, &h, &b.texts)
		} else {
			win.drawDockActivity(gtx, &b.prev, a, inner, bc, &h, &b.texts)
		}
	}
	b.prevHits = h
}
