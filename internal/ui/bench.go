package ui

import (
	"runtime"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/patrick-gogo/kaze/internal/bench"
	"github.com/patrick-gogo/kaze/internal/render"
	"github.com/patrick-gogo/kaze/internal/tabs"
)

// benchFrame is what the bench recorder learns about the frame being built.
type benchFrame struct {
	before render.Stats
	frame  bench.Frame
}

func benchCursor(t *tabs.Tab) bench.Cursor {
	t.Session.Lock()
	col, row := t.Session.Screen().Cursor()
	t.Session.Unlock()
	return bench.Cursor{Tab: int(t.ID), Col: col, Row: row}
}

// benchKey records a key about to be written to t's shell.
func (win *window) benchKey(t *tabs.Tab) {
	if win.bench != nil {
		win.bench.KeyArrived(time.Now(), benchCursor(t))
	}
}

// benchBatch drops the frame's real input (unless the run allows it) and adds the scripted input.
func (win *window) benchBatch(gtx layout.Context, t *tabs.Tab, batch []event.Event, ch int) []event.Event {
	if win.bench.IgnoreInput() {
		batch = batch[:0]
	}
	return win.benchInput(gtx, t, batch, ch)
}

// benchInput adds the bench run's scripted input to this frame's batch: the
// command typed into every tab, a synthetic key, or a page of wheel scroll.
// Keys and scroll go through route like real input.
func (win *window) benchInput(gtx layout.Context, t *tabs.Tab, batch []event.Event, ch int) []event.Event {
	now := time.Now()
	if s, ok := win.bench.TakeType(now); ok {
		for _, tab := range win.mgr.Tabs() {
			tab.Session.Write([]byte(s))
		}
	}
	for _, k := range win.bench.TakeKeys(now) {
		switch k {
		case bench.KeyText:
			batch = append(batch, key.Event{Name: "X", State: key.Press}, key.EditEvent{Text: "x"}, key.Event{Name: "X", State: key.Release})
		case bench.KeyBackspace:
			batch = append(batch, key.Event{Name: key.NameDeleteBackward, State: key.Press}, key.Event{Name: key.NameDeleteBackward, State: key.Release})
		}
	}
	s := t.Session
	s.Lock()
	sb, off := s.Screen().ScrollbackLen(), s.Screen().ViewOffset()
	s.Unlock()
	_, rows := win.mgr.Size()
	if n := win.bench.ScrollStep(now, sb, off, rows); n > 0 {
		dy := float32(n) * 120 // Windows wheel units: 3 lines a notch
		if runtime.GOOS == "darwin" {
			dy = float32(n * 3 * ch) // macOS scrolls by pixels, a line per cell height
		}
		pos := f32.Pt(float32(win.gridMin.X+1), float32(win.gridMin.Y+1))
		batch = append(batch, pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: pos, Scroll: f32.Pt(0, -dy)})
		gtx.Execute(op.InvalidateCmd{}) // the next page on the next frame
	}
	return batch
}

// benchBeforeDraw notes the cursor and renderer counters before the active tab is drawn.
func (win *window) benchBeforeDraw(t *tabs.Tab) {
	win.bf = benchFrame{before: win.rend.Stats()}
	win.bf.frame.Cursor = benchCursor(t)
}

// benchAfterDraw fills in what the draw did.
func (win *window) benchAfterDraw(t *tabs.Tab) {
	st := win.rend.Stats()
	win.bf.frame.Drawn = st.FramesDrawn > win.bf.before.FramesDrawn
	win.bf.frame.Rows = st.RowsRedrawn - win.bf.before.RowsRedrawn
	if win.bench.WantText() {
		win.bf.frame.HasText = hasText(t)
	}
}

// benchScreen logs the active tab's visible text, trailing blanks trimmed.
func (win *window) benchScreen() {
	if win.mgr == nil {
		return
	}
	t := win.mgr.Active()
	if t == nil {
		return
	}
	s := t.Session
	s.Lock()
	scr := s.Screen()
	cols, rows := scr.Size()
	lines := make([]string, 0, rows)
	for row := 0; row < rows; row++ {
		var b strings.Builder
		for col := 0; col < cols; col++ {
			c := scr.VisibleCell(col, row)
			if c.Width == 0 || c.Rune == 0 {
				continue // the second half of a wide character
			}
			b.WriteRune(c.Rune)
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	s.Unlock()
	win.bench.Screen(lines)
}

// hasText reports whether the visible screen has any non-blank cell.
func hasText(t *tabs.Tab) bool {
	s := t.Session
	s.Lock()
	defer s.Unlock()
	scr := s.Screen()
	cols, rows := scr.Size()
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			if r := scr.VisibleCell(col, row).Rune; r != ' ' && r != 0 {
				return true
			}
		}
	}
	return false
}
