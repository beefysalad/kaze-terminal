package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// findState is the find bar (issue #155): one bar per window, on the session
// terminal or the shown docked member tab. The text and Aa are window-wide and
// live as long as the window; nothing here runs while the bar is closed.
type findState struct {
	open bool
	keys bool // the bar's field has the keyboard (else the pane under it does)

	tab    tabs.TabID // the pane the bar is on: the session, or a member tab
	member bool       // tab is a docked member, not the shown session

	ed        widget.Editor
	text      string // the field's text as last seen, window-wide
	matchCase bool   // Aa, window-wide, not saved

	// Where the view was when the bar opened, for Esc: a stable line (Evicted
	// plus the absolute row) of the view's top, or the live bottom.
	restoreTop  int
	restoreLive bool

	pane      image.Rectangle // the pane drawn last frame
	hits      findHits
	edKeys    []event.Filter // the field's own keys, made once
	edFocused bool           // the field had the focus last frame
}

// findHits are the bar's parts as drawn last frame, in window pixels.
type findHits struct {
	bar, field, caseBtn, prev, next, close image.Rectangle
}

// openFind opens the bar on the pane with the keyboard, or takes the keyboard
// back to it, with the last text all selected. Settings, the board, a doc and
// the empty window have no pane: the shortcut does nothing there (Ruling 12).
func (win *window) openFind() {
	v, ok := win.keyView()
	if !ok {
		return
	}
	f := &win.find
	member := v.tag != win.termTag
	if !f.open || f.tab != v.tab.ID || f.member != member {
		// Newly open, or moved from the other pane without restoring it (Ruling 13).
		f.tab, f.member = v.tab.ID, member
		s := v.tab.Session
		s.Lock()
		scr := s.Screen()
		off := scr.ViewOffset()
		f.restoreLive = off == 0
		f.restoreTop = int(scr.Evicted()) + scr.ScrollbackLen() - off
		s.Unlock()
	}
	f.open, f.keys = true, true
	f.ed.SetCaret(f.ed.Len(), 0)
	win.wantFocus = true // drawFind lays the field out this frame, so the focus takes
	win.invalidate()
}

// closeFind closes the bar. With restore the view goes back to where it was
// when the bar opened (Esc, ×); the keyboard goes back to the pane if the bar
// had it.
func (win *window) closeFind(restore bool) {
	f := &win.find
	if !f.open {
		return
	}
	hadKeys := f.keys
	f.open, f.keys = false, false
	if restore && win.mgr != nil {
		if t := win.mgr.Lookup(f.tab); t != nil {
			s := t.Session
			s.Lock()
			scr := s.Screen()
			if f.restoreLive {
				scr.SetViewOffset(0)
			} else {
				// SetViewOffset clamps: a top evicted meanwhile is the oldest line.
				scr.SetViewOffset(scr.ScrollbackLen() - (f.restoreTop - int(scr.Evicted())))
			}
			s.Unlock()
		}
	}
	if hadKeys {
		if f.member {
			if win.shownMember() == f.tab {
				win.dockFocusMember(true)
			}
		} else {
			win.dropDockKeys()
		}
	}
	win.wantFocus = true
	win.invalidate()
}

// findCheck closes the bar, without restoring, when its pane is no longer
// shown: another session, the tab closed, the member hidden or swapped, or
// settings, the board or a doc in front (Ruling 13).
func (win *window) findCheck() {
	if !win.find.open {
		return
	}
	if _, ok := win.findView(); !ok {
		win.closeFind(false)
	}
}

// findView is the view the bar is on, ok only while it is open and shown.
func (win *window) findView() (termView, bool) {
	f := &win.find
	if !f.open || win.mgr == nil {
		return termView{}, false
	}
	if f.member {
		if v, ok := win.dockMemberView(); ok && v.tab.ID == f.tab {
			return v, true
		}
		return termView{}, false
	}
	t := win.shownTab()
	if t == nil || t.ID != f.tab {
		return termView{}, false
	}
	return win.mainView(t), true
}

// findEvents runs the bar's field while the bar is open: it keeps the text in
// step with the field (the search reads it once per frame).
func (win *window) findEvents(gtx layout.Context) {
	f := &win.find
	if !f.open {
		return
	}
	f.ed.SingleLine = true
	for {
		ev, ok := f.ed.Update(gtx)
		if !ok {
			return
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			f.text = f.ed.Text()
			win.ui.fit(f.text) // loads a fallback font for any rune the bundled ones lack
			win.invalidate()
		}
	}
}
