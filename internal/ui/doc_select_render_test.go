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
