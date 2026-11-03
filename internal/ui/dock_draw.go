package ui

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/patrick-gogo/kaze-terminal/internal/mdoc"
	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// Panel sizes in dp (mockup docs/design/agent-pane-dock-mockup.html).
const (
	dockHandleDp = 6
	dockHeadDp   = 32
	dockPromptDp = 24
	dockRowDp    = 20
	dockPadDp    = 12
)

// mdMarks drops inline markdown marks from a one-line preview.
var mdMarks = strings.NewReplacer("**", "", "__", "", "`", "")

// mdCache is one markdown text parsed and laid out, kept until the text,
// width, metric or shaper changes.
type mdCache struct {
	src    string
	width  int
	metric unit.Metric
	shaper *text.Shaper
	lay    docLayout
	ok     bool
}

// dockCache holds a panel's laid-out markdown. Notes are keyed by their text,
// since the transcript drops old messages off the front and indexes shift.
type dockCache struct {
	notes          map[string]*mdCache
	prompt, report mdCache
	parses         int // markdown parses so far; tests count them
}

// dockLink is a clickable link drawn in the panel.
type dockLink struct {
	r   image.Rectangle
	url string
}

// dockHits is where the panel's parts were drawn last frame, in window px.
type dockHits struct {
	panel, handle, header, actTab, repTab, copy, reader, close, prompt, finish, body image.Rectangle
	links                                                                            []dockLink
	scrollMax                                                                        int
}

// dockPalette is the panel's text colors on one background, each nudged to
// at least 4.5:1 so every theme stays readable.
type dockPalette struct{ text, muted, accent, needs, done, kind color.NRGBA }

func (c *chrome) dockColors(bg color.NRGBA) dockPalette {
	r := func(fg color.NRGBA) color.NRGBA { return render.Readable(fg, bg) }
	return dockPalette{text: r(c.th.ActiveFG), muted: r(c.th.Muted), accent: r(c.th.Accent), needs: r(c.th.Needs), done: r(c.th.Done), kind: r(c.memberColor("magenta"))}
}

// mdLayout is src laid out width px wide at the panel's scale, from c unless
// something it depends on changed.
func (win *window) mdLayout(gtx layout.Context, cache *dockCache, c *mdCache, src string, width int) *docLayout {
	if c.ok && c.src == src && c.width == width && c.metric == gtx.Metric && c.shaper == win.ui.shaper {
		return &c.lay
	}
	d := mdoc.Parse([]byte(src))
	cache.parses++
	win.fitDoc(d)
	c.lay = win.layoutMarkdown(gtx, d, width, panelStyle)
	c.src, c.width, c.metric, c.shaper, c.ok = src, width, gtx.Metric, win.ui.shaper, true
	return &c.lay
}

// dockReport is a's report for Copy and the Report tab: the panel's
// transcript chain when it has one, else the hook's (#85 fallback order).
func (win *window) dockReport(a *subagents.Agent) string {
	for _, s := range win.dock.by {
		if s.agent == a.Key && s.tr != nil && s.tr.Available {
			return s.tr.Report(a.Report)
		}
	}
	return new(subagents.Transcript).Report(a.Report)
}

// dockTick reads s's transcript at most once per agentTranscriptPoll while
// the panel is drawn, and says when to look again: never once a finished
// agent's transcript has been read after it ended. Drawing calls it only.
func (win *window) dockTick(s *dockSession, a *subagents.Agent, now time.Time) (time.Time, bool) {
	path := win.agentTranscriptPath(a)
	if path == "" {
		s.tr = nil
		return time.Time{}, false
	}
	if s.tr == nil || s.trPath != path || s.trStarted != a.Started {
		s.tr, s.trPath, s.trStarted, s.trAt = &subagents.Transcript{}, path, a.Started, time.Time{}
	}
	if !now.Before(s.trAt.Add(agentTranscriptPoll)) {
		_ = s.tr.Read(path)
		s.trAt = now
	}
	if a.State == subagents.Done && s.trAt.After(a.Ended) && s.tr.AtEnd() {
		return time.Time{}, false // read to the end after it finished: nothing more will come
	}
	return s.trAt.Add(agentTranscriptPoll), true
}

// drawDock draws session's panel for a in r and takes its input areas.
func (win *window) drawDock(gtx layout.Context, session tabs.TabID, s *dockSession, a *subagents.Agent, r image.Rectangle) {
	if at, ok := win.dockTick(s, a, gtx.Now); ok {
		gtx.Execute(op.InvalidateCmd{At: at})
	}
	if a.State == subagents.Done && !s.wasDone && !s.picked {
		s.tab = dockReport // finishing shows the report, unless the user picked a tab
	}
	s.wasDone = a.State == subagents.Done
	ops, c, th := gtx.Ops, win.ui, win.ui.th
	dp := gtx.Dp
	win.dock.texts = win.dock.texts[:0]
	h := dockHits{panel: r}
	paint.FillShape(ops, win.pal.BG, clip.Rect(r).Op())
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()

	// Handle: a thin bar with a resize cursor.
	h.handle = image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+dp(dockHandleDp))
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+max(dp(1), 1))).Op())
	grip := image.Rect(r.Min.X+r.Dx()/2-dp(18), h.handle.Min.Y+dp(2), r.Min.X+r.Dx()/2+dp(18), h.handle.Min.Y+dp(4))
	paint.FillShape(ops, th.Border, clip.UniformRRect(grip, dp(1)).Op(ops))

	// Header.
	h.header = image.Rect(r.Min.X, h.handle.Max.Y, r.Max.X, min(h.handle.Max.Y+dp(dockHeadDp), r.Max.Y))
	paint.FillShape(ops, th.Chrome, clip.Rect(h.header).Op())
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(h.header.Min.X, h.header.Max.Y-1, h.header.Max.X, h.header.Max.Y)).Op())
	hc := c.dockColors(th.Chrome)
	mid := h.header.Min.Y + h.header.Dy()/2
	text := func(call op.CallOp, sz image.Point, x int, s string) int {
		place(ops, call, image.Pt(x, mid-sz.Y/2))
		win.dock.texts = append(win.dock.texts, s)
		return x + sz.X
	}
	right := h.header.Max.X - dp(6)
	button := func(label string, size unit.Sp) image.Rectangle {
		call, sz := c.sans(gtx, label, size, hc.muted, font.Medium)
		b := image.Rect(right-sz.X-dp(16), h.header.Min.Y, right, h.header.Max.Y)
		text(call, sz, b.Min.X+dp(8), label)
		right = b.Min.X
		return b
	}
	h.close = button("×", 15)
	if a.State == subagents.Done {
		h.reader = button("Open in reader", 11)
		h.copy = button("Copy report", 11)
	}
	x := h.header.Min.X + dp(dockPadDp)
	mk := markNone
	switch a.State {
	case subagents.Waiting:
		mk = markNeeds
	case subagents.Done:
		mk = markDone
	}
	d := dp(8)
	c.stateDot(gtx, image.Rect(x, mid-d/2, x+d, mid-d/2+d), mk, a.State == subagents.Running, false)
	x += d + dp(8)
	call, sz := c.label(gtx, a.Type, 11, hc.kind)
	x = text(call, sz, x, a.Type) + dp(8)
	// The switch sits after the elapsed time; measure it first so the task can be cut to fit.
	el, elCol := agentElapsed(a, time.Now()), hc.muted
	if a.State == subagents.Waiting {
		el, elCol = "waiting for you", hc.needs
	}
	eCall, eSize := c.sans(gtx, el, 11, elCol, font.Normal)
	aCall, aSize := c.sans(gtx, "Activity", 11, hc.text, font.Medium)
	rCall, rSize := c.sans(gtx, "Report", 11, hc.text, font.Medium)
	segW := aSize.X + rSize.X + dp(40)
	room := max(right-dp(12)-segW-dp(12)-eSize.X-dp(8)-x, 0)
	tCall, tSize := c.fitSide(gtx, a.Task, 11, room, hc.text)
	x = text(tCall, tSize, x, a.Task) + dp(8)
	x = text(eCall, eSize, x, el) + dp(12)
	seg := image.Rect(x, mid-dp(11), x+segW, mid+dp(11))
	if seg.Max.X <= right {
		paint.FillShape(ops, tint(th.Border, 0x80), clip.UniformRRect(seg, dp(6)).Op(ops))
		h.actTab = image.Rect(seg.Min.X+dp(2), seg.Min.Y+dp(2), seg.Min.X+dp(2)+aSize.X+dp(18), seg.Max.Y-dp(2))
		h.repTab = image.Rect(h.actTab.Max.X, seg.Min.Y+dp(2), seg.Max.X-dp(2), seg.Max.Y-dp(2))
		on := h.actTab
		if s.tab == dockReport {
			on = h.repTab
		}
		paint.FillShape(ops, th.Chrome, clip.UniformRRect(on, dp(5)).Op(ops))
		text(aCall, aSize, h.actTab.Min.X+dp(9), "Activity")
		text(rCall, rSize, h.repTab.Min.X+dp(9), "Report")
	}

	// Prompt row, collapsed to one muted line.
	y := h.header.Max.Y
	bc := c.dockColors(win.pal.BG)
	inner := image.Rect(r.Min.X+dp(dockPadDp), y, r.Max.X-dp(dockPadDp), r.Max.Y)
	prompt := trimBlock(a.Prompt)
	if prompt != "" {
		h.prompt = image.Rect(r.Min.X, y, r.Max.X, y+dp(dockPromptDp))
		first, _, _ := strings.Cut(strings.TrimSpace(prompt), "\n")
		first = mdMarks.Replace(first) // a one-line preview: no raw markdown marks
		car := "▸ Prompt: "
		if s.promptOpen {
			car = "▾ Prompt"
			first = ""
		}
		pCall, pSize := c.fitSide(gtx, car+first, 11, inner.Dx(), bc.muted)
		place(ops, pCall, image.Pt(inner.Min.X, h.prompt.Min.Y+(h.prompt.Dy()-pSize.Y)/2))
		win.dock.texts = append(win.dock.texts, car+first)
		y = h.prompt.Max.Y
		if s.promptOpen {
			l := win.mdLayout(gtx, &s.cache, &s.cache.prompt, prompt, inner.Dx()-dp(10))
			view := image.Rect(inner.Min.X+dp(10), y, inner.Max.X, min(y+l.height, r.Max.Y))
			paint.FillShape(ops, th.Border, clip.Rect(image.Rect(inner.Min.X, y, inner.Min.X+dp(2), view.Max.Y)).Op())
			win.drawPanelMarkdown(gtx, l, view, view.Min, 0, &h)
			y = view.Max.Y + dp(6)
		}
	}

	// Body.
	h.body = image.Rect(r.Min.X, min(y, r.Max.Y), r.Max.X, r.Max.Y)
	inner = image.Rect(inner.Min.X, h.body.Min.Y+dp(6), inner.Max.X, h.body.Max.Y-dp(6))
	if inner.Dx() > 0 && inner.Dy() > 0 {
		if s.tab == dockReport {
			win.drawDockReport(gtx, s, a, inner, bc, &h)
		} else {
			win.drawDockActivity(gtx, s, a, inner, bc, &h)
		}
	}
	win.dock.hits = h

	// Input areas: the body and prompt first, then the header, then the handle on top.
	area := clip.Rect(h.body).Push(ops)
	event.Op(ops, &win.dock.body)
	area.Pop()
	for _, b := range []image.Rectangle{h.header, h.prompt} {
		if !b.Empty() {
			area = clip.Rect(b).Push(ops)
			event.Op(ops, &win.dock.head)
			area.Pop()
		}
	}
	for _, b := range []image.Rectangle{h.actTab, h.repTab, h.copy, h.reader, h.close, h.prompt, h.finish} {
		if !b.Empty() {
			area = clip.Rect(b).Push(ops)
			pointer.CursorPointer.Add(ops)
			area.Pop()
		}
	}
	for _, l := range h.links {
		area = clip.Rect(l.r).Push(ops)
		pointer.CursorPointer.Add(ops)
		area.Pop()
	}
	area = clip.Rect(h.handle).Push(ops)
	pointer.CursorRowResize.Add(ops)
	event.Op(ops, &win.dock.handle)
	area.Pop()
}

// drawPanelMarkdown draws l and keeps its links as panel hits.
func (win *window) drawPanelMarkdown(gtx layout.Context, l *docLayout, view image.Rectangle, origin image.Point, scroll int, h *dockHits) {
	var hits []docLinkHit
	win.drawMarkdown(gtx, l, view, origin, scroll, &hits, &win.dock.texts)
	for _, x := range hits {
		h.links = append(h.links, dockLink{r: x.rect, url: l.links[x.link]})
	}
}

// drawDockReport draws the Report tab.
func (win *window) drawDockReport(gtx layout.Context, s *dockSession, a *subagents.Agent, inner image.Rectangle, bc dockPalette, h *dockHits) {
	if a.State != subagents.Done {
		call, sz := win.ui.sans(gtx, "No report yet", 12, bc.muted, font.Normal)
		place(gtx.Ops, call, inner.Min)
		win.dock.texts = append(win.dock.texts, "No report yet")
		_ = sz
		return
	}
	l := win.mdLayout(gtx, &s.cache, &s.cache.report, win.dockReport(a), inner.Dx())
	h.scrollMax = max(l.height-inner.Dy(), 0)
	s.scroll = min(max(s.scroll, 0), h.scrollMax)
	win.drawPanelMarkdown(gtx, l, inner, inner.Min, s.scroll, h)
}

// drawDockActivity draws the Activity tab: rows, notes and the last row.
func (win *window) drawDockActivity(gtx layout.Context, s *dockSession, a *subagents.Agent, inner image.Rectangle, bc dockPalette, h *dockHits) {
	ops, c, th := gtx.Ops, win.ui, win.ui.th
	dp := gtx.Dp
	items := dockTimeline(a, s.tr)
	rowH := dp(dockRowDp)
	noteW := max(inner.Dx()-dp(12), 1)
	// Lay out the notes first (cached), dropping caches of notes that are gone.
	keep := make(map[string]*mdCache, len(s.cache.notes))
	heights := make([]int, len(items))
	total := 0
	for i, it := range items {
		switch it.kind {
		case itemNote:
			mc := s.cache.notes[it.text]
			if mc == nil {
				mc = &mdCache{}
			}
			keep[it.text] = mc
			heights[i] = win.mdLayout(gtx, &s.cache, mc, it.text, noteW).height + dp(6)
		case itemWaiting:
			heights[i] = dp(30)
		default:
			heights[i] = rowH
		}
		total += heights[i]
	}
	s.cache.notes = keep
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
		y += heights[i]
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
			win.dock.texts = append(win.dock.texts, s)
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
			win.drawPanelMarkdown(gtx, l, view, image.Pt(x+dp(12), top+dp(2)), 0, h)
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

// drawShownDock draws the shown session's panel in the rect dockFit laid out.
func (win *window) drawShownDock(gtx layout.Context) {
	owner := win.dockOwner()
	s := win.dockOf(owner)
	var a *subagents.Agent
	if s != nil {
		a = win.subs.Get(s.agent)
	}
	if s == nil || a == nil || win.dock.lastRect.Empty() {
		win.dock.hits = dockHits{}
		return
	}
	win.drawDock(gtx, owner, s, a, win.dock.lastRect)
}
