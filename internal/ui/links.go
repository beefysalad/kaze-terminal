package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/render"
	"image"

	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

// linkUnder is what the web link or .md path at grid pixel p in t may mean,
// best first (see render.Links.RefsAt), if there is one. It holds the session
// lock only to read the grid; the disk is checked later, by openDocRefs.
func (win *window) linkUnder(t *tabs.Tab, p image.Point, cw, ch int) ([]string, bool) {
	s := t.Session
	s.Lock()
	scr := s.Screen()
	refs := win.links.RefsAt(scr, win.pointAt(scr, p, cw, ch))
	s.Unlock()
	return refs, len(refs) > 0
}

// open hands a clicked link to the system browser. Only http and https links
// are ever found, so terminal output can't make kaze open another scheme.
func (win *window) open(u string) {
	if !render.IsWebLink(u) {
		return
	}
	f := win.openLink
	if f == nil {
		f = openURL
	}
	_ = f(u) // a browser that fails to start is not worth an error in the terminal
}

// linkRects are the window rectangles of the links shown in s's grid.
func (win *window) linkRects(s *term.Session, grid image.Rectangle, cw, ch int) []image.Rectangle {
	s.Lock()
	scr := s.Screen()
	_, rows := scr.Size()
	top := scr.ScrollbackLen() - scr.ViewOffset()
	spans := win.links.Visible(scr, top, rows)
	s.Unlock()
	if len(spans) == 0 {
		return nil
	}
	out := make([]image.Rectangle, 0, len(spans))
	for _, sp := range spans {
		y := grid.Min.Y + (sp.Row-top)*ch
		out = append(out, image.Rect(grid.Min.X+sp.From*cw, y, grid.Min.X+(sp.To+1)*cw, y+ch).Intersect(grid))
	}
	return out
}

// linkCursors shows the hand cursor over the links in s's grid. Call inside the
// grid's input area, so a press on a link still reaches the terminal. Cursor
// areas need no extra frames: hovering still never wakes an idle window.
func (win *window) linkCursors(ops *op.Ops, s *term.Session, grid image.Rectangle, cw, ch int) {
	if cw <= 0 || ch <= 0 {
		return
	}
	for _, r := range win.linkRects(s, grid, cw, ch) {
		a := clip.Rect(r).Push(ops)
		pointer.CursorPointer.Add(ops)
		a.Pop()
	}
}
