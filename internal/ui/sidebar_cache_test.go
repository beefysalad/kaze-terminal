package ui

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
	"github.com/patrick-gogo/kaze-terminal/internal/vt"
	"github.com/patrick-gogo/kaze-terminal/internal/workspaces"
)

func TestSidebarCacheUnchangedDrawAllocatesNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo/api"))
	var router input.Router
	sideFrame(t, win, &router)
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 10 {
		draw()
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged sidebar allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged sidebar rebuilt or reshaped")
	}
}

func TestSidebarCacheTracksModelAndDrawingInputs(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	b := openIn(t, win, osPath("/code/repo/web"))
	var router input.Router
	sideFrame(t, win, &router)
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"rename", func() { win.names = map[tabs.TabID]string{a.ID: "Named"} }},
		{"agent", func() { win.agents = map[tabs.TabID]agentState{a.ID: {agent: "codex", running: true}} }},
		{"location", func() { e := win.locs[a.ID]; e.branch = "different"; win.locs[a.ID] = e }},
		{"counts", func() { win.lc.counts = map[tabs.TabID]lineCount{a.ID: {add: 7}} }},
		{"mark", func() { win.alerts.marks = map[tabs.TabID]mark{a.ID: markNeeds} }},
		{"subagent", func() {
			win.subs.Apply(subagents.Event{Kind: subagents.EvStart, Tab: int(a.ID), AgentID: "new", At: time.Now()})
		}},
		{"fold", func() { win.side.folded = map[string]bool{"key-repo": true} }},
		{"filter", func() { win.side.ed.SetText("api") }},
		{"selection", func() { win.mgr.SetActive(a.ID) }},
		{"hover", func() { win.side.st.hover = b.ID }},
		{"settings", func() { win.set.open = true; win.set.shown = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := win.side.cache.draws
			tc.change()
			sideFrame(t, win, &router)
			if win.side.cache.draws <= before {
				t.Fatal("stale sidebar")
			}
		})
	}
}

func TestSidebarCacheRetainsFilteredAndEditorRows(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo/api"))
	win.side.ed.SetText("api")
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 5 {
		draw()
	}
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged filtered sidebar allocated %g times", n)
	}
	win.side.filtering = true
	draw()
	before := win.side.cache.draws
	for range 5 {
		draw()
	}
	if win.side.cache.draws != before {
		t.Fatal("filter editor reshaped rows")
	}
	win.side.filtering = false
	win.openBranchLauncher()
	draw()
	before = win.side.cache.draws
	for range 5 {
		draw()
	}
	if win.side.cache.draws != before {
		t.Fatal("branch editor reshaped rows")
	}
}

func TestSidebarCacheWithWorkspaceRunsAllocatesNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo/api"))
	openMember(t, win, "server")
	win.runs[1].members[0].run = workspaces.TabRun{Line: "serve", ShellReady: true, Wait: workspaces.WaitPort, Port: 3000}
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 10 {
		draw()
	}
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged workspace sidebar allocates %g times", n)
	}
}

func TestSidebarCacheChangesWithDocsRunsSizeThemeAndTitles(t *testing.T) {
	win, st := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	var r input.Router
	sideFrame(t, win, &r)
	changed := func(name string, f func()) {
		t.Helper()
		before := win.side.cache.draws
		f()
		sideFrame(t, win, &r)
		if win.side.cache.draws <= before {
			t.Fatalf("%s left stale rows", name)
		}
	}
	changed("new tab", func() { openIn(t, win, osPath("/code/repo/web")) })
	changed("new document", func() { win.openDoc(a.ID, writeDoc(t, t.TempDir(), "notes.md", "# Notes")) })
	changed("document hover", func() { win.side.st.hoverDoc = win.docs.list[0].id })
	changed("close document", func() { win.closeDoc(win.docs.list[0].id) })
	changed("workspace member", func() { openMember(t, win, "api server"); win.runs[1].cancel = func() {} })
	changed("workspace status", func() { win.runs[1].members[0].run.State = workspaces.Failed })
	changed("workspace fold", func() { win.side.st.runFolded = map[uint64]bool{1: true} })
	changed("tab title", func() {
		a.Session.Lock()
		vt.NewParser(a.Session.Applier()).Feed([]byte("\x1b]0;New agent summary\x07"))
		a.Session.Unlock()
	})
	changed("theme", func() { th, _ := theme.ByKey("washi"); win.ui.setTheme(th) })
	before := win.side.cache.draws
	sizedFrame(t, win, &r, 1000, 700)
	if win.side.cache.draws <= before {
		t.Fatal("resize left stale geometry")
	}
	st.ptys[0].Close()
	deadline := time.Now().Add(time.Second)
	for !a.Exited {
		win.mgr.Tabs()
		if time.Now().After(deadline) {
			t.Fatal("exit timeout")
		}
		time.Sleep(time.Millisecond)
	}
	changed("exit", func() {})
	changed("tab close", func() { win.mgr.Close(a.ID) })
}

func TestSidebarCacheTrayAllocatesNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	b := openIn(t, win, osPath("/code/repo/web"))
	c := openIn(t, win, osPath("/code/other/cli"))
	var router input.Router
	sideFrame(t, win, &router)
	startAgents(win, a.ID, 12)
	for i := range 12 {
		startAgent(win, b.ID, "sb", fmt.Sprintf("b%02d", i))
	}
	for i := range 4 {
		startAgent(win, c.ID, "sc", fmt.Sprintf("c%02d", i))
	}
	win.side.trayOpen = map[tabs.TabID]bool{b.ID: true}
	win.side.trayFolded = map[tabs.TabID]bool{c.ID: true}
	sizedFrame(t, win, &router, 1000, 900)
	if n := len(win.side.hits.agents); n < trayCap+1+trayMax+2 {
		t.Fatalf("tray hits %d: fixture did not draw the trays", n)
	}
	var ops op.Ops
	g := pageContext(1000, 900)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 868)) }
	for range 10 {
		draw()
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged sidebar with trays allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged sidebar with trays rebuilt or reshaped")
	}
}

func TestSidebarCacheTooltipAllocatesNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	k := stepAgent(win, a.ID, "x")
	var router input.Router
	sideFrame(t, win, &router)
	win.side.st.hoverAgent = k
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 10 {
		draw()
	}
	if win.side.hits.tip.Empty() {
		t.Fatal("fixture drew no tooltip")
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged sidebar with a tooltip allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged sidebar with a tooltip rebuilt or reshaped")
	}
}
