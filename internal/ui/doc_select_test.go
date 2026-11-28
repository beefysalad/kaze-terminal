package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/pointer"
)

func docEvent(kind pointer.Kind, p image.Point) pointer.Event {
	e := pointer.Event{Kind: kind, Buttons: pointer.ButtonPrimary}
	e.Position.X, e.Position.Y = float32(p.X), float32(p.Y)
	return e
}

// lineAt is the window point on line i of the shown layout, at doc x.
func lineAt(win *window, d *docView, i, x int) image.Point {
	v := win.docs.hits.view
	l := d.lay.lines[i]
	return image.Pt(v.Min.X+x, v.Min.Y+l.y+l.h/2-d.scroll)
}

func copiedFrom(win *window) string {
	var got string
	win.toClipboard = func(s string) { got = s }
	win.runShortcut(pageContext(900, 600), actCopy)
	return got
}

func TestDocSelectDragCopiesText(t *testing.T) {
	win, d := layoutTestDoc(t, "alpha beta\n\ngamma delta")
	gtx := pageContext(900, 600)
	r := image.Rect(0, 0, 900, 600)
	win.drawDoc(gtx, d, r)
	last := len(d.lay.lines) - 1
	end := d.lay.lines[last].runs[len(d.lay.lines[last].runs)-1]
	win.docPointer(gtx, docEvent(pointer.Press, lineAt(win, d, 0, 0)))
	win.docPointer(gtx, docEvent(pointer.Drag, lineAt(win, d, last, end.x+end.w+20)))
	win.docPointer(gtx, docEvent(pointer.Release, lineAt(win, d, last, end.x+end.w+20)))
	win.drawDoc(gtx, d, r)
	if got := copiedFrom(win); got != "alpha beta\n\ngamma delta" {
		t.Fatalf("copied %q", got)
	}
}

func TestDocSelectPartialLine(t *testing.T) {
	win, d := layoutTestDoc(t, "hello world")
	gtx := pageContext(900, 600)
	r := image.Rect(0, 0, 900, 600)
	win.drawDoc(gtx, d, r)
	var world docRun
	for _, run := range d.lay.lines[0].runs {
		if run.text == "world" {
			world = run
		}
	}
	win.docPointer(gtx, docEvent(pointer.Press, lineAt(win, d, 0, 0)))
	win.docPointer(gtx, docEvent(pointer.Drag, lineAt(win, d, 0, world.x+1)))
	win.docPointer(gtx, docEvent(pointer.Release, lineAt(win, d, 0, world.x+1)))
	if got := copiedFrom(win); got != "hello " {
		t.Fatalf("copied %q", got)
	}
}

func TestDocSelectWrappedParagraphCopiesAsOneLine(t *testing.T) {
	body := strings.TrimSpace(strings.Repeat("word ", 60))
	win, d := layoutTestDoc(t, body)
	gtx := pageContext(900, 600)
	r := image.Rect(0, 0, 360, 600)
	win.drawDoc(gtx, d, r)
	if len(d.lay.lines) < 3 {
		t.Fatal("did not wrap")
	}
	last := len(d.lay.lines) - 1
	win.docPointer(gtx, docEvent(pointer.Press, lineAt(win, d, 0, 0)))
	win.docPointer(gtx, docEvent(pointer.Drag, lineAt(win, d, last, 5000)))
	win.docPointer(gtx, docEvent(pointer.Release, lineAt(win, d, last, 5000)))
	if got := copiedFrom(win); got != body {
		t.Fatalf("copied %q", got)
	}
}

func TestDocSelectClickClearsAndNothingCopies(t *testing.T) {
	win, d := layoutTestDoc(t, "hello world")
	gtx := pageContext(900, 600)
	r := image.Rect(0, 0, 900, 600)
	win.drawDoc(gtx, d, r)
	win.docPointer(gtx, docEvent(pointer.Press, lineAt(win, d, 0, 0)))
	win.docPointer(gtx, docEvent(pointer.Drag, lineAt(win, d, 0, 5000)))
	win.docPointer(gtx, docEvent(pointer.Release, lineAt(win, d, 0, 5000)))
	win.docPointer(gtx, docEvent(pointer.Press, lineAt(win, d, 0, 3)))
	win.docPointer(gtx, docEvent(pointer.Release, lineAt(win, d, 0, 3)))
	if got := copiedFrom(win); got != "" {
		t.Fatalf("copied %q after a plain click", got)
	}
}
