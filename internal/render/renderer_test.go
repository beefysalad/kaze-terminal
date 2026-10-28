package render

import (
	"image"
	"image/color"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/patrick-gogo/kaze/internal/screen"
	"github.com/patrick-gogo/kaze/internal/term"
)

// fakePTY is a shell that only produces what the test writes to it.
type fakePTY struct {
	outR *io.PipeReader
	outW *io.PipeWriter
	once sync.Once
	done chan struct{}
}

func newFakePTY() *fakePTY {
	r, w := io.Pipe()
	return &fakePTY{outR: r, outW: w, done: make(chan struct{})}
}

func (f *fakePTY) Read(b []byte) (int, error)  { return f.outR.Read(b) }
func (f *fakePTY) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakePTY) Resize(int, int) error       { return nil }
func (f *fakePTY) Wait() (int, error)          { <-f.done; return 0, nil }
func (f *fakePTY) Close() error {
	f.once.Do(func() { f.outW.Close(); close(f.done) })
	return nil
}

type harness struct {
	t       *testing.T
	pty     *fakePTY
	s       *term.Session
	changed chan struct{}
	r       *Renderer
}

func newHarness(t *testing.T, cols, rows int) *harness {
	t.Helper()
	h := &harness{t: t, pty: newFakePTY(), changed: make(chan struct{}, 64)}
	h.s = term.NewSession(h.pty, cols, rows, func() { h.changed <- struct{}{} })
	t.Cleanup(h.s.Close)
	h.r = NewRenderer(14, DefaultPalette())
	return h
}

// out feeds shell output and waits until the session has applied it.
func (h *harness) out(s string) {
	h.t.Helper()
	if _, err := h.pty.outW.Write([]byte(s)); err != nil {
		h.t.Fatal(err)
	}
	select {
	case <-h.changed:
	case <-time.After(2 * time.Second):
		h.t.Fatal("output was not applied")
	}
}

// frame lays out one frame and returns how many rows it rasterized.
func (h *harness) frame() int {
	before := h.r.Stats().RowsRedrawn
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Exact(image.Pt(2000, 2000)), Now: time.Now()}
	h.r.Layout(gtx, h.s, nil)
	return h.r.Stats().RowsRedrawn - before
}

func TestOnlyDirtyRowsAreRedrawn(t *testing.T) {
	h := newHarness(t, 80, 24)
	if n := h.frame(); n != 24 {
		t.Fatalf("first frame redrew %d rows, want 24", n)
	}
	h.out("a")
	if n := h.frame(); n != 1 {
		t.Fatalf("after one char, redrew %d rows, want 1", n)
	}
}

func TestNoChangeMeansNoRowsRedrawn(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	if n := h.frame(); n != 0 {
		t.Fatalf("idle frame redrew %d rows, want 0", n)
	}
}

func TestOnlyDirtyStripsGetNewImage(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	before := h.r.Stats().StripsUploaded
	h.out("a")
	h.frame()
	if d := h.r.Stats().StripsUploaded - before; d != 1 {
		t.Fatalf("one-cell change made %d new strip images, want 1", d)
	}
}

func TestGlyphRasterizedOnce(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.r = NewRenderer(14.25, DefaultPalette()) // a size no other test uses, so its shared cache starts empty
	h.frame()
	for i := 0; i < 10; i++ {
		h.out(strings.Repeat("a", 100))
		h.frame()
	}
	if n := h.r.Stats().GlyphsCached; n != 1 {
		t.Fatalf("GlyphsCached = %d after 1000 'a', want 1", n)
	}
}

func TestSynchronizedOutputHoldsFrame(t *testing.T) {
	h := newHarness(t, 80, 24)
	var offset time.Duration
	h.r.now = func() time.Time { return time.Now().Add(offset) }
	h.frame()

	h.out("\x1b[?2026hx")
	if n := h.frame(); n != 0 {
		t.Fatalf("frame during sync redrew %d rows, want 0", n)
	}
	offset = 200 * time.Millisecond // past the 150ms hold, mode still set
	if n := h.frame(); n != 1 {
		t.Fatalf("frame after sync timeout redrew %d rows, want 1", n)
	}

	offset = 0
	h.out("\x1b[?2026l\x1b[?2026h\r\ny")
	if n := h.frame(); n != 0 {
		t.Fatalf("frame in a new sync redrew %d rows, want 0", n)
	}
	h.out("\x1b[?2026l")
	if n := h.frame(); n != 1 {
		t.Fatalf("frame after sync reset redrew %d rows, want 1", n)
	}
}

func TestJapaneseUsesFallbackFont(t *testing.T) {
	r := NewRenderer(14, DefaultPalette())
	g := r.glyphs.get('日', false, true)
	if g.face != faceCJK {
		t.Fatalf("'日' came from face %d, want the CJK face", g.face)
	}
	if g.mask == nil || g.mask.Rect.Dx() != 2*r.cellW {
		t.Fatalf("'日' mask should span two cells")
	}
	if !hasInk(g.mask) {
		t.Fatal("'日' rasterized to an empty glyph")
	}
	if a := r.glyphs.get('a', false, false); a.face != faceMono {
		t.Fatalf("'a' came from face %d, want the mono face", a.face)
	}
}

func hasInk(m *image.Alpha) bool {
	for _, p := range m.Pix {
		if p != 0 {
			return true
		}
	}
	return false
}

func TestCellSizeComesFromMonoFont(t *testing.T) {
	r := NewRenderer(14, DefaultPalette())
	w, h := r.CellSize()
	if w < 6 || w > 12 || h < w || h > 3*w {
		t.Fatalf("CellSize = %dx%d, implausible for 14px", w, h)
	}
}

// rasterized cell pixels: these check colors land where they should.

func rowsImage(r *Renderer, row int) *image.RGBA {
	s := r.strips[row/r.rowsPerStrip]
	return s.img
}

func pixelAt(r *Renderer, col, row, dx, dy int) [4]uint8 {
	img := rowsImage(r, row)
	y := (row%r.rowsPerStrip)*r.cellH + dy
	x := col*r.cellW + dx
	i := img.PixOffset(x, y)
	return [4]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
}

func toArr(c color.NRGBA) [4]uint8 { return [4]uint8{c.R, c.G, c.B, c.A} }

func TestBackgroundAndInverse(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.out("\x1b[7m \x1b[0m\x1b[41m \x1b[0m")
	h.frame()
	p := DefaultPalette()
	want := func(c [4]uint8, col [4]uint8, what string) {
		t.Helper()
		if c != col {
			t.Fatalf("%s: got %v want %v", what, c, col)
		}
	}
	want(pixelAt(h.r, 0, 0, 0, 0), toArr(p.FG), "inverse blank shows FG as background")
	want(pixelAt(h.r, 1, 0, 0, 0), toArr(p.ANSI[1]), "red background")
	want(pixelAt(h.r, 5, 1, 0, 0), toArr(p.BG), "default background")
}

func TestBoldBrightensBasicColors(t *testing.T) {
	p := DefaultPalette()
	c := resolveFG(p, screen.Cell{FG: screen.Indexed(1), BG: screen.DefaultColor, Attrs: screen.AttrBold})
	if c != p.ANSI[9] {
		t.Fatalf("bold red = %v, want bright red %v", c, p.ANSI[9])
	}
	c = resolveFG(p, screen.Cell{FG: screen.Indexed(200), BG: screen.DefaultColor, Attrs: screen.AttrBold})
	if c != indexed(p, 200) {
		t.Fatal("bold must not change colors outside 0-7")
	}
}

func TestDimIsHalfwayToBackground(t *testing.T) {
	p := DefaultPalette()
	fg, bg := cellColors(p, screen.Cell{FG: screen.RGB(200, 100, 0), BG: screen.RGB(0, 0, 0), Attrs: screen.AttrDim})
	if fg.R != 100 || fg.G != 50 || fg.B != 0 || bg.R != 0 {
		t.Fatalf("dim fg = %v", fg)
	}
}

func TestIndexedCube(t *testing.T) {
	p := DefaultPalette()
	if c := indexed(p, 16); c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("16 = %v", c)
	}
	if c := indexed(p, 231); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("231 = %v", c)
	}
	if c := indexed(p, 196); c.R != 255 || c.G != 0 || c.B != 0 {
		t.Fatalf("196 = %v", c)
	}
	if c := indexed(p, 232); c.R != 8 {
		t.Fatalf("232 = %v", c)
	}
	if c := indexed(p, 255); c.R != 238 {
		t.Fatalf("255 = %v", c)
	}
}

func TestCursorMoveDoesNotReuploadStrips(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	before := h.r.Stats().StripsUploaded
	h.out("\x1b[10;10H") // cursor only, no cell changes
	h.frame()
	h.r.SetFocused(false)
	h.frame()
	if d := h.r.Stats().StripsUploaded - before; d != 0 {
		t.Fatalf("cursor-only changes made %d new strip images, want 0", d)
	}
}

func TestResizeRedrawsEverything(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	h.s.Resize(100, 30)
	if n := h.frame(); n != 30 {
		t.Fatalf("after resize redrew %d rows, want 30", n)
	}
	if len(h.r.strips) != stripCount {
		t.Fatalf("strips = %d, want %d", len(h.r.strips), stripCount)
	}
}
