package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
	"image"
	"testing"
)

func TestWSListToggleOpensOneRow(t *testing.T) {
	w := &window{}
	w.wsp.doc.Workspaces = []workspaces.Workspace{{ID: "a"}, {ID: "b"}}
	wsAct(w, "ws.toggle", 0)
	if w.wsp.open != "a" {
		t.Fatal("first")
	}
	wsAct(w, "ws.toggle", 1)
	if w.wsp.open != "b" {
		t.Fatal("second")
	}
	wsAct(w, "ws.toggle", 1)
	if w.wsp.open != "" {
		t.Fatal("close")
	}
}

func countText(f *runFixture, s string) int {
	n := 0
	for _, t := range f.win.set.texts {
		if t == "STARTUP" {
			break
		}
		if t == s {
			n++
		}
	}
	return n
}

func TestWSListOpenRowDrawsNameOnce(t *testing.T) {
	f := newWSPage(t)
	name := f.win.wsp.doc.Workspaces[0].Name
	f.press("ws.toggle", 0)
	if n := countText(f, name); n != 1 {
		t.Fatalf("name %q drawn %d times", name, n)
	}
}

func TestWSListStartupOnlyInMenu(t *testing.T) {
	f := newWSPage(t)
	f.press("ws.toggle", 0)
	if len(controlsOf(f.win, "ws.row.startup")) != 0 {
		t.Fatal("startup shown without the menu")
	}
	if len(controlsOf(f.win, "ws.edit")) != 1 || len(controlsOf(f.win, "ws.delete")) != 1 {
		t.Fatal("edit and delete missing in open row")
	}
	f.press("ws.menu", 0)
	if len(controlsOf(f.win, "ws.row.startup")) != 1 {
		t.Fatal("startup missing after menu")
	}
}

func TestWSListControlsStayInPane(t *testing.T) {
	f := newWSPage(t)
	f.press("ws.toggle", 0)
	f.press("ws.menu", 0)
	var rects []image.Rectangle
	for _, id := range []string{"ws.edit", "ws.delete", "ws.row.startup"} {
		c := controlsOf(f.win, id)[0]
		if !c.rect.In(f.win.set.pane) {
			t.Fatalf("%s %v outside %v", id, c.rect, f.win.set.pane)
		}
		for _, o := range rects {
			if c.rect.Overlaps(o) {
				t.Fatalf("%s overlaps", id)
			}
		}
		rects = append(rects, c.rect)
	}
	head := controlsOf(f.win, "ws.toggle")[0].rect
	for _, id := range []string{"ws.run", "ws.menu"} {
		if c := controlsOf(f.win, id)[0]; !c.rect.In(head) {
			t.Fatalf("%s %v outside header %v", id, c.rect, head)
		}
	}
	for _, id := range []string{"ws.edit", "ws.delete"} {
		if controlsOf(f.win, id)[0].rect.Overlaps(head) {
			t.Fatalf("%s overlaps header", id)
		}
	}
}
