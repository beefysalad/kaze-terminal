package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/pointer"
	"github.com/beefysalad/kaze-terminal/internal/mdoc"
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

func TestDocSelectWrappedTokens(t *testing.T) {
	for _, body := range []string{strings.Repeat("日本語", 60), "```\n" + strings.Repeat("abcdef", 60) + "\n```"} {
		win, d := layoutTestDoc(t, body)
		l := win.layoutDoc(pageContext(900, 600), d, 150)
		last := len(l.lines) - 1
		got := selectedDocText(l, docPos{}, docPos{last, lineRunes(l.lines[last])})
		want := strings.TrimSuffix(strings.TrimPrefix(body, "```\n"), "\n```")
		if got != want {
			t.Fatalf("wrapped token copied %q, want %q", got, want)
		}
	}
}

func TestDocSelectSharedRows(t *testing.T) {
	for _, body := range []string{"- first item", "| left | right |\n|---|---|\n| one | two |"} {
		win, d := layoutTestDoc(t, body)
		g := pageContext(900, 600)
		l := win.layoutDoc(g, d, 500)
		for i, line := range l.lines {
			for _, r := range line.runs {
				if strings.TrimSpace(r.text) == "" {
					continue
				}
				p, ok := win.docPosAt(g, l, image.Pt(r.x, line.y+line.h/2))
				if !ok || p.line != i {
					t.Errorf("%q hit line %d, want %d", r.text, p.line, i)
				}
			}
		}
		last := len(l.lines) - 1
		got := selectedDocText(l, docPos{}, docPos{last, lineRunes(l.lines[last])})
		if strings.Contains(body, "|") && got != "left\tright\n\none\ttwo" {
			t.Fatalf("table copied %q", got)
		}
		if strings.HasPrefix(body, "-") && !strings.HasSuffix(got, " first item") {
			t.Errorf("list copied %q", got)
		}
	}
}

func TestDocSelectLinkClickOrDrag(t *testing.T) {
	win, d := layoutTestDoc(t, "[hello world](https://example.com)")
	g := pageContext(900, 600)
	win.drawDoc(g, d, image.Rect(0, 0, 900, 600))
	opened := 0
	win.openLink = func(string) error { opened++; return nil }
	p := win.docs.hits.links[0].rect.Min.Add(image.Pt(1, 1))
	win.docPointer(g, docEvent(pointer.Press, p))
	if opened != 0 {
		t.Fatal("link opened before release")
	}
	win.docPointer(g, docEvent(pointer.Release, p))
	if opened != 1 {
		t.Fatal("click did not open link")
	}
	win.docPointer(g, docEvent(pointer.Press, p))
	end := lineAt(win, d, 0, 5000)
	win.docPointer(g, docEvent(pointer.Drag, end))
	win.docPointer(g, docEvent(pointer.Release, end))
	if opened != 1 || copiedFrom(win) != "hello world" {
		t.Fatalf("drag opened %d, copied %q", opened, copiedFrom(win))
	}
}

func TestDocSelectionInvalidatedBeforeRedraw(t *testing.T) {
	for _, action := range []string{"reload", "hide", "switch"} {
		t.Run(action, func(t *testing.T) {
			win, d := layoutTestDoc(t, "old text")
			g := pageContext(900, 600)
			win.drawDoc(g, d, image.Rect(0, 0, 900, 600))
			win.docs.sel = docSel{doc: d.id, key: d.lay.key, on: true, drag: true, head: docPos{0, 3}}
			switch action {
			case "reload":
				d.doc = mdoc.Parse([]byte("new text"))
			case "hide":
				win.hideDoc()
				win.showDoc(d.id)
			case "switch":
				other := win.openReportDoc(d.session, "Other", "other")
				win.showDoc(other.id)
				win.showDoc(d.id)
			}
			if got := win.docSelText(d); got != "" {
				t.Fatalf("stale selection copied %q", got)
			}
		})
	}
}

func TestDocSelectionResizeAndCancel(t *testing.T) {
	win, d := layoutTestDoc(t, "hello 日本語 world")
	g := pageContext(900, 600)
	win.drawDoc(g, d, image.Rect(0, 0, 900, 600))
	win.docs.sel = docSel{doc: d.id, key: d.lay.key, on: true, drag: true, head: docPos{0, 8}}
	win.drawDoc(g, d, image.Rect(0, 0, 400, 600))
	if win.docs.sel.on || win.docs.sel.drag || copiedFrom(win) != "" {
		t.Fatal("resize retained selection")
	}
	p := lineAt(win, d, 0, 0)
	win.docPointer(g, docEvent(pointer.Press, p))
	win.docPointer(g, docEvent(pointer.Drag, lineAt(win, d, 0, 5000)))
	win.docPointer(g, docEvent(pointer.Cancel, p))
	if win.docs.sel.on || win.docs.sel.drag {
		t.Fatal("cancel retained drag")
	}
}

func TestDocSelectionCodeTabsAndRuneBoundaries(t *testing.T) {
	win, d := layoutTestDoc(t, "```\n\t日本語abc\n```")
	g := pageContext(900, 600)
	l := win.layoutDoc(g, d, 500)
	var off int
	for _, r := range l.lines[0].runs {
		for n := 0; n < len([]rune(r.text)); n++ {
			p, ok := win.docPosAt(g, l, image.Pt(r.x+win.runAdv(g, r, n), l.lines[0].y+1))
			if !ok || p.off != off+n {
				t.Fatalf("rune boundary %d: %+v", off+n, p)
			}
		}
		off += len([]rune(r.text))
	}
	if got := selectedDocText(l, docPos{}, docPos{0, off}); got != "    日本語abc" {
		t.Fatalf("code copied %q", got)
	}
}

func TestDocSelectionDragOutsideView(t *testing.T) {
	win, d := layoutTestDoc(t, strings.Repeat("paragraph\n\n", 100))
	g := pageContext(900, 600)
	win.drawDoc(g, d, image.Rect(0, 0, 900, 600))
	win.docPointer(g, docEvent(pointer.Press, lineAt(win, d, 0, 0)))
	v := win.docs.hits.view
	win.docPointer(g, docEvent(pointer.Drag, v.Max.Add(image.Pt(20, 20))))
	if d.scroll != 12 || win.docs.sel.head.line <= 0 {
		t.Fatalf("outside drag: scroll %d, head %+v", d.scroll, win.docs.sel.head)
	}
	win.docPointer(g, docEvent(pointer.Drag, v.Min.Sub(image.Pt(20, 100))))
	if d.scroll != 0 || win.docs.sel.head != (docPos{}) {
		t.Fatalf("top clamp: scroll %d, head %+v", d.scroll, win.docs.sel.head)
	}
}

func TestDocSelectionWrappedListAndStyledWord(t *testing.T) {
	for _, body := range []string{"- " + strings.TrimSpace(strings.Repeat("word ", 50)), "prefix**suffix**"} {
		win, d := layoutTestDoc(t, body)
		width := 150
		if strings.HasPrefix(body, "prefix") {
			width = 55
		}
		l := win.layoutDoc(pageContext(900, 600), d, width)
		last := len(l.lines) - 1
		got := selectedDocText(l, docPos{}, docPos{last, lineRunes(l.lines[last])})
		want := "prefixsuffix"
		if strings.HasPrefix(body, "-") {
			want = l.lines[0].runs[0].text + " " + strings.TrimPrefix(body, "- ")
		}
		if got != want {
			t.Errorf("copied %q, want %q", got, want)
		}
	}
}
