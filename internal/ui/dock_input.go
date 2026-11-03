package ui

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/input"
	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// dockEvents runs the shown panel's input from last frame's hits: the handle
// resizes, the header switches tabs and runs its buttons, the body scrolls,
// follows links and takes the keyboard when clicked.
func (win *window) dockEvents(gtx layout.Context, cw, ch int) {
	owner := win.dockOwner()
	s := win.dockOf(owner)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &win.dock.handle, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		// A release or cancel always ends a drag, even after its panel closed.
		if e, ok := ev.(pointer.Event); ok && !win.bench.IgnoreInput() && (s != nil || e.Kind == pointer.Release || e.Kind == pointer.Cancel) {
			win.dockHandle(gtx, e)
		}
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &win.dock.head, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && s != nil && !win.bench.IgnoreInput() && e.Buttons.Contain(pointer.ButtonPrimary) {
			win.dockHead(gtx, owner, s, e)
			s = win.dockOf(owner) // × may have closed it
		}
	}
	tag := &win.dock.body
	var pass []event.Event // keys the panel does not use: the terminal's
loop:
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: tag},
			key.Filter{Focus: tag, Optional: allMods},
			key.Filter{Focus: tag, Name: key.NameTab, Optional: allMods},
			pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		)
		if !ok {
			break
		}
		if s == nil || win.bench.IgnoreInput() {
			continue
		}
		switch e := ev.(type) {
		case key.Event:
			if e.Modifiers == 0 && (e.Name == key.NameEscape || e.Name == key.NameUpArrow || e.Name == key.NameDownArrow) {
				if e.State != key.Press || !win.dock.keys {
					continue
				}
				switch e.Name {
				case key.NameEscape:
					win.closeDock(owner)
					s = nil
				case key.NameUpArrow:
					win.dockStep(owner, -1)
				case key.NameDownArrow:
					win.dockStep(owner, 1)
				}
				continue
			}
			pass = append(pass, ev)
			if e.State == key.Press && win.km[input.Shortcut{Name: e.Name, Mods: e.Modifiers}] == actPalette {
				// As in termEvents: leave the rest in Gio's queue for the
				// palette's editor, so typing after the shortcut lands there.
				break loop
			}
		case key.EditEvent, key.SnippetEvent, key.SelectionEvent, key.CompositionEvent:
			pass = append(pass, ev)
		case pointer.Event:
			win.dockBody(gtx, owner, s, e)
		}
	}
	if len(pass) > 0 {
		// Typing and shortcuts belong to whoever has the keyboard without the
		// panel (a floating pane or the session): hand it back and deliver
		// them there, so nothing typed into the panel is lost.
		win.dropDockKeys()
		if v, ok := win.keyView(); ok {
			win.routeView(gtx, v, pass, cw, ch)
		}
	}
}

// dockHandle drags the panel's height; the setting is saved once, on release.
func (win *window) dockHandle(gtx layout.Context, e pointer.Event) {
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	y := e.Position.Round().Y
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) {
			return
		}
		win.dock.drag = dockDrag{on: true, fromY: y, startDp: int(float32(win.dock.lastRect.Dy())/scale + 0.5)}
	case pointer.Drag:
		if !win.dock.drag.on {
			return
		}
		win.dock.heightDp = max(win.dock.drag.startDp+int(float32(win.dock.drag.fromY-y)/scale), dockMinDp)
		win.invalidate()
	case pointer.Release, pointer.Cancel:
		if !win.dock.drag.on {
			return
		}
		win.dock.drag.on = false
		if win.dock.heightDp > 0 {
			win.saveSetting("agent_panel_height", win.dock.heightDp)
		}
		win.invalidate() // the next frame gives the PTY its new size
	}
}

// dockHead handles a press on the header or the prompt row.
func (win *window) dockHead(gtx layout.Context, owner tabs.TabID, s *dockSession, e pointer.Event) {
	p, h := e.Position.Round(), win.dock.hits
	a := win.subs.Get(s.agent)
	switch {
	case p.In(h.close):
		win.closeDock(owner)
	case p.In(h.actTab):
		s.tab, s.picked, s.scroll, s.follow = dockActivity, true, 0, true
	case p.In(h.repTab):
		s.tab, s.picked, s.scroll, s.follow = dockReport, true, 0, false
	case p.In(h.copy) && a != nil:
		win.copyReport(gtx, a)
	case p.In(h.reader) && a != nil:
		win.openReportDoc(owner, a.Type+" · "+a.Task, win.dockReport(a))
	case p.In(h.prompt):
		s.promptOpen = !s.promptOpen
	}
	win.invalidate()
}

// dockBody handles a press or scroll in the panel body.
func (win *window) dockBody(gtx layout.Context, owner tabs.TabID, s *dockSession, e pointer.Event) {
	p := e.Position.Round()
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) {
			return
		}
		if e.Modifiers.Contain(key.ModShortcut) {
			for _, l := range win.dock.hits.links {
				if p.In(l.r) {
					if render.IsWebLink(l.url) {
						win.open(l.url)
					} else {
						win.openDocRef(win.mgr.Lookup(owner), false, l.url)
					}
					win.invalidate()
					return
				}
			}
		}
		if p.In(win.dock.hits.finish) {
			s.tab, s.picked, s.scroll = dockReport, true, 0
		}
		win.dropSideEdit()
		win.dock.keys, win.wantFocus = true, true
	case pointer.Scroll:
		s.scroll = min(max(s.scroll+int(e.Scroll.Y), 0), win.dock.hits.scrollMax)
		s.follow = s.scroll >= win.dock.hits.scrollMax
	}
	win.invalidate()
}

// dockStep moves session owner's panel to the previous (d < 0) or next agent
// in sidebar order, stopping at the ends.
func (win *window) dockStep(owner tabs.TabID, d int) {
	s := win.dockOf(owner)
	if s == nil {
		return
	}
	list, _ := win.subs.Tree(int(owner))
	for i, a := range list {
		if a.Key != s.agent {
			continue
		}
		if j := i + d; j >= 0 && j < len(list) {
			win.openDock(owner, list[j].Key)
		}
		return
	}
}
