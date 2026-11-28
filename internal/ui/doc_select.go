package ui

import (
	"image"
	"strings"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// docPos is a place in a laid-out document: a line and a rune offset within the
// text of that line (its runs joined).
type docPos struct{ line, off int }

func (a docPos) before(b docPos) bool {
	return a.line < b.line || a.line == b.line && a.off < b.off
}

// docSel is the highlighted text of the shown document.
type docSel struct {
	doc          docID
	anchor, head docPos
	on, drag     bool
	key          docLayKey // the layout it was made on; a new layout drops it
}

// span returns the selection in reading order, and whether it covers any text.
func (s docSel) span() (from, to docPos, ok bool) {
	from, to = s.anchor, s.head
	if to.before(from) {
		from, to = to, from
	}
	return from, to, s.on && from != to
}

func lineRunes(line docLine) int {
	n := 0
	for _, r := range line.runs {
		n += utf8.RuneCountInString(r.text)
	}
	return n
}

// selectedDocText is the text between from and to. Lines that only wrapped join
// with a space, a line break stays one, and a paragraph gap becomes a blank line.
func selectedDocText(l *docLayout, from, to docPos) string {
	var b strings.Builder
	for i := from.line; i <= to.line && i < len(l.lines); i++ {
		line := l.lines[i]
		lo, hi := 0, lineRunes(line)
		if i == from.line {
			lo = from.off
		}
		if i == to.line {
			hi = to.off
		}
		pos := 0
		for _, r := range line.runs {
			rs := []rune(r.text)
			a, z := max(lo-pos, 0), min(hi-pos, len(rs))
			if a < z {
				b.WriteString(string(rs[a:z]))
			}
			pos += len(rs)
		}
		if i == to.line || i+1 >= len(l.lines) {
			break
		}
		switch next := l.lines[i+1]; {
		case line.soft:
			if !strings.HasSuffix(b.String(), " ") {
				b.WriteByte(' ')
			}
		case next.y-(line.y+line.h) > 0:
			b.WriteString("\n\n")
		default:
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// runAdv is the width of the first n runes of a run.
func (win *window) runAdv(gtx layout.Context, r docRun, n int) int {
	var w fixed.Int26_6
	for _, q := range r.text {
		if n <= 0 {
			break
		}
		w += win.runeAdv(gtx, r.face, r.size, q)
		n--
	}
	return w.Ceil()
}

// docPosAt is the nearest place in l to pt, in layout coordinates (the top left
// of the document is 0,0). Outside the text it clamps to the nearest line end.
func (win *window) docPosAt(gtx layout.Context, l *docLayout, pt image.Point) (docPos, bool) {
	if len(l.lines) == 0 {
		return docPos{}, false
	}
	i := 0
	for i+1 < len(l.lines) && l.lines[i+1].y <= pt.Y {
		i++
	}
	line := l.lines[i]
	off := 0
	for _, r := range line.runs {
		n := utf8.RuneCountInString(r.text)
		if pt.X >= r.x+r.w {
			off += n
			continue
		}
		if pt.X < r.x {
			break
		}
		// Inside this run: the nearest boundary between its runes.
		best, bestD := 0, 1<<30
		for k := 0; k <= n; k++ {
			d := pt.X - (r.x + win.runAdv(gtx, r, k))
			if d < 0 {
				d = -d
			}
			if d < bestD {
				best, bestD = k, d
			}
		}
		off += best
		break
	}
	return docPos{i, off}, true
}

// paintDocSel fills the selected part of every visible line before its text.
func (win *window) paintDocSel(gtx layout.Context, l *docLayout, from, to docPos, offset image.Point, view image.Rectangle) {
	ops := gtx.Ops
	col := tint(win.ui.th.Accent, 0x55)
	for i := from.line; i <= to.line && i < len(l.lines); i++ {
		line := l.lines[i]
		lo, hi := 0, lineRunes(line)
		if i == from.line {
			lo = from.off
		}
		if i == to.line {
			hi = to.off
		}
		pos := 0
		for _, r := range line.runs {
			n := utf8.RuneCountInString(r.text)
			a, z := max(lo-pos, 0), min(hi-pos, n)
			pos += n
			if a >= z {
				continue
			}
			x0, x1 := r.x+win.runAdv(gtx, r, a), r.x+win.runAdv(gtx, r, z)
			if z == n && i < to.line {
				x1 = r.x + r.w
			}
			rr := image.Rect(x0, line.y, x1, line.y+line.h).Add(offset)
			if rr.Overlaps(view) {
				paint.FillShape(ops, col, clip.Rect(rr.Intersect(view)).Op())
			}
		}
	}
}

// clearDocSel drops any selection.
func (win *window) clearDocSel() { win.docs.sel = docSel{} }

// docSelPress starts a selection at p, in window pixels, when p is in the text.
func (win *window) docSelPress(gtx layout.Context, d *docView, p image.Point) {
	win.clearDocSel()
	v := win.docs.hits.view
	if !p.In(v) {
		return
	}
	pos, ok := win.docPosAt(gtx, &d.lay, p.Sub(v.Min).Add(image.Pt(0, d.scroll)))
	if !ok {
		return
	}
	win.docs.sel = docSel{doc: d.id, anchor: pos, head: pos, drag: true, key: d.lay.key}
}

// docSelDrag extends the selection to p, scrolling when p is past the top or bottom.
func (win *window) docSelDrag(gtx layout.Context, d *docView, p image.Point) {
	s := &win.docs.sel
	if !s.drag || s.doc != d.id {
		return
	}
	v := win.docs.hits.view
	switch {
	case p.Y < v.Min.Y:
		win.docScrollTo(d, d.scroll-gtx.Dp(unit.Dp(12)))
	case p.Y > v.Max.Y:
		win.docScrollTo(d, d.scroll+gtx.Dp(unit.Dp(12)))
	}
	pos, ok := win.docPosAt(gtx, &d.lay, p.Sub(v.Min).Add(image.Pt(0, d.scroll)))
	if !ok {
		return
	}
	s.head, s.on = pos, true
	win.w.Invalidate()
}

// docSelRelease ends a drag; a click with no movement selects nothing.
func (win *window) docSelRelease() {
	s := &win.docs.sel
	s.drag = false
	if s.anchor == s.head {
		win.clearDocSel()
	}
}

// docSelText is the selected text of d, "" when nothing is selected.
func (win *window) docSelText(d *docView) string {
	s := win.docs.sel
	if s.doc != d.id || s.key != d.lay.key {
		return ""
	}
	from, to, ok := s.span()
	if !ok {
		return ""
	}
	return selectedDocText(&d.lay, from, to)
}

// copyDocSel copies the shown document's selection. It reports whether there was one.
func (win *window) copyDocSel(gtx layout.Context) bool {
	d := win.shownDoc()
	if d == nil {
		return false
	}
	text := win.docSelText(d)
	if text == "" {
		return false
	}
	win.writeClipboard(gtx, text)
	return true
}
