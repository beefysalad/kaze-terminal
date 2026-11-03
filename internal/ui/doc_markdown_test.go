package ui

import (
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/mdoc"
)

func TestLayoutMarkdownPanelScaleIsSmaller(t *testing.T) {
	win, _ := layoutTestDoc(t, "x")
	d := mdoc.Parse([]byte("# Title\n\nSome **bold** body text that wraps a little.\n\n- one\n- two\n"))
	gtx := pageContext(900, 600)
	reader := win.layoutMarkdown(gtx, d, 400, readerStyle)
	panel := win.layoutMarkdown(gtx, d, 400, panelStyle)
	if panel.height >= reader.height {
		t.Fatalf("panel %d not smaller than reader %d", panel.height, reader.height)
	}
	found := false
	for _, ln := range panel.lines {
		for _, r := range ln.runs {
			if r.text == "Some" {
				found = true
				if r.size != 12 {
					t.Fatalf("body size %v, want 12", r.size)
				}
			}
		}
	}
	if !found {
		t.Fatal("body run not found")
	}
}

func TestLayoutMarkdownTableFitsWidth(t *testing.T) {
	win, _ := layoutTestDoc(t, "x")
	row := "| " + strings.Repeat("longword | ", 6)
	src := row + "\n|" + strings.Repeat("---|", 6) + "\n" + row + "\n"
	l := win.layoutMarkdown(pageContext(900, 600), mdoc.Parse([]byte(src)), 240, panelStyle)
	for _, ln := range l.lines {
		for _, r := range ln.runs {
			if r.x+r.w > 240 {
				t.Fatalf("run past the width: %+v", r)
			}
		}
	}
	for _, b := range l.boxes {
		if b.r.Max.X > 240 {
			t.Fatalf("box past the width: %v", b.r)
		}
	}
}
