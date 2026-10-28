// Command gio is a throwaway speed spike: it measures whether Gio can draw a
// 200x60 terminal grid fast enough, via Gio's text path or a glyph atlas.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	gioopentype "gioui.org/font/opentype"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	xfont "golang.org/x/image/font"
	xopentype "golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const (
	cols = 200
	rows = 60
	// prefixCols is the ASCII-only prefix of every line; single-cell edits land
	// here so they never split a double-width character.
	prefixCols = 20
	fontSizeSp = 13
)

var (
	modeFlag      = flag.String("mode", "text", "text|atlas|empty|bare (empty = fonts loaded, no grid; bare = no fonts, no grid)")
	streamFlag    = flag.Bool("stream", false, "rewrite 5 random lines every 2ms")
	autotypeFlag  = flag.Bool("autotype", false, "change one cell every 50ms")
	durationFlag  = flag.Duration("duration", 0, "exit after this long and print frame stats (0 = run until closed)")
	logFlag       = flag.String("log", "", "write per-frame times (ms) to this file")
	atlasDrawFlag = flag.String("atlasdraw", "clip", "atlas mode only: clip (one atlas texture, clipped per cell) | sub (one sub-image ImageOp per glyph)")
	memprofFlag   = flag.String("memprofile", "", "write a heap profile here on exit")
	monoFlag      = flag.String("mono", `C:\Windows\Fonts\consola.ttf`, "monospace font file (falls back to Go Mono)")
	cjkFlag       = flag.String("cjk", `C:\Windows\Fonts\msgothic.ttc`, "CJK fallback font file (.ttf or .ttc)")
)

var cjkSamples = []rune("日本語テスト")
var asciiAlphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,:;-_/()[]{}<>=+*#@$%&!?")

// grid holds the cells; a 0 rune marks the right half of a double-width char.
type grid struct {
	mu      sync.Mutex
	cells   [rows][cols]rune
	version [rows]uint64
}

func (g *grid) fillLine(r int, rng *rand.Rand, seed int) {
	line := &g.cells[r]
	prefix := []rune(fmt.Sprintf("L%03d s%06d:       ", r, seed))[:prefixCols]
	c := copy(line[:], prefix)
	for c < cols {
		if rng.Intn(3) == 0 && c+2 <= cols {
			for _, ch := range cjkSamples {
				if c+2 > cols {
					break
				}
				line[c], line[c+1] = ch, 0
				c += 2
			}
			continue
		}
		line[c] = asciiAlphabet[rng.Intn(len(asciiAlphabet))]
		c++
	}
	g.version[r]++
}

func (g *grid) lineString(r int) string {
	var b strings.Builder
	for _, ch := range g.cells[r] {
		if ch != 0 {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func (g *grid) setCell(r, c int, ch rune) {
	g.mu.Lock()
	g.cells[r][c] = ch
	g.version[r]++
	g.mu.Unlock()
}

// frameStats collects per-frame durations.
type frameStats struct {
	mu     sync.Mutex
	total  []float64 // layout + Frame submit, ms
	layout []float64 // building ops only, ms
	start  []float64 // frame start timestamp, ms
}

func (s *frameStats) add(startMs, layoutMs, totalMs float64) {
	s.mu.Lock()
	s.start = append(s.start, startMs)
	s.layout = append(s.layout, layoutMs)
	s.total = append(s.total, totalMs)
	s.mu.Unlock()
}

func pct(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

func summarize(name string, v []float64) string {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	late := 0
	for _, x := range s {
		if x > 16.7 {
			late++
		}
	}
	max := 0.0
	if len(s) > 0 {
		max = s[len(s)-1]
	}
	return fmt.Sprintf("%s: n=%d p50=%.3fms p99=%.3fms max=%.3fms late(>16.7ms)=%d",
		name, len(s), pct(s, 0.50), pct(s, 0.99), max, late)
}

func (s *frameStats) report() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Drop the first frames: they include font loading, shaping every line and
	// the atlas upload, which is startup cost, not steady-state frame cost.
	skip := 3
	if len(s.total) <= skip {
		skip = 0
	}
	fmt.Printf("mode=%s stream=%v autotype=%v\n", *modeFlag, *streamFlag, *autotypeFlag)
	fmt.Println(summarize("frame(layout+submit)", s.total[skip:]))
	fmt.Println(summarize("layout-only", s.layout[skip:]))
	// Collect first so HeapAlloc and the heap profile show live memory, not garbage.
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("go heap after GC: HeapAlloc=%.1fMB HeapSys=%.1fMB Sys=%.1fMB\n",
		float64(ms.HeapAlloc)/1e6, float64(ms.HeapSys)/1e6, float64(ms.Sys)/1e6)
	if *memprofFlag != "" {
		if f, err := os.Create(*memprofFlag); err == nil {
			pprof.WriteHeapProfile(f)
			f.Close()
		}
	}
	if *logFlag != "" {
		f, err := os.Create(*logFlag)
		if err == nil {
			fmt.Fprintln(f, "frame,start_ms,layout_ms,total_ms")
			for i := range s.total {
				fmt.Fprintf(f, "%d,%.1f,%.4f,%.4f\n", i, s.start[i], s.layout[i], s.total[i])
			}
			f.Close()
		}
	}
}

// renderer draws the grid into ops.
type renderer interface {
	layout(gtx layout.Context, g *grid)
}

// lineCache keeps one recorded macro per line, re-recorded only when the line changes.
type lineCache struct {
	version uint64
	valid   bool
	ops     op.Ops
	call    op.CallOp
}

var (
	bgColor = color.NRGBA{R: 0x1e, G: 0x1e, B: 0x22, A: 0xff}
	fgColor = color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
)

// ---- text mode: Gio's shaper, one widget.Label per line -------------------

type textRenderer struct {
	shaper *text.Shaper
	font   font.Font
	lines  [rows]lineCache
	lineH  int
}

func newTextRenderer(faces []font.FontFace) *textRenderer {
	sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
	return &textRenderer{shaper: sh, font: faces[0].Font}
}

func (t *textRenderer) layout(gtx layout.Context, g *grid) {
	if t.lineH == 0 {
		t.lineH = gtx.Sp(fontSizeSp * 1.3)
	}
	g.mu.Lock()
	for r := 0; r < rows; r++ {
		lc := &t.lines[r]
		if !lc.valid || lc.version != g.version[r] {
			s := g.lineString(r)
			lc.ops.Reset()
			lgtx := gtx
			lgtx.Ops = &lc.ops
			lgtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, t.lineH))
			lgtx.Constraints.Min = image.Point{}
			m := op.Record(&lc.ops)
			// The material lives in the line's own ops so the cached macro stays valid.
			mm := op.Record(&lc.ops)
			paint.ColorOp{Color: fgColor}.Add(&lc.ops)
			mat := mm.Stop()
			widget.Label{MaxLines: 1}.Layout(lgtx, t.shaper, t.font, unit.Sp(fontSizeSp), s, mat)
			lc.call = m.Stop()
			lc.version = g.version[r]
			lc.valid = true
		}
		st := op.Offset(image.Pt(0, r*t.lineH)).Push(gtx.Ops)
		lc.call.Add(gtx.Ops)
		st.Pop()
	}
	g.mu.Unlock()
}

// ---- atlas mode: glyphs rasterized once into one RGBA atlas ---------------

type glyphSlot struct {
	pos  image.Point   // top-left in the atlas
	size image.Point   // cell size (double width for CJK)
	img  paint.ImageOp // sub-image of the atlas, bounded to the glyph cell
}

type atlasRenderer struct {
	monoSrc, cjkSrc []byte
	built           bool
	atlas           paint.ImageOp
	slots           map[rune]glyphSlot
	cellW, cellH    int
	lines           [rows]lineCache
}

func (a *atlasRenderer) build(px float64) {
	mono, err := xopentype.Parse(a.monoSrc)
	if err != nil {
		log.Fatal(err)
	}
	cjk, err := parseFirstFont(a.cjkSrc)
	if err != nil {
		log.Fatal(err)
	}
	opts := &xopentype.FaceOptions{Size: px, DPI: 72, Hinting: xfont.HintingFull}
	monoFace, _ := xopentype.NewFace(mono, opts)
	cjkFace, _ := xopentype.NewFace(cjk, opts)
	m := monoFace.Metrics()
	adv, _ := monoFace.GlyphAdvance('M')
	a.cellW = adv.Ceil()
	a.cellH = (m.Ascent + m.Descent).Ceil() + 2
	ascent := m.Ascent.Ceil() + 1

	var runes []rune
	runes = append(runes, asciiAlphabet...)
	runes = append(runes, cjkSamples...)
	const perRow = 32
	atlasW := perRow * a.cellW * 2
	atlasH := (len(runes)/perRow + 1) * a.cellH
	img := image.NewRGBA(image.Rect(0, 0, atlasW, atlasH))
	a.slots = make(map[rune]glyphSlot, len(runes))
	var buf sfnt.Buffer
	for i, ch := range runes {
		pos := image.Pt((i%perRow)*a.cellW*2, (i/perRow)*a.cellH)
		face := monoFace
		if idx, _ := mono.GlyphIndex(&buf, ch); idx == 0 {
			face = cjkFace
		}
		d := xfont.Drawer{Dst: img, Src: image.NewUniform(fgColor), Face: face,
			Dot: fixed.P(pos.X, pos.Y+ascent)}
		d.DrawString(string(ch))
		w := a.cellW
		if face == cjkFace {
			w *= 2
		}
		sub := img.SubImage(image.Rectangle{Min: pos, Max: pos.Add(image.Pt(w, a.cellH))}).(*image.RGBA)
		a.slots[ch] = glyphSlot{pos: pos, size: sub.Rect.Size(), img: paint.NewImageOp(sub)}
	}
	a.atlas = paint.NewImageOp(img)
	a.built = true
}

func parseFirstFont(b []byte) (*sfnt.Font, error) {
	if f, err := xopentype.Parse(b); err == nil {
		return f, nil
	}
	c, err := xopentype.ParseCollection(b)
	if err != nil {
		return nil, err
	}
	return c.Font(0)
}

func (a *atlasRenderer) layout(gtx layout.Context, g *grid) {
	if !a.built {
		a.build(float64(gtx.Sp(fontSizeSp)))
	}
	g.mu.Lock()
	for r := 0; r < rows; r++ {
		lc := &a.lines[r]
		if !lc.valid || lc.version != g.version[r] {
			lc.ops.Reset()
			ops := &lc.ops
			m := op.Record(ops)
			for c := 0; c < cols; c++ {
				ch := g.cells[r][c]
				if ch == 0 || ch == ' ' {
					continue
				}
				slot, ok := a.slots[ch]
				if !ok {
					continue
				}
				st := op.Offset(image.Pt(c*a.cellW, 0)).Push(ops)
				if *atlasDrawFlag == "sub" {
					// A bounded image paints only its own rect, so no clip is needed,
					// but each glyph becomes its own GPU texture.
					slot.img.Add(ops)
					paint.PaintOp{}.Add(ops)
				} else {
					// One shared atlas texture, clipped to the cell.
					cl := clip.Rect{Max: slot.size}.Push(ops)
					st2 := op.Offset(slot.pos.Mul(-1)).Push(ops)
					a.atlas.Add(ops)
					paint.PaintOp{}.Add(ops)
					st2.Pop()
					cl.Pop()
				}
				st.Pop()
			}
			lc.call = m.Stop()
			lc.version = g.version[r]
			lc.valid = true
		}
		st := op.Offset(image.Pt(0, r*a.cellH)).Push(gtx.Ops)
		lc.call.Add(gtx.Ops)
		st.Pop()
	}
	g.mu.Unlock()
}

type emptyRenderer struct{}

func (emptyRenderer) layout(layout.Context, *grid) {}

// ---- fonts ---------------------------------------------------------------

func loadFaces() ([]font.FontFace, []byte, []byte) {
	var faces []font.FontFace
	monoSrc, err := os.ReadFile(*monoFlag)
	if err == nil {
		f, perr := gioopentype.Parse(monoSrc)
		if perr != nil {
			log.Fatalf("parse mono: %v", perr)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Mono"}, Face: f})
	} else {
		log.Printf("mono font %s unavailable (%v); using Go Mono", *monoFlag, err)
		for _, ff := range gofont.Collection() {
			if strings.Contains(string(ff.Font.Typeface), "Mono") && ff.Font.Weight == font.Normal && ff.Font.Style == font.Regular {
				ff.Font.Typeface = "Mono"
				faces = append(faces, ff)
				break
			}
		}
	}
	cjkSrc, err := os.ReadFile(*cjkFlag)
	if err != nil {
		log.Fatalf("cjk font: %v", err)
	}
	cfs, err := gioopentype.ParseCollection(cjkSrc)
	if err != nil {
		log.Fatalf("parse cjk: %v", err)
	}
	// Keep only the first face of the collection, as a separate typeface, so
	// the shaper falls back to it for glyphs the mono font lacks.
	cf := cfs[0]
	cf.Font.Typeface = "CJK"
	faces = append(faces, cf)
	return faces, monoSrc, cjkSrc
}

// ---- main loop -----------------------------------------------------------

func main() {
	flag.Parse()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("kaze gio spike ("+*modeFlag+")"), app.Size(unit.Dp(1500), unit.Dp(1000)))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	var faces []font.FontFace
	var monoSrc, cjkSrc []byte
	if *modeFlag != "bare" {
		faces, monoSrc, cjkSrc = loadFaces()
	}
	var r renderer
	switch *modeFlag {
	case "text":
		r = newTextRenderer(faces)
	case "atlas":
		if monoSrc == nil {
			return fmt.Errorf("atlas mode needs -mono pointing at a font file")
		}
		r = &atlasRenderer{monoSrc: monoSrc, cjkSrc: cjkSrc}
	case "empty":
		// Baseline: same window, background only, no grid drawn.
		r = emptyRenderer{}
	case "bare":
		// Truly empty: no fonts parsed, background only. Memory and Present floor.
		r = emptyRenderer{}
	default:
		return fmt.Errorf("unknown -mode %q", *modeFlag)
	}

	g := &grid{}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < rows; i++ {
		g.fillLine(i, rng, 0)
	}

	stats := &frameStats{}
	if *durationFlag > 0 {
		time.AfterFunc(*durationFlag, func() {
			stats.report()
			os.Exit(0)
		})
	}

	if *streamFlag {
		go func() {
			srng := rand.New(rand.NewSource(2))
			seed := 0
			t := time.NewTicker(2 * time.Millisecond)
			for range t.C {
				g.mu.Lock()
				for i := 0; i < 5; i++ {
					seed++
					g.fillLine(srng.Intn(rows), srng, seed)
				}
				g.mu.Unlock()
				w.Invalidate()
			}
		}()
	}
	if *autotypeFlag {
		go func() {
			arng := rand.New(rand.NewSource(3))
			t := time.NewTicker(50 * time.Millisecond)
			for range t.C {
				g.setCell(arng.Intn(rows), arng.Intn(prefixCols), asciiAlphabet[arng.Intn(len(asciiAlphabet))])
				w.Invalidate()
			}
		}()
	}

	var ops op.Ops
	keyTag := new(int)
	cursor := 0
	focused := false
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			if *durationFlag == 0 {
				stats.report()
			}
			return e.Err
		case app.FrameEvent:
			t0 := nowMs()
			gtx := app.NewContext(&ops, e)

			// A keystroke changes one cell (row 0, advancing within the ASCII prefix).
			for {
				ev, ok := gtx.Event(key.FocusFilter{Target: keyTag}, key.Filter{Focus: keyTag, Name: ""})
				if !ok {
					break
				}
				if ke, ok := ev.(key.Event); ok && ke.State == key.Press {
					g.setCell(0, cursor%prefixCols, asciiAlphabet[cursor%len(asciiAlphabet)])
					cursor++
					w.Invalidate()
				}
			}
			event.Op(gtx.Ops, keyTag)
			if !focused {
				gtx.Execute(key.FocusCmd{Tag: keyTag})
				focused = true
			}

			paint.Fill(gtx.Ops, bgColor)
			r.layout(gtx, g)
			t1 := nowMs()
			e.Frame(gtx.Ops)
			t2 := nowMs()
			stats.add(t0, t1-t0, t2-t0)
		}
	}
}
