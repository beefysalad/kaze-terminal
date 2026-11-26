package ui

import (
	"image"
	"strconv"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

type workerPopover struct {
	open      bool
	tag       bool
	box, stop image.Rectangle
	rows      []workerPopoverRow
	tabs      []*tabs.Tab
	inputs    []workerPopoverInput
	key       workerPopoverKey
	valid     bool
	ops       op.Ops
	call      op.CallOp
	draws     uint64
}
type workerPopoverInput struct {
	id                 tabs.TabID
	name, agent, state string
}
type workerPopoverKey struct {
	size   image.Point
	anchor image.Rectangle
	count  int
	metric unit.Metric
	locale system.Locale
	theme  theme.Chrome
	shaper *text.Shaper
}

type workerPopoverRow struct {
	id   tabs.TabID
	rect image.Rectangle
	name string
}

func (win *window) drawWorkerPopover(gtx layout.Context) {
	s := &win.stat.workers
	if !s.open {
		return
	}
	if win.workerCount() == 0 || win.set.shown || win.palette.open {
		s.open = false
		return
	}
	// Compare raw presentation values before formatting, fitting or shaping text.
	// Retain both the tab scratch and Gio ops, as the sidebar does.
	key := workerPopoverKey{gtx.Constraints.Max, win.statusRect(grpWorkers), win.workerCount(), gtx.Metric, gtx.Locale, win.ui.th, win.ui.shaper}
	dirty := !s.valid || s.key != key
	s.tabs = win.mgr.TabsInto(s.tabs)
	n := 0
	for _, t := range s.tabs {
		if t.Role != "worker" {
			continue
		}
		name := win.names[t.ID]
		if name == "" {
			name = t.Title
		}
		agent, first, needs := win.workerPresentation(t.ID)
		state := "idle"
		a := win.rowAgent(t.ID)
		switch {
		case t.Exited:
			state = "exited"
		case needs || first == "failed" || first == "unconfirmed":
			state = "needs you"
		case a.running:
			state = "working"
		case first == "pending":
			state = "starting"
		}
		in := workerPopoverInput{t.ID, name, agent, state}
		if n >= len(s.inputs) {
			s.inputs = append(s.inputs, in)
			dirty = true
		} else if s.inputs[n] != in {
			s.inputs[n] = in
			dirty = true
		}
		n++
	}
	if n != len(s.inputs) {
		clear(s.inputs[n:])
		s.inputs = s.inputs[:n]
		dirty = true
	}
	if dirty {
		s.ops.Reset()
		cached := gtx
		cached.Ops = &s.ops
		rec := op.Record(cached.Ops)
		win.buildWorkerPopover(cached)
		s.call = rec.Stop()
		// Fitting a new name can load a fallback font and replace the shaper.
		key.shaper = win.ui.shaper
		s.key, s.valid = key, true
		s.draws++
	}
	s.call.Add(gtx.Ops)
}

func (win *window) buildWorkerPopover(gtx layout.Context) {
	s := &win.stat.workers
	c, ops, th := win.ui, gtx.Ops, win.ui.th
	w := min(gtx.Dp(340), gtx.Constraints.Max.X-gtx.Dp(16))
	pad, rowH := gtx.Dp(12), gtx.Dp(42)
	anchor := win.statusRect(grpWorkers)
	x := max(gtx.Dp(8), min(anchor.Max.X-w, gtx.Constraints.Max.X-w-gtx.Dp(8)))
	bottom := anchor.Min.Y - gtx.Dp(8)
	h := pad*2 + win.workerCount()*rowH + gtx.Dp(36)
	s.box = image.Rect(x, max(gtx.Dp(8), bottom-h), x+w, bottom)
	paint.FillShape(ops, th.ToastBG, clip.UniformRRect(s.box, gtx.Dp(8)).Op(ops))
	rr := clip.UniformRRect(s.box, gtx.Dp(8))
	paint.FillShape(ops, th.ToastLine, clip.Stroke{Path: rr.Path(ops), Width: float32(max(gtx.Dp(1), 1))}.Op())
	cl := clip.Rect(s.box).Push(ops)
	y := s.box.Min.Y + pad
	s.rows = s.rows[:0]
	for _, in := range s.inputs {
		name := in.name
		if name == "" {
			name = "t" + strconv.Itoa(int(in.id))
		}
		agent, state := in.agent, in.state
		rect := image.Rect(x+pad, y, x+w-pad, y+rowH)
		col := th.ToastFG
		if state == "needs you" {
			col = th.Needs
		}
		call, sz, shownName := c.fitSideText(gtx, name, 12, rect.Dx(), col, font.Bold)
		place(ops, call, rect.Min)
		call, _ = c.fitSide(gtx, agent+" · "+state, 11, rect.Dx(), th.Muted)
		place(ops, call, image.Pt(rect.Min.X, y+sz.Y+gtx.Dp(3)))
		s.rows = append(s.rows, workerPopoverRow{in.id, rect, shownName})
		y += rowH
	}
	s.stop = image.Rect(x+pad, y, x+w-pad, y+gtx.Dp(32))
	paint.FillShape(ops, th.Active, clip.UniformRRect(s.stop, gtx.Dp(5)).Op(ops))
	call, sz := c.fitSideWeight(gtx, "Stop all workers", 12, s.stop.Dx()-pad, th.ToastFG, font.Bold)
	place(ops, call, image.Pt(s.stop.Min.X+pad/2, s.stop.Min.Y+(s.stop.Dy()-sz.Y)/2))
	cl.Pop()
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(ops)
	event.Op(ops, &s.tag)
	area.Pop()
}

func (win *window) workerPopoverEvents(gtx layout.Context) {
	s := &win.stat.workers
	if !s.open {
		return
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &s.tag, Kinds: pointer.Press}, key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		if win.bench.IgnoreInput() {
			continue
		}
		switch e := ev.(type) {
		case key.Event:
			if e.State == key.Press {
				s.open = false
				win.w.Invalidate()
			}
		case pointer.Event:
			if !e.Buttons.Contain(pointer.ButtonPrimary) {
				continue
			}
			win.workerPopoverClick(e.Position.Round())
		}
	}
}
func (win *window) workerPopoverClick(p image.Point) {
	s := &win.stat.workers
	if p.In(s.stop) {
		win.stopWorkerTabs(0)
	} else {
		for _, r := range s.rows {
			if p.In(r.rect) {
				win.selectSession(r.id)
				break
			}
		}
		s.open = false
	}
	if win.w != nil {
		win.w.Invalidate()
	}
}

// workerCount includes exited workers until their tabs are closed.
func (win *window) workerCount() int {
	return len(win.workers)
}
func (win *window) hasWorkers(parent tabs.TabID) bool {
	for _, w := range win.workers {
		if w.parent == parent {
			return true
		}
	}
	return false
}

// stopWorkerTabs is called only by UI actions, sharing the normal close path.
func (win *window) stopWorkerTabs(parent tabs.TabID) {
	for _, t := range win.mgr.Tabs() {
		if t.Role == "worker" && (parent == 0 || t.Parent == parent) {
			win.closeTab(t.ID)
		}
	}
	win.stat.workers.open = false
}

func workerCounter(n int) string {
	if n == 0 {
		return ""
	}
	if n == 1 {
		return "1 worker"
	}
	return strconv.Itoa(n) + " workers"
}
