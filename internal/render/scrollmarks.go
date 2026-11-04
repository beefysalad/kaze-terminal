package render

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// scrollKey is everything the scroll marks are drawn from; the palette is fixed per renderer.
type scrollKey struct {
	gen                uint64
	cur                int32
	first, total, view int
	rows               int
	track              image.Rectangle
}

// scrollPaint is what the last recording drew, read by tests.
type scrollPaint struct {
	band, cur image.Rectangle
	curLast   bool // the current mark was the last shape drawn
}

// DrawScrollMarks draws where every find match sits in the lines held (history
// plus the screen) into track, a narrow column the caller places beside the
// grid: one mark per pixel row holding a match start, the current match taller
// and brighter on top, and a faint band over the visible part. Call it after
// Layout, outside the grid's clip. The ops are recorded once per change and
// replayed, so a still frame allocates nothing.
func (r *Renderer) DrawScrollMarks(gtx layout.Context, track image.Rectangle) {
	m := r.marks
	if m == nil || track.Empty() || r.markTotal <= 0 {
		return
	}
	k := scrollKey{gen: m.Gen, cur: m.Cur, first: r.markFirst, total: r.markTotal, view: r.markView, rows: r.rows, track: track}
	if !r.scrollReady || k != r.scrollKey {
		r.recordScrollMarks(gtx, m, k)
	}
	r.scrollCall.Add(gtx.Ops)
	r.marksAdded++
}

func (r *Renderer) recordScrollMarks(gtx layout.Context, m *Marks, k scrollKey) {
	r.scrollKey, r.scrollReady = k, true
	r.marksRecorded++
	r.scrollPaint = scrollPaint{}
	f := r.findColors()
	t := k.track
	y := func(line int) int { return t.Min.Y + (line-k.first)*t.Dy()/k.total }
	dp := func(v unit.Dp) int { return gtx.Dp(v) }

	r.scrollOps.Reset()
	rec := op.Record(&r.scrollOps)
	ops := &r.scrollOps
	if k.total > k.rows { // something to scroll: show where the view is
		band := image.Rect(t.Min.X-dp(1), y(k.view), t.Max.X+dp(1), y(k.view+k.rows))
		paint.FillShape(ops, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x12}, clip.UniformRRect(band, dp(3)).Op(ops))
		r.scrollPaint.band = band
	}

	// One mark per pixel row holding a match start. Spans are sorted, so the
	// rows only grow; a match over a wrap is marked at its first span.
	r.markYs = r.markYs[:0]
	curLine, last := -1, int32(-1)
	h := dp(2)
	for i, sp := range m.Spans {
		if i > 0 && sp.Match == last {
			continue
		}
		last = sp.Match
		if sp.Match == m.Cur {
			curLine = sp.Line
		}
		if sp.Line < k.first || sp.Line >= k.first+k.total {
			continue
		}
		yy := int32(y(sp.Line))
		if n := len(r.markYs); n > 0 && r.markYs[n-1] == yy {
			continue
		}
		r.markYs = append(r.markYs, yy)
		top := min(int(yy), t.Max.Y-h)
		paint.FillShape(ops, f.mark, clip.UniformRRect(image.Rect(t.Min.X, top, t.Max.X, top+h), dp(1)).Op(ops))
	}
	if curLine >= k.first && curLine < k.first+k.total {
		ch := dp(3)
		top := min(y(curLine), t.Max.Y-ch)
		cur := image.Rect(t.Min.X, top, t.Max.X, top+ch)
		paint.FillShape(ops, f.cur, clip.UniformRRect(cur, dp(1)).Op(ops))
		r.scrollPaint.cur, r.scrollPaint.curLast = cur, true
	}
	r.scrollCall = rec.Stop()
}
