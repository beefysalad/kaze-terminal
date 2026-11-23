package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
	"github.com/beefysalad/kaze-terminal/internal/vt"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
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

func TestSidebarCacheSpacesAndAgentsAllocateNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	b := openIn(t, win, osPath("/code/repo/web"))
	c := openIn(t, win, osPath("/code/other/cli"))
	var router input.Router
	sideFrame(t, win, &router)
	setAgent(win, a.ID, agentState{claude: true, running: true, since: time.Now()})
	setAgent(win, b.ID, agentState{claude: true})
	setAgent(win, c.ID, agentState{agent: "codex"})
	win.alerts.marks = map[tabs.TabID]mark{c.ID: markNeeds}
	sizedFrame(t, win, &router, 1000, 900)
	if n := len(win.side.hits.rows); n != 3 || len(win.side.hits.spaces) != 2 {
		t.Fatalf("%d agent rows and %d spaces: the fixture did not draw both sections", n, len(win.side.hits.spaces))
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
		t.Fatalf("unchanged sidebar with agents allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged sidebar with agents rebuilt or reshaped")
	}
}

func TestSidebarCacheAgeRebuildsOnlyWhenAMinuteTurns(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/api"))
	setAgent(win, a.ID, agentState{claude: true, running: true, since: time.Unix(1000, 0).Add(-30 * time.Second)})
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func(now time.Time) { ops.Reset(); g.Now = now; win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	now := time.Unix(1000, 0)
	for range 3 {
		draw(now)
	}
	draws := win.side.cache.draws
	draw(now.Add(20 * time.Second))
	if win.side.cache.draws != draws {
		t.Fatal("seconds passing rebuilt the sidebar though the age text is the same")
	}
	draw(now.Add(45 * time.Second)) // 75 seconds old: "1m"
	if win.side.cache.draws != draws+1 {
		t.Fatalf("a minute turning gave %d rebuilds, want 1", win.side.cache.draws-draws)
	}
}

func TestSidebarCacheHoverSettlesAndAllocatesNothing(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo/api"))
	var router input.Router
	sideFrame(t, win, &router)
	win.side.st.hoverSpace = "key-repo"
	win.side.st.hoverBtn = btnPlus
	var ops op.Ops
	g := pageContext(1000, 600)
	g.Ops = &ops
	draw := func() { ops.Reset(); win.drawSide(g, image.Rect(0, 42, 260, 568)) }
	for range 10 {
		draw()
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("unchanged sidebar with a hovered space allocates %g times", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("unchanged sidebar with a hovered space rebuilt or reshaped")
	}
}
