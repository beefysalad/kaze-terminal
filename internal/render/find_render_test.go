package render_test

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/screen"
	"github.com/beefysalad/kaze-terminal/internal/term"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// pipePTY is a shell that only produces what the test writes to it.
type pipePTY struct {
	r    *io.PipeReader
	w    *io.PipeWriter
	once sync.Once
	done chan struct{}
}

func newPipePTY() *pipePTY {
	r, w := io.Pipe()
	return &pipePTY{r: r, w: w, done: make(chan struct{})}
}

func (p *pipePTY) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipePTY) Write(b []byte) (int, error) { return len(b), nil }
func (p *pipePTY) Resize(int, int) error       { return nil }
func (p *pipePTY) Wait() (int, error)          { <-p.done; return 0, nil }
func (p *pipePTY) Close() error {
	p.once.Do(func() { p.w.Close(); close(p.done) })
	return nil
}

// apiLogs is the find mockup's "api logs" member tab, a few screens of it so
// there is history to mark, plus a match that crosses a soft wrap and a line
// of wide characters.
func apiLogs() string {
	svc := func(name, text string) string { return fmt.Sprintf("\x1b[36m%-22s\x1b[0m| %s", name, text) }
	var b strings.Builder
	for round := 0; round < 4; round++ {
		for i := 0; i < 6; i++ {
			b.WriteString(svc("samurai_cart_api", fmt.Sprintf(`INFO:     127.0.0.1:5%d12 - "GET /api/v1/orders?page=%d HTTP/1.1" 200 OK`, i, i+1)) + "\r\n")
			b.WriteString(svc("samurai_cart_api", fmt.Sprintf(`INFO:     127.0.0.1:5%d13 - "GET /api/v1/reply_templates HTTP/1.1" 200 OK`, i)) + "\r\n")
		}
		b.WriteString(svc("samurai_cart_api", "* 'orm_mode' has been renamed to 'from_attributes'") + "\r\n")
		b.WriteString(svc("samurai_cart_api", "  warnings.warn(message, UserWarning)") + "\r\n")
		b.WriteString(svc("samurai_cart_api", "/usr/local/lib/python3.11/site-packages/pydantic/_internal/_config.py:334: UserWarning: Valid config keys have changed in V2:") + "\r\n")
		b.WriteString(svc("samurai_cart_api", "* 'schema_extra' has been renamed to 'json_schema_extra'") + "\r\n")
		b.WriteString(svc("samurai_cart_api", "  warnings.warn(message, UserWarning)") + "\r\n")
	}
	b.WriteString(svc("samurai_cart_api", "/app/app/api/api_v1/endpoints/reply_templates.py:95: FastAPIDeprecationWarning: `regex` has been deprecated") + "\r\n")
	b.WriteString(svc("samurai_cart_api", "INFO:     Started server process [67]") + "\r\n")
	b.WriteString(svc("samurai_cart_api", "INFO:     Application startup complete.") + "\r\n")
	b.WriteString(svc("samurai_cart_api", "tests/services/test_sony_reconcile_sweep_db.py [ 22%] wa") + "rning\r\n") // "wa" ends the row at columns 78-79
	b.WriteString(svc("samurai_cart_api", "警告 warn: 日本語のログ") + "\r\n")
	b.WriteString(svc("romantic_herschel", "====================== 110 passed in 803.38s (0:13:23) ======================") + "\r\n")
	b.WriteString("\x1b[35mromantic_herschel exited with code 0\x1b[0m\x1b[?25l")
	return b.String()
}

type cellAt struct{ row, col int }

// findSpans is a plain stand-in for the find package: each needle, case
// folded, over logical lines (rows joined across soft wraps), as spans on
// stable lines. Call with the session locked.
func findSpans(scr *screen.Screen, needles ...string) []render.MatchSpan {
	cols, rows := scr.Size()
	sb := scr.ScrollbackLen()
	total := sb + rows
	cell := func(col, row int) screen.Cell {
		if row >= sb {
			return scr.Cell(col, row-sb)
		}
		line := scr.ScrollbackLine(row)
		if col >= len(line) {
			return screen.Cell{Rune: ' ', Width: 1}
		}
		return line[col]
	}
	width := func(row int) int {
		if row < sb {
			return scr.ScrollbackWidth(row)
		}
		return cols
	}
	type found struct {
		at   []cellAt
		from int
	}
	var all []found
	for row := 0; row < total; {
		var rs []rune
		var at []cellAt
		end := row
		for {
			w := width(end)
			for c := 0; c < w; c++ {
				x := cell(c, end)
				if x.Width == 0 {
					continue
				}
				r := x.Rune
				if r == 0 {
					r = ' '
				}
				rs = append(rs, unicode.ToLower(r))
				at = append(at, cellAt{end, c})
			}
			if end+1 >= total || w == 0 || cell(w-1, end).Attrs&screen.AttrWrapped == 0 {
				break
			}
			end++
		}
		for _, n := range needles {
			nr := []rune(strings.ToLower(n))
			for i := 0; i+len(nr) <= len(rs); {
				if string(rs[i:i+len(nr)]) == string(nr) {
					all = append(all, found{at: at[i : i+len(nr)], from: i})
					i += len(nr)
					continue
				}
				i++
			}
		}
		row = end + 1
	}
	// Matches in reading order, one span per row each.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && less(all[j].at[0], all[j-1].at[0]); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	ev := int(scr.Evicted())
	var spans []render.MatchSpan
	for k, f := range all {
		for i := 0; i < len(f.at); {
			j := i
			for j+1 < len(f.at) && f.at[j+1].row == f.at[i].row {
				j++
			}
			to := f.at[j].col
			if cell(to, f.at[j].row).Width == 2 {
				to++
			}
			spans = append(spans, render.MatchSpan{Line: ev + f.at[i].row, From: f.at[i].col, To: to, Match: int32(k)})
			i = j + 1
		}
	}
	return spans
}

func savePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// zoom scales the part r of img up by k, nearest neighbour, to see the marks.
func zoom(img *image.RGBA, r image.Rectangle, k int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, r.Dx()*k, r.Dy()*k))
	for y := 0; y < out.Rect.Dy(); y++ {
		for x := 0; x < out.Rect.Dx(); x++ {
			out.Set(x, y, img.At(r.Min.X+x/k, r.Min.Y+y/k))
		}
	}
	return out
}

func less(a, b cellAt) bool { return a.row < b.row || a.row == b.row && a.col < b.col }

// Opt-in development capture of find's grid highlights and scroll marks as
// the renderer draws them, in every theme. Not a product screenshot.
func TestFindGridRenders(t *testing.T) {
	dir := os.Getenv("KAZE_FIND_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_FIND_RENDER_DIR for captures of find's highlights")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	const cols, rows = 80, 20
	for _, th := range theme.All() {
		pty := newPipePTY()
		changed := make(chan struct{}, 1)
		s := term.NewSession(pty, cols, rows, func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		})
		text := apiLogs()
		go pty.w.Write([]byte(text))
		deadline := time.After(3 * time.Second)
		for done := false; !done; {
			select {
			case <-changed:
			case <-deadline:
				t.Fatal("output did not land in time")
			case <-time.After(10 * time.Millisecond):
			}
			s.Lock()
			done = !s.Applier().Modes().CursorVisible
			s.Unlock()
		}

		s.Lock()
		spans := findSpans(s.Screen(), "warn", "警告")
		top := int(s.Screen().Evicted()) + s.Screen().ScrollbackLen()
		s.Unlock()
		// The current match: the last one starting on screen above the last four
		// rows; a selection over part of the one before it.
		var onScreen []render.MatchSpan // the first span of each such match
		for i, sp := range spans {
			if (i == 0 || spans[i-1].Match != sp.Match) && sp.Line >= top && sp.Line < top+rows-4 {
				onScreen = append(onScreen, sp)
			}
		}
		if len(onScreen) < 2 {
			t.Fatalf("%s: %d matches on screen, the capture needs two", th.Key, len(onScreen))
		}
		cur, selSpan := onScreen[len(onScreen)-1].Match, onScreen[len(onScreen)-2]
		marks := &render.Marks{Spans: spans, Cur: cur, Gen: 1}
		sel := &render.Selection{Active: true,
			Start: render.Point{Col: selSpan.From + 2, Row: selSpan.Line},
			End:   render.Point{Col: selSpan.To + 6, Row: selSpan.Line}}

		r := render.NewRenderer(16, th.Term, render.Options{})
		r.SetMarks(marks)
		cw, ch := r.CellSize()
		grid := image.Rect(12, 8, 12+cols*cw, 8+rows*ch)
		size := image.Pt(grid.Max.X+18, grid.Max.Y+8)
		track := image.Rect(size.X-9, 6, size.X-3, size.Y-6) // the mockup's 6 px column, 3 px in

		ops := new(op.Ops)
		for i := 0; i < 2; i++ { // the first frame builds the strips
			ops.Reset()
			paint.FillShape(ops, th.Term.BG, clip.Rect{Max: size}.Op())
			off := op.Offset(grid.Min).Push(ops)
			r.Layout(layout.Context{Ops: ops, Constraints: layout.Exact(grid.Size()), Now: time.Now()}, s, sel)
			off.Pop()
			r.DrawScrollMarks(layout.Context{Ops: ops, Constraints: layout.Exact(size), Now: time.Now()}, track)
		}
		gpu, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			t.Fatal(err)
		}
		if err = gpu.Frame(ops); err != nil {
			gpu.Release()
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		err = gpu.Screenshot(img)
		gpu.Release()
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		savePNG(t, filepath.Join(dir, "grid-"+th.Key+".png"), img)
		savePNG(t, filepath.Join(dir, "grid-"+th.Key+"-edge.png"), zoom(img, image.Rect(size.X-40, 0, size.X, size.Y), 4))
		matches := 0
		if len(spans) > 0 {
			matches = int(spans[len(spans)-1].Match) + 1
		}
		t.Logf("%s: %d matches in %d spans, current %d", th.Key, matches, len(spans), cur)
	}
}
