package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// A workspace member's tab in the docked panel (issue #133): its live
// terminal in the panel body, at the PTY size dockFit gave it.

// drawDockMember draws member m's terminal in body with the panel's one
// member renderer and takes its input on the member view's tag.
func (win *window) drawDockMember(gtx layout.Context, m tabs.TabID, body image.Rectangle) {
	t := win.mgr.Lookup(m)
	if t == nil || body.Empty() {
		return
	}
	ops := gtx.Ops
	tag := &win.dock.term
	if win.dock.drawn != m {
		// Register a newly shown member's keyboard filters before moving
		// focus to it, so following input is not left at the old view.
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: tag}, key.Filter{Focus: tag, Optional: allMods}, key.Filter{Focus: tag, Name: key.NameTab, Optional: allMods}); !ok {
				break
			}
		}
	}
	rend := win.dock.rend
	if rend == nil {
		rend = render.NewRenderer(win.rendPx, win.pal, rendOptions(win.opt.Config))
		win.dock.rend = rend
	} else if win.dock.rendFor != m {
		rend.Release() // another member's strips: draw this one in full
	}
	win.dock.rendFor = m
	rend.SetFocused(win.focused && win.dockMemberKeys())
	grid := dockMemberGrid(win.dock.lastRect, gtx.Dp, gtx.Dp(8))
	sel := win.checkSel(t)
	g := clip.Rect(grid.Intersect(body)).Push(ops)
	off := op.Offset(grid.Min).Push(ops)
	tg := gtx
	tg.Constraints = layout.Exact(image.Pt(max(grid.Dx(), 0), max(grid.Dy(), 0)))
	rend.Layout(tg, t.Session, &sel)
	off.Pop()
	g.Pop()
	area := clip.Rect(body).Push(ops)
	event.Op(ops, tag)
	cw, ch := rend.CellSize()
	win.linkCursors(ops, t.Session, grid, cw, ch) // the body clip trims it
	area.Pop()
	win.dock.drewNow = m
}

// syncDockMember marks the member drawn this frame visible, so its output
// asks for a frame, and the one drawn before not; with no member drawn the
// renderer goes.
func (win *window) syncDockMember() {
	now := win.dock.drewNow
	win.dock.drewNow = 0
	if now == 0 {
		win.dock.rend, win.dock.rendFor = nil, 0
	}
	if now == win.dock.drawn {
		return
	}
	if win.dock.drawn != 0 {
		win.mgr.SetVisible(win.dock.drawn, false)
	}
	if now != 0 {
		win.mgr.SetVisible(now, true)
		win.invalidate() // one more frame, so its input filters are up before the next click
	}
	win.dock.drawn = now
}

// dockMemberView is the drawn member's terminal view, when a member tab shows.
func (win *window) dockMemberView() (termView, bool) {
	m := win.shownMember()
	if m == 0 {
		return termView{}, false
	}
	t := win.mgr.Lookup(m)
	if t == nil {
		return termView{}, false
	}
	return termView{tab: t, tag: &win.dock.term, grid: win.dock.grid, cols: win.dock.cols, rows: win.dock.rows}, true
}

// dockMemberKeys reports whether the shown member tab has the keyboard.
func (win *window) dockMemberKeys() bool {
	return win.dock.keys && win.dock.keysMember != 0 && win.dock.keysMember == win.shownMember()
}

// dockFocusMember gives the shown member tab the keyboard (on) or gives it
// back to the session, moving the focus-in and focus-out reports with it. A
// floating pane with the keyboard gives it up first: one keyboard at a time.
func (win *window) dockFocusMember(on bool) {
	if !on {
		win.dropDockKeys()
		return
	}
	m := win.shownMember()
	if m == 0 {
		return
	}
	if win.dockMemberKeys() {
		win.wantFocus = true
		return
	}
	if win.fl.keys != 0 {
		win.focusSession()
	}
	win.dropSideEdit()
	win.dock.keys, win.dock.keysMember, win.wantFocus = true, m, true
	if win.focused {
		if s := win.shownTab(); s != nil {
			sendFocus(s, false)
		}
		if t := win.mgr.Lookup(m); t != nil {
			sendFocus(t, true)
		}
	}
	if win.dock.drawn != m {
		win.dock.refocus = true // Gio ignores focus on a tag with no filters yet
	}
	win.invalidate()
}
