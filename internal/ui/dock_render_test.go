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
