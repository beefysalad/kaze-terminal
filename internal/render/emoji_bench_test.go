package render

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// benchEmoji are the 20 marks bench/flood -emoji cycles through.
var benchEmoji = []string{"✅", "❌", "🔄", "🟢", "🔴", "🟡", "⏳", "🚀", "📝", "⚠", "✔", "❤", "🐛", "🔥", "💡", "📦", "🧪", "🔧", "✨", "🎉"}

// emojiRows is 40 status-table rows starting at row i, the way flood -emoji prints them.
// Each batch of 40 shifts the emoji and state by one, so every row's emoji cell changes.
func emojiRows(i int) string {
	var b strings.Builder
	for j := i; j < i+40; j++ {
		state := [...]string{"Done", "Running", "Waiting", "Failed"}[(j+j/40)%4]
		fmt.Fprintf(&b, "| %04d | %-24s | %s %-8s |\r\n", j%10000, "Shared 3D Secure helper", benchEmoji[(j+j/40)%len(benchEmoji)], state)
	}
	return b.String()
}

// emojiBench prints 40 emoji rows and lays out a frame per op on a 120x40 screen
// at px. fresh empties the glyph cache first, so every op pays first sight of each
// glyph; mono loads the emoji font without its colour tables (the "before" path).
func emojiBench(b *testing.B, px float32, fresh, mono bool) {
	segoeEmoji(b)
	if mono {
		e := realEmojiFont(b)
		e.colr, e.sbix = nil, nil
		swapEmojiOpen(b, func(string) *emojiFont { return e })
	}
	h := newHarness(b, 120, 40)
	h.r = NewRenderer(px, DefaultPalette(), Options{})
	h.outRows(emojiRows(0), 39)
	h.frame()
	b.ReportAllocs()
	b.ResetTimer()
	i, rows := 0, h.r.Stats().RowsRedrawn
	for b.Loop() {
		if fresh {
			clear(h.r.glyphs.m)
			clear(h.r.glyphs.faces)
		}
		i += 40
		h.outRows(emojiRows(i), i+39)
		h.frame()
	}
	b.ReportMetric(float64(h.r.Stats().RowsRedrawn-rows)/float64(b.N), "rows/op")
}

func BenchmarkEmojiRows(b *testing.B)           { emojiBench(b, 13.125, true, false) }
func BenchmarkEmojiRowsMonochrome(b *testing.B) { emojiBench(b, 13.375, true, true) }
func BenchmarkEmojiRowsCached(b *testing.B)     { emojiBench(b, 13.625, false, false) }

// outRows feeds s and waits (at most 2s) until row last's number shows on the
// screen's second-to-last row, so a frame never draws half-applied output.
func (h *harness) outRows(s string, last int) {
	h.t.Helper()
	if _, err := h.pty.outW.Write([]byte(s)); err != nil {
		h.t.Fatal(err)
	}
	want := fmt.Sprintf("%04d", last%10000)
	deadline := time.After(2 * time.Second)
	for {
		h.s.Lock()
		scr := h.s.Screen()
		_, rows := scr.Size()
		got := make([]rune, 0, 4)
		for c := 2; c < 6; c++ {
			got = append(got, scr.Cell(c, rows-2).Rune)
		}
		h.s.Unlock()
		if string(got) == want {
			return
		}
		select {
		case <-h.changed:
		case <-deadline:
			h.t.Fatalf("row %s was not applied (row shows %q)", want, string(got))
		}
	}
}

func BenchmarkEmojiRowsMonochromeCached(b *testing.B) { emojiBench(b, 13.875, false, true) }
