package ui

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/theme"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
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

// act21Step is one tool call of the 21-step fixture: its hook summary, the
// transcript input and, when it has one, its result.
type act21Step struct {
	tool, summary string
	input         map[string]any
	out           string
	failed        bool
}

// act21 is the readable fixture of the mockup (docs/design/readable-activity-mockup.html):
// 21 steps and 3 messages. A string is a note.
func act21() []any {
	cmd := func(c, desc string) map[string]any { return map[string]any{"command": c, "description": desc} }
	file := func(p string) map[string]any { return map[string]any{"file_path": "/w/kaze/" + p} }
	edit := func(p, old, nw string) map[string]any {
		return map[string]any{"file_path": "/w/kaze/" + p, "old_string": old, "new_string": nw}
	}
	grep := func(p string) map[string]any { return map[string]any{"pattern": p, "path": "/w/kaze/internal/ui"} }
	return []any{
		"Starting D6: removing the scratch shell. I will merge the feature branch first so the alert card changes are in, then update the tests that still use it.",
		act21Step{tool: "Bash", summary: "cd kaze-lane-d && git merge feat/dock-alerts-links", input: cmd("cd kaze-lane-d && git merge feat/dock-alerts-links", "Merge feat/dock-alerts-links into this lane"),
			out: "Merge made by the ort strategy.\n 21 files changed, 1170 insertions(+), 212 deletions(-)"},
		act21Step{tool: "Grep", summary: "scratch|Scratch", input: grep("scratch|Scratch")},
		act21Step{tool: "Read", summary: "internal/ui/agent.go", input: file("internal/ui/agent.go")},
		act21Step{tool: "Edit", summary: "internal/ui/agent.go", input: edit("internal/ui/agent.go",
			"\t// The scratch shell shares its parent agent state.\n\tif t.Parent != 0 {\n\t\treturn win.agentOf(t.Parent)\n\t}\n\treturn nil",
			"\t// Every session keeps its own agent state.\n\treturn nil"), out: "ok"},
		act21Step{tool: "Read", summary: "internal/ui/window.go", input: file("internal/ui/window.go")},
		act21Step{tool: "Grep", summary: "win.fl", input: grep(`win\.fl\b`)},
		act21Step{tool: "Grep", summary: "paneOwner", input: grep("paneOwner|focusPane")},
		act21Step{tool: "Bash", summary: "sleep 1; cat task.output", input: cmd("sleep 1; cat task.output", "Read the background task output"), out: "ok"},
		act21Step{tool: "ToolSearch", input: map[string]any{"query": "monitor"}},
		act21Step{tool: "Monitor", input: map[string]any{}},
		act21Step{tool: "Bash", summary: "sed -n 110,130p internal/ui/settings_commands_test.go", input: cmd("sed -n 110,130p internal/ui/settings_commands_test.go", "Show the settings test lines"), out: "func TestX(t *testing.T) {"},
		act21Step{tool: "Read", summary: "internal/ui/scratch_gone_test.go", input: file("internal/ui/scratch_gone_test.go")},
		act21Step{tool: "Edit", summary: "internal/ui/scratch_gone_test.go", input: edit("internal/ui/scratch_gone_test.go",
			"func TestOldScratchKey(t *testing.T) {\n\t// Ctrl+Shift+` is now the shell key.\n\twin, st, r, _, _, _, _ := paneWindow(t)\n\tn := len(win.mgr.Tabs())\n\tif got := ptyOf(st, 0).sent(); got == \"\" {\n\t\tt.Fatal(\"the key never reached the shell\")\n\t}\n}",
			"func TestOldScratchKey(t *testing.T) {\n\t// Ctrl+Shift+` (Ctrl+` on macOS) is no kaze action any more.\n\tfor goos, mods := range map[string]key.Modifiers{\"windows\": key.ModCtrl | key.ModShift, \"darwin\": key.ModCtrl} {\n\t\tkm, _ := buildKeymap(goos, nil)\n\t\tif out := routeKey(goos, km, \"`\", mods, term.Modes{}); out.act != actNone {\n\t\t\tt.Fatalf(\"%s: the old scratch key runs %v\", goos, out.act)\n\t\t}\n\t}\n\twin, _, r, _, _, _, _ := paneWindow(t)\n\tn := len(win.mgr.Tabs())\n}"), out: "ok"},
		act21Step{tool: "Bash", summary: "sed -i imports internal/ui/scratch_gone_test.go", input: cmd("sed -i s/x/y/ internal/ui/scratch_gone_test.go", "Fix the test imports"), out: ""},
		act21Step{tool: "Bash", summary: "cd kaze-lane-d && gofmt -l internal cmd; go vet ./... && go test ./internal/ui/", input: cmd("cd kaze-lane-d && gofmt -l internal cmd; go vet ./... && go test ./internal/ui/", "Run go vet and the ui tests"),
			out: "Exit code 1\n--- FAIL: TestOldScratchKeyIsTheShells (0.01s)\n    scratch_gone_test.go:26: windows: the old scratch key runs actScratch\nFAIL\tgithub.com/beefysalad/kaze-terminal/internal/ui\t4.812s", failed: true},
		"RED as expected: the old Ctrl+Shift+backtick key still opens the scratch shell. Next I remove `actScratch` from the keymap and keep an old `scratch_shell` line in config.toml loading with a warning.",
		act21Step{tool: "Edit", summary: "internal/ui/commands.go", input: edit("internal/ui/commands.go", "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl", ""), out: "ok"},
		act21Step{tool: "Edit", summary: "internal/ui/logic.go", input: edit("internal/ui/logic.go", "a\nb\nc\nd\ne\nf\ng\nh\ni", "x\ny"), out: "ok"},
		act21Step{tool: "Edit", summary: "internal/config/config.go", input: edit("internal/config/config.go", "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\no\np\nq\nr\ns\nt", "w\nx\ny\nz"), out: "ok"},
		act21Step{tool: "Bash", summary: "go test ./internal/ui/ ./internal/config/", input: cmd("go test ./internal/ui/ ./internal/config/", "Run the ui and config tests"),
			out: "ok  \tgithub.com/beefysalad/kaze-terminal/internal/ui\t5.102s\nok  \tgithub.com/beefysalad/kaze-terminal/internal/config\t0.412s"},
		act21Step{tool: "Bash", summary: `cd kaze-lane-d && git commit -q -m "refactor(ui): Remove the scratch shell key"`, input: cmd(`cd kaze-lane-d && git commit -m "refactor(ui): Remove the scratch shell key"`, ""),
			out: "[lane/d c3f9a12] refactor(ui): Remove the scratch shell key\n 3 files changed, 6 insertions(+), 41 deletions(-)"},
		"D6 key part done. Now taking `Tab.Parent` out of the sidebar, Ctrl+Tab and session restore.",
		act21Step{tool: "Read", summary: "internal/ui/sidebar.go", input: file("internal/ui/sidebar.go")},
	}
}

// act21Line is one transcript record of the fixture.
func act21Line(kind string, at time.Time, block map[string]any) string {
	b, _ := json.Marshal(map[string]any{"type": kind, "cwd": "/w/kaze", "timestamp": at.UTC().Format(time.RFC3339Nano), "message": map[string]any{"content": []any{block}}})
	return string(b) + "\n"
}

// act21Scene starts agent "x" in tab a's session with the 21-step fixture as
// its transcript and hook steps, and returns its key.
func act21Scene(t testing.TB, win *window, a int) subagents.Key {
	main := filepath.Join(t.TempDir(), "s.jsonl")
	path := subagents.TranscriptPath(main, "x")
	os.MkdirAll(filepath.Dir(path), 0o755)
	var b strings.Builder
	at := time.Date(2026, 11, 4, 9, 7, 0, 0, time.Local)
	for i, p := range act21() {
		at = at.Add(20 * time.Second)
		switch v := p.(type) {
		case string:
			b.WriteString(act21Line("assistant", at, map[string]any{"type": "text", "text": v}))
		case act21Step:
			id := "t" + strconv.Itoa(i)
			b.WriteString(act21Line("assistant", at, map[string]any{"type": "tool_use", "id": id, "name": v.tool, "input": v.input}))
			if v.out != "" || v.tool == "Bash" {
				b.WriteString(act21Line("user", at, map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": v.failed, "content": v.out}))
			}
		}
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
	win.liveMgr.Store(win.mgr)
	send := func(m notify.Msg) { m.Tab, m.Session = a, "s"; win.onNotify(m) }
	send(notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: "general-purpose", Description: "Lane D: dock tabs #133", TaskPrompt: "Build lane D of the dock tabs plan."})
	send(notify.Msg{Kind: notify.KindAgentStart, AgentID: "x", AgentType: "general-purpose", Transcript: main})
	for _, p := range act21() {
		if v, ok := p.(act21Step); ok {
			send(notify.Msg{Kind: notify.KindTool, AgentID: "x", AgentType: "general-purpose", Tool: v.tool, Summary: v.summary})
		}
	}
	return subagents.Key{Session: "s", ID: "x"}
}

// Opt-in development evidence for the readable Activity (issue #134), in the
// docked panel and the board preview, at 1400x860 in Midnight and Sumi.
func TestReadableRenders(t *testing.T) {
	dir := os.Getenv("KAZE_DOCK_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_DOCK_RENDER_DIR for headless visual evidence")
	}
	os.MkdirAll(dir, 0o700)
	size := image.Pt(1400, 860)
	for _, key := range []string{"midnight", "sumi"} {
		for _, state := range []string{"dock", "open", "every", "board"} {
			win, _, r, a, _, _, _ := memberWindow(t)
			win.km, _ = buildKeymap(runtime.GOOS, nil)
			win.binds, _ = resolveBindings(runtime.GOOS, nil)
			th, _ := theme.ByKey(key)
			win.applyTheme(th)
			win.dock.heightDp = 560
			k := act21Scene(t, win, int(a.ID))
			frame := func() { sizedFrame(t, win, r, size.X, size.Y) }
			frame()
			g := pageContext(size.X, size.Y)
			switch state {
			case "board":
				win.openBoard(0)
				win.boardSelect(k)
			case "open":
				win.openDock(a.ID, k)
				s := win.dockOf(a.ID)
				s.open = map[string]bool{"t13": true, "t15": true}
			case "every":
				win.actEvery = true
				win.openDock(a.ID, k)
			default:
				win.openDock(a.ID, k)
			}
			frame()
			frame()
			g.Source = r.Source()
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			writeRender(t, g.Ops, size, filepath.Join(dir, "activity-"+key+"-"+state+".png"))
		}
	}
}

// Opt-in timing for the audit (KAZE_DOCK_BENCH=1): a still 1000x600 frame
// with the 21-step agent tab in Readable, in Every step and with one detail
// open. Run the "every" case on main (this fixture copied) to compare.
func TestReadableFrameTiming(t *testing.T) {
	if os.Getenv("KAZE_DOCK_BENCH") == "" {
		t.Skip("set KAZE_DOCK_BENCH=1 to time panel frames")
	}
	win, _, r, a, _, _, _ := memberWindow(t)
	k := act21Scene(t, win, int(a.ID))
	win.openDock(a.ID, k)
	s := win.dockOf(a.ID)
	measure := func(name string) {
		twoFrames(t, win, r)
		const n = 200
		start := time.Now()
		for range n {
			sideFrame(t, win, r)
		}
		per := time.Since(start) / n
		allocs := testing.AllocsPerRun(50, func() { sideFrame(t, win, r) })
		t.Logf("21 steps, %s: %v per frame, %.0f allocations per frame", name, per, allocs)
	}
	measure("readable")
	s.open = map[string]bool{"t13": true}
	s.follow = false
	s.scroll = 0
	measure("readable, one detail open")
	s.open = nil
	s.follow = true
	win.actEvery = true
	measure("every step")
}

// readableFoldedAllocs is what a still frame of the 21-step scene with every
// step folded allocated on the commit before inline diffs (eb17e90); folded
// must stay within 5% of it.
const readableFoldedAllocs = 74

// diffScene is three edits of Go code with real line numbers: about 30 diff
// rows, drawn at w by h in a tall panel, at least minRows changed rows showing.
// frame draws one still frame of it.
func diffScene(t *testing.T, w, h, minRows int) (frame func()) {
	t.Helper()
	var tr strings.Builder
	for e := range 3 {
		lines := make([]string, 0, 12)
		for i := range 12 {
			switch i % 3 {
			case 0:
				lines = append(lines, " func keep"+strconv.Itoa(i)+"(a int) string { return \"x\" }")
			case 1:
				lines = append(lines, "-	var old = "+strconv.Itoa(i)+" // gone")
			default:
				lines = append(lines, "+	for i := range n { fresh(i, \"y\") } // new")
			}
		}
		tr.WriteString(rdPatchEdit("e"+strconv.Itoa(e), "/w/kaze/f"+strconv.Itoa(e)+".go", 100*(e+1), lines...))
	}
	win, r, _, s, _ := rdDock(t, tr.String())
	win.dock.heightDp = 800
	s.follow = false
	s.scroll = 0
	frame = func() { sizedFrame(t, win, r, w, h) }
	frame()
	frame()
	n := rdCount(win.dock.texts, "+ ") + rdCount(win.dock.texts, "− ")
	if n < minRows {
		t.Fatalf("scene shows only %d changed rows", n)
	}
	return frame
}

func TestOpenDiffsStayWithinAllocations(t *testing.T) {
	frame := diffScene(t, 1400, 1000, 18)
	if got := testing.AllocsPerRun(50, frame); got > 120 {
		t.Fatalf("%.0f allocations on a still frame with edits open, limit 120", got)
	}

	win2, _, r2, a, _, _, _ := memberWindow(t)
	win2.openDock(a.ID, act21Scene(t, win2, int(a.ID)))
	s := win2.dockOf(a.ID)
	twoFrames(t, win2, r2)
	s.open = map[string]bool{}
	for i := range s.read.items {
		if s.read.items[i].hasDetail() {
			s.open[s.read.items[i].id] = false
		}
	}
	twoFrames(t, win2, r2)
	limit := float64(readableFoldedAllocs) * 1.05
	got := testing.AllocsPerRun(50, func() { sideFrame(t, win2, r2) })
	t.Logf("folded: %.0f allocations, limit %.1f", got, limit)
	if got > limit {
		t.Fatalf("%.0f allocations folded, limit %.1f (today's %d +5%%)", got, limit, readableFoldedAllocs)
	}
}

// Opt-in (KAZE_DOCK_BENCH=1): the panel full of open edits at the 1000x600
// frame the limits were set on. The best of five batches, so a busy machine
// does not fail it.
func TestOpenDiffsFrameTime(t *testing.T) {
	if os.Getenv("KAZE_DOCK_BENCH") == "" {
		t.Skip("set KAZE_DOCK_BENCH=1 to time panel frames")
	}
	frame := diffScene(t, 1000, 600, 6)
	best := time.Hour
	const n = 200
	for range 5 {
		start := time.Now()
		for range n {
			frame()
		}
		best = min(best, time.Since(start)/n)
	}
	t.Logf("edits open, still frame: best batch %v, %.0f allocations", best, testing.AllocsPerRun(50, frame))
	if best > 400*time.Microsecond {
		t.Fatalf("%v per still frame, limit 400µs", best)
	}
}
