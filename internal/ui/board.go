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

	"github.com/beefysalad/kaze-terminal/internal/claudesetup"
	kinput "github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
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
	preview  image.Rectangle // the preview pane; empty while narrow
	openIn   image.Rectangle // the preview's "Open in session"
	copy     image.Rectangle // the preview's "Copy report"; empty unless done
	reader   image.Rectangle // the preview's "Open in reader"; empty unless done
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
	edFocused bool          // the search field had the focus last frame
	edKeys    []event.Filter
	sel       subagents.Key // the previewed agent
	hover     subagents.Key
	showDone  map[tabs.TabID]bool // sessions whose finished rows are unfolded
	lastRow   boardClick          // the last row press, for double clicks
	prev      dockSession         // the preview's own panel state
	prevHits  dockHits
	wide      bool   // the preview pane fits beside the list
	reveal    bool   // scroll the selected row into view on the next draw
	selY      [2]int // the selected row's top and bottom as drawn, unscrolled; [0,0] when not drawn
	redraws   int    // hover changes that asked for a frame; tests count them

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
	win.board.prev = dockSession{} // nobody sees the preview: hold no transcript or parsed report
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
	// A token whose session closed, or a selection whose agent is gone, goes now.
	if b.tab != 0 && win.mgr.Lookup(b.tab) == nil {
		b.tab = 0
	}
	if b.sel != (subagents.Key{}) && win.subs.Get(b.sel) == nil {
		b.sel, b.prev = subagents.Key{}, dockSession{}
	}
	ink := win.boardInk()
	ops := gtx.Ops
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()
	paint.FillShape(ops, ink.bg, clip.Rect(r).Op())
	area := clip.Rect(r).Push(ops)
	event.Op(ops, &b.tag)
	area.Pop()

	b.wide = r.Dx() >= gtx.Dp(boardWideDp)
	list := r
	if b.wide {
		cut := r.Min.X + r.Dx()*105/205
		list = image.Rect(r.Min.X, r.Min.Y, cut, r.Max.Y)
		b.preview = image.Rect(cut+1, r.Min.Y, r.Max.X, r.Max.Y)
		paint.FillShape(ops, ink.line, clip.Rect(image.Rect(cut, r.Min.Y, cut+1, r.Max.Y)).Op())
	}
	v := win.boardView()
	win.drawBoardList(gtx, list, &v, ink)
	if b.wide {
		win.drawBoardPreview(gtx, b.preview, ink)
	} else {
		b.openIn, b.copy, b.reader, b.prevHits = image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, dockHits{}
		b.prev.dropTranscript() // nobody sees a preview: hold no transcript
	}
}

// boardBack is the session the board returns to.
func (win *window) boardBack() tabs.TabID {
	if a := win.mgr.Active(); a != nil {
		return a.ID
	}
	return 0
}

func (win *window) boardEvents(gtx layout.Context) {
	b := &win.board
	tag := &b.tag
	win.boardSearchEvents(gtx)
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: tag},
			key.Filter{Focus: tag, Optional: allMods},
			pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Scroll | pointer.Move | pointer.Enter | pointer.Leave, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
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
			p := e.Position.Round()
			switch {
			case e.Kind == pointer.Scroll:
				if p.In(b.prevHits.body) {
					b.prev.scroll += int(e.Scroll.Y)
					b.prev.follow = b.prev.scroll >= b.prevHits.scrollMax
				} else {
					b.scroll += int(e.Scroll.Y)
				}
				win.w.Invalidate()
			case e.Kind == pointer.Move || e.Kind == pointer.Enter || e.Kind == pointer.Leave:
				var k subagents.Key
				if e.Kind != pointer.Leave {
					for _, h := range b.rows {
						if p.In(h.rect) && !h.row.session {
							k = h.row.key
						}
					}
				}
				if k != b.hover { // redraw only when the hovered row changes
					b.hover = k
					b.redraws++
					win.w.Invalidate()
				}
			case e.Kind == pointer.Press && e.Buttons.Contain(pointer.ButtonPrimary):
				win.boardPress(gtx, p, e.Time)
			}
		}
	}
}

// boardSearchEvents runs the search field: Esc clears it (or closes the board
// when it is empty), Enter keeps the text and gives the keyboard back, kaze's
// shortcuts still work.
func (win *window) boardSearchEvents(gtx layout.Context) {
	b := &win.board
	b.ed.SingleLine, b.ed.Submit = true, true
	if b.edKeys == nil {
		b.edKeys = win.edKeys(&b.ed)
	}
	// Only a field that newly has the focus is a click straight into it: one
	// that still has it from before may be about to lose it to a FocusCmd that
	// runs at the end of this frame (a "+N more" chip, say).
	focused := gtx.Focused(&b.ed)
	if focused && !b.edFocused {
		b.searching = true
	}
	b.edFocused = focused
	if win.edShortcuts(gtx, &b.ed, b.edKeys, win.boardSearchEsc) {
		return // a shortcut ran; the layout drains what is left
	}
	for {
		ev, ok := b.ed.Update(gtx)
		if !ok {
			return
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			win.ui.fit(b.ed.Text()) // loads a fallback font for any rune the bundled ones lack
			b.scroll = 0
			win.invalidate()
		case widget.SubmitEvent:
			b.searching = false
			win.wantFocus = true
			win.invalidate()
		}
	}
}

// boardSearchEsc is Esc on the board: a search with text is cleared and gives
// the keyboard back; an empty one closes the board.
func (win *window) boardSearchEsc() {
	b := &win.board
	if b.ed.Text() == "" {
		win.closeBoard()
		return
	}
	b.ed.SetText("")
	b.searching, b.scroll = false, 0
	win.wantFocus = true
	win.invalidate()
}

// boardKey handles a key while the board, not its search field, has the keyboard.
func (win *window) boardKey(e key.Event) {
	if e.State != key.Press {
		return
	}
	b := &win.board
	switch e.Name {
	case key.NameEscape:
		win.boardSearchEsc()
	case "/":
		b.searching = true
		win.wantFocus = true
		win.invalidate()
	case key.NameUpArrow:
		win.boardMove(-1)
	case key.NameDownArrow:
		win.boardMove(1)
	case key.NameReturn, key.NameEnter:
		for _, r := range win.boardView().order {
			if r.key == b.sel {
				win.boardOpenAgent(r)
				return
			}
		}
		if a := win.boardPreviewAgent(); a != nil { // previewed, but hidden by the filter, search or a fold
			win.boardOpenAgent(boardRow{key: a.Key, tab: tabs.TabID(a.Tab)})
		}
	}
}

// boardMove moves the selection d rows through the selectable rows, stopping at
// the ends; from no selection, Down picks the first and Up the last.
func (win *window) boardMove(d int) {
	b := &win.board
	order := win.boardView().order
	if len(order) == 0 {
		return
	}
	i := -1
	for j, r := range order {
		if r.key == b.sel {
			i = j
		}
	}
	switch {
	case i < 0 && d > 0:
		i = 0
	case i < 0:
		i = len(order) - 1
	default:
		i = min(max(i+d, 0), len(order)-1)
	}
	win.boardSelect(order[i].key)
	b.reveal = true
	win.invalidate()
}

// boardOpenAgent closes the board and shows r in its session: the agent's
// panel, or the session itself for a session wait.
func (win *window) boardOpenAgent(r boardRow) {
	win.closeBoard()
	win.board.lastRow = boardClick{}
	if r.session {
		win.selectTab(r.tab)
		return
	}
	win.showAgent(r.tab, r.key, false)
}

// boardPress handles a click on the board at p; at is the press time.
func (win *window) boardPress(gtx layout.Context, p image.Point, at time.Duration) {
	b := &win.board
	if b.searching && !p.In(b.search) {
		b.searching = false // a click elsewhere gives the keyboard back to the board
		win.wantFocus = true
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
		win.invalidate()
		return
	}
	if a := win.boardPreviewAgent(); a != nil {
		tab := tabs.TabID(a.Tab)
		switch {
		case win.readPress(&b.prev, &b.prevHits, p):
			return
		case p.In(b.openIn):
			win.boardOpenAgent(boardRow{key: a.Key, tab: tab})
			return
		case p.In(b.copy):
			win.copyReport(gtx, a)
			return
		case p.In(b.reader):
			report := win.dockReport(a) // before closing: it reads the preview's transcript
			win.closeBoard()
			win.openReportDoc(tab, a.Type+" · "+a.Task, report)
			return
		}
	}
	for i, s := range b.seg {
		if p.In(s) {
			b.filter, b.scroll = boardFilter(i), 0
			win.invalidate()
			return
		}
	}
	for _, h := range b.rows {
		if p.In(h.close) {
			win.subs.Dismiss(h.row.key)
			win.dropGoneAgents()
			win.invalidate()
			return
		}
	}
	for _, h := range b.rows {
		if !p.In(h.rect) {
			continue
		}
		last := b.lastRow
		switch {
		case !b.wide || h.row.session:
			win.boardOpenAgent(h.row) // no preview to show it in
		case last.key == h.row.key && at-last.at < doubleClick:
			win.boardOpenAgent(h.row)
		default:
			win.boardSelect(h.row.key)
			b.lastRow = boardClick{key: h.row.key, at: at}
			win.invalidate()
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
			win.invalidate()
			return
		case p.In(c.fold):
			if b.showDone == nil {
				b.showDone = map[tabs.TabID]bool{}
			}
			b.showDone[c.tab] = !b.showDone[c.tab]
			win.invalidate()
			return
		}
	}
}
