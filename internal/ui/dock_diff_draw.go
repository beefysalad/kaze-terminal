package ui

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/codecolor"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

// An open edit is drawn once into its own op list and replayed every frame,
// so a still frame shapes none of its rows.

// Row tones, indexing readInk.code and readInk.num.
const (
	diffTonePlain = iota
	diffToneAdd
	diffToneDel
)

const (
	diffIndentDp = 16 // rows start here, under the step's words
	diffPadDp    = 8  // from a row's left edge to its number
	diffGapDp    = 8  // from the number to the marker

	// diffRecordingsKept bounds the retained rows a long run can pile up;
	// past it, the recordings not on screen are let go and made again if
	// they scroll back.
	diffRecordingsKept = 32

	// diffAdvanceRunes is how many characters a code advance is measured
	// over: a width in whole pixels, in hundredths of a character.
	diffAdvanceRunes = 100
)

// diffKey is everything a recording's pixels depend on. first tells a diff
// from the one that replaced it with the same line count, as when the real
// patch arrives for an edit shown from its old and new strings.
type diffKey struct {
	first       *subagents.DiffLine
	lines, more int
	width       int
	metric      unit.Metric
	ink         readInk
}

// diffRowTexts is what one row shows, for the panel's text list.
type diffRowTexts struct {
	s [3]string
	n uint8
}

// diffRecording is one open diff's retained rows.
type diffRecording struct {
	key    diffKey
	ops    op.Ops
	call   op.CallOp
	rows   []diffRowTexts
	height int
	adv    int    // width of diffAdvanceRunes code characters, measured once
	drawn  uint64 // the frame it was last replayed in
}

// diffDigits is how many digits the diff's widest line number has, 0 when it
// has no numbers. One width for the whole diff keeps its code in one column.
func diffDigits(lines []subagents.DiffLine) int {
	widest := 0
	for i := range lines {
		widest = max(widest, lines[i].Old, lines[i].New)
	}
	if widest == 0 {
		return 0
	}
	return len(strconv.Itoa(widest))
}

// diffRows is a diff's row count: its lines and the "N more lines" row.
func diffRows(d *readDetail) int {
	if d.more > 0 {
		return len(d.lines) + 1
	}
	return len(d.lines)
}

// diffLineNumber is the number a row shows: a removed row's old line, any
// other row's new one. 0 means none.
func diffLineNumber(ln *subagents.DiffLine) int {
	if ln.Kind == '-' || ln.New == 0 {
		return ln.Old
	}
	return ln.New
}

// recordDiff draws the item's rows from the origin into a new recording.
func (win *window) recordDiff(gtx layout.Context, it *readItem, width int, ink *readInk, key diffKey) *diffRecording {
	rec := &diffRecording{key: key}
	g := gtx
	g.Ops = &rec.ops
	c, dp, d := win.ui, g.Dp, &it.detail
	rec.adv = win.codeAdvance(g)
	cellW := (rec.adv + diffAdvanceRunes - 1) / diffAdvanceRunes
	lineH := dp(readLineDp)
	numEnd := dp(diffPadDp) + diffDigits(d.lines)*cellW
	markX := numEnd
	if numEnd > dp(diffPadDp) {
		markX += dp(diffGapDp)
	}
	codeX := markX + 2*cellW
	maxRunes := max(width-codeX, 0)*diffAdvanceRunes/max(rec.adv, 1) + 1

	m := op.Record(g.Ops)
	rec.rows = make([]diffRowTexts, 0, diffRows(d))
	for i := range d.lines {
		ln := &d.lines[i]
		y := i * lineH
		mid := y + lineH/2
		tone, marker := diffTonePlain, ""
		switch ln.Kind {
		case '+':
			tone, marker = diffToneAdd, "+ "
			paint.FillShape(g.Ops, ink.addBG, clip.Rect(image.Rect(0, y, width, y+lineH)).Op())
		case '-':
			tone, marker = diffToneDel, "− "
			paint.FillShape(g.Ops, ink.delBG, clip.Rect(image.Rect(0, y, width, y+lineH)).Op())
		}
		var row diffRowTexts
		add := func(s string) { row.s[row.n] = s; row.n++ }
		if n := diffLineNumber(ln); n > 0 {
			s := strconv.Itoa(n)
			call, sz := c.label(g, s, 12, ink.num[tone])
			place(g.Ops, call, image.Pt(numEnd-sz.X, mid-sz.Y/2))
			add(s)
		}
		if marker != "" {
			col := ink.addFG
			if tone == diffToneDel {
				col = ink.delFG
			}
			call, sz := c.label(g, marker, 12, col)
			place(g.Ops, call, image.Pt(markX, mid-sz.Y/2))
			add(marker)
		}
		var spans []codecolor.Span
		if i < len(d.spans) {
			spans = d.spans[i]
		}
		win.recordCode(g, ln.Text, spans, image.Pt(codeX, mid), maxRunes, rec.adv, tone, ink)
		add(ln.Text)
		rec.rows = append(rec.rows, row)
	}
	if d.more > 0 {
		s := plural(d.more, "more line")
		call, sz := c.label(g, s, 12, ink.meta)
		place(g.Ops, call, image.Pt(codeX, len(d.lines)*lineH+lineH/2-sz.Y/2))
		rec.rows = append(rec.rows, diffRowTexts{s: [3]string{s}, n: 1})
	}
	rec.call = m.Stop()
	rec.height = len(rec.rows) * lineH
	return rec
}

// codeAdvance measures how wide diffAdvanceRunes code characters are, so a
// character's true width is known to a fraction of a pixel. It draws into ops
// of its own: nothing of the measuring is kept with the rows.
func (win *window) codeAdvance(g layout.Context) int {
	g.Ops = new(op.Ops)
	_, sz := win.ui.label(g, strings.Repeat("0", diffAdvanceRunes), 12, color.NRGBA{})
	return sz.X
}

// recordCode draws text from at (its vertical middle), one label per span,
// each at the column its first character belongs in. A label's own width is
// not used: it is rounded, and leaves out trailing spaces. adv is the width
// of diffAdvanceRunes characters. Text past the pane's edge is not shaped:
// maxRunes is more than fit.
func (win *window) recordCode(g layout.Context, text string, spans []codecolor.Span, at image.Point, maxRunes, adv, tone int, ink *readInk) {
	c, limit := win.ui, len(text)
	for i := range text {
		if maxRunes == 0 {
			limit = i
			break
		}
		maxRunes--
	}
	col := func(n int) int { return (n*adv + diffAdvanceRunes/2) / diffAdvanceRunes }
	pos, runes, wide := 0, 0, 0
	draw := func(end int, kind codecolor.Kind) {
		if end = min(end, limit); end <= pos {
			return
		}
		part := text[pos:end]
		pos = end
		call, sz := c.label(g, part, 12, ink.code[tone][kind])
		place(g.Ops, call, image.Pt(at.X+wide+col(runes), at.Y-sz.Y/2))
		n := utf8.RuneCountInString(part)
		runes += n
		// Rounding is at most a pixel. More is wide characters from the
		// Japanese fallback font: what follows moves past them.
		if over := sz.X - col(n); over > 1 {
			wide += over
		}
	}
	for _, sp := range spans {
		draw(sp.Start, codecolor.Plain)
		draw(sp.End, sp.Kind)
	}
	draw(limit, codecolor.Plain)
}

// diffRecordingFor is the item's recording for this width, theme and scale, made
// when there is none that matches.
func (win *window) diffRecordingFor(gtx layout.Context, s *dockSession, it *readItem, width int, ink *readInk) *diffRecording {
	rc, d := &s.read, &it.detail
	key := diffKey{lines: len(d.lines), more: d.more, width: width, metric: gtx.Metric, ink: *ink}
	if len(d.lines) > 0 {
		key.first = &d.lines[0]
	}
	if rec := rc.diffs[it.id]; rec != nil && rec.key == key {
		return rec
	}
	if len(rc.diffs) >= diffRecordingsKept {
		for id, r := range rc.diffs {
			if r.drawn != rc.frame {
				delete(rc.diffs, id)
			}
		}
	}
	if rc.diffs == nil {
		rc.diffs = map[string]*diffRecording{}
	}
	rec := win.recordDiff(gtx, it, width, ink, key)
	rc.diffs[it.id] = rec
	rc.diffRecords++
	return rec
}

// drawDiffBody replays the item's rows at at and lists the rows on screen in texts.
// It returns the height. h.body is the part of the pane rows can be seen in.
func (win *window) drawDiffBody(gtx layout.Context, s *dockSession, it *readItem, at image.Point, width int, ink *readInk, h *dockHits, texts *[]string) int {
	rec := win.diffRecordingFor(gtx, s, it, width, ink)
	rec.drawn = s.read.frame
	place(gtx.Ops, rec.call, at)
	lineH := max(gtx.Dp(readLineDp), 1)
	first := max((h.body.Min.Y-at.Y)/lineH, 0)
	last := min((h.body.Max.Y-at.Y)/lineH, len(rec.rows)-1)
	for i := first; i <= last; i++ {
		row := &rec.rows[i]
		*texts = append(*texts, row.s[:row.n]...)
	}
	return rec.height
}

// dropDiffRecordings lets go of the recordings of steps no longer in the list.
func (s *dockSession) dropDiffRecordings() {
	rc := &s.read
	for id := range rc.diffs {
		found := false
		for i := range rc.items {
			found = found || rc.items[i].id == id
		}
		if !found {
			delete(rc.diffs, id)
		}
	}
}
