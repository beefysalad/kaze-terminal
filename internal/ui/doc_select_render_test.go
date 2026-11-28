package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/io/pointer"
)

// Render a document with a selection, without a native window or input.
func TestDocSelectionRenders(t *testing.T) {
	dir := os.Getenv("KAZE_DOCSEL_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_DOCSEL_RENDER_DIR for PNG evidence")
	}
	win, d := layoutTestDoc(t, "# Plan\n\nThe quick brown fox jumps over the lazy dog and keeps running across the field until the end of the line wraps.\n\n- first item\n- second item\n\n```\ngo test ./...\n```\n")
	size := image.Pt(900, 600)
	g := pageContext(size.X, size.Y)
	r := image.Rectangle{Max: size}
	win.drawDoc(g, d, r)
	last := len(d.lay.lines) - 2
	win.docPointer(g, docEvent(pointer.Press, lineAt(win, d, 1, 40)))
	win.docPointer(g, docEvent(pointer.Drag, lineAt(win, d, last, 60)))
	win.docPointer(g, docEvent(pointer.Release, lineAt(win, d, last, 60)))
	g = pageContext(size.X, size.Y)
	win.drawDoc(g, d, r)
	gpu, e := headless.NewWindow(size.X, size.Y)
	if e != nil {
		t.Fatal(e)
	}
	defer gpu.Release()
	if e = gpu.Frame(g.Ops); e != nil {
		t.Fatal(e)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if e = gpu.Screenshot(img); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(filepath.Join(dir, "doc-selection.png"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
}

// Inline code's background must not cover the selection beneath it.
func TestDocSelectionInlineCodePaintOrder(t *testing.T) {
	if os.Getenv("KAZE_DOCSEL_RENDER_DIR") == "" {
		t.Skip("opt-in GPU check")
	}
	win, d := layoutTestDoc(t, "`selected code`")
	size := image.Pt(900, 600)
	g := pageContext(size.X, size.Y)
	win.drawDoc(g, d, image.Rectangle{Max: size})
	gpu, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Release()
	render := func(selected bool) *image.RGBA {
		g = pageContext(size.X, size.Y)
		if selected {
			win.docs.sel = docSel{doc: d.id, key: d.lay.key, on: true, head: docPos{0, lineRunes(d.lay.lines[0])}}
		}
		win.drawDoc(g, d, image.Rectangle{Max: size})
		if err := gpu.Frame(g.Ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		if err := gpu.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	before, after := render(false), render(true)
	run := d.lay.lines[0].runs[0]
	p := win.docs.hits.view.Min.Add(image.Pt(run.x+2, d.lay.lines[0].y+1))
	if before.RGBAAt(p.X, p.Y) == after.RGBAAt(p.X, p.Y) {
		t.Fatal("inline code hides selection")
	}
}
