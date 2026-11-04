package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// apiLog is n lines of a server log in the mockup's style, a warn line every
// so often.
func apiLog(n, every int) []string {
	paths := []string{"/api/users", "/api/orders", "/api/health", "/api/cart/42", "/api/login"}
	out := make([]string, n)
	for i := range out {
		ts := fmt.Sprintf("12:%02d:%02d", (i/60)%60, i%60)
		if every > 0 && i%every == every-1 {
			out[i] = fmt.Sprintf("%s warn  slow query on %s took %d ms", ts, paths[i%len(paths)], 300+i%200)
			continue
		}
		out[i] = fmt.Sprintf("%s info  GET %s 200 %d ms", ts, paths[i%len(paths)], 4+i%40)
	}
	return out
}

// findScene sets up one capture state and returns the window size to draw.
func findScene(t *testing.T, win *window, st *recStarter, r *input.Router, a, m1 *tabs.Tab, state string) image.Point {
	t.Helper()
	size := image.Pt(1000, 600)
	frame := func() { sizedFrame(t, win, r, size.X, size.Y) }
	settle := func() {
		for range 20 {
			frame()
			if w := &win.find.work; w.settled && w.text == win.find.text && !w.pick && !w.reveal {
				return
			}
		}
		t.Fatal("search did not settle")
	}
	set := func(text string) {
		r.Queue(key.EditEvent{Range: key.Range{Start: 0, End: win.find.ed.Len()}, Text: text})
		settle()
	}
	if state == "narrow" {
		size.X = 520
	}
	if state == "member" {
		lines(t, st, 2, m1, apiLog(120, 9)...)
		lines(t, st, 0, a, "claude: reading the api logs", "no find here")
		win.openDockMember(a.ID, m1.ID)
		frame()
		frame()
		clickAt(t, win, r, centre(win.dock.hits.body))
	} else {
		lines(t, st, 0, a, apiLog(200, 7)...)
	}
	frame()
	win.openFind()
	frame()
	switch state {
	case "none":
		set("zzz")
	case "case":
		set("Warn")
		win.find.matchCase = true
		settle()
	default:
		set("warn")
	}
	if state == "selection" {
		frame()
		// Select the start of the last visible warn line's match.
		w := &win.find.work
		top, _ := viewLines(a)
		cw, ch := win.rend.CellSize()
		row := w.curPos.Line - top
		from := win.gridRect.Min.Add(image.Pt(w.curPos.Col*cw+cw/4, row*ch+ch/2))
		dragFrom(t, win, r, from, image.Pt(8*cw, 0))
	}
	settle()
	return size
}

func TestFindRenders(t *testing.T) {
	dir := os.Getenv("KAZE_FIND_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_FIND_RENDER_DIR for headless captures of the find bar")
	}
	os.MkdirAll(dir, 0o700)
	for _, th := range theme.All() {
		states := []string{"session", "member", "none", "case", "narrow", "selection"}
		if th.Key != "midnight" && th.Key != "sumi" && th.Key != "washi" {
			states = []string{"session"}
		}
		for _, state := range states {
			win, st, r, a, _, m1, _ := memberWindow(t)
			win.applyTheme(th)
			size := findScene(t, win, st, r, a, m1, state)
			g := pageContext(size.X, size.Y)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			gpu, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Fatal(err)
			}
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "window-"+th.Key+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, img)
			f.Close()
		}
	}
}

// TestFindFrameAllocs compares a still window frame with the bar open to the
// same frame closed, against the sidebar's filter field opened with the same
// text (the same widget.Editor drawn), and checks findTick adds nothing.
func TestFindFrameAllocs(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, apiLog(200, 37)...)
	g := pageContext(1000, 600)
	g.Ops = new(op.Ops)
	frame := func() {
		g.Ops.Reset()
		g.Source = r.Source()
		if err := win.frame(g); err != nil {
			t.Fatal(err)
		}
		r.Frame(g.Ops)
	}
	measure := func() float64 {
		for range 3 {
			frame()
		}
		return testing.AllocsPerRun(20, frame)
	}
	closed := measure()
	findFor(t, win, r, "warn")
	open := measure()
	locks := win.find.work.locks
	tick := testing.AllocsPerRun(20, func() { win.findTick(g) })
	if win.find.work.locks != locks {
		t.Fatalf("findTick locked the session on still frames")
	}
	pressKey(t, win, r, key.NameEscape, 0)
	closedAgain := measure()
	focusFilter(t, win, r)
	typeFilter(t, win, r, "warn")
	filter := measure()
	t.Logf("allocs per still frame: closed %.0f, find open %.0f (+%.0f), closed again %.0f, sidebar filter %.0f (+%.0f), findTick %.0f",
		closed, open, open-closed, closedAgain, filter, filter-closedAgain, tick)
	if tick != 0 {
		t.Fatalf("findTick allocates %v on a still frame", tick)
	}
	if open-closed > filter-closedAgain {
		t.Fatalf("the open bar adds %.0f allocations a frame, the sidebar filter %.0f", open-closed, filter-closedAgain)
	}
}
