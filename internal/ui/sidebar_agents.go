package ui

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// sideAgentH is one subagent's two lines under its row; sideMoreH the "+N more" line.
const sideAgentH, sideMoreH = 34, 20

// sideAgent is one subagent line in the sidebar tree (spec 6.1).
type sideAgent struct {
	Key                         subagents.Key
	Type, Task, Action, Elapsed string
	State                       subagents.State
	Depth                       int
}

// sideAgentHit is where a subagent line, or a row's "+N more" line, was drawn.
type sideAgentHit struct {
	key  subagents.Key
	tab  tabs.TabID
	more bool
	open bool // its panel is the one shown
	rect image.Rectangle
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

// rowSpan is a row's height with its subagent lines.
func rowSpan(gtx layout.Context, r sideRow) int {
	h := gtx.Dp(sideRowH) + len(r.Docs)*gtx.Dp(sideDocH) + len(r.Agents)*gtx.Dp(sideAgentH)
	if r.Hidden.Total() > 0 {
		h += gtx.Dp(sideMoreH)
	}
	return h
}

// agentLines draws a row's subagent lines from y down, inside x0..x1.
func (c *chrome) agentLines(gtx layout.Context, h *sideHits, r sideRow, x0, x1, y int, open subagents.Key) {
	ops, th := gtx.Ops, c.th
	gap, dot := gtx.Dp(8), gtx.Dp(6)
	mag := c.memberColor("magenta")
	for _, a := range r.Agents {
		band := image.Rect(x0, y, x1, y+gtx.Dp(sideAgentH))
		y = band.Max.Y
		if !band.Overlaps(h.list) {
			continue
		}
		isOpen := open != (subagents.Key{}) && a.Key == open
		if isOpen {
			// The agent whose panel is shown, highlighted like the selected session row.
			paint.FillShape(ops, th.Active, clip.UniformRRect(image.Rect(x0-gtx.Dp(6), band.Min.Y, x1+gtx.Dp(4), band.Max.Y), gtx.Dp(6)).Op(ops))
		}
		x := x0 + gtx.Dp(12)*min(a.Depth, 1)
		mid := band.Min.Y + gtx.Dp(10)
		d := image.Rect(x, mid-dot/2, x+dot, mid-dot/2+dot)
		mk, running := markNone, a.State == subagents.Running
		switch a.State {
		case subagents.Waiting:
			mk = markNeeds
		case subagents.Done:
			mk = markDone
		}
		c.stateDot(gtx, d, mk, running, false)
		tx := x + dot + gap
		eCall, eSize := c.label(gtx, a.Elapsed, 10, th.Muted)
		right := x1 - eSize.X
		place(ops, eCall, image.Pt(right, mid-eSize.Y/2))
		room := right - gap - tx
		tCall, tSize := c.fitSide(gtx, a.Type, 11, room/2, mag)
		place(ops, tCall, image.Pt(tx, mid-tSize.Y/2))
		col := th.Folder
		if a.State == subagents.Done {
			col = th.Muted
		}
		kCall, kSize := c.fitSide(gtx, a.Task, 11, room-tSize.X-gap/2, col)
		place(ops, kCall, image.Pt(tx+tSize.X+gap/2, mid-kSize.Y/2))
		if a.Action != "" {
			amid := band.Min.Y + gtx.Dp(25)
			aCall, aSize := c.fitSide(gtx, a.Action, 10, x1-tx, th.Muted)
			place(ops, aCall, image.Pt(tx, amid-aSize.Y/2))
		}
		h.agents = append(h.agents, sideAgentHit{key: a.Key, tab: r.ID, open: isOpen, rect: band.Intersect(h.list)})
	}
	if r.Hidden.Total() > 0 {
		band := image.Rect(x0, y, x1, y+gtx.Dp(sideMoreH))
		if band.Overlaps(h.list) {
			call, sz := c.label(gtx, "+"+strconv.Itoa(r.Hidden.Total())+" more", 10, th.Muted)
			place(ops, call, image.Pt(x0+dot+gap, band.Min.Y+band.Dy()/2-sz.Y/2))
			h.agents = append(h.agents, sideAgentHit{tab: r.ID, more: true, rect: band.Intersect(h.list)})
		}
	}
}

// agentChipAt draws the chip right-aligned in band and returns its width.
func (c *chrome) agentChipAt(gtx layout.Context, n int, band image.Rectangle) int {
	s := agentChip(n)
	if s == "" {
		return 0
	}
	ops, th := gtx.Ops, c.th
	call, sz := c.label(gtx, s, 10, th.Accent)
	pad := gtx.Dp(6)
	w := sz.X + 2*pad
	box := image.Rect(band.Max.X-w, band.Min.Y+band.Dy()/2-sz.Y/2-gtx.Dp(2), band.Max.X, band.Min.Y+band.Dy()/2+sz.Y/2+gtx.Dp(2))
	paint.FillShape(ops, tint(th.Accent, 0x22), clip.UniformRRect(box, box.Dy()/2).Op(ops))
	place(ops, call, image.Pt(box.Min.X+pad, box.Min.Y+gtx.Dp(2)))
	return w
}
