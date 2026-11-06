package ui

import (
	"image"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"

	kinput "github.com/beefysalad/kaze-terminal/internal/input"
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

	work findWork // the search; runs only while open

	tag         int // the bar's buttons take presses and hover here
	hover       int // the hovered button, findBtnNone for none
	placeholder string
	phTab       tabs.TabID // the pane placeholder was made for
	phMember    bool
	countText   string // built only when countKey changes (Review Focus 2)
	countKey    findCountKey
	countBuilds int      // times countText was built, for tests
	texts       []string // the texts drawn last frame, for tests
}

// findHits are the bar's parts as drawn last frame, in window pixels.
type findHits struct {
	bar, field, count, caseBtn, prev, next, close image.Rectangle
}

// openFind opens the bar on the pane with the keyboard, or takes the keyboard
// back to it, with the last text all selected. Settings, the board, a doc and
// the empty window have no pane: the shortcut does nothing there (Ruling 12).
func (win *window) openFind() {
	v, ok := win.keyView()
	if !ok {
		return
	}
	win.dropSideEdit() // the filter and rename fields give the keyboard up
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
		f.work.fresh = true // output while closed was not followed: search again
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

// findMaxText bounds the field; the search uses the first find.MaxNeedle runes.
const findMaxText = 1024

// findEvents runs the bar while it is open: Enter or Down steps to the next
// match, Shift+Enter or Up to the previous one, Esc closes, kaze's shortcuts
// still work, and the text is kept in step with the field (the search reads it
// once per frame, however many keys arrived). Nothing runs while closed.
func (win *window) findEvents(gtx layout.Context) {
	f := &win.find
	if !f.open {
		return
	}
	f.ed.SingleLine, f.ed.MaxLen = true, findMaxText
	if f.edKeys == nil {
		ed := &f.ed
		f.edKeys = append(win.edKeys(ed),
			key.Filter{Focus: ed, Name: key.NameReturn, Optional: key.ModShift},
			key.Filter{Focus: ed, Name: key.NameEnter, Optional: key.ModShift},
			key.Filter{Focus: ed, Name: key.NameUpArrow},
			key.Filter{Focus: ed, Name: key.NameDownArrow},
		)
	}
	// Only a field that newly has the focus is a click straight into it, as
	// boardSearchEvents does.
	focused := gtx.Focused(&f.ed)
	if focused && !f.edFocused {
		f.keys = true
	}
	f.edFocused = focused
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &f.tag, Kinds: pointer.Press | pointer.Move | pointer.Enter | pointer.Leave})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && !win.bench.IgnoreInput() {
			win.findPointer(e)
			if !f.open {
				return // ×
			}
		}
	}
	for {
		ev, ok := gtx.Event(f.edKeys...)
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && !win.bench.IgnoreInput() {
			win.findKey(gtx, e)
			if !f.open {
				return // Esc, or a shortcut that took the pane away
			}
		}
	}
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

// findKey handles one key the bar's filters caught.
func (win *window) findKey(gtx layout.Context, e key.Event) {
	f := &win.find
	if a, ok := win.km[kinput.Shortcut{Name: e.Name, Mods: e.Modifiers}]; ok {
		switch a {
		case actPaste:
			gtx.Execute(clipboard.ReadCmd{Tag: &f.ed})
		case actCopy:
			// The field's selection, else the terminal's under the bar, so
			// "select in the terminal, copy" works with the bar open (Ruling 17).
			if s := f.ed.SelectedText(); s != "" {
				win.writeClipboard(gtx, s)
			} else if v, ok := win.findView(); ok {
				win.copySel(gtx, v.tab)
			}
		case actFind:
			f.ed.SetCaret(f.ed.Len(), 0)
			win.invalidate()
		default:
			win.runShortcut(gtx, a)
		}
		return
	}
	switch e.Name {
	case key.NameEscape:
		win.closeFind(true)
	case key.NameReturn, key.NameEnter:
		if e.Modifiers.Contain(key.ModShift) {
			win.findStep(-1)
		} else {
			win.findStep(1)
		}
	case key.NameDownArrow:
		win.findStep(1)
	case key.NameUpArrow:
		win.findStep(-1)
	}
}

// takesKeys reports whether a opens an editor that should get the keys typed
// right after its shortcut: the palette and the find bar.
func takesKeys(a action) bool { return a == actPalette || a == actFind }
