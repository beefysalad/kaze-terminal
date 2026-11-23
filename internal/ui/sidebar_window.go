package ui

import (
	"image"
	"slices"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	kinput "github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const (
	// minSideWindow is the narrowest window, in dp, that still shows the full sidebar;
	// below it a saved full sidebar shows as the rail.
	minSideWindow = 560
	// doubleClick is the longest gap between two presses on a row's title that renames it.
	doubleClick = 500 * time.Millisecond
)

// sideUI is the window's sidebar: the state the drawing needs, the folds kept
// while kaze runs and where the last frame put things. UI goroutine only.
type sideUI struct {
	runIDs        []uint64
	runScratch    []sideRun
	filterCache   string
	filterScratch []byte
	cache         sideCache
	tabs          []*tabs.Tab
	st            sideState
	folded        map[string]bool // repo groups the owner folded, by key
	hidden        bool            // this frame's mode is hidden (saved so, or the window is too narrow)
	w             int             // this frame's sidebar width in px, set once at the frame's start
	hits          sideHits        // sidebar-local, like the pointer events on tag
	at            image.Point     // the sidebar's top-left in the window
	tag           int             // pointer events on the sidebar
	reveal        bool            // scroll the shown row into view next frame
	redraws       int             // frames the sidebar's pointer input asked for; tests count them
	ed            widget.Editor   // the filter field
	filtering     bool            // the filter field has the keyboard
	mode          sideMode        // this frame's effective mode, set by sideWidth
	filters       []event.Filter  // the filter's own keys, built once
	ren           widget.Editor   // the title of the row being renamed
	renaming      tabs.TabID      // 0 when no row is being renamed
	renFrom       string          // the text the rename started with
	renKeys       []event.Filter  // the rename editor's own keys, built once
}

// isMember reports whether t belongs to an open workspace run; members are not sessions.
func (win *window) isMember(t *tabs.Tab) (uint64, bool) {
	if t.Group != nil && win.runs[t.Group.Run] != nil {
		return t.Group.Run, true
	}
	return 0, false
}

// sideIn is the sidebar model's input, unfiltered and without the workspaces section.
func (win *window) sideIn() sideInput {
	in := sideInput{
		Tabs:     win.mgr.Tabs(),
		Loc:      win.locOf,
		Mark:     win.alerts.markOf,
		Member:   win.isMember,
		Folded:   win.side.folded,
		Home:     win.home,
		DefTitle: win.defTitle,
		Agent:    win.rowAgent,
		Name:     func(id tabs.TabID) string { return win.names[id] },
		Lines:    win.linesOf,
		Agents:   win.sideAgents,
		Docs:     win.sideDocs,
	}
	if win.ui != nil {
		in.Clean = win.ui.fit
	}
	return in
}

// sessionIDs is every session in sidebar order, folded groups included, members left out.
func (win *window) sessionIDs() []tabs.TabID {
	win.cachedSide(time.Now())
	return win.side.cache.order
}

// selectSession shows a session picked by keyboard and scrolls its row into view.
func (win *window) selectSession(id tabs.TabID) {
	win.selectTab(id)
	win.side.reveal = true
	win.w.Invalidate()
}

// closeTab closes a tab. When it is the shown one, the next session in sidebar
// order (else the one before) is shown first, so closing never lands on a member
// while a session is left; with none left, the first open member is shown.
func (win *window) closeTab(id tabs.TabID) {
	win.system.remove(id)
	win.closeSessionDocs(id)
	win.closeSessionDock(id)
	if a := win.mgr.Active(); a != nil && a.ID == id {
		if next := win.afterClose(id); next != 0 {
			win.mgr.SetActive(next)
			win.side.reveal = true
		}
	}
	win.mgr.Close(id)
	win.w.Invalidate()
}

// afterClose is the tab to show once id closes: its space mate (the next, else
// the one before), else the neighbouring space's last used tab.
func (win *window) afterClose(id tabs.TabID) tabs.TabID {
	win.cachedSide(time.Now())
	if sp := win.side.cache.spaces.Spaces; spaceIndex(sp, id) >= 0 {
		i := spaceIndex(sp, id)
		mates := sp[i].Tabs
		switch j := slices.Index(mates, id); {
		case j+1 < len(mates):
			return mates[j+1]
		case j > 0:
			return mates[j-1]
		}
		for _, k := range [2]int{i + 1, i - 1} {
			if k >= 0 && k < len(sp) {
				return win.spaceTarget(sp[k])
			}
		}
	}
	order := win.sessionIDs()
	switch i := slices.Index(order, id); {
	case i >= 0 && i+1 < len(order):
		return order[i+1]
	case i > 0:
		return order[i-1]
	case i < 0 && len(order) > 0:
		return order[0]
	}
	for _, t := range win.mgr.Tabs() {
		if _, ok := win.isMember(t); ok && t.ID != id {
			return t.ID
		}
	}
	return 0
}

// toggleSidebar steps the saved mode full, rail, hidden, full and saves it. A
// narrow window still steps the saved mode, even while it shows full as the rail.
func (win *window) toggleSidebar() {
	win.setSideMode(win.savedSideMode().next())
}

// sideWidth is how wide the sidebar is in a window this wide (0 while hidden) and
// sets this frame's effective mode. A narrow window narrows it without touching the setting.
func (win *window) sideWidth(gtx layout.Context, width int) int {
	m := effectiveSideMode(win.savedSideMode(), width, gtx.Dp(minRailWindow), gtx.Dp(minSideWindow))
	if m != win.side.mode {
		// The rows and tiles move: what was under the pointer is not now, and the
		// other mode's scroll need not show the shown session.
		win.side.st.hover, win.side.st.hoverSpace, win.side.st.hoverBtn = 0, "", btnNone
		win.side.reveal = true
	}
	win.side.mode = m
	win.side.hidden = m == sideHidden
	if m != sideFull {
		win.dropFilter() // only the full sidebar has the filter; an invisible field must not keep the keyboard
	}
	switch m {
	case sideFull:
		return min(gtx.Dp(sidebarW), width)
	case sideRail:
		return min(gtx.Dp(railW), width)
	}
	return 0 // its buttons live in the title bar
}

// drawSide draws the sidebar into r and
// takes its pointer input there.
func (win *window) drawSide(gtx layout.Context, r image.Rectangle) {
	s := &win.side
	s.at = r.Min
	if r.Empty() {
		s.hits, s.st.hover, s.st.hoverSpace, s.st.hoverBtn, s.reveal = sideHits{}, 0, "", btnNone, false
		s.cache.drawValid = false // the cached ops still match the full list; its hits are gone
		return
	}
	ops := gtx.Ops
	off := op.Offset(r.Min).Push(ops)
	defer off.Pop()
	sg := gtx
	sg.Constraints = layout.Exact(r.Size())
	{ // r is this frame's sidebar, decided before any toggle in it: draw it, never leave it bare
		m := win.cachedSide(gtx.Now)
		s.st.filter, s.st.ed = s.filterText(), nil
		if s.filtering {
			s.st.ed = &s.ed
		}
		s.st.filterOpen = s.filtering || s.st.filter != ""
		s.st.shown, s.st.activeRepo, s.st.space = 0, "", ""
		if a := win.mgr.Active(); a != nil {
			s.st.shown = a.ID
			s.st.activeRepo = win.locOf(a).repo
		}
		s.st.space = win.selectedSpaceKey(s.st.shown)
		s.st.doc = win.docs.shown
		s.st.settings, s.st.settingsOn = win.set.open, win.set.shown
		s.st.docked = win.shownMember()
		s.st.right = win.sideRight()
		s.st.tipRoom = max(gtx.Constraints.Max.X-r.Dx(), 1) // the tooltip stays inside the window
		key := sideDrawKey{activeRepo: s.st.activeRepo, space: s.st.space, hoverSpace: s.st.hoverSpace, hoverBtn: s.st.hoverBtn, filterOpen: s.st.filterOpen, ages: agentAges(gtx.Now, m.Agents), size: r.Size(), metric: sg.Metric, locale: sg.Locale, shown: s.st.shown, doc: s.st.doc, hoverDoc: s.st.hoverDoc, hover: s.st.hover, scroll: s.st.scroll, settings: s.st.settings, settingsOn: s.st.settingsOn, right: s.st.right, tipRoom: s.st.tipRoom, mode: s.mode, docked: s.st.docked, theme: win.ui.th, shaper: win.ui.shaper, generation: s.cache.builds}
		if !s.cache.drawValid || s.cache.drawKey != key || s.reveal {
			s.cache.ops.Reset()
			cg := sg
			cg.Ops = &s.cache.ops
			rec := op.Record(cg.Ops)
			static := s.st
			static.ed = nil
			static.static = true
			h := win.layoutSide(cg, m, static, r.Size())
			call := rec.Stop()
			if s.reveal {
				s.reveal = false
				if sc, ok := win.sideReveal(cg, m, s.st, h); ok && sc != s.st.scroll {
					s.st.scroll = sc
					static.scroll = sc
					rec = op.Record(cg.Ops)
					h = win.layoutSide(cg, m, static, r.Size())
					call = rec.Stop()
				}
			}
			s.cache.call = call
			s.hits = h
			key.scroll = s.st.scroll
			s.cache.drawKey = key
			s.cache.drawValid = true
			s.cache.draws++
		}
		s.cache.call.Add(ops)
		win.drawSideEditors(sg)
		h := s.hits
		if s.filtering && h.filter.Empty() {
			win.leaveFilter() // no field is drawn (the rail): the keyboard must not stay in it
		}
		s.st.scroll = min(max(s.st.scroll, 0), h.scrollMax)
	}
	area := clip.Rect{Max: r.Size()}.Push(ops)
	event.Op(ops, &s.tag)
	area.Pop()
}

// layoutSide lays out this frame's mode: the full sidebar or the rail.
func (win *window) layoutSide(gtx layout.Context, m sideModel, st sideState, size image.Point) sideHits {
	if win.side.mode == sideRail {
		return win.ui.layoutRail(gtx, m, st, size)
	}
	return win.ui.layoutSidebar(gtx, m, st, size)
}

// sideReveal is the scroll that shows the shown session in this frame's mode.
func (win *window) sideReveal(gtx layout.Context, m sideModel, st sideState, h sideHits) (int, bool) {
	if win.side.mode == sideRail {
		return railReveal(gtx, m, st, h)
	}
	return revealScroll(gtx, m, st, h)
}

func (win *window) sideEvents(gtx layout.Context) {
	kinds := pointer.Press | pointer.Scroll | pointer.Move | pointer.Enter | pointer.Leave
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &win.side.tag, Kinds: kinds, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}})
		if !ok {
			return
		}
		if e, ok := ev.(pointer.Event); ok && !win.bench.IgnoreInput() {
			win.sidePointer(e)
		}
	}
}

// edKeys are a sidebar editor's own keys, taken before the editor sees them: Esc,
// Tab and kaze's Ctrl / Cmd shortcuts.
func (win *window) edKeys(ed *widget.Editor) []event.Filter {
	fs := []event.Filter{
		key.Filter{Focus: ed, Name: key.NameEscape, Optional: allMods},
		key.Filter{Focus: ed, Name: key.NameTab, Optional: allMods},
	}
	for sc := range win.km {
		if sc.Mods&(key.ModCtrl|key.ModCommand) != 0 {
			fs = append(fs, key.Filter{Focus: ed, Name: sc.Name, Required: sc.Mods})
		}
	}
	return fs
}

// edShortcuts runs the keys edKeys caught for ed, esc on Esc; it reports whether a
// kaze shortcut ran.
func (win *window) edShortcuts(gtx layout.Context, ed *widget.Editor, keys []event.Filter, esc func()) bool {
	for {
		ev, ok := gtx.Event(keys...)
		if !ok {
			return false
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && !win.bench.IgnoreInput() && win.edKey(gtx, ed, e, esc) {
			return true
		}
	}
}

// filterEvents takes Esc, Enter and kaze's Ctrl / Cmd shortcuts before the editor,
// which gets every other key.
func (win *window) filterEvents(gtx layout.Context) {
	s := &win.side
	s.ed.SingleLine, s.ed.Submit = true, true
	if s.filters == nil {
		s.filters = win.edKeys(&s.ed)
	}
	clearFilter := func() {
		s.ed.SetText("")
		win.leaveFilter()
	}
	if win.edShortcuts(gtx, &s.ed, s.filters, clearFilter) {
		return // a shortcut ran; the layout drains what is left
	}
	// Layout drains Update itself and would drop a change still queued; take it first.
	for {
		ev, ok := s.ed.Update(gtx)
		if !ok {
			return
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			win.ui.fit(s.ed.Text()) // loads a fallback font for any rune the bundled ones lack
			s.st.scroll = 0
			win.sideRedraw()
		case widget.SubmitEvent:
			win.filterEnter()
		}
	}
}

// edKey handles one key edShortcuts caught; it reports whether a kaze shortcut ran.
func (win *window) edKey(gtx layout.Context, ed *widget.Editor, e key.Event, esc func()) bool {
	if a, ok := win.km[kinput.Shortcut{Name: e.Name, Mods: e.Modifiers}]; ok {
		switch a {
		case actPaste:
			gtx.Execute(clipboard.ReadCmd{Tag: ed})
		case actCopy:
			win.writeClipboard(gtx, ed.SelectedText())
		default:
			win.runShortcut(gtx, a)
			return true
		}
		return false
	}
	if e.Name == key.NameEscape {
		esc()
	}
	return false
}

// renameEvents runs the rename editor's keys the way filterEvents runs the filter's:
// Enter saves, Esc cancels, kaze's shortcuts still work.
func (win *window) renameEvents(gtx layout.Context) {
	s := &win.side
	s.ren.SingleLine, s.ren.Submit, s.ren.MaxLen = true, true, maxSideText
	if s.renKeys == nil {
		s.renKeys = win.edKeys(&s.ren)
	}
	if win.edShortcuts(gtx, &s.ren, s.renKeys, func() { win.endRename(false) }) {
		return
	}
	for {
		ev, ok := s.ren.Update(gtx)
		if !ok {
			return
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			win.ui.fit(s.ren.Text()) // loads a fallback font for any rune the bundled ones lack
			win.sideRedraw()
		case widget.SubmitEvent:
			win.endRename(true)
		}
	}
}

// startRename puts an editor over id's title, holding the title all selected, and
// gives it the keyboard. Workspace tabs keep their fixed titles.
func (win *window) startRename(id tabs.TabID) {
	t := win.mgr.Lookup(id)
	if t == nil || t.FixedTitle != "" {
		return
	}
	s := &win.side
	s.renaming = id
	s.renFrom = win.sideIn().renameText(t, win.locOf(t), win.agentOf(id))
	s.ren.SetText(s.renFrom)
	s.ren.SetCaret(s.ren.Len(), 0)
	win.wantFocus = true
	win.sideRedraw()
}

// endRename closes the rename editor; save keeps a changed text as the row's name,
// and a blank one gives the row back to the title rule.
func (win *window) endRename(save bool) {
	s := &win.side
	id := s.renaming
	if id == 0 {
		return
	}
	s.renaming = 0
	win.wantFocus = true
	win.sideRedraw()
	name := strings.TrimSpace(s.ren.Text())
	if !save || name == s.renFrom {
		return // an untouched title must not pin what the rule would change later
	}
	if name == "" {
		delete(win.names, id)
		if win.restore != nil {
			win.restore.Rename(int(id), "")
		}
		return
	}
	if win.names == nil {
		win.names = map[tabs.TabID]string{}
	}
	win.names[id] = name
	if win.restore != nil {
		win.restore.Rename(int(id), name)
	}
}

// leaveFilter gives the keyboard back to the shown session or the settings page.
func (win *window) leaveFilter() {
	win.side.filtering = false
	win.wantFocus = true
	win.sideRedraw()
}

// filterEnter shows the first session the filter leaves visible and gives it the
// keyboard; on an empty filter it only gives the keyboard back.
func (win *window) filterEnter() {
	in := win.sideIn()
	in.Filter = win.side.ed.Text()
	in.Runs = win.sideRuns()
	if strings.TrimSpace(in.Filter) != "" {
		if ids := sessionOrder(buildSide(in)); len(ids) > 0 {
			win.selectSession(ids[0])
		}
	}
	win.leaveFilter()
}

// sideRedraw asks for one frame for a change the sidebar's input made.
func (win *window) sideRedraw() {
	win.side.redraws++
	win.w.Invalidate()
}

// clickRow is a click on a session row or a workspace member line: a member
// docks as a tab under its session (a second click hides it), a session is
// shown. It reports whether id was a session.
func (win *window) clickRow(id tabs.TabID) bool {
	if t := win.mgr.Lookup(id); t != nil {
		if _, member := win.isMember(t); member {
			win.dockMember(id, true)
			return false
		}
	}
	win.selectTab(id)
	return true
}

// lineAt is the session row or workspace member line under p.
func (h *sideHits) lineAt(p image.Point) (sideRowHit, bool) {
	for _, list := range [2][]sideRowHit{h.rows, h.members} {
		for _, r := range list {
			if p.In(r.rect) {
				return r, true
			}
		}
	}
	return sideRowHit{}, false
}

// flip folds k, or unfolds it by forgetting it.
func flip[K comparable](m map[K]bool, k K) {
	if m[k] {
		delete(m, k)
	} else {
		m[k] = true
	}
}

// nextWaiting shows the next session after the shown one that needs the owner,
// wrapping; with none it only says so in the status bar.
func (win *window) nextWaiting() {
	var shown tabs.TabID
	if a := win.mgr.Active(); a != nil {
		shown = a.ID
	}
	// With alert cards showing, the key opens the newest card's session, as
	// the card's key caps say (issue #9). A card on the shown tab (a run
	// summary, which visiting keeps) is skipped, so the key moves on.
	for i := len(win.alerts.toasts) - 1; i >= 0; i-- {
		if t := win.alerts.toasts[i].Tab; t != shown {
			win.toastGo(t)
			win.side.reveal = true
			return
		}
	}
	order := win.sessionIDs()
	from := slices.Index(order, shown)
	for i := 1; i <= len(order); i++ {
		id := order[(from+i+len(order))%len(order)]
		if win.alerts.markOf(id) == markNeeds {
			win.selectSession(id)
			return
		}
	}
	win.note("Nothing is waiting on you.")
	win.w.Invalidate()
}

// Read the bounded field into retained storage: Editor.Text copies its buffer
// into a new string, even when the filter has not changed.
func (s *sideUI) filterText() string {
	length, _ := s.ed.Seek(0, 2)
	if int64(cap(s.filterScratch)) < length {
		s.filterScratch = make([]byte, length)
	}
	s.filterScratch = s.filterScratch[:length]
	s.ed.Seek(0, 0)
	n, _ := s.ed.Read(s.filterScratch)
	if string(s.filterScratch[:n]) != s.filterCache {
		s.filterCache = string(s.filterScratch[:n])
	}
	return s.filterCache
}

// selectedSpaceKey is the key of the space holding tab id; while a workspace member
// (in no space) is shown it keeps the space the strip last showed.
func (win *window) selectedSpaceKey(id tabs.TabID) string {
	if k := win.spaceKeyOf(id); k != "" {
		return k
	}
	return win.strip.spaceKey
}

// revealScroll is the scroll that shows the selected space line (or the shown
// document under it) whole.
func revealScroll(gtx layout.Context, m sideModel, st sideState, h sideHits) (int, bool) {
	y, top, rH, found := 0, 0, gtx.Dp(sideSpaceH), false
	for _, s := range m.Spaces {
		if !found {
			docAt := -1
			for i, d := range s.Docs {
				if st.doc != 0 && d.ID == st.doc {
					docAt = i
				}
			}
			switch {
			case docAt >= 0:
				top, rH, found = y+gtx.Dp(sideSpaceH)+gtx.Dp(sideSpaceGap)+docAt*gtx.Dp(sideDocH), gtx.Dp(sideDocH), true
			case s.Key == st.space:
				top, found = y, true
			}
		}
		y += spaceSpan(gtx, s)
	}
	if !found {
		return 0, false
	}
	sc := min(max(st.scroll, 0), h.scrollMax)
	sc = max(min(sc, top), top+rH-h.list.Dy())
	return min(max(sc, 0), h.scrollMax), true
}

// dropFilter gives the keyboard back from the filter field; it keeps its text.
func (win *window) dropFilter() {
	if win.side.filtering {
		win.side.filtering = false
		win.wantFocus = true
	}
}

// dropSideEdit gives the keyboard back from the sidebar's filter and the tab
// rename field when the main view changes or the sidebar goes away: the filter
// keeps its text, a rename is saved.
func (win *window) dropSideEdit() {
	win.dropFilter()
	win.endRename(true)
}

// sidePointer handles one pointer event on the sidebar. Hover redraws only when
// the hovered line or button changes, so moving the mouse over the sidebar stays cheap.
func (win *window) sidePointer(e pointer.Event) {
	s := &win.side
	p := e.Position.Round()
	switch e.Kind {
	case pointer.Move, pointer.Enter, pointer.Leave:
		in := e.Kind != pointer.Leave
		var hoverDoc docID
		for _, d := range s.hits.docs {
			if p.In(d.rect) && in {
				hoverDoc = d.id
				break
			}
		}
		var hover tabs.TabID
		if r, ok := s.hits.lineAt(p); ok && in {
			hover = r.id
		}
		var hoverSpace string
		for _, sp := range s.hits.spaces {
			if p.In(sp.rect) && in {
				hoverSpace = sp.key
				break
			}
		}
		hoverBtn := btnNone
		switch {
		case !in:
		case p.In(s.hits.search):
			hoverBtn = btnSearch
		case p.In(s.hits.plus):
			hoverBtn = btnPlus
		}
		if hover != s.st.hover || hoverDoc != s.st.hoverDoc || hoverSpace != s.st.hoverSpace || hoverBtn != s.st.hoverBtn {
			s.st.hoverDoc, s.st.hover, s.st.hoverSpace, s.st.hoverBtn = hoverDoc, hover, hoverSpace, hoverBtn
			win.sideRedraw()
		}
	case pointer.Scroll:
		if !p.In(s.hits.list) {
			return
		}
		if sc := min(max(s.st.scroll+int(e.Scroll.Y), 0), s.hits.scrollMax); sc != s.st.scroll {
			s.st.scroll = sc
			if s.mode == sideRail {
				// Tiles moved under the pointer. Drop the old title until the
				// next pointer move identifies the tile now under it.
				s.st.hover = 0
			}
			win.sideRedraw()
		}
	case pointer.Press:
		win.sidePress(p, e.Buttons)
	}
}

// sidePress handles a click: a space line shows that space's last used tab, an
// agent row or member line shows its tab, a middle click on one closes it; a
// document line shows or closes the document; the header icons open the filter
// or a new session; Settings and run headers as before. A click off the rename
// editor saves the rename.
func (win *window) sidePress(p image.Point, b pointer.Buttons) {
	primary, middle := b.Contain(pointer.ButtonPrimary), b.Contain(pointer.ButtonTertiary)
	if !primary && !middle {
		return
	}
	h := &win.side.hits
	win.endRename(true)
	win.stopRecording() // a click outside Settings' pane cancels a shortcut row's recording
	win.wantFocus = true
	win.side.filtering = primary && (p.In(h.filter) || p.In(h.search)) // any other click leaves the field
	defer win.sideRedraw()
	switch {
	case p.In(h.settings):
		switch {
		case middle, primary && p.In(h.settingsClose):
			win.closeSettings()
		case primary:
			win.openSettings()
		}
		return
	case p.In(h.filter), p.In(h.search):
		return // the frame gives the field the keyboard
	case p.In(h.plus):
		if primary {
			win.newTab()
		}
		return
	}
	for _, d := range h.docs {
		if p.In(d.rect) {
			if middle || p.In(d.close) {
				win.closeDoc(d.id)
			} else {
				win.showDoc(d.id)
			}
			return
		}
	}
	if r, ok := h.lineAt(p); ok {
		switch {
		case middle, p.In(r.close):
			win.closeTab(r.id)
		default:
			win.clickRow(r.id)
		}
		return
	}
	if !primary {
		return
	}
	for _, sp := range h.spaces {
		if p.In(sp.rect) {
			win.selectSpace(sp.key)
			return
		}
	}
	for _, r := range h.runs {
		if p.In(r.rect) {
			if win.side.st.runFolded == nil {
				win.side.st.runFolded = map[uint64]bool{}
			}
			flip(win.side.st.runFolded, r.run)
			return
		}
	}
}

// Editors keep their input registrations, selections and caret timers live;
// the surrounding lines replay without shaping while an editor has the focus.
func (win *window) drawSideEditors(gtx layout.Context) {
	s, c := &win.side, win.ui
	if s.filtering && !s.hits.filter.Empty() {
		r := s.hits.filter
		cl := clip.Rect(r.Inset(1)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, c.th.Chrome, clip.Rect(r).Op())
		c.lineEditor(gtx, &s.ed, r, "Filter spaces and agents…", c.th.Muted, gtx.Dp(8))
		cl.Pop()
	}
}
