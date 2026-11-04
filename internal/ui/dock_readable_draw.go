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

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// Readable Activity sizes in dp (mockup docs/design/readable-activity-mockup.html).
const (
	readModeDp   = 26 // the mode row: ACTIVITY, the counts, Readable | Every step
	readRowDp    = 24 // a step row
	readChevDp   = 22 // where a step's words start, after the chevron
	readLineDp   = 18 // a detail line
	readCapDp    = 22 // a detail's "$ command" caption
	readInsetDp  = 24 // a detail box's left inset
	readNoteTime = 48 // room kept right of a note for its time
)

// readInk is the readable Activity's colours, all from the theme, each text
// nudged to 4.5:1 on the surface it is drawn on.
type readInk struct {
	say, sayBar, step, strong, mono, meta color.NRGBA // on the panel
	add, del, ok, bad                     color.NRGBA // meta tones on the panel
	detailBG, detailLine                  color.NRGBA // an open step's box
	dim, list, detailMono                 color.NRGBA // on the box
	addBG, addFG, delBG, delFG            color.NRGBA // diff lines
}

// readInk works the colours out for panel background bg.
func (c *chrome) readInk(bg color.NRGBA) readInk {
	th := c.th
	r := func(fg, on color.NRGBA) color.NRGBA { return render.Readable(fg, on) }
	k := readInk{
		sayBar:     agentLogoColor("claude", th.Accent),
		say:        r(th.ActiveFG, bg),
		step:       r(mix(th.Muted, th.ActiveFG, 0.35), bg),
		strong:     r(th.ActiveFG, bg),
		mono:       r(th.ActiveFG, bg),
		meta:       r(th.Muted, bg),
		add:        r(th.DiffAddFG, bg),
		del:        r(th.DiffDelFG, bg),
		ok:         r(th.Done, bg),
		bad:        r(th.UsageHigh, bg),
		detailBG:   th.Chrome,
		detailLine: th.Border,
		addBG:      th.DiffAddBG,
		delBG:      th.DiffDelBG,
	}
	k.dim = r(th.Muted, k.detailBG)
	k.list = r(mix(th.Muted, th.ActiveFG, 0.35), k.detailBG)
	k.detailMono = r(th.ActiveFG, k.detailBG)
	k.addFG = r(th.DiffAddFG, k.addBG)
	k.delFG = r(th.DiffDelFG, k.delBG)
	return k
}

// readHit is a drawn readable step with a detail: a click opens or closes it.
type readHit struct {
	id   string
	rect image.Rectangle
}

// readCache is a panel's Activity items, rebuilt only when what they are
// built from changes (Ruling 20): the transcript (its value and Version),
// the hook steps, the dropped count, the agent's state, availability.
type readCache struct {
	agent           subagents.Key
	tr              *subagents.Transcript
	ver             uint64
	nSteps, dropped int
	state           subagents.State
	trOK            bool
	items           []readItem // the readable list
	itemsOK         bool
	every           []dockItem // the Every step list
	everyOK         bool
	counts          string // "21 steps · 3 messages"
	heights         []int  // row heights, reused every frame
	builds          int    // list builds so far; tests count them

	ink   readInk
	inkOf theme.Chrome
	inkBG color.NRGBA
	inkOK bool
}

// readFresh drops the built lists when anything they come from changed.
func (s *dockSession) readFresh(a *subagents.Agent) {
	rc, tr := &s.read, s.tr
	var ver uint64
	ok := tr != nil && tr.Available
	if tr != nil {
		ver = tr.Version
	}
	if rc.agent == a.Key && rc.tr == tr && rc.ver == ver && rc.nSteps == len(a.Steps) && rc.dropped == a.Dropped && rc.state == a.State && rc.trOK == ok {
		return
	}
	rc.agent, rc.tr, rc.ver, rc.nSteps, rc.dropped, rc.state, rc.trOK = a.Key, tr, ver, len(a.Steps), a.Dropped, a.State, ok
	rc.itemsOK, rc.everyOK = false, false
	steps, msgs := readableCounts(a, tr)
	rc.counts = plural(steps, "step") + " · " + plural(msgs, "message")
}

// readable is a's readable list, built at most once per change.
func (s *dockSession) readable(a *subagents.Agent) []readItem {
	s.readFresh(a)
	rc := &s.read
	if !rc.itemsOK {
		rc.items = appendReadable(rc.items[:0], a, s.tr)
		for i := range rc.items {
			if it := &rc.items[i]; it.kind == readNote && !it.at.IsZero() {
				it.clock = it.at.Local().Format("15:04")
			}
		}
		rc.itemsOK = true
		rc.builds++
		s.pruneNotes()
		for id := range s.open {
			found := false
			for i := range rc.items {
				found = found || rc.items[i].id == id
			}
			if !found {
				delete(s.open, id)
			}
		}
	}
	return rc.items
}

// everyItems is today's list (dockTimeline), built at most once per change.
func (s *dockSession) everyItems(a *subagents.Agent) []dockItem {
	s.readFresh(a)
	rc := &s.read
	if !rc.everyOK {
		rc.every = dockTimeline(a, s.tr)
		rc.everyOK = true
		rc.builds++
		s.pruneNotes()
	}
	return rc.every
}

// pruneNotes drops the laid-out notes the transcript no longer has.
func (s *dockSession) pruneNotes() {
	for text := range s.cache.notes {
		found := false
		if s.tr != nil {
			for i := range s.tr.Messages {
				if s.tr.Messages[i].Text == text {
					found = true
					break
				}
			}
		}
		if !found {
			delete(s.cache.notes, text)
		}
	}
}

// noteLayout is a note laid out width px wide, from the panel's note cache.
func (win *window) noteLayout(gtx layout.Context, s *dockSession, text string, width int) *docLayout {
	if s.cache.notes == nil {
		s.cache.notes = map[string]*mdCache{}
	}
	mc := s.cache.notes[text]
	if mc == nil {
		mc = &mdCache{}
		s.cache.notes[text] = mc
	}
	return win.mdLayout(gtx, &s.cache, mc, text, width)
}

func (s *dockSession) inkFor(c *chrome, bg color.NRGBA) *readInk {
	rc := &s.read
	if !rc.inkOK || rc.inkOf != c.th || rc.inkBG != bg {
		rc.ink, rc.inkOf, rc.inkBG, rc.inkOK = c.readInk(bg), c.th, bg, true
	}
	return &rc.ink
}

// drawActMode draws the Activity's mode row at the top of inner (mockup
// .tools) and returns what is left below it.
func (win *window) drawActMode(gtx layout.Context, s *dockSession, a *subagents.Agent, inner image.Rectangle, ink *readInk, h *dockHits, texts *[]string) image.Rectangle {
	ops, c, th, dp := gtx.Ops, win.ui, win.ui.th, gtx.Dp
	s.readFresh(a)
	h.mode = image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, min(inner.Min.Y+dp(readModeDp), inner.Max.Y))
	mid := h.mode.Min.Y + h.mode.Dy()/2
	put := func(call op.CallOp, sz image.Point, x int, s string) int {
		place(ops, call, image.Pt(x, mid-sz.Y/2))
		*texts = append(*texts, s)
		return x + sz.X
	}
	x := h.mode.Min.X
	lc, ls := c.sans(gtx, "ACTIVITY", 11, ink.meta, font.SemiBold)
	x = put(lc, ls, x, "ACTIVITY") + dp(10)
	rCall, rSize := c.sans(gtx, "Readable", 12, ink.strong, font.Normal)
	eCall, eSize := c.sans(gtx, "Every step", 12, ink.strong, font.Normal)
	segW := rSize.X + eSize.X + dp(4) + 2*dp(20)
	seg := image.Rect(h.mode.Max.X-segW, mid-dp(11), h.mode.Max.X, mid+dp(11))
	cc, cs := c.fitSans(gtx, s.read.counts, 12, max(seg.Min.X-dp(10)-x, 0), ink.meta, font.Normal)
	put(cc, cs, x, s.read.counts)
	if seg.Min.X < x {
		h.modeRead, h.modeEvery = image.Rectangle{}, image.Rectangle{}
		return image.Rect(inner.Min.X, h.mode.Max.Y, inner.Max.X, inner.Max.Y)
	}
	paint.FillShape(ops, tint(th.Border, 0x80), clip.UniformRRect(seg, dp(7)).Op(ops))
	h.modeRead = image.Rect(seg.Min.X+dp(2), seg.Min.Y+dp(2), seg.Min.X+dp(2)+rSize.X+dp(20), seg.Max.Y-dp(2))
	h.modeEvery = image.Rect(h.modeRead.Max.X, seg.Min.Y+dp(2), seg.Max.X-dp(2), seg.Max.Y-dp(2))
	on, offCol := h.modeRead, ink.meta
	if win.actEvery {
		on = h.modeEvery
	}
	paint.FillShape(ops, th.Chrome, clip.UniformRRect(on, dp(5)).Op(ops))
	rc, ec := ink.strong, offCol
	if win.actEvery {
		rc, ec = offCol, ink.strong
	}
	if rc != ink.strong { // the colour changed: lay out again in it
		rCall, rSize = c.sans(gtx, "Readable", 12, rc, font.Normal)
	}
	if ec != ink.strong {
		eCall, eSize = c.sans(gtx, "Every step", 12, ec, font.Normal)
	}
	put(rCall, rSize, h.modeRead.Min.X+(h.modeRead.Dx()-rSize.X)/2, "Readable")
	put(eCall, eSize, h.modeEvery.Min.X+(h.modeEvery.Dx()-eSize.X)/2, "Every step")
	return image.Rect(inner.Min.X, h.mode.Max.Y+dp(4), inner.Max.X, inner.Max.Y)
}

// readItemHeight is an item's height this frame: notes from their cached
// layout, steps with their open detail.
func (win *window) readItemHeight(gtx layout.Context, s *dockSession, it *readItem, noteW int) int {
	dp := gtx.Dp
	switch it.kind {
	case readNote:
		return win.noteLayout(gtx, s, it.text, noteW).height + dp(14)
	case readStep:
		if it.hasDetail() && s.open[it.id] {
			return dp(readRowDp) + readDetailHeight(gtx, it) + dp(10)
		}
		return dp(readRowDp)
	case readWaiting:
		return dp(30)
	}
	return dp(readRowDp)
}

func readDetailHeight(gtx layout.Context, it *readItem) int {
	dp := gtx.Dp
	hgt := dp(12) + len(it.detail.lines)*dp(readLineDp)
	if it.detail.cap != "" {
		hgt += dp(readCapDp)
	}
	return hgt
}

// drawReadable draws the readable list in inner: the agent's notes with the
// coral bar, one row per step with its meta on the right, open details.
func (win *window) drawReadable(gtx layout.Context, s *dockSession, a *subagents.Agent, inner image.Rectangle, ink *readInk, bc dockPalette, h *dockHits, texts *[]string) {
	ops, c, th, dp := gtx.Ops, win.ui, win.ui.th, gtx.Dp
	items := s.readable(a)
	noteW := max(inner.Dx()-dp(14)-dp(readNoteTime), 1)
	hs := s.read.heights[:0]
	total := 0
	for i := range items {
		hgt := win.readItemHeight(gtx, s, &items[i], noteW)
		hs = append(hs, hgt)
		total += hgt
	}
	s.read.heights = hs
	h.scrollMax = max(total-inner.Dy(), 0)
	if s.follow {
		s.scroll = h.scrollMax
	}
	s.scroll = min(max(s.scroll, 0), h.scrollMax)
	cl := clip.Rect(inner).Push(ops)
	defer cl.Pop()
	y := inner.Min.Y - s.scroll
	x0 := inner.Min.X
	for i := range items {
		it := &items[i]
		top := y
		y += hs[i]
		if y < inner.Min.Y || top > inner.Max.Y {
			continue
		}
		rowMid := top + dp(readRowDp)/2
		put := func(call op.CallOp, sz image.Point, x, mid int, s string) int {
			place(ops, call, image.Pt(x, mid-sz.Y/2))
			*texts = append(*texts, s)
			return x + sz.X
		}
		switch it.kind {
		case readNote:
			paint.FillShape(ops, ink.sayBar, clip.UniformRRect(image.Rect(x0, top+dp(6), x0+dp(2), y-dp(8)), dp(1)).Op(ops))
			l := &s.cache.notes[it.text].lay
			view := image.Rect(x0+dp(14), top+dp(4), x0+dp(14)+noteW, y-dp(10)).Intersect(inner)
			win.drawPanelMarkdown(gtx, l, view, image.Pt(x0+dp(14), top+dp(4)), 0, h, texts)
			if it.clock != "" {
				call, sz := c.label(gtx, it.clock, 11, ink.meta)
				place(ops, call, image.Pt(inner.Max.X-sz.X, top+dp(4)))
				*texts = append(*texts, it.clock)
			}
		case readStep:
			win.drawReadStep(gtx, s, it, image.Rect(x0, top, inner.Max.X, y), inner, ink, h, texts)
		case readWaiting:
			box := image.Rect(x0, top+dp(3), inner.Max.X, y-dp(3))
			paint.FillShape(ops, tint(th.Needs, 0x22), clip.UniformRRect(box, dp(5)).Op(ops))
			wc := c.dockColors(mix(win.pal.BG, th.Needs, 0x22/255.0))
			call, sz := c.fitSans(gtx, "⏸ "+it.text, 12, box.Dx()-dp(20), wc.needs, font.Normal)
			put(call, sz, x0+dp(10), box.Min.Y+box.Dy()/2, "⏸ "+it.text)
		case readFinished:
			call, sz := c.sans(gtx, "✓ "+it.text+" · ", 12, bc.done, font.Normal)
			x := put(call, sz, x0, rowMid, "✓ "+it.text+" · ")
			call, sz = c.sans(gtx, "read the report", 12, bc.accent, font.Normal)
			end := put(call, sz, x, rowMid, "read the report")
			h.finish = image.Rect(x, top, end, y).Intersect(inner)
		case readLive:
			d := dp(10)
			ring := image.Rect(x0+dp(3), rowMid-d/2, x0+dp(3)+d, rowMid-d/2+d)
			paint.FillShape(ops, bc.accent, clip.Ellipse(ring).Op(ops))
			paint.FillShape(ops, win.pal.BG, clip.Ellipse(ring.Inset(dp(2))).Op(ops))
			call, sz := c.sans(gtx, it.text, 13, ink.step, font.Normal)
			put(call, sz, x0+dp(readChevDp), rowMid, it.text)
		case readDropped:
			call, sz := c.sans(gtx, it.text, 12, ink.meta, font.Normal)
			put(call, sz, x0, rowMid, it.text)
		}
	}
}

// drawReadStep draws one step in r: chevron, verb, object, meta and, when
// open, its detail box. Its click target is kept inside list.
func (win *window) drawReadStep(gtx layout.Context, s *dockSession, it *readItem, r, list image.Rectangle, ink *readInk, h *dockHits, texts *[]string) {
	ops, c, dp := gtx.Ops, win.ui, gtx.Dp
	row := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+dp(readRowDp))
	mid := row.Min.Y + row.Dy()/2
	open := it.hasDetail() && s.open[it.id]
	if it.hasDetail() {
		chev := "›"
		if open {
			chev = "⌄"
		}
		call, sz := c.sans(gtx, chev, 12, ink.meta, font.Normal)
		place(ops, call, image.Pt(row.Min.X+dp(3), mid-sz.Y/2))
		if hit := row.Intersect(list); !hit.Empty() {
			h.steps = append(h.steps, readHit{id: it.id, rect: hit})
		}
	}
	// Meta first, right-aligned, so the words can be cut to what is left.
	var calls [2]op.CallOp
	var sizes [2]image.Point
	metas := it.metas()
	mw := 0
	for i, m := range metas {
		calls[i], sizes[i] = c.label(gtx, m.text, 11, toneInk(ink, m.tone))
		mw += sizes[i].X
		if i > 0 {
			mw += dp(6)
		}
	}
	mx := row.Max.X - mw
	for i, m := range metas {
		place(ops, calls[i], image.Pt(mx, mid-sizes[i].Y/2))
		*texts = append(*texts, m.text)
		mx += sizes[i].X + dp(6)
	}
	x := row.Min.X + dp(readChevDp)
	room := row.Max.X - mw - dp(10) - x
	if it.verb != "" {
		call, sz := c.fitSans(gtx, it.verb, 13, room, ink.step, font.Normal)
		place(ops, call, image.Pt(x, mid-sz.Y/2))
		*texts = append(*texts, it.verb)
		x += sz.X + dp(4)
		room -= sz.X + dp(4)
	}
	if it.obj != "" && room > dp(16) {
		var call op.CallOp
		var sz image.Point
		if it.objMono {
			call, sz = c.fitSide(gtx, it.obj, 12, room, ink.mono)
		} else {
			call, sz = c.fitSans(gtx, it.obj, 13, room, ink.strong, font.SemiBold)
		}
		place(ops, call, image.Pt(x, mid-sz.Y/2))
		*texts = append(*texts, it.obj)
	}
	if !open {
		return
	}
	box := image.Rect(r.Min.X+dp(readInsetDp), row.Max.Y+dp(2), r.Max.X, row.Max.Y+dp(2)+readDetailHeight(gtx, it))
	rad := dp(8)
	paint.FillShape(ops, ink.detailLine, clip.UniformRRect(box, rad).Op(ops))
	paint.FillShape(ops, ink.detailBG, clip.UniformRRect(box.Inset(1), max(rad-1, 0)).Op(ops))
	bcl := clip.UniformRRect(box.Inset(1), max(rad-1, 0)).Push(ops)
	defer bcl.Pop()
	y := box.Min.Y
	tx := box.Min.X + dp(10)
	tw := max(box.Dx()-dp(20), 0)
	if it.detail.cap != "" {
		call, sz := c.fitSans(gtx, it.detail.cap, 11, tw, ink.dim, font.Normal)
		place(ops, call, image.Pt(tx, y+(dp(readCapDp)-sz.Y)/2))
		*texts = append(*texts, it.detail.cap)
		y += dp(readCapDp)
		paint.FillShape(ops, ink.detailLine, clip.Rect(image.Rect(box.Min.X, y-1, box.Max.X, y)).Op())
	}
	y += dp(6)
	clipR := h.body
	for _, ln := range it.detail.lines {
		lr := image.Rect(box.Min.X, y, box.Max.X, y+dp(readLineDp))
		y += dp(readLineDp)
		if !clipR.Empty() && (lr.Max.Y < clipR.Min.Y || lr.Min.Y > clipR.Max.Y) {
			continue
		}
		lmid := lr.Min.Y + lr.Dy()/2
		x, w, col := tx, tw, ink.dim
		prefix := ""
		switch ln.Kind {
		case '+':
			paint.FillShape(ops, ink.addBG, clip.Rect(lr).Op())
			col, prefix = ink.addFG, "+ "
		case '-':
			paint.FillShape(ops, ink.delBG, clip.Rect(lr).Op())
			col, prefix = ink.delFG, "− "
		case 'l':
			col = ink.list
		}
		if prefix != "" {
			call, sz := c.label(gtx, prefix, 12, col)
			place(ops, call, image.Pt(x, lmid-sz.Y/2))
			*texts = append(*texts, prefix)
			x += sz.X
			w -= sz.X
		}
		call, sz := c.fitSide(gtx, ln.Text, 12, max(w, 0), col)
		place(ops, call, image.Pt(x, lmid-sz.Y/2))
		*texts = append(*texts, ln.Text)
	}
}

func toneInk(ink *readInk, tone uint8) color.NRGBA {
	switch tone {
	case toneAdd:
		return ink.add
	case toneDel:
		return ink.del
	case toneOK:
		return ink.ok
	case toneBad:
		return ink.bad
	}
	return ink.meta
}

// drawEveryStep draws today's list (Ruling 19) from the cached items.
func (win *window) drawEveryStep(gtx layout.Context, s *dockSession, a *subagents.Agent, inner image.Rectangle, bc dockPalette, h *dockHits, texts *[]string) {
	ops, c, th := gtx.Ops, win.ui, win.ui.th
	dp := gtx.Dp
	items := s.everyItems(a)
	rowH := dp(dockRowDp)
	noteW := max(inner.Dx()-dp(12), 1)
	hs := s.read.heights[:0]
	total := 0
	for _, it := range items {
		hgt := rowH
		switch it.kind {
		case itemNote:
			hgt = win.noteLayout(gtx, s, it.text, noteW).height + dp(6)
		case itemWaiting:
			hgt = dp(30)
		}
		hs = append(hs, hgt)
		total += hgt
	}
	s.read.heights = hs
	h.scrollMax = max(total-inner.Dy(), 0)
	if s.follow {
		s.scroll = h.scrollMax
	}
	s.scroll = min(max(s.scroll, 0), h.scrollMax)
	cl := clip.Rect(inner).Push(ops)
	defer cl.Pop()
	y := inner.Min.Y - s.scroll
	for i, it := range items {
		top := y
		y += hs[i]
		if y < inner.Min.Y || top > inner.Max.Y {
			continue
		}
		mid := top + rowH/2
		put := func(s string, x int, col color.NRGBA, mono bool, room int) int {
			var call op.CallOp
			var sz image.Point
			if mono {
				call, sz = c.fitSide(gtx, s, 11, room, col)
			} else {
				call, sz = c.sans(gtx, s, 11, col, font.Normal)
			}
			place(ops, call, image.Pt(x, mid-sz.Y/2))
			*texts = append(*texts, s)
			return x + sz.X
		}
		x := inner.Min.X
		switch it.kind {
		case itemStep:
			x = put(it.at.Local().Format("15:04"), x, bc.muted, true, inner.Dx()) + dp(10)
			x = put(it.tool, x, bc.accent, true, inner.Max.X-x) + dp(10)
			put(it.target, x, bc.text, true, inner.Max.X-x)
		case itemNote:
			paint.FillShape(ops, th.Accent, clip.Rect(image.Rect(x, top+dp(2), x+dp(2), y-dp(4))).Op())
			l := &s.cache.notes[it.text].lay
			view := image.Rect(x+dp(12), top+dp(2), inner.Max.X, y-dp(4)).Intersect(inner)
			win.drawPanelMarkdown(gtx, l, view, image.Pt(x+dp(12), top+dp(2)), 0, h, texts)
		case itemWaiting:
			box := image.Rect(x, top+dp(3), inner.Max.X, y-dp(3))
			paint.FillShape(ops, tint(th.Needs, 0x22), clip.UniformRRect(box, dp(5)).Op(ops))
			wc := c.dockColors(mix(win.pal.BG, th.Needs, 0x22/255.0))
			mid = box.Min.Y + box.Dy()/2
			put("⏸ "+it.text, x+dp(10), wc.needs, false, 0)
		case itemFinished:
			x = put("✓ "+it.text+" · ", x, bc.done, false, 0)
			end := put("read the report", x, bc.accent, false, 0)
			h.finish = image.Rect(x, top, end, y).Intersect(inner)
		case itemLive:
			x = put(time.Now().Format("15:04"), x, bc.muted, true, inner.Dx()) + dp(10)
			put("… "+it.text, x, bc.muted, true, inner.Max.X-x)
		case itemDropped:
			put(it.text, x, bc.muted, false, 0)
		}
	}
}

// readPress handles a press on the Activity's mode buttons or a readable
// step with a detail, in the dock or the board preview (h is that view's
// hits). It reports whether it took the press.
func (win *window) readPress(s *dockSession, h *dockHits, p image.Point) bool {
	switch {
	case p.In(h.modeRead), p.In(h.modeEvery):
		if every := p.In(h.modeEvery); every != win.actEvery {
			win.actEvery = every
			win.invalidate()
		}
		return true
	case win.actEvery:
		return false // every step: nothing opens
	}
	for _, x := range h.steps {
		if p.In(x.rect) {
			if s.open == nil {
				s.open = map[string]bool{}
			}
			// Following would snap the list to its new bottom and slide the
			// clicked row away; only the last step keeps following.
			if s.follow && x.id != lastStepID(s.read.items) {
				s.follow = false
			}
			if s.open[x.id] {
				delete(s.open, x.id)
			} else {
				s.open[x.id] = true
			}
			win.invalidate()
			return true
		}
	}
	return false
}

// lastStepID is the id of the last step in items, "" when there is none.
func lastStepID(items []readItem) string {
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].kind == readStep {
			return items[i].id
		}
	}
	return ""
}

// dropTranscript lets go of s's transcript and of the lists built from it,
// so nothing keeps the old transcript alive.
func (s *dockSession) dropTranscript() {
	s.tr = nil
	if rc := &s.read; rc.tr != nil {
		rc.tr, rc.items, rc.every, rc.itemsOK, rc.everyOK = nil, nil, nil, false, false
		s.pruneNotes()
	}
}
