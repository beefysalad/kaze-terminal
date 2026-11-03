package ui

import (
	"image"
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// The docked agent panel (spec 2026-11-03-kaze-agent-dock-design): one
// subagent's Activity and Report under the terminal, remembered per session.

const (
	dockMinDp    = 120 // the panel's least height
	dockHideDp   = 40  // below this, a squeezed panel is not drawn at all
	dockMinRows  = 8   // the terminal keeps at least this many rows
	dockShareDef = 0.4 // default height: this share of the terminal area
)

type dockTab uint8

const (
	dockActivity dockTab = iota
	dockReport
)

// dockSession is what one session remembers about its open panel.
type dockSession struct {
	agent      subagents.Key
	tab        dockTab
	picked     bool // the user chose the tab: finishing does not switch it
	scroll     int  // px the body is scrolled down
	follow     bool // keep the newest row in view
	promptOpen bool
	wasDone    bool // the agent was already done last frame

	tr              *subagents.Transcript
	trPath          string
	trAt, trStarted time.Time
	cache           dockCache
}

// dockDrag is a handle drag in progress.
type dockDrag struct {
	on      bool
	fromY   int // press position, window px
	startDp int // height at the press
}

// dockState is the window's panel state; UI goroutine only.
type dockState struct {
	by                      map[tabs.TabID]*dockSession
	sized                   map[tabs.TabID]bool // sessions whose PTY the panel shrank
	heightDp                int                 // 0 until dragged: the config value, else the default share
	keys                    bool                // the panel has the keyboard
	tag, handle, head, body int                 // event tags
	drag                    dockDrag
	hits                    dockHits
	lastRect                image.Rectangle // the panel as drawn last frame, empty when none
	texts                   []string        // text drawn last frame; tests read it
}

// dockSplit divides term into the terminal (top) and the panel below it.
// heightDp <= 0 means the default share. The panel is at least dockMinDp
// tall, the terminal keeps dockMinRows rows plus pad, and the terminal's
// minimum wins when both cannot fit; a panel squeezed under dockHideDp is
// not drawn.
func dockSplit(term image.Rectangle, heightDp int, pxPerDp float32, cellH, pad int) (top, panel image.Rectangle) {
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	px := func(dp int) int { return int(float32(dp)*pxPerDp + 0.5) }
	h := int(float32(term.Dy()) * dockShareDef)
	if heightDp > 0 {
		h = px(heightDp)
	}
	h = max(h, px(dockMinDp))
	h = min(h, term.Dy()-(dockMinRows*cellH+pad))
	if h < px(dockHideDp) {
		return term, image.Rectangle{}
	}
	cut := term.Max.Y - h
	return image.Rect(term.Min.X, term.Min.Y, term.Max.X, cut), image.Rect(term.Min.X, cut, term.Max.X, term.Max.Y)
}

// dockHeight is the panel height in dp to split with: dragged, else
// configured, else 0 (the default share).
func (win *window) dockHeight() int {
	if win.dock.heightDp > 0 {
		return win.dock.heightDp
	}
	return win.opt.Config.AgentPanelHeight
}

// dockOwner is the session whose panel shows under the terminal: the shown
// tab, members included, unless it is a scratch shell; 0 when Settings, the
// board or a document covers the terminal.
func (win *window) dockOwner() tabs.TabID {
	if win.mgr == nil {
		return 0
	}
	t := win.shownTab()
	if t == nil || t.Parent != 0 {
		return 0
	}
	return t.ID
}

// dockOf is session id's open panel, nil when none.
func (win *window) dockOf(id tabs.TabID) *dockSession { return win.dock.by[id] }

// openDock shows subagent k in session's panel; another agent's panel there
// is replaced and starts over on Activity.
func (win *window) openDock(session tabs.TabID, k subagents.Key) {
	if s := win.dock.by[session]; s != nil && s.agent == k {
		return
	}
	if win.dock.by == nil {
		win.dock.by = map[tabs.TabID]*dockSession{}
	}
	win.dock.by[session] = &dockSession{agent: k, follow: true}
	win.invalidate()
}

// closeDock closes session's panel; its height is kept.
func (win *window) closeDock(session tabs.TabID) {
	if _, ok := win.dock.by[session]; !ok {
		return
	}
	delete(win.dock.by, session)
	// The panel keyboard belongs to the active session's panel (it never
	// follows a tab switch), even while the reader or a page covers it.
	owns := win.dockOwner() == session
	if win.mgr != nil {
		if a := win.mgr.Active(); a != nil && a.ID == session {
			owns = true
		}
	}
	if owns {
		win.dropDockKeys()
		win.dock.drag.on = false
	}
	win.invalidate()
}

// dropDockKeys gives the keyboard back to the terminal if the panel had it.
func (win *window) dropDockKeys() {
	if win.dock.keys {
		win.dock.keys, win.wantFocus = false, true
	}
}

// toggleDock closes session's panel when it already shows k, else opens k.
func (win *window) toggleDock(session tabs.TabID, k subagents.Key) {
	if s := win.dock.by[session]; s != nil && s.agent == k {
		win.closeDock(session)
		return
	}
	win.openDock(session, k)
}

func (win *window) invalidate() {
	if win.w != nil {
		win.w.Invalidate()
	}
}

// dockFit splits term for the shown session's panel and sizes PTYs: the
// owner's session gets the grid of the area above the panel (not while the
// handle is being dragged), and a session whose panel closed goes back to the
// shared size. It returns the terminal's part of term.
func (win *window) dockFit(gtx layout.Context, term image.Rectangle, cw, ch, pad int) image.Rectangle {
	owner := win.dockOwner()
	top, panel := term, image.Rectangle{}
	if win.dockOf(owner) != nil {
		top, panel = dockSplit(term, win.dockHeight(), gtx.Metric.PxPerDp, ch, pad)
	}
	win.dock.lastRect = panel
	if panel.Empty() {
		win.dock.hits = dockHits{} // no panel this frame: nothing of it takes input or the keyboard
	}
	for id := range win.dock.sized {
		if win.dockOf(id) == nil || (id == owner && panel.Empty()) {
			win.mgr.Dock(id)
			delete(win.dock.sized, id)
		}
	}
	if panel.Empty() || win.dock.drag.on {
		return top
	}
	grid := image.Rect(top.Min.X+pad, top.Min.Y+pad/2, top.Max.X-pad, top.Max.Y-pad/2)
	cols, rows := gridSize(grid.Dx(), grid.Dy(), cw, ch)
	win.mgr.ResizeTab(owner, cols, rows)
	if win.dock.sized == nil {
		win.dock.sized = map[tabs.TabID]bool{}
	}
	win.dock.sized[owner] = true
	return top
}

// dockRect is the panel as laid out this frame, empty when none.
func (win *window) dockRect() image.Rectangle { return win.dock.lastRect }

// pruneDock forgets the panels of closed sessions.
func (win *window) pruneDock(list []*tabs.Tab) {
	for id := range win.dock.by {
		if !slices.ContainsFunc(list, func(t *tabs.Tab) bool { return t.ID == id }) {
			delete(win.dock.by, id)
			delete(win.dock.sized, id)
		}
	}
	for id := range win.dock.sized {
		if !slices.ContainsFunc(list, func(t *tabs.Tab) bool { return t.ID == id }) {
			delete(win.dock.sized, id)
		}
	}
}
