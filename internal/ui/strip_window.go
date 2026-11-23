package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
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

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// stripLayKey is everything the strip's layout depends on; an equal key means
// the cached layout is still right.
type stripLayKey struct {
	builds uint64 // the sidebar model's generation: titles, marks and tab set
	space  string
	active tabs.TabID
	width  int
	scroll int
	px     float32
}

// stripDrawKey is everything the strip's ops depend on.
type stripDrawKey struct {
	lay       stripLayKey
	hover     stripHitKind
	hoverID   tabs.TabID
	theme     theme.Chrome
	termBG    color.NRGBA
	shaper    *text.Shaper
	metric    unit.Metric
	locale    string
	generated bool
}

// stripState is the tab strip: its rect, cached layout and ops, hover and the
// last used tab of each space. UI goroutine only.
type stripState struct {
	tag      bool
	rect     image.Rectangle // this frame's strip in window pixels (empty: none)
	lay      tabStripLayout  // as last drawn; the pointer reads it
	layKey   stripLayKey
	layOK    bool
	scroll   int
	hover    stripHitKind
	hoverID  tabs.TabID
	spaceKey string // the space the strip shows; kept while a workspace member is shown
	last     map[string]tabs.TabID
	ops      op.Ops
	call     op.CallOp
	key      stripDrawKey
	keyOK    bool
	draws    uint64
}

// spaceIndex is the index in spaces of the space holding id, or -1.
func spaceIndex(spaces []spaceEntry, id tabs.TabID) int {
	for i, s := range spaces {
		if slices.Contains(s.Tabs, id) {
			return i
		}
	}
	return -1
}

// spaces is the current spaces, from the sidebar cache.
func (win *window) spaces() []spaceEntry {
	win.cachedSide(time.Now())
	return win.side.cache.spaces.Spaces
}

// spaceKeyOf is the key of the space holding id, "" for none.
func (win *window) spaceKeyOf(id tabs.TabID) string {
	sp := win.spaces()
	if i := spaceIndex(sp, id); i >= 0 {
		return sp[i].Key
	}
	return ""
}

// spaceTarget is the tab selecting sp shows: its last used one, else its first.
func (win *window) spaceTarget(sp spaceEntry) tabs.TabID {
	if id, ok := win.strip.last[sp.Key]; ok && slices.Contains(sp.Tabs, id) {
		return id
	}
	if len(sp.Tabs) > 0 {
		return sp.Tabs[0]
	}
	return 0
}

// selectSpace shows the last used tab of the space with this key.
func (win *window) selectSpace(key string) {
	for _, sp := range win.spaces() {
		if sp.Key == key {
			if id := win.spaceTarget(sp); id != 0 {
				win.selectSession(id)
			}
			return
		}
	}
}

// spaceFolder is where a new tab in sp starts: the repo's main checkout, else
// the folder of the space's last used tab; "" means the default start folder.
func (win *window) spaceFolder(sp spaceEntry) string {
	if sp.Key == shellSpaceKey {
		return ""
	}
	if filepath.Base(sp.Key) == ".git" {
		dir := filepath.Dir(sp.Key)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	if t := win.mgr.Lookup(win.spaceTarget(sp)); t != nil {
		return win.locOf(t).cwd
	}
	return ""
}

// rowByID finds a session's row in a model built without a filter.
func rowByID(m sideModel, id tabs.TabID) (sideRow, bool) {
	for _, g := range m.Groups {
		for _, r := range g.Rows {
			if r.ID == id {
				return r, true
			}
		}
	}
	for _, r := range m.Loose {
		if r.ID == id {
			return r, true
		}
	}
	return sideRow{}, false
}

// stripEvents takes the strip's pointer input: hover, wheel and clicks.
func (win *window) stripEvents(gtx layout.Context) {
	rng := pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}
	kinds := pointer.Press | pointer.Scroll | pointer.Move | pointer.Enter | pointer.Leave
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &win.strip.tag, Kinds: kinds, ScrollX: rng, ScrollY: rng})
		if !ok {
			return
		}
		if e, ok := ev.(pointer.Event); ok && !win.bench.IgnoreInput() {
			win.stripPointer(e)
		}
	}
}

func (win *window) stripPointer(e pointer.Event) {
	s := &win.strip
	p := e.Position.Round()
	switch e.Kind {
	case pointer.Move, pointer.Enter, pointer.Leave:
		h := stripHit{}
		if e.Kind != pointer.Leave {
			h = s.lay.Hit(p)
		}
		var id tabs.TabID
		if h.Kind == hitTab || h.Kind == hitClose {
			id = s.lay.Tabs[h.Index].ID
		}
		if h.Kind != s.hover || id != s.hoverID {
			s.hover, s.hoverID = h.Kind, id
			win.w.Invalidate()
		}
	case pointer.Scroll:
		if !s.lay.Scrolls() {
			return
		}
		d := int(e.Scroll.X + e.Scroll.Y)
		if sc := min(max(s.scroll+d, 0), s.lay.MaxScroll); sc != s.scroll {
			s.scroll = sc
			win.w.Invalidate()
		}
	case pointer.Press:
		primary, middle := e.Buttons.Contain(pointer.ButtonPrimary), e.Buttons.Contain(pointer.ButtonTertiary)
		if !primary && !middle {
			return
		}
		h := s.lay.Hit(p)
		switch {
		case h.Kind == hitPlus && primary:
			win.newTabInSpace()
		case h.Kind == hitClose && primary, h.Kind == hitTab && middle:
			win.closeTab(s.lay.Tabs[h.Index].ID)
		case h.Kind == hitTab && primary:
			win.selectTab(s.lay.Tabs[h.Index].ID)
		}
		win.w.Invalidate()
	}
}

// newTabInSpace opens a tab in the shown space's folder.
func (win *window) newTabInSpace() {
	dir := ""
	sp := win.spaces()
	for _, s := range sp {
		if s.Key == win.strip.spaceKey {
			dir = win.spaceFolder(s)
			break
		}
	}
	if dir == "" {
		dir = win.plainStartDir()
	}
	win.newTabIn(dir)
}

// drawStrip lays out and draws the tab strip into r (window pixels) and takes
// its pointer input there. Ops and layout are cached: an unchanged frame
// replays them and allocates nothing.
func (win *window) drawStrip(gtx layout.Context, r image.Rectangle) {
	s := &win.strip
	s.rect = r
	if r.Empty() || win.mgr == nil {
		s.lay, s.layOK, s.keyOK = tabStripLayout{}, false, false
		s.hover, s.hoverID = hitNone, 0
		return
	}
	win.cachedSide(gtx.Now)
	c := &win.side.cache
	spaces := c.spaces.Spaces
	var active tabs.TabID
	if a := win.mgr.Active(); a != nil {
		active = a.ID
	}
	if i := spaceIndex(spaces, active); i >= 0 {
		s.spaceKey = spaces[i].Key
		if s.last == nil {
			s.last = map[string]tabs.TabID{}
		}
		s.last[s.spaceKey] = active
	}
	var ids []tabs.TabID
	for _, sp := range spaces {
		if sp.Key == s.spaceKey {
			ids = sp.Tabs
			break
		}
	}
	lk := stripLayKey{builds: c.builds, space: s.spaceKey, active: active, width: r.Dx(), scroll: s.scroll, px: gtx.Metric.PxPerDp}
	if !s.layOK || lk != s.layKey {
		// Scrolling away from the active tab sticks; only a new active tab or space scrolls it into view.
		reveal := active
		if s.layOK && lk.active == s.layKey.active && lk.space == s.layKey.space {
			reveal = 0
		}
		s.lay = layoutTabStrip(newStripMetrics(gtx.Metric.PxPerDp), r.Dx(), ids, reveal, s.scroll)
		s.scroll = s.lay.Scroll
		lk.scroll = s.scroll
		s.layKey, s.layOK = lk, true
	}
	if s.hover == hitTab || s.hover == hitClose {
		if !slices.ContainsFunc(s.lay.Tabs, func(t stripTab) bool { return t.ID == s.hoverID }) {
			s.hover, s.hoverID = hitNone, 0 // the hovered tab is gone
		}
	}
	key := stripDrawKey{lay: s.layKey, hover: s.hover, hoverID: s.hoverID, theme: win.ui.th, termBG: win.pal.BG,
		shaper: win.ui.shaper, metric: gtx.Metric, locale: gtx.Locale.Language, generated: true}
	if !s.keyOK || key != s.key {
		s.ops.Reset()
		cg := gtx
		cg.Ops = &s.ops
		cg.Constraints = layout.Exact(r.Size())
		rec := op.Record(cg.Ops)
		win.ui.layoutStrip(cg, c.base, s.lay, active, s.hover, s.hoverID, win.pal.BG)
		s.call = rec.Stop()
		s.key, s.keyOK = key, true
		s.draws++
	}
	ops := gtx.Ops
	off := op.Offset(r.Min).Push(ops)
	s.call.Add(ops)
	area := clip.Rect{Max: r.Size()}.Push(ops)
	event.Op(ops, &s.tag)
	area.Pop()
	off.Pop()
}

// layoutStrip draws the strip's tabs and + in strip-local coordinates.
func (c *chrome) layoutStrip(gtx layout.Context, m sideModel, lay tabStripLayout, active tabs.TabID, hover stripHitKind, hoverID tabs.TabID, termBG color.NRGBA) {
	ops, th := gtx.Ops, c.sideTheme()
	size := gtx.Constraints.Max
	paint.FillShape(ops, th.Chrome, clip.Rect(image.Rectangle{Max: size}).Op())
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())

	inset, top, rad, gap := gtx.Dp(1), gtx.Dp(4), gtx.Dp(9), gtx.Dp(8)
	logoPx, dot := gtx.Dp(14), gtx.Dp(7)
	cl := clip.Rect(image.Rect(0, 0, lay.ViewW, size.Y)).Push(ops)
	for _, t := range lay.Tabs {
		if t.Rect.Max.X <= 0 || t.Rect.Min.X >= lay.ViewW {
			continue
		}
		on := t.ID == active
		hot := t.ID == hoverID && (hover == hitTab || hover == hitClose)
		body := image.Rect(t.Rect.Min.X+inset, top, t.Rect.Max.X-inset, size.Y)
		shape := clip.RRect{Rect: body, NW: rad, NE: rad}
		fg := th.Muted
		switch {
		case on:
			fg = th.ActiveFG
			paint.FillShape(ops, th.Border, shape.Op(ops))
			in := image.Rect(body.Min.X+inset, body.Min.Y+inset, body.Max.X-inset, body.Max.Y)
			paint.FillShape(ops, termBG, clip.RRect{Rect: in, NW: rad - inset, NE: rad - inset}.Op(ops))
			ax := gtx.Dp(10)
			paint.FillShape(ops, th.Accent, clip.RRect{Rect: image.Rect(body.Min.X+ax, body.Min.Y, body.Max.X-ax, body.Min.Y+gtx.Dp(2)), SE: gtx.Dp(2), SW: gtx.Dp(2)}.Op(ops))
		case hot:
			fg = th.ActiveFG
			paint.FillShape(ops, th.Active, shape.Op(ops))
		}
		row, _ := rowByID(m, t.ID)
		mid := (top + size.Y) / 2
		x := body.Min.X + gtx.Dp(12)
		if logo, img := agentLogo(row.Agent, logoPx, agentLogoColor(row.Agent, fg)); img != nil {
			o := op.Offset(image.Pt(x, mid-logoPx/2)).Push(ops)
			logo.Add(ops)
			paint.PaintOp{}.Add(ops)
			o.Pop()
			x += logoPx + gap
		} else {
			c.icon(gtx, "terminal", image.Rect(x, mid-logoPx/2, x+logoPx, mid+logoPx/2), fg)
			x += logoPx + gap
		}
		right := t.Close.Min.X - gap/2
		if row.Mark != markNone || row.Running {
			d := image.Rect(right-dot, mid-dot/2, right, mid-dot/2+dot)
			c.sideDot(gtx, d, row.Mark, row.Running)
			right = d.Min.X - gap
		}
		if room := right - x; room > 0 {
			w := font.Normal
			if on {
				w = font.Bold
			}
			call, sz := c.fitSideWeight(gtx, row.Title, 12, room, fg, w)
			place(ops, call, image.Pt(x, mid-sz.Y/2))
		}
		cr := image.Rect(t.Close.Min.X, mid-t.Close.Dy()/2, t.Close.Max.X, mid-t.Close.Dy()/2+t.Close.Dy()) // centred on the tab body
		if hover == hitClose && hoverID == t.ID {
			paint.FillShape(ops, th.Active, clip.UniformRRect(cr, gtx.Dp(5)).Op(ops))
		}
		ix := gtx.Dp(5)
		c.icon(gtx, "close", cr.Inset(ix), fg)
	}
	cl.Pop()

	pr := lay.Plus
	if hover == hitPlus {
		paint.FillShape(ops, th.Active, clip.UniformRRect(image.Rect(pr.Min.X+gtx.Dp(3), pr.Min.Y+gtx.Dp(6), pr.Max.X-gtx.Dp(3), pr.Max.Y-gtx.Dp(6)), gtx.Dp(6)).Op(ops))
	}
	pi := min(gtx.Dp(14), pr.Dx(), pr.Dy())
	cx, cy := (pr.Min.X+pr.Max.X)/2, (top+size.Y)/2 // level with the tab titles
	fg := th.Muted
	if hover == hitPlus {
		fg = th.ActiveFG
	}
	c.icon(gtx, "plus", image.Rect(cx-pi/2, cy-pi/2, cx-pi/2+pi, cy-pi/2+pi), fg)
}
