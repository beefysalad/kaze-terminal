package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
	"github.com/patrick-gogo/kaze-terminal/internal/workspaces"
)

const dockSampleReport = "## Summary\n\n**No existing binding** for split panes.\n\n- Cmd+D and Cmd+Shift+D are free on macOS.\n- `panes.go` keeps panels per session.\n\n1. **Existing panes ignored.** See `docs/notes.md`.\n2. Tests pass.\n\n| Key | Action |\n|---|---|\n| Cmd+D | split right |\n| Cmd+Shift+D | split down |\n\n```go\nfunc split() {}\n```\n"

// dockScene puts tab a's session in one of the panel's states: "running",
// "waiting" or "report".
func dockScene(t testing.TB, win *window, a int, state string) subagents.Key {
	main := filepath.Join(t.TempDir(), "s.jsonl")
	path := subagents.TranscriptPath(main, "x")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(fixtureAssistant("Looking at **panes.go** first, then the sidebar.")), 0o644)
	win.liveMgr.Store(win.mgr)
	send := func(m notify.Msg) { m.Tab, m.Session = a, "s"; win.onNotify(m) }
	send(notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: "Explore", Description: "Split pane keys", TaskPrompt: "Find **existing** split bindings.\n\nReport file:line."})
	send(notify.Msg{Kind: notify.KindAgentStart, AgentID: "x", AgentType: "Explore", Transcript: main})
	for _, s := range [][2]string{{"Read", "internal/ui/panes.go"}, {"Grep", "split"}, {"Bash", "go test ./internal/mdoc/"}} {
		send(notify.Msg{Kind: notify.KindTool, AgentID: "x", AgentType: "Explore", Tool: s[0], Summary: s[1]})
	}
	switch state {
	case "waiting":
		send(notify.Msg{Kind: notify.KindNeeds, AgentID: "x"})
	case "report":
		send(notify.Msg{Kind: notify.KindAgentStop, AgentID: "x", Report: dockSampleReport})
	}
	return subagents.Key{Session: "s", ID: "x"}
}

// Opt-in development evidence from the actual Gio frame; no native window,
// global input or owner configuration.
func TestDockRenders(t *testing.T) {
	dir := os.Getenv("KAZE_DOCK_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_DOCK_RENDER_DIR for headless visual evidence")
	}
	os.MkdirAll(dir, 0o700)
	for _, key := range []string{"midnight", "washi"} {
		for _, state := range []string{"running", "waiting", "report", "tabs"} {
			win, _, r, a, _, _, _ := memberWindow(t)
			win.km, _ = buildKeymap(runtime.GOOS, nil)
			win.binds, _ = resolveBindings(runtime.GOOS, nil)
			th, _ := theme.ByKey(key)
			win.applyTheme(th)
			k := dockScene(t, win, int(a.ID), state)
			twoFrames(t, win, r)
			if state == "tabs" {
				// A second agent's tab beside the shown one.
				win.onNotify(notify.Msg{Tab: int(a.ID), Session: "s", Kind: notify.KindTool, Tool: "Agent", AgentType: "Plan", Description: "Dock tabs plan"})
				win.onNotify(notify.Msg{Tab: int(a.ID), Session: "s", Kind: notify.KindAgentStart, AgentID: "y", AgentType: "Plan"})
				win.openDock(a.ID, subagents.Key{Session: "s", ID: "y"})
			}
			win.showAgent(a.ID, k, false)
			twoFrames(t, win, r)
			size := image.Pt(1000, 600)
			g := pageContext(size.X, size.Y)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			gpu, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Fatal(err)
			}
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			f, _ := os.Create(filepath.Join(dir, key+"-"+state+".png"))
			png.Encode(f, img)
			f.Close()
		}
	}
}

// longReport is about n bytes of mixed markdown.
func longReport(n int) string {
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		b.WriteString(dockSampleReport)
		b.WriteString("Paragraph with `code`, **bold**, a docs/notes.md link and enough words to wrap across the panel width a few times over.\n\n")
	}
	return b.String()
}

// A long report is parsed once; scrolling it parses nothing more.
func TestDockLongReportScrollNoReparse(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := dockScene(t, win, int(a.ID), "running")
	win.onNotify(notify.Msg{Tab: int(a.ID), Session: "s", Kind: notify.KindAgentStop, AgentID: "x", Report: longReport(100 << 10)})
	twoFrames(t, win, r)
	win.subs.Get(k).Report = longReport(100 << 10) // past the 24 KB hook cap: the worst case
	win.showAgent(a.ID, k, false)
	twoFrames(t, win, r)
	s := win.dockOf(a.ID)
	if s.tab != dockReport || win.dock.hits.scrollMax == 0 {
		t.Fatalf("report not shown scrollable: tab %d max %d", s.tab, win.dock.hits.scrollMax)
	}
	parses := s.cache.parses
	for i := range 10 {
		s.scroll = i * 500
		sideFrame(t, win, r)
	}
	if s.cache.parses != parses {
		t.Fatalf("scrolling parsed %d more times", s.cache.parses-parses)
	}
}

// Opt-in timing for the audit note: KAZE_DOCK_BENCH=1.
func TestDockFrameTiming(t *testing.T) {
	if os.Getenv("KAZE_DOCK_BENCH") == "" {
		t.Skip("set KAZE_DOCK_BENCH=1 to time panel frames")
	}
	win, _, r, a, _, _, _ := memberWindow(t)
	k := dockScene(t, win, int(a.ID), "running")
	win.onNotify(notify.Msg{Tab: int(a.ID), Session: "s", Kind: notify.KindAgentStop, AgentID: "x", Report: longReport(100 << 10)})
	sideFrame(t, win, r)
	win.subs.Get(k).Report = longReport(100 << 10) // past the 24 KB hook cap: the worst case
	win.showAgent(a.ID, k, false)
	start := time.Now()
	sideFrame(t, win, r) // the first frame parses and lays the report out
	first := time.Since(start)
	s := win.dockOf(a.ID)
	const n = 100
	start = time.Now()
	for i := range n {
		s.scroll = (i * 397) % max(win.dock.hits.scrollMax, 1)
		sideFrame(t, win, r)
	}
	scrolled := time.Since(start) / n
	start = time.Now()
	for range n {
		sideFrame(t, win, r)
	}
	idle := time.Since(start) / n
	win.closeDock(a.ID)
	sideFrame(t, win, r)
	start = time.Now()
	for range n {
		sideFrame(t, win, r)
	}
	closed := time.Since(start) / n
	t.Logf("100 KB report: first frame %v, scrolled frame %v, still frame %v, frame with panel closed %v, parses %d", first, scrolled, idle, closed, s.cache.parses)
}

// Opt-in development evidence for the dock tabs (issue #133): a member tab
// with the keyboard beside an agent tab, an agent tab, six tabs with cut
// names, an exited member and a hidden panel, at 1400x860 in Sumi and Midnight.
func TestDockTabsRender(t *testing.T) {
	dir := os.Getenv("KAZE_DOCK_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_DOCK_RENDER_DIR for headless visual evidence")
	}
	os.MkdirAll(dir, 0o700)
	size := image.Pt(1400, 860)
	log := "$ uvicorn app.main:app --reload --port 8000\r\nINFO:     Uvicorn running on http://127.0.0.1:8000\r\nINFO:     Started reloader process [21844]\r\nINFO:     Application startup complete.\r\nINFO:     127.0.0.1 - \"GET /api/v1/orders?page=2 HTTP/1.1\" 200\r\nWARN:     slow query: orders.list 412ms\r\n"
	for _, key := range []string{"sumi", "midnight"} {
		for _, state := range []string{"member", "agent", "six", "exited", "hidden"} {
			win, st, r, a, _, m1, m2 := memberWindow(t)
			win.km, _ = buildKeymap(runtime.GOOS, nil)
			win.binds, _ = resolveBindings(runtime.GOOS, nil)
			th, _ := theme.ByKey(key)
			win.applyTheme(th)
			emitTo(t, st, 2, m1, log, func() bool { return textShown(m1, 2, 'u')() })
			k := dockScene(t, win, int(a.ID), "running")
			frame := func() { sizedFrame(t, win, r, size.X, size.Y) }
			frame()
			switch state {
			case "member":
				win.openDock(a.ID, k)
				win.dockMember(m1.ID, false)
			case "agent":
				win.dockMember(m1.ID, false)
				win.showAgent(a.ID, k, false)
			case "six":
				for _, name := range []string{"worker", "scheduler", "storybook"} {
					win.openDockMember(a.ID, openMember(t, win, name).ID)
					win.mgr.SetActive(a.ID) // a new tab is made active; the session stays in front
				}
				win.openDockMember(a.ID, m2.ID)
				win.openDock(a.ID, k)
				win.dockMember(m1.ID, false)
			case "exited":
				win.dockMember(m1.ID, false)
				ptyOf(st, 2).Close()
				deadline := time.Now().Add(3 * time.Second)
				for done, _ := m1.Session.Exited(); !done && time.Now().Before(deadline); done, _ = m1.Session.Exited() {
					time.Sleep(time.Millisecond)
				}
			case "hidden":
				win.openDock(a.ID, k)
				win.dockMember(m1.ID, false)
				frame()
				win.closeDock(a.ID)
			}
			launchMembers(win)
			frame()
			frame()
			g := pageContext(size.X, size.Y)
			g.Source = r.Source()
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			writeRender(t, g.Ops, size, filepath.Join(dir, "dock-"+key+"-"+state+".png"))
		}
	}
}

// launchMembers gives every fixture member a launched run, so the sidebar
// and the status bar read "Running" and count the servers up, as a real run.
func launchMembers(win *window) {
	lines := map[string]string{
		"api": "uvicorn app.main:app --reload --port 8000", "web": "npm run dev",
		"worker": "celery -A app worker", "scheduler": "celery -A app beat", "storybook": "npm run storybook",
	}
	for _, r := range win.runs {
		for _, m := range r.members {
			m.run = workspaces.TabRun{Line: lines[m.title], State: workspaces.Launched, ShellReady: true}
		}
	}
}

// Opt-in timing for the audit note (KAZE_DOCK_BENCH=1): a still 1000x600
// frame with no panel, with an agent tab and with a member tab.
func TestDockTabsFrameTiming(t *testing.T) {
	if os.Getenv("KAZE_DOCK_BENCH") == "" {
		t.Skip("set KAZE_DOCK_BENCH=1 to time panel frames")
	}
	win, st, r, a, _, m1, _ := memberWindow(t)
	emitTo(t, st, 2, m1, "$ npm run dev\r\nready on http://localhost:3000\r\n", func() bool { return textShown(m1, 2, 'n')() })
	k := dockScene(t, win, int(a.ID), "running")
	measure := func(name string) {
		twoFrames(t, win, r)
		const n = 200
		start := time.Now()
		for range n {
			sideFrame(t, win, r)
		}
		per := time.Since(start) / n
		allocs := testing.AllocsPerRun(50, func() { sideFrame(t, win, r) })
		t.Logf("%s: %v per frame, %.0f allocations per frame", name, per, allocs)
	}
	measure("no panel")
	win.openDock(a.ID, k)
	measure("agent tab")
	win.dockMember(m1.ID, false)
	measure("member tab")
}
