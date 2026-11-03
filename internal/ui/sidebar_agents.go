package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// Tray sizes in dp (spec 3, mockup .tray): an agent line, the gap above a tray, the
// padding below it and its chip or footer line; sideMoreH is the "finished worktrees" line.
const sideAgentH, sideTrayGap, sideTrayPad, sideChipH, sideMoreH = 22, 3, 4, 22, 20

// sideTrayIndent is the tray's tree line from a row's left edge: 30dp from the sidebar's.
const sideTrayIndent = 22

// sideAgent is one subagent line in the sidebar tray.
type sideAgent struct {
	Key                         subagents.Key
	Type, Task, Action, Elapsed string
	State                       subagents.State
	Depth                       int
}

// agentHitKind is what part of a tray a sideAgentHit is.
type agentHitKind uint8

const (
	hitAgentLine  agentHitKind = iota // an agent's line; btn is its › button
	hitAgentChip                      // "+N more ⌄": expands the tray
	hitAgentFewer                     // "Show fewer": collapses it
	hitAgentBoard                     // "N more on the board": opens the board
)

// sideAgentHit is where a tray line, its chip or a footer link was drawn.
type sideAgentHit struct {
	key       subagents.Key
	tab       tabs.TabID
	kind      agentHitKind
	open      bool            // its panel is the one shown
	rect, btn image.Rectangle // btn: the › box, full line height
	tip       string          // the current step, for the hover tooltip
}

// sideAgentOf is a tracker subagent as its sidebar line at now.
func sideAgentOf(a *subagents.Agent, now time.Time) sideAgent {
	s := sideAgent{Key: a.Key, Type: a.Type, Task: a.Task, State: a.State, Depth: a.Depth, Elapsed: agentElapsed(a, now)}
	switch {
	case a.State == subagents.Done:
		s.Action, _, _ = strings.Cut(a.Report, "\n")
	case len(a.Steps) > 0:
		st := a.Steps[len(a.Steps)-1]
		s.Action = strings.TrimSpace(st.Tool + " " + st.Summary)
	default:
		s.Action = "Starting"
	}
	return s
}

// agentElapsed is "<1m" or "4m" while it runs, "took 6m" once done.
func agentElapsed(a *subagents.Agent, now time.Time) string {
	if a.State == subagents.Done {
		return "took " + minutes(a.Ended.Sub(a.Started))
	}
	return minutes(now.Sub(a.Started))
}

func minutes(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	return strconv.Itoa(int(d/time.Minute)) + "m"
}

// agentChip is the row's "3 agents" chip; "" for none.
func agentChip(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}

// trayCap and trayMax are a tray's agent lines collapsed and expanded (spec 3).
const trayCap, trayMax = 3, 8

// The fold control's carets: open, and folded.
const trayOpenCaret, trayShutCaret = "⌄", "›"

// sideTray is one session's subagent tray as the sidebar model sees it.
type sideTray struct {
	Shown            []sideAgent          // the lines to draw; nil while folded
	Hidden           subagents.TrayCounts // agents past the lines, by state; zero while folded
	Total, Waiting   int                  // every agent in the tray, and those waiting
	Active           int                  // running and waiting, for the group chip
	Expanded, Folded bool
}

// sideAgents is the sidebar input's view of the tracker for one session.
func (win *window) sideAgents(id tabs.TabID) sideTray {
	tr := sideTray{Expanded: win.side.trayOpen[id], Folded: win.side.trayFolded[id], Active: win.subs.Active(int(id))}
	limit := trayCap
	if tr.Expanded {
		limit = trayMax
	}
	shown, hidden := win.subs.Tray(int(id), limit)
	tr.Total, tr.Waiting = len(shown)+hidden.Total(), hidden.Waiting
	for _, a := range shown {
		if a.State == subagents.Waiting {
			tr.Waiting++
		}
	}
	if tr.Folded {
		return tr
	}
	tr.Hidden = hidden
	now := time.Now()
	tr.Shown = make([]sideAgent, 0, len(shown))
	for _, a := range shown {
		tr.Shown = append(tr.Shown, sideAgentOf(a, now))
	}
	return tr
}

// trayFold is the row's fold control: "20 agents ⌄" open, "20 agents ›" folded.
func trayFold(total int, folded bool) string {
	if folded {
		return agentChip(total) + " " + trayShutCaret
	}
	return agentChip(total) + " " + trayOpenCaret
}

// trayChip is the chip under a capped tray, in three colored parts: "+17 more",
// " · 2 need you" (empty when none waits) and " · 15 running, 2 done ⌄".
func trayChip(h subagents.TrayCounts) (lead, needs, rest string) {
	lead = "+" + strconv.Itoa(h.Total()) + " more"
	switch {
	case h.Waiting == 1:
		needs = " · 1 needs you"
	case h.Waiting > 1:
		needs = " · " + strconv.Itoa(h.Waiting) + " need you"
	}
	var parts []string
	if h.Running > 0 {
		parts = append(parts, strconv.Itoa(h.Running)+" running")
	}
	if h.Done > 0 {
		parts = append(parts, strconv.Itoa(h.Done)+" done")
	}
	if len(parts) > 0 {
		sep := " · "
		if needs != "" {
			sep = ", "
		}
		rest = sep + strings.Join(parts, ", ")
	}
	return lead, needs, rest + " " + trayOpenCaret
}

// trayFooter is an expanded tray's last line: "Show fewer", and past trayMax
// " · N more on the board".
func trayFooter(beyond int) (fewer, board string) {
	if beyond > 0 {
		board = " · " + strconv.Itoa(beyond) + " more on the board"
	}
	return "Show fewer", board
}

// trayChipLine reports whether r's tray ends in a chip or footer line: a collapsed
// tray with agents past its lines, or an expanded one holding more than trayCap.
func (r sideRow) trayChipLine() bool {
	switch {
	case r.TrayFolded:
		return false
	case r.TrayExpanded:
		return r.TrayTotal > trayCap
	}
	return r.Hidden.Total() > 0
}

// trayHeight is the room r's tray takes under its row and documents; 0 for none.
func trayHeight(gtx layout.Context, r sideRow) int {
	chip := r.trayChipLine()
	if len(r.Agents) == 0 && !chip {
		return 0
	}
	h := gtx.Dp(sideTrayGap) + len(r.Agents)*gtx.Dp(sideAgentH) + gtx.Dp(sideTrayPad)
	if chip {
		h += gtx.Dp(sideChipH)
	}
	return h
}

// rowSpan is a row's height with its documents and agent tray.
func rowSpan(gtx layout.Context, r sideRow) int {
	return gtx.Dp(sideRowH) + len(r.Docs)*gtx.Dp(sideDocH) + trayHeight(gtx, r)
}

// trayLines draws r's agent tray from y down under row: a thin tree line, one line
// per agent (mark, type, task, elapsed, ›) and the chip or footer.
func (c *chrome) trayLines(gtx layout.Context, h *sideHits, r sideRow, row image.Rectangle, y int, open subagents.Key, ink sideInk) {
	if trayHeight(gtx, r) == 0 {
		return
	}
	ops, th := gtx.Ops, c.th
	lineH, px := gtx.Dp(sideAgentH), max(gtx.Dp(1), 1)
	tree, x1 := row.Min.X+gtx.Dp(sideTrayIndent), row.Max.X
	dot, gap := gtx.Dp(6), gtx.Dp(7)
	y += gtx.Dp(sideTrayGap)
	top, end := y, y+len(r.Agents)*lineH-lineH/2
	if r.trayChipLine() {
		end = y + len(r.Agents)*lineH + gtx.Dp(sideChipH)/2
	}
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(tree, top, tree+px, end)).Op())
	for _, a := range r.Agents {
		band := image.Rect(tree, y, x1, y+lineH)
		y = band.Max.Y
		if !band.Overlaps(h.list) {
			continue
		}
		mid := band.Min.Y + lineH/2
		isOpen := open != (subagents.Key{}) && a.Key == open
		if isOpen {
			// The agent whose panel is shown, highlighted like the selected session row.
			paint.FillShape(ops, th.Active, clip.RRect{Rect: band, NE: gtx.Dp(6), SE: gtx.Dp(6)}.Op(ops))
			paint.FillShape(ops, th.Accent, clip.Rect(image.Rect(tree, band.Min.Y, tree+gtx.Dp(2), band.Max.Y)).Op())
		}
		paint.FillShape(ops, th.Border, clip.Rect(image.Rect(tree, mid, tree+gtx.Dp(7), mid+px)).Op())
		mx := tree + gtx.Dp(10)
		mk := markNone
		switch a.State {
		case subagents.Waiting:
			mk = markNeeds
		case subagents.Done:
			mk = markDone
		}
		c.stateDot(gtx, image.Rect(mx, mid-dot/2, mx+dot, mid-dot/2+dot), mk, a.State == subagents.Running, false)
		box := image.Rect(x1-gtx.Dp(16), band.Min.Y, x1, band.Max.Y)
		bCall, bSize := c.label(gtx, "›", 12, ink.muted)
		place(ops, bCall, image.Pt(box.Min.X+(box.Dx()-bSize.X)/2, mid-bSize.Y/2))
		eCall, eSize := c.label(gtx, a.Elapsed, 10, ink.muted)
		right := box.Min.X - gtx.Dp(2) - eSize.X
		place(ops, eCall, image.Pt(right, mid-eSize.Y/2))
		tx := mx + dot + gap
		tCall, tSize := c.fitSide(gtx, a.Type, 11, max(right-gap-tx, 0)/2, ink.kind)
		place(ops, tCall, image.Pt(tx, mid-tSize.Y/2))
		col := th.Folder
		switch a.State {
		case subagents.Waiting:
			col = ink.needs
		case subagents.Done:
			col = ink.muted
		}
		kx := tx + tSize.X + gap
		kCall, kSize := c.fitSide(gtx, a.Task, 11, max(right-gap-kx, 0), col)
		place(ops, kCall, image.Pt(kx, mid-kSize.Y/2))
		btn := image.Rect(box.Min.X-gtx.Dp(4), band.Min.Y, x1, band.Max.Y)
		h.agents = append(h.agents, sideAgentHit{key: a.Key, tab: r.ID, kind: hitAgentLine, open: isOpen,
			rect: band.Intersect(h.list), btn: btn.Intersect(h.list), tip: a.Action})
	}
	if r.trayChipLine() {
		if band := image.Rect(tree, y, x1, y+gtx.Dp(sideChipH)); band.Overlaps(h.list) {
			c.trayChip(gtx, h, r, band, ink)
		}
	}
}

// trayChip draws the chip ("+17 more · 15 running, 2 done ⌄") or, on an expanded
// tray, the footer ("Show fewer · 13 more on the board") as a pill in band.
func (c *chrome) trayChip(gtx layout.Context, h *sideHits, r sideRow, band image.Rectangle, ink sideInk) {
	ops := gtx.Ops
	type part struct {
		s    string
		col  color.NRGBA
		kind agentHitKind
	}
	var parts [3]part
	if r.TrayExpanded {
		fewer, board := trayFooter(r.Hidden.Total())
		parts[0], parts[1] = part{fewer, ink.chipLead, hitAgentFewer}, part{board, ink.muted, hitAgentBoard}
	} else {
		lead, needs, rest := trayChip(r.Hidden)
		parts = [3]part{{lead, ink.chipLead, hitAgentChip}, {needs, ink.needs, hitAgentChip}, {rest, ink.muted, hitAgentChip}}
	}
	padX, rad := gtx.Dp(8), gtx.Dp(9)
	mid := band.Min.Y + band.Dy()/2
	pill := image.Rect(band.Min.X+gtx.Dp(10), mid-rad, band.Max.X, mid+rad)
	room := pill.Dx() - 2*padX
	if !r.TrayExpanded {
		c.narrowChip(gtx, &parts[1].s, &parts[2].s, parts[0].s, room)
	}
	var calls [3]op.CallOp
	var sizes [3]image.Point
	w := 0
	for i, p := range parts {
		if p.s != "" {
			calls[i], sizes[i], h.chip[i] = c.fitSideText(gtx, p.s, 11, max(room-w, 0), p.col, font.Normal)
			w += sizes[i].X
		}
	}
	pill.Max.X = min(pill.Min.X+w+2*padX, band.Max.X)
	paint.FillShape(ops, ink.chipBG, clip.UniformRRect(pill, rad).Op(ops))
	stroke(ops, pill, rad, c.th.Border)
	x := pill.Min.X + padX
	for i := range parts {
		place(ops, calls[i], image.Pt(x, mid-sizes[i].Y/2))
		x += sizes[i].X
	}
	hit := func(kind agentHitKind, x0, x1 int) {
		h.agents = append(h.agents, sideAgentHit{tab: r.ID, kind: kind, rect: image.Rect(x0, band.Min.Y, x1, band.Max.Y).Intersect(h.list)})
	}
	switch {
	case !r.TrayExpanded:
		hit(hitAgentChip, pill.Min.X, pill.Max.X)
	case sizes[1].X > 0:
		split := pill.Min.X + padX + sizes[0].X
		hit(hitAgentFewer, pill.Min.X, split)
		hit(hitAgentBoard, split, pill.Max.X)
	default:
		hit(hitAgentFewer, pill.Min.X, pill.Max.X)
	}
}

// narrowChip shortens a chip that does not fit in room px so it never loses
// "+N more" or its caret: the counts go first, then the "need you" part.
func (c *chrome) narrowChip(gtx layout.Context, needs, rest *string, lead string, room int) {
	width := func(s string) int {
		if s == "" {
			return 0
		}
		_, sz := c.labelW(gtx, s, 11, c.th.Border, font.Normal)
		return sz.X
	}
	lw := width(lead)
	if lw+width(*needs)+width(*rest) <= room {
		return
	}
	*rest = " " + trayOpenCaret
	if *needs != "" && lw+width(*needs)+width(*rest) > room {
		*needs = ""
	}
}

// agentChipAt draws the chip right-aligned in band and returns its width.
func (c *chrome) agentChipAt(gtx layout.Context, n int, band image.Rectangle) int {
	s := agentChip(n)
	if s == "" {
		return 0
	}
	ops, th := gtx.Ops, c.sideTheme()
	call, sz := c.label(gtx, s, 10, th.Accent)
	pad := gtx.Dp(6)
	w := sz.X + 2*pad
	box := image.Rect(band.Max.X-w, band.Min.Y+band.Dy()/2-sz.Y/2-gtx.Dp(2), band.Max.X, band.Min.Y+band.Dy()/2+sz.Y/2+gtx.Dp(2))
	paint.FillShape(ops, tint(th.Accent, 0x22), clip.UniformRRect(box, box.Dy()/2).Op(ops))
	place(ops, call, image.Pt(box.Min.X+pad, box.Min.Y+gtx.Dp(2)))
	return w
}
