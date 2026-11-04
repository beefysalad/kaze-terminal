package ui

import (
	"image"
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// The docked panel (spec 2026-11-03-kaze-agent-dock-design, issue #133): a
// deck of tabs under the terminal, remembered per session. A tab is one
// subagent's Activity and Report, or a workspace member's terminal.

const (
	dockMinDp    = 120 // the panel's least height
	dockHideDp   = 40  // below this, a squeezed panel is not drawn at all
	dockMinRows  = 8   // the terminal keeps at least this many rows
	dockShareDef = 0.4 // default height: this share of the terminal area
	dockMaxTabs  = 6   // per session, members and agents together
)

type dockTab uint8

const (
	dockActivity dockTab = iota
	dockReport
)

// dockSession is one agent tab's view: what it shows and how far it is read.
// The board's preview keeps one too.
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

// dockEntry is one tab of a session's panel: an agent's view, or a member.
type dockEntry struct {
	member tabs.TabID   // 0 for an agent
	view   *dockSession // agent tabs only
	opened uint64       // when it was last shown: the oldest not shown is replaced first
}

// dockDeck is one session's panel: its tabs, the one shown, and whether the
// panel is hidden (its tabs kept).
type dockDeck struct {
	entries []dockEntry
	on      int // index shown, -1 none
	hidden  bool
}

// shown is the entry the panel shows, nil when none.
func (d *dockDeck) shown() *dockEntry {
	if d == nil || d.on < 0 || d.on >= len(d.entries) {
		return nil
	}
	return &d.entries[d.on]
}

// dockDrag is a handle drag in progress.
type dockDrag struct {
	on      bool
	fromY   int // press position, window px
	startDp int // height at the press
}

// dockState is the window's panel state; UI goroutine only.
type dockState struct {
	by                           map[tabs.TabID]*dockDeck
	seq                          uint64              // tab show stamps
	sized                        map[tabs.TabID]bool // sessions whose PTY the panel shrank
	heightDp                     int                 // 0 until dragged: the config value, else the default share
	keys                         bool                // the panel has the keyboard
	tag, handle, head, row, body int                 // event tags
	hover                        int                 // the tab under the pointer, while hovering
	hovering                     bool                // the pointer is over a tab
	redraws                      int                 // frames hover changes asked for; tests count them
	drag                         dockDrag
	hits                         dockHits
	lastRect                     image.Rectangle // the panel as drawn last frame, empty when none
	texts                        []string        // text drawn last frame; tests read it
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

// deckOf is session id's panel tabs, nil when it has none.
func (win *window) deckOf(id tabs.TabID) *dockDeck { return win.dock.by[id] }

// dockOf is the agent view session id's panel shows now: nil when the session
// has no tabs, its panel is hidden or a member tab shows.
func (win *window) dockOf(id tabs.TabID) *dockSession {
	d := win.dock.by[id]
	if d == nil || d.hidden {
		return nil
	}
	if e := d.shown(); e != nil {
		return e.view
	}
	return nil
}

// dockShown reports whether session id's panel shows a tab.
func (win *window) dockShown(id tabs.TabID) bool {
	d := win.dock.by[id]
	return d != nil && !d.hidden && d.shown() != nil
}

// deck is session's panel, made when missing.
func (win *window) deck(session tabs.TabID) *dockDeck {
	if win.dock.by == nil {
		win.dock.by = map[tabs.TabID]*dockDeck{}
	}
	d := win.dock.by[session]
	if d == nil {
		d = &dockDeck{on: -1}
		win.dock.by[session] = d
	}
	return d
}

// showEntry shows entry i of d and unhides the panel.
func (win *window) showEntry(d *dockDeck, i int) {
	win.dock.seq++
	d.entries[i].opened = win.dock.seq
	d.on, d.hidden = i, false
	win.invalidate()
}

// addEntry appends e to session's panel and shows it; past dockMaxTabs the
// tab shown longest ago (never the new one) leaves.
func (win *window) addEntry(session tabs.TabID, e dockEntry) {
	d := win.deck(session)
	d.entries = append(d.entries, e)
	win.showEntry(d, len(d.entries)-1)
	if len(d.entries) <= dockMaxTabs {
		return
	}
	old := -1
	for i, x := range d.entries {
		if i != d.on && (old < 0 || x.opened < d.entries[old].opened) {
			old = i
		}
	}
	win.removeEntry(d, old)
}

// removeEntry takes entry i out of d; the shown index follows its tab, or
// moves to the left neighbour (else the right) when i was the one shown.
func (win *window) removeEntry(d *dockDeck, i int) {
	d.entries = slices.Delete(d.entries, i, i+1)
	switch {
	case len(d.entries) == 0:
		d.on = -1
	case i < d.on:
		d.on--
	case i == d.on:
		d.on = max(i-1, 0)
	}
}

// openDock shows subagent k in session's panel: its tab when it has one, else
// a new tab, which starts on Activity.
func (win *window) openDock(session tabs.TabID, k subagents.Key) {
	d := win.deck(session)
	for i, e := range d.entries {
		if e.view != nil && e.view.agent == k {
			win.showEntry(d, i)
			return
		}
	}
	win.addEntry(session, dockEntry{view: &dockSession{agent: k, follow: true}})
}

// closeDockTab is a tab's ×: entry i leaves session's panel (an agent is
// untouched); closing the last tab removes the panel.
func (win *window) closeDockTab(session tabs.TabID, i int) {
	d := win.dock.by[session]
	if d == nil || i < 0 || i >= len(d.entries) {
		return
	}
	shown := i == d.on
	win.removeEntry(d, i)
	if len(d.entries) == 0 {
		delete(win.dock.by, session)
	}
	if shown {
		win.dropKeysOf(session)
	}
	win.invalidate()
}

// closeDock hides session's panel; its tabs and height are kept.
func (win *window) closeDock(session tabs.TabID) {
	d := win.dock.by[session]
	if d == nil || d.hidden {
		return
	}
	d.hidden = true
	win.dropKeysOf(session)
	win.invalidate()
}

// dropKeysOf takes the keyboard from session's panel when it has it.
func (win *window) dropKeysOf(session tabs.TabID) {
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
}

// dropDockKeys gives the keyboard back to the terminal if the panel had it.
func (win *window) dropDockKeys() {
	if win.dock.keys {
		win.dock.keys, win.wantFocus = false, true
	}
}

// toggleDock hides session's panel when it shows k, else shows k.
func (win *window) toggleDock(session tabs.TabID, k subagents.Key) {
	if s := win.dockOf(session); s != nil && s.agent == k {
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
	if win.dockShown(owner) {
		top, panel = dockSplit(term, win.dockHeight(), gtx.Metric.PxPerDp, ch, pad)
	}
	win.dock.lastRect = panel
	if panel.Empty() {
		win.dock.hits = dockHits{} // no panel this frame: nothing of it takes input or the keyboard
	}
	for id := range win.dock.sized {
		if !win.dockShown(id) || (id == owner && panel.Empty()) {
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
