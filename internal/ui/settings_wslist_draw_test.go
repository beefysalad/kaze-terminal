package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
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
