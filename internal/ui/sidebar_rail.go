package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/beefysalad/kaze-terminal/internal/render"
)

// The rail's parts in dp: its tiles, the gap between items, the group labels and
// the workspace members' bars.
const (
	railTile    = 36
	railGap     = 4
	railPad     = 8
	railLabelH  = 14
	railLabelW  = 44
	railLogo    = 18
	railDisc    = 14
	railDot     = 11
	railBarW    = 4
	railBarH    = 16
	railBarGap  = 3
	railDivW    = 24
	railTipGap  = 6
	railTipMaxW = 280
)

type railKind uint8

const (
	railLabel railKind = iota
	railRow
	railDivider
)

// railItem is one thing in the rail's scrolled list, y px below the list's top.
type railItem struct {
	kind railKind
	y    int
	name string   // a label's group name
	row  *sideRow // a tile's session
}

// railItems lays the groups and loose rows out top to bottom: a label and the tiles
// per group, a divider between groups. Folds are ignored, so every session has a
// tile. total is the list's height.
func railItems(gtx layout.Context, m sideModel) (items []railItem, total int) {
	gap, tile := gtx.Dp(railGap), gtx.Dp(railTile)
	y, seen := 0, false
	divider := func() {
		y += gtx.Dp(railGap)
		items = append(items, railItem{kind: railDivider, y: y})
		y += 1 + gtx.Dp(railGap) + gap
	}
	rows := func(rs []sideRow) {
		for i := range rs {
			items = append(items, railItem{kind: railRow, y: y, row: &rs[i]})
			y += tile + gap
		}
	}
	for _, g := range m.Groups {
		if len(g.Rows) == 0 {
			continue
		}
		if seen {
			divider()
		}
		seen = true
		items = append(items, railItem{kind: railLabel, y: y, name: g.Name})
		y += gtx.Dp(railLabelH) + gap
		rows(g.Rows)
	}
	if len(m.Loose) > 0 {
		if seen {
			divider()
		}
		rows(m.Loose)
	}
	return items, y
}

// railWord is a tile's status in its tooltip: needs you, done, its agents, running.
func railWord(r sideRow) string {
	switch {
	case r.Mark == markNeeds:
		return "needs you"
	case r.Mark == markDone:
		return "done"
	case r.TrayTotal > 0:
		return agentChip(r.TrayTotal)
	case r.AgentCount > 0:
		return agentChip(r.AgentCount)
	case r.Running:
		return "running"
	}
	return ""
}

// railTipFor is what the hovered tile's tooltip shows.
type railTipFor struct {
	tile          image.Rectangle
	mark          mark
	running       bool
	title, word   string
	wordCol       color.NRGBA
	member, found bool
}

// layoutRail paints the 52 dp rail into size and returns where its parts are: the
// session tiles in rows, the members' bars in members, so the sidebar's pointer
// code works unchanged. The tiles scroll inside hits.list; the bars are pinned to
// the bottom. The hovered tile's tooltip is recorded into hits.tipCall, to be drawn
// over the terminal; hits.tip is its box, outside size.
func (c *chrome) layoutRail(gtx layout.Context, m sideModel, st sideState, size image.Point) sideHits {
	ops, th, ink := gtx.Ops, c.sideTheme(), c.sideInk()
	var h sideHits
	paint.FillShape(ops, th.Chrome, clip.Rect{Max: size}.Op())
	edge, listL, listR := size.X-1, 0, size.X-1
	if st.right {
		edge, listL, listR = 0, 1, size.X
	}
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(edge, 0, edge+1, size.Y)).Op())
	root := clip.Rect{Max: size}.Push(ops)

	pad, gap, tile := gtx.Dp(railPad), gtx.Dp(railGap), gtx.Dp(railTile)
	cx := listL + (listR-listL)/2
	tx := cx - tile/2
	var tip railTipFor

	// tileBG draws a tile's fill and, while it is shown, the accent on the window edge.
	tileBG := func(r image.Rectangle, on bool) {
		if !on {
			return
		}
		paint.FillShape(ops, th.Active, clip.UniformRRect(r, gtx.Dp(8)).Op(ops))
		ax := 0
		if st.right {
			ax = size.X - gtx.Dp(2)
		}
		line := image.Rect(ax, r.Min.Y+gtx.Dp(6), ax+gtx.Dp(2), r.Max.Y-gtx.Dp(6))
		paint.FillShape(ops, th.Accent, clip.UniformRRect(line, gtx.Dp(1)).Op(ops))
	}

	y := pad
	if st.settings {
		r := image.Rect(tx, y, tx+tile, y+tile)
		tileBG(r, st.settingsOn)
		fg := th.Muted
		if st.settingsOn {
			fg = th.ActiveFG
		}
		c.toolIcon(gtx, "cog", r, fg)
		h.settings = r
		y += tile + gap
	}

	// The workspaces' bars take at most half the rail, from the bottom.
	barW, barH, barGap := gtx.Dp(railBarW), gtx.Dp(railBarH), gtx.Dp(railBarGap)
	perLine := max((tile+barGap)/max(barW+barGap, 1), 1)
	lines := 0
	for _, run := range m.Runs {
		lines += (len(run.Members) + perLine - 1) / perLine
	}
	secTop := size.Y
	if lines > 0 {
		secH := pad + lines*barH + (lines-1)*gap + pad
		secTop = size.Y - min(secH, max(size.Y/2, 0))
	}
	h.list = image.Rect(listL, min(y, secTop), listR, secTop)

	items, total := railItems(gtx, m)
	total += pad - gap // the last tile's gap becomes the bottom pad
	h.scrollMax = max(0, total-h.list.Dy())
	scroll := min(max(st.scroll, 0), h.scrollMax)
	if !h.list.Empty() {
		cl := clip.Rect(h.list).Push(ops)
		top := h.list.Min.Y - scroll
		for _, it := range items {
			at := top + it.y
			switch it.kind {
			case railLabel:
				r := image.Rect(listL, at, listR, at+gtx.Dp(railLabelH))
				if !r.Overlaps(h.list) {
					continue
				}
				call, sz := c.fitSide(gtx, it.name, 9.5, gtx.Dp(railLabelW), th.Muted)
				place(ops, call, image.Pt(cx-sz.X/2, at+(r.Dy()-sz.Y)/2))
			case railDivider:
				w := gtx.Dp(railDivW)
				paint.FillShape(ops, th.Border, clip.Rect(image.Rect(cx-w/2, at, cx-w/2+w, at+1)).Op())
			case railRow:
				row := it.row
				r := image.Rect(tx, at, tx+tile, at+tile)
				if !r.Overlaps(h.list) {
					continue
				}
				// A shown doc or agent panel keeps its session's tile highlighted.
				on := row.ID == st.shown && !st.settingsOn || row.ID != 0 && row.ID == st.docked
				tileBG(r, on)
				fg := th.Folder
				if on {
					fg = th.ActiveFG
				}
				c.railLogo(gtx, row.Agent, r, fg)
				d, dot := gtx.Dp(railDisc), gtx.Dp(railDot)
				disc := image.Rect(r.Max.X-gtx.Dp(1)-d, r.Max.Y-gtx.Dp(1)-d, r.Max.X-gtx.Dp(1), r.Max.Y-gtx.Dp(1))
				paint.FillShape(ops, th.Chrome, clip.Ellipse(disc).Op(ops))
				in := (d - dot) / 2
				c.stateDot(gtx, image.Rect(disc.Min.X+in, disc.Min.Y+in, disc.Min.X+in+dot, disc.Min.Y+in+dot), row.Mark, row.Running, false)
				hit := sideRowHit{id: row.ID, rect: r.Intersect(h.list), on: on}
				h.rows = append(h.rows, hit)
				if st.hover != 0 && row.ID == st.hover {
					wcol := th.Accent
					switch row.Mark {
					case markNeeds:
						wcol = th.Needs
					case markDone:
						wcol = th.Done
					}
					tip = railTipFor{tile: hit.rect, mark: row.Mark, running: row.Running, title: row.Title, word: railWord(*row), wordCol: wcol, found: true}
				}
			}
		}
		cl.Pop()
	}

	if lines > 0 {
		area := image.Rect(listL, secTop, listR, size.Y)
		paint.FillShape(ops, th.Chrome, clip.Rect(area).Op())
		w := gtx.Dp(railDivW)
		paint.FillShape(ops, th.Border, clip.Rect(image.Rect(cx-w/2, secTop, cx-w/2+w, secTop+1)).Op())
		by := secTop + pad
		for _, run := range m.Runs {
			for i := 0; i < len(run.Members); i += perLine {
				line := run.Members[i:min(i+perLine, len(run.Members))]
				if by+barH > size.Y-pad/2 {
					break // a capped section drops the lines it has no room for
				}
				bx := cx - (len(line)*barW+(len(line)-1)*barGap)/2
				for _, mem := range line {
					bar := image.Rect(bx, by, bx+barW, by+barH)
					bx += barW + barGap
					col := c.memberColor(mem.Color)
					if mem.Down {
						col = th.CloseHover
					}
					paint.FillShape(ops, col, clip.UniformRRect(bar, gtx.Dp(2)).Op(ops))
					if mem.ID == 0 {
						continue
					}
					hit := sideRowHit{id: mem.ID, rect: image.Rect(bar.Min.X-gtx.Dp(1), bar.Min.Y-gtx.Dp(2), bar.Max.X+gtx.Dp(1), bar.Max.Y+gtx.Dp(2)), mark: mem.Mark}
					h.members = append(h.members, hit)
					if st.hover != 0 && mem.ID == st.hover {
						wcol := th.Muted
						if mem.Down {
							wcol = th.UsageHigh
						} else if mem.Mark == markNeeds {
							wcol = th.Needs
						}
						tip = railTipFor{tile: bar, mark: mem.Mark, title: mem.Title, word: mem.Status, wordCol: wcol, member: true, found: true}
					}
				}
				by += barH + gap
			}
		}
	}
	root.Pop()

	if tip.found {
		rec := op.Record(ops)
		h.tip = c.railTip(gtx, tip, size, st.right, st.tipRoom, ink)
		h.tipCall = rec.Stop()
	}
	return h
}

// railLogo draws a session's agent logo centred in r, or the terminal icon for a shell.
func (c *chrome) railLogo(gtx layout.Context, agent string, r image.Rectangle, fg color.NRGBA) {
	px := gtx.Dp(railLogo)
	at := r.Min.Add(image.Pt((r.Dx()-px)/2, (r.Dy()-px)/2))
	if logo, img := agentLogo(agent, px, agentLogoColor(agent, fg)); img != nil {
		off := op.Offset(at).Push(gtx.Ops)
		logo.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		off.Pop()
		return
	}
	c.icon(gtx, "terminal", image.Rectangle{Min: at, Max: at.Add(image.Pt(px, px))}, fg)
}

// railTip draws the hovered tile's tooltip beside it, on the terminal's side of the
// rail, and returns its box: the tile's state dot, its title and its status word.
func (c *chrome) railTip(gtx layout.Context, t railTipFor, size image.Point, right bool, room int, ink sideInk) image.Rectangle {
	ops := gtx.Ops
	padX, padY, dot, gap := gtx.Dp(10), gtx.Dp(6), gtx.Dp(8), gtx.Dp(6)
	var wCall op.CallOp
	var wSize image.Point
	if t.word != "" {
		wCall, wSize = c.label(gtx, t.word, 12, render.Readable(t.wordCol, ink.tipBG))
	}
	wordW := 0
	if wSize.X > 0 {
		wordW = gap + gap/3 + wSize.X
	}
	// The title gets what the window leaves on the terminal's side, a gap off each edge.
	titleRoom := gtx.Dp(railTipMaxW)
	if room > 0 {
		titleRoom = min(titleRoom, max(room-2*gtx.Dp(railTipGap)-(padX+dot+gap+padX)-wordW, 0))
	}
	tCall, tSize := c.fitSide(gtx, t.title, 12, titleRoom, ink.tipFG)
	w := padX + dot + gap + tSize.X + padX + wordW
	ht := max(tSize.Y, wSize.Y, dot) + 2*padY
	x := size.X + gtx.Dp(railTipGap)
	if right {
		x = -gtx.Dp(railTipGap) - w
	}
	if room > 0 { // never past the window's far edge, even when the word alone is too wide
		if right {
			x = max(x, -room) // the window's left edge, in the rail's coordinates
		} else {
			x = min(x, size.X+room-w)
		}
	}
	y := t.tile.Min.Y + (t.tile.Dy()-ht)/2
	y = max(min(y, size.Y-ht), 0)
	box := image.Rect(x, y, x+w, y+ht)
	rad := gtx.Dp(7)
	paint.FillShape(ops, ink.tipBG, clip.UniformRRect(box, rad).Op(ops))
	stroke(ops, box, rad, c.th.ToastLine)
	mid := y + ht/2
	c.stateDot(gtx, image.Rect(x+padX, mid-dot/2, x+padX+dot, mid-dot/2+dot), t.mark, t.running, false)
	tx := x + padX + dot + gap
	place(ops, tCall, image.Pt(tx, mid-tSize.Y/2))
	if wSize.X > 0 {
		place(ops, wCall, image.Pt(tx+tSize.X+gap+gap/3, mid-wSize.Y/2))
	}
	return box
}

// railReveal is the scroll that shows the shown session's tile whole.
func railReveal(gtx layout.Context, m sideModel, st sideState, h sideHits) (int, bool) {
	items, _ := railItems(gtx, m)
	for _, it := range items {
		if it.kind != railRow || it.row.ID != st.shown {
			continue
		}
		sc := min(max(st.scroll, 0), h.scrollMax)
		sc = max(min(sc, it.y), it.y+gtx.Dp(railTile)-h.list.Dy())
		return min(max(sc, 0), h.scrollMax), true
	}
	return 0, false
}

// drawSideTip draws the rail's tooltip over the terminal, so call it after the
// terminal is drawn. It replays ops the cached rail recorded and takes no input.
func (win *window) drawSideTip(gtx layout.Context) {
	s := &win.side
	if s.mode != sideRail || s.hits.tip.Empty() {
		return
	}
	off := op.Offset(s.at).Push(gtx.Ops)
	s.hits.tipCall.Add(gtx.Ops)
	off.Pop()
}
