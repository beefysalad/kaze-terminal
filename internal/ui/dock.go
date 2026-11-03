package ui

import (
	"image"
	"time"

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
	if win.dockOwner() == session {
		win.dock.keys = false
	}
	win.invalidate()
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

// Filled in by the drawing code.
type (
	dockCache struct{}
	dockHits  struct{}
)
