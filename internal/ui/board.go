package ui

import (
	"image"
	"strconv"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/patrick-gogo/kaze-terminal/internal/claudesetup"
	kinput "github.com/patrick-gogo/kaze-terminal/internal/input"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// boardState is the agents board (spec 2026-11-03-kaze-agents-board-design): a
// page over the terminal area with a filtered list of session cards on the
// left and a preview of the selected agent on the right.
type boardState struct {
	open     bool
	tab      tabs.TabID // the session token: the session it is filtered to; 0 for all
	back     image.Rectangle
	setup    image.Rectangle // the hooks hint's button
	search   image.Rectangle // the search field
	token    image.Rectangle // the "<session> ×" token; empty when none
	list     image.Rectangle // the list pane
	preview  image.Rectangle // the preview pane; empty while narrow
	seg      [4]image.Rectangle
	rows     []boardRowHit
	cardHits []boardCardHit
	texts    []string // drawn text, for tests
	scroll   int
	missing  bool // the subagent hooks are not all set up
	tag      int

	filter    boardFilter
	ed        widget.Editor // the search field
	searching bool          // the search field has the keyboard
	edKeys    []event.Filter
	sel       subagents.Key // the previewed agent
	hover     subagents.Key
	showDone  map[tabs.TabID]bool // sessions whose finished rows are unfolded
	lastRow   boardClick          // the last row press, for double clicks
	prev      dockSession         // the preview's own panel state
	prevHits  dockHits
	wide      bool // the preview pane fits beside the list
	redraws   int

	ink   boardInk // colours for inkOf, worked out once per theme
	inkOf theme.Chrome
	inkOK bool
}

// boardFilter is the board's segment; the values index [4] arrays.
type boardFilter uint8

const (
	boardAll boardFilter = iota
	boardNeeds
	boardRunning
	boardDone
)

// boardClick is a press on a board row.
type boardClick struct {
	key subagents.Key
	at  time.Duration
}

// agentsRunning is the status bar's text for n running and waiting subagents.
func agentsRunning(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "1 agent running"
	}
	return strconv.Itoa(n) + " agents running"
}

func (win *window) openBoard(tab tabs.TabID) {
	win.hideDoc()
	win.dropSideEdit()
	win.hideSettings()
	b := &win.board
	b.open, b.tab, b.scroll = true, tab, 0
	b.filter, b.searching = boardAll, false
	b.ed.SetText("")
	b.prev, b.lastRow = dockSession{}, boardClick{}
	for id := range b.showDone {
		if win.mgr.Lookup(id) == nil {
			delete(b.showDone, id)
		}
	}
	b.sel = subagents.Key{}
	win.boardSelect(win.firstWaiting(tab))
	b.missing = true
	if win.claudePath != "" {
		if _, r, err := win.readClaude(); err == nil {
			b.missing = false
			for _, e := range []claudesetup.Entry{r.AgentStart, r.AgentStop, r.Tools} {
				b.missing = b.missing || e.State != claudesetup.OK
			}
		}
	}
	win.endGesture()
	win.wantFocus = true
	if win.w != nil {
		win.w.Invalidate()
	}
}

// clearBoardSession drops the board's session token: it shows all sessions.
func (win *window) clearBoardSession() {
	win.board.tab = 0
	if win.w != nil {
		win.w.Invalidate()
	}
}

// boardSelect previews k; a new agent starts a fresh preview that follows.
func (win *window) boardSelect(k subagents.Key) {
	b := &win.board
	b.sel = k
	if k != b.prev.agent {
		b.prev = dockSession{agent: k, follow: true}
	}
}

// firstWaiting is the first subagent waiting on you, in start order, in tab
// (any tab when tab is 0); zero when none is.
func (win *window) firstWaiting(tab tabs.TabID) subagents.Key {
	_, wait, _ := win.subs.Board()
	for _, a := range wait {
		if tab == 0 || tabs.TabID(a.Tab) == tab {
			return a.Key
		}
	}
	return subagents.Key{}
}

func (win *window) closeBoard() {
	if !win.board.open {
		return
	}
	win.board.open = false
	win.wantFocus = true
	if win.w != nil {
		win.w.Invalidate()
	}
}

func (win *window) toggleBoard() {
	if win.board.open {
		win.closeBoard()
		return
	}
	win.openBoard(0)
}

// sessionName is a session's title as the sidebar shows it, and its repo.
func (win *window) sessionName(id tabs.TabID) (title, repo string) {
	title, repo, _ = win.sessionMeta(id)
	return title, repo
}

// sessionMeta is a session's title as the sidebar shows it, its repo and branch;
// all "" for a session that is gone.
func (win *window) sessionMeta(id tabs.TabID) (title, repo, branch string) {
	t := win.mgr.Lookup(id)
	if t == nil {
		return "", "", ""
	}
	loc := win.locOf(t)
	title = win.sideIn().label(t, loc, win.rowAgent(id)) // use the sidebar's title priority and detected agent
	if title == "" {
		title = "session"
	}
	return title, loc.repoName, loc.branch
}

// boardInk is the board's colours for the current theme, worked out once per theme.
func (win *window) boardInk() boardInk {
	b := &win.board
	if !b.inkOK || b.inkOf != win.ui.th {
		b.ink, b.inkOf, b.inkOK = win.ui.boardInk(), win.ui.th, true
	}
	return b.ink
}

// boardWideDp is the least board width that shows the preview beside the list
// (mockup: a 460 px list and a 400 px preview).
const boardWideDp = 860

// drawBoard lays the board out in r: the list, and the preview when it fits.
func (win *window) drawBoard(gtx layout.Context, r image.Rectangle) {
	b := &win.board
	b.rows, b.cardHits, b.texts = b.rows[:0], b.cardHits[:0], b.texts[:0]
	b.back, b.setup, b.search, b.token, b.preview = image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, image.Rectangle{}
	b.seg = [4]image.Rectangle{}
	ink := win.boardInk()
	ops := gtx.Ops
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()
	paint.FillShape(ops, ink.bg, clip.Rect(r).Op())
	area := clip.Rect(r).Push(ops)
	event.Op(ops, &b.tag)
	area.Pop()

	b.wide = r.Dx() >= gtx.Dp(boardWideDp)
	b.list = r
	if b.wide {
		cut := r.Min.X + r.Dx()*105/205
		b.list = image.Rect(r.Min.X, r.Min.Y, cut, r.Max.Y)
		b.preview = image.Rect(cut+1, r.Min.Y, r.Max.X, r.Max.Y)
		paint.FillShape(ops, ink.line, clip.Rect(image.Rect(cut, r.Min.Y, cut+1, r.Max.Y)).Op())
	}
	v := win.boardView()
	win.drawBoardList(gtx, b.list, &v, ink)
}

// boardBack is the session the board returns to.
func (win *window) boardBack() tabs.TabID {
	if a := win.mgr.Active(); a != nil {
		return a.ID
	}
	return 0
}

func (win *window) boardEvents(gtx layout.Context) {
	tag := &win.board.tag
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: tag},
			key.Filter{Focus: tag, Optional: allMods},
			pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		)
		if !ok {
			return
		}
		if win.bench.IgnoreInput() {
			continue
		}
		switch e := ev.(type) {
		case key.Event:
			if e.State == key.Press {
				if a, ok := win.km[kinput.Shortcut{Name: e.Name, Mods: e.Modifiers}]; ok {
					win.runShortcut(gtx, a)
					continue
				}
			}
			win.boardKey(e)
		case pointer.Event:
			switch {
			case e.Kind == pointer.Scroll:
				win.board.scroll += int(e.Scroll.Y)
				win.w.Invalidate()
			case e.Buttons.Contain(pointer.ButtonPrimary):
				win.boardPress(gtx, e.Position.Round(), e.Time)
			}
		}
	}
}

// boardKey handles a key on the board: Esc closes it.
func (win *window) boardKey(e key.Event) {
	if e.State == key.Press && e.Name == key.NameEscape {
		win.closeBoard()
	}
}

// boardPress handles a click on the board at p; at is the press time.
func (win *window) boardPress(gtx layout.Context, p image.Point, at time.Duration) {
	b := &win.board
	invalidate := func() {
		if win.w != nil {
			win.w.Invalidate()
		}
	}
	switch {
	case p.In(b.back):
		win.closeBoard()
		return
	case p.In(b.setup):
		win.closeBoard()
		win.openSettings()
		for i, s := range win.sections() {
			if s.title == "Agents" {
				win.selectSection(i)
			}
		}
		return
	case p.In(b.token):
		win.clearBoardSession()
		return
	case p.In(b.search):
		b.searching = true
		win.wantFocus = true
		invalidate()
		return
	}
	for i, s := range b.seg {
		if p.In(s) {
			b.filter, b.scroll = boardFilter(i), 0
			invalidate()
			return
		}
	}
	for _, h := range b.rows {
		if p.In(h.close) {
			win.subs.Dismiss(h.row.key)
			win.dropGoneAgents()
			invalidate()
			return
		}
	}
	for _, h := range b.rows {
		if !p.In(h.rect) {
			continue
		}
		win.closeBoard()
		if h.row.session {
			win.selectTab(h.row.tab)
		} else {
			win.showAgent(h.row.tab, h.row.key, false)
		}
		return
	}
	for _, c := range b.cardHits {
		switch {
		case p.In(c.goTo):
			win.closeBoard()
			win.selectTab(c.tab)
			return
		case p.In(c.clear):
			win.subs.ClearDone(int(c.tab))
			win.dropGoneAgents()
			invalidate()
			return
		case p.In(c.fold):
			if b.showDone == nil {
				b.showDone = map[tabs.TabID]bool{}
			}
			b.showDone[c.tab] = !b.showDone[c.tab]
			invalidate()
			return
		}
	}
}
