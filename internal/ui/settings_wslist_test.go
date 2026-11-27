package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
	"testing"
)

func TestWSListEmptyAndSingle(t *testing.T) {
	w := &window{}
	if !buildWSList(w, wsAll, "", "").Empty {
		t.Fatal("empty")
	}
	w.wsp.doc.Workspaces = []workspaces.Workspace{{ID: "a", Name: "日本語", Tabs: []workspaces.Tab{{Title: "API", Commands: []string{" ", "echo a", "", "echo b"}}}}}
	m := buildWSList(w, wsAll, "api", "a")
	if len(m.Rows) != 1 || m.ShowSearch || m.Rows[0].Tabs[0].More != "+1 more" || !m.Rows[0].Open {
		t.Fatal(m)
	}
}
