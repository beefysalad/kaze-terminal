package ui

import (
	"image"
	"image/color"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// alertCard is what one alert card shows (issue #9): the mark, a context line
// (branch and slash command), the session's sidebar name, the body and a status.
type alertCard struct {
	tab  tabs.TabID
	run  uint64
	kind notify.Kind
	mark string // "claude", "codex", "workspace" or "" (no agent known)

	ctx, title, body, status string
	branch, cmd              string // ctx's parts, drawn in their own colours
}

// cardOf is the card for one toast. A session's title follows the sidebar row's
// rule (never the bare shell title); a run summary keeps its own.
func (win *window) cardOf(t toast) alertCard {
	c := alertCard{tab: t.Tab, run: t.Run, kind: t.Kind, body: t.Text, mark: t.Agent}
	c.status = cardStatus(t.Kind, t.Text, t.Ran, t.Run != 0)
	if t.Run != 0 {
		c.title, c.ctx = t.Title, "workspace"
		c.body = strings.TrimPrefix(t.Text, ": ") // the summary's text follows the name the title shows
		if c.mark == "" {
			c.mark = "workspace"
		}
		return c
	}
	title, _, branch := win.sessionMeta(t.Tab)
	c.title = title
	if c.mark == "" {
		c.mark = win.rowAgent(t.Tab).agentName()
	}
	c.cmd = win.agentOf(t.Tab).command
	if branch != "" {
		c.branch = "⎇ " + branch
	}
	switch {
	case c.branch != "" && c.cmd != "":
		c.ctx = c.branch + " · " + c.cmd
	case c.branch != "":
		c.ctx = c.branch
	default:
		c.ctx = c.cmd
	}
	return c
}

// cardStatus is the card's status line: why a session waits, how long a done
// turn ran, or that a workspace run finished.
func cardStatus(kind notify.Kind, reason string, ran time.Duration, run bool) string {
	switch {
	case run:
		return "Run finished"
	case kind == notify.KindNeeds:
		r := strings.ToLower(reason)
		if strings.Contains(r, "permission") || strings.Contains(r, "approve") {
			return "Needs your permission"
		}
		return "Waiting for your answer"
	case ran >= time.Second:
		return "Done · ran " + ranText(ran)
	}
	return "Done"
}

// ranText is a turn's length the way a person says it: "45s", "6m 40s", "1h 2m".
func ranText(d time.Duration) string {
	s := int64(d / time.Second)
	h, m := s/3600, s/60%60
	switch {
	case h > 0:
		return strconv.FormatInt(h, 10) + "h " + strconv.FormatInt(m, 10) + "m"
	case m > 0:
		return strconv.FormatInt(m, 10) + "m " + strconv.FormatInt(s%60, 10) + "s"
	}
	return strconv.FormatInt(s, 10) + "s"
}

// cardInk is the alert cards' colours for one theme: the surface from the
// theme's toast tokens, every text nudged to at least 4.5:1 on what it sits on.
type cardInk struct {
	bg, line, tile, capBG, capLine                             color.NRGBA
	title, body, muted, branch, cmd, go_, needs, done, badgeFG color.NRGBA
}

func (c *chrome) cardInk() cardInk {
	th := c.th
	k := cardInk{
		bg:      th.ToastBG,
		line:    th.ToastLine,
		tile:    mix(th.ToastBG, th.ToastLine, 0.6),
		capBG:   th.Active,
		capLine: th.ToastLine,
	}
	on := func(fg color.NRGBA, bgs ...color.NRGBA) color.NRGBA {
		for range 2 { // each nudge moves away from the surfaces, so a second pass settles
			for _, bg := range bgs {
				fg = render.Readable(fg, bg)
			}
		}
		return fg
	}
	k.title = on(th.ToastTitle, k.bg)
	k.body = on(th.ToastFG, k.bg, k.capBG) // also the key caps' text
	k.muted = on(th.Muted, k.bg)
	k.branch = on(th.Done, k.bg)
	k.cmd = on(th.ToastGo, k.bg)
	k.go_ = k.cmd
	k.needs = on(th.Needs, k.bg)
	k.done = on(th.Done, k.bg)
	k.badgeFG = on(th.ToastBG, k.needs, k.done)
	return k
}

// cardInk is cardInk for the current theme, worked out once per theme.
func (win *window) cardInk() cardInk {
	a := &win.alerts
	if !a.inkOK || a.inkOf != win.ui.th {
		a.ink, a.inkOf, a.inkOK = win.ui.cardInk(), win.ui.th, true
	}
	return a.ink
}

// cardWidthDp is an alert card's width (issue #9 mockup); a narrower terminal
// area gets narrower cards, and none under cardMinDp.
const (
	cardWidthDp = 380
	cardMinDp   = 260
)

// cardWrap is a card body's two lines, kept while the text, the room, the
// metric and the theme stay the same, so a still card measures nothing.
type cardWrap struct {
	ok       bool
	src      string
	width    int
	metric   unit.Metric
	th       theme.Chrome
	lines    [2]string
	measures int // text measures made, for tests
}

// cardText is one text a card drew and where, for tests.
type cardText struct {
	s string
	r image.Rectangle
}

// wrapTwo splits s into two lines for room px: the first is the longest prefix
// that ends on a word (or, for one long word, on a rune) and fits, the second
// the rest cut with "…". A binary search bounds the measures.
func (c *chrome) wrapTwo(gtx layout.Context, s string, size unit.Sp, room int, w *cardWrap) [2]string {
	if w.ok && w.src == s && w.width == room && w.metric == gtx.Metric && w.th == c.th {
		return w.lines
	}
	*w = cardWrap{ok: true, src: s, width: room, metric: gtx.Metric, th: c.th, measures: w.measures}
	width := func(t string) int {
		w.measures++
		_, sz := c.sans(gtx, t, size, c.th.ToastFG, font.Normal)
		return sz.X
	}
	if s == "" || width(s) <= room {
		w.lines[0] = s
		return w.lines
	}
	// Candidate ends of line 1: after each word, else after each rune.
	var ends []int
	for i := 1; i < len(s); i++ {
		if s[i] == ' ' && s[i-1] != ' ' {
			ends = append(ends, i)
		}
	}
	if len(ends) == 0 || width(s[:ends[0]]) > room {
		ends = ends[:0]
		for i := range s {
			if i > 0 {
				ends = append(ends, i)
			}
		}
	}
	lo, hi := 0, len(ends)-1 // the answer is the last end that fits, or none
	best := -1
	for steps := 0; lo <= hi && steps < 32; steps++ {
		mid := (lo + hi) / 2
		if width(s[:ends[mid]]) <= room {
			best, lo = mid, mid+1
		} else {
			hi = mid - 1
		}
	}
	rest := s
	if best >= 0 {
		w.lines[0], rest = s[:ends[best]], strings.TrimLeft(s[ends[best]:], " ")
	}
	w.lines[1] = c.cutSans(gtx, rest, size, room, font.Normal, &w.measures)
	return w.lines
}

// cutSans is s in the UI font cut with a trailing "…" to fit room px; it adds
// its measures to n.
func (c *chrome) cutSans(gtx layout.Context, s string, size unit.Sp, room int, wt font.Weight, n *int) string {
	for i := 0; i < 8 && s != ""; i++ {
		*n++
		_, sz := c.sans(gtx, s, size, c.th.ToastFG, wt)
		if sz.X <= room {
			return s
		}
		k := utf8.RuneCountInString(s)
		keep := min(k*max(room, 0)/max(sz.X, 1), k-1)
		if keep < 2 {
			return ""
		}
		s = cutRunes(s, keep)
	}
	return s
}

// nextWaitingCaps is the next_waiting shortcut as key-cap labels, worked out
// again only when the binding changes; nil when it is unbound.
func (win *window) nextWaitingCaps() []string {
	a := &win.alerts
	sc, ok := win.binds["next_waiting"]
	if a.capsOK && a.capsOf == sc && a.capsOn == ok {
		return a.caps
	}
	a.capsOK, a.capsOf, a.capsOn, a.caps = true, sc, ok, nil
	if ok {
		for _, p := range strings.Split(formatShortcut(runtime.GOOS, sc), "+") {
			if p == "" {
				p = "+" // the key itself was "+"
			}
			a.caps = append(a.caps, p)
		}
	}
	return a.caps
}

// drawToasts draws the alert cards bottom right of area, the terminal's part of
// the window right of the sidebar: newest nearest the corner, the stack growing
// up, above the dock panel when it shows. When the room runs out the oldest are
// left out; in an area too narrow for a card, all of them (the sidebar's marks
// still show).
func (win *window) drawToasts(gtx layout.Context, area image.Rectangle) {
	c, a := win.ui, &win.alerts
	c.toastHits, a.texts = c.toastHits[:0], a.texts[:0]
	if len(a.toasts) == 0 {
		return
	}
	dp := gtx.Dp
	margin, gap := dp(12), dp(8)
	w := min(dp(cardWidthDp), area.Dx()-2*margin)
	if w < dp(cardMinDp) {
		return
	}
	bottom := area.Max.Y - margin
	if d := win.dockRect(); !d.Empty() && d.Min.Y-gap < bottom {
		bottom = d.Min.Y - gap
	}
	top := area.Min.Y + gap
	ink := win.cardInk()
	caps := win.nextWaitingCaps()
	for i := len(a.toasts) - 1; i >= 0; i-- { // newest first, lowest
		h := win.drawCard(gtx, &a.toasts[i], image.Rect(area.Max.X-margin-w, top, area.Max.X-margin, bottom), ink, caps, i == len(a.toasts)-1)
		if h == 0 {
			break // no room: the older cards are left out
		}
		bottom -= h + gap
	}
}

// drawCard draws one card with its bottom on room.Max.Y and returns its height;
// 0, drawing nothing, when room is too short for it. Layout from the mockup's
// .card: padding 12/14/11/12, a 34 dp mark column, 11 dp to the text.
func (win *window) drawCard(gtx layout.Context, t *toast, room image.Rectangle, ink cardInk, caps []string, newest bool) int {
	c, a, ops, dp := win.ui, &win.alerts, gtx.Ops, gtx.Dp
	k := win.cardOf(*t)
	padT, padR, padB, padL := dp(12), dp(14), dp(11), dp(12)
	line := max(dp(1), 1)
	colX := room.Min.X + padL + dp(34) + dp(11)
	colW := room.Max.X - padR - colX
	if colW <= 0 {
		return 0
	}

	// Measure and record every text first: the height decides where it goes.
	type txt struct {
		call op.CallOp
		sz   image.Point
		s    string
	}
	rec := func(call op.CallOp, sz image.Point, s string) txt { return txt{call, sz, s} }
	sans := func(s string, size unit.Sp, col color.NRGBA, wt font.Weight) txt {
		call, sz := c.sans(gtx, s, size, col, wt)
		return txt{call, sz, s}
	}
	mono := func(s string, size unit.Sp, col color.NRGBA) txt {
		call, sz := c.label(gtx, s, size, col)
		return txt{call, sz, s}
	}
	var ctx [3]txt // branch, "·", command
	ctxH := 0
	ctxRoom := colW - dp(20)
	if k.cmd != "" && k.run == 0 {
		ctx[2] = rec(c.fitSideText(gtx, k.cmd, 11, ctxRoom, ink.cmd, font.Normal))
	}
	if k.branch != "" {
		r := ctxRoom
		if ctx[2].s != "" {
			ctx[1] = mono("·", 11, ink.muted)
			r -= ctx[2].sz.X + ctx[1].sz.X + 2*dp(6)
		}
		ctx[0] = rec(c.fitSideText(gtx, k.branch, 11, r, ink.branch, font.Normal))
		if ctx[0].s == "" {
			ctx[1] = txt{}
		}
	} else if k.run != 0 {
		ctx[0] = rec(c.fitSideText(gtx, k.ctx, 11, ctxRoom, ink.muted, font.Normal))
	}
	for _, x := range ctx {
		ctxH = max(ctxH, x.sz.Y)
	}
	var measured int
	title := sans(c.cutSans(gtx, k.title, 13.5, colW-dp(6), font.Bold, &measured), 13.5, ink.title, font.Bold)
	var body [2]txt
	if k.body != "" {
		for j, s := range c.wrapTwo(gtx, k.body, 12.5, colW, &t.wrap) {
			if s != "" {
				body[j] = sans(s, 12.5, ink.body, font.Normal)
			}
		}
	}
	status := sans(k.status, 12, ink.muted, font.Normal)
	open := sans("Open session", 12, ink.go_, font.SemiBold)
	x := sans("×", 14, ink.muted, font.Normal)
	var capT [4]txt
	capH, capPad, capGap := dp(16)+line, dp(5), dp(6)
	capsW := 0
	if newest {
		for j, s := range caps[:min(len(caps), len(capT))] {
			capT[j] = mono(s, 10.5, ink.body)
			capH = max(capH, capT[j].sz.Y+line)
			if j > 0 {
				capsW += capGap
			}
			capsW += capT[j].sz.X + 2*capPad
		}
	}
	footH := max(open.sz.Y, capH)

	h := padT
	if ctxH > 0 {
		h += ctxH + dp(2)
	}
	h += title.sz.Y
	bodyH := 0
	for _, b := range body {
		bodyH += b.sz.Y
	}
	if bodyH > 0 {
		h += dp(3) + bodyH
	}
	h += dp(5) + status.sz.Y + dp(7) + line + dp(7) + footH + padB
	h = max(h, padT+dp(2)+dp(30)+dp(5)+padB) // the mark and its badge always fit
	if h > room.Dy() {
		return 0
	}

	r := image.Rect(room.Min.X, room.Max.Y-h, room.Max.X, room.Max.Y)
	radius := dp(10)
	paint.FillShape(ops, ink.line, clip.UniformRRect(r, radius).Op(ops))
	paint.FillShape(ops, ink.bg, clip.UniformRRect(r.Inset(line), max(radius-line, 0)).Op(ops))
	put := func(x txt, at image.Point) {
		if x.s == "" {
			return
		}
		place(ops, x.call, at)
		a.texts = append(a.texts, cardText{x.s, image.Rectangle{Min: at, Max: at.Add(x.sz)}})
	}

	// The mark: a tile with the agent's logo or the workspace icon, and a badge.
	tile := image.Rect(r.Min.X+padL, r.Min.Y+padT+dp(2), r.Min.X+padL+dp(30), r.Min.Y+padT+dp(32))
	paint.FillShape(ops, ink.tile, clip.UniformRRect(tile, dp(8)).Op(ops))
	lp := dp(18)
	lr := image.Rectangle{Min: tile.Min.Add(image.Pt((tile.Dx()-lp)/2, (tile.Dy()-lp)/2))}
	lr.Max = lr.Min.Add(image.Pt(lp, lp))
	switch k.mark {
	case "workspace":
		c.icon(gtx, "workspaces", lr, ink.title)
	case "":
		c.icon(gtx, "terminal", lr, ink.title)
	default:
		if logo, img := agentLogo(k.mark, lp, agentLogoColor(k.mark, ink.title)); img != nil {
			off := op.Offset(lr.Min).Push(ops)
			logo.Add(ops)
			paint.PaintOp{}.Add(ops)
			off.Pop()
		}
	}
	bd, ring := dp(15), dp(2)
	bo := dp(5) - ring - bd/2 // the ringed badge sticks out 5 dp past the tile's corner
	bc := image.Pt(tile.Max.X+bo, tile.Max.Y+bo)
	paint.FillShape(ops, ink.bg, clip.Ellipse(image.Rect(bc.X-bd/2-ring, bc.Y-bd/2-ring, bc.X+bd-bd/2+ring, bc.Y+bd-bd/2+ring)).Op(ops))
	badgeBG, glyph := ink.done, "✓"
	if k.kind == notify.KindNeeds && k.run == 0 {
		badgeBG, glyph = ink.needs, "!"
	}
	paint.FillShape(ops, badgeBG, clip.Ellipse(image.Rect(bc.X-bd/2, bc.Y-bd/2, bc.X+bd-bd/2, bc.Y+bd-bd/2)).Op(ops))
	g := sans(glyph, 9, ink.badgeFG, font.ExtraBold)
	place(ops, g.call, bc.Sub(g.sz.Div(2)))

	// The text column, clipped so a mismeasured label never leaves the card.
	cl := clip.Rect(image.Rect(colX, r.Min.Y+line, r.Max.X-line, r.Max.Y-line)).Push(ops)
	y := r.Min.Y + padT
	if ctxH > 0 {
		cx := colX
		for j, p := range ctx {
			if p.s == "" {
				continue
			}
			if j > 0 && cx > colX {
				cx += dp(6)
			}
			put(p, image.Pt(cx, y+(ctxH-p.sz.Y)/2))
			cx += p.sz.X
		}
		y += ctxH + dp(2)
	}
	put(title, image.Pt(colX, y))
	y += title.sz.Y
	if bodyH > 0 {
		y += dp(3)
		for _, b := range body {
			put(b, image.Pt(colX, y))
			y += b.sz.Y
		}
	}
	y += dp(5)
	pip := dp(6)
	pipCol := ink.done
	if k.kind == notify.KindNeeds {
		pipCol = ink.needs
	}
	py := y + (status.sz.Y-pip)/2
	paint.FillShape(ops, pipCol, clip.Ellipse(image.Rect(colX, py, colX+pip, py+pip)).Op(ops))
	put(status, image.Pt(colX+pip+dp(6), y))
	y += status.sz.Y + dp(7)
	paint.FillShape(ops, ink.line, clip.Rect(image.Rect(colX, y, r.Max.X-padR, y+line)).Op())
	y += line + dp(7)
	put(open, image.Pt(colX, y+(footH-open.sz.Y)/2))
	if capsW > 0 && open.sz.X+dp(12)+capsW <= colW {
		cx := r.Max.X - padR - capsW
		for _, ct := range capT {
			if ct.s == "" {
				continue
			}
			box := image.Rect(cx, y+(footH-capH)/2, cx+ct.sz.X+2*capPad, y+(footH-capH)/2+capH)
			rr := dp(4)
			paint.FillShape(ops, ink.capLine, clip.UniformRRect(box, rr).Op(ops))
			inner := image.Rect(box.Min.X+line, box.Min.Y+line, box.Max.X-line, box.Max.Y-2*line) // a 2 px bottom edge, as a key
			paint.FillShape(ops, ink.capBG, clip.UniformRRect(inner, max(rr-line, 0)).Op(ops))
			put(ct, image.Pt(box.Min.X+capPad, inner.Min.Y+(inner.Dy()-ct.sz.Y)/2))
			cx = box.Max.X + capGap
		}
	}
	cl.Pop()

	// × in a 20 dp box, 7 dp from the top and 8 dp from the right.
	xb := image.Rect(r.Max.X-dp(8)-dp(20), r.Min.Y+dp(7), r.Max.X-dp(8), r.Min.Y+dp(27))
	put(x, xb.Min.Add(xb.Size().Sub(x.sz).Div(2)))
	win.ui.toastHits = append(win.ui.toastHits, toastHit{id: k.tab, run: k.run, rect: r, closeRect: xb})
	return h
}
