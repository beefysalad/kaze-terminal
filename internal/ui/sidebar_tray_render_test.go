package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/op"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// trayRenderAgents is n tray lines in the mockup's mix: one waiting, the rest running.
func trayRenderAgents(n int) []sideAgent {
	tasks := []string{"Find the auth middleware", "Map the checkout flow", "Read the tenant scoping", "Draft the refund tests",
		"Trace webhook retries", "Check 3D Secure helper", "List the cache keys", "Audit the session store"}
	types := []string{"Explore", "general-purpose", "Plan", "Explore"}
	out := make([]sideAgent, n)
	for i := range out {
		out[i] = sideAgent{Key: subagents.Key{Session: "s", ID: fmt.Sprintf("a%d", i)}, Type: types[i%len(types)],
			Task: tasks[i%len(tasks)], Action: "Grep \"tenantID\" internal/store", Elapsed: fmt.Sprintf("%dm", 2+i), State: subagents.Running}
	}
	if n > 0 {
		out[0].State = subagents.Waiting
	}
	return out
}

// Opt-in actual Gio GPU render of the agent tray (no project rail); creates no
// native window and injects no input. Development captures, not product screenshots.
func TestSidebarTrayRenders(t *testing.T) {
	dir := os.Getenv("KAZE_TRAY_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_TRAY_RENDER_DIR for headless visual evidence")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	// Columns: default (3 lines and the chip), expanded (8 lines and the footer),
	// folded, and the hovered line's tooltip. The active project shows only its bold name.
	states := []string{"default", "expanded", "folded", "tooltip"}
	for _, key := range []string{"sumi", "washi", "midnight"} {
		th, ok := theme.ByKey(key)
		if !ok {
			t.Fatalf("no theme %q", key)
		}
		c := newChrome()
		c.setTheme(th)
		size := image.Pt(len(states)*sidebarW, 640)
		g := pageContext(size.X, size.Y)
		for col, state := range states {
			tray := sideRow{ID: 1, Title: "Sidebar agent tray", Agent: "claude", Branch: "feat/sidebar-tray-emoji", Add: 212, Del: 40,
				Running: true, Mark: markNeeds, AgentCount: 18, TrayTotal: 20, TrayWaiting: 1}
			st := sideState{shown: 1, activeRepo: "repo"}
			switch state {
			case "expanded":
				tray.Agents, tray.TrayExpanded = trayRenderAgents(trayMax), true
				tray.Hidden = subagents.TrayCounts{Running: 10, Done: 2}
			case "folded":
				tray.TrayFolded = true
			default:
				tray.Agents = trayRenderAgents(trayCap)
				tray.Hidden = subagents.TrayCounts{Running: 15, Done: 2}
			}
			if state == "tooltip" { // also a hidden agent that waits: the chip keeps "1 needs you"
				st.hoverAgent = tray.Agents[1].Key
				tray.Hidden, tray.TrayWaiting = subagents.TrayCounts{Waiting: 1, Running: 14, Done: 2}, 2
			}
			model := sideModel{Groups: []sideGroup{
				{Key: "repo", Name: "kaze-terminal", Count: 2, Waiting: 1, Rows: []sideRow{
					tray,
					{ID: 2, Title: "codex", Agent: "codex", Branch: "main", Add: 3},
				}},
				{Key: "todo", Name: "todo-api", Count: 1, Rows: []sideRow{{ID: 3, Title: "Add login rate limit", Agent: "claude", Branch: "feature/auth", Mark: markDone, Add: 7}}},
			}}
			off := op.Offset(image.Pt(col*sidebarW, 0)).Push(g.Ops)
			c.layoutSidebar(g, model, st, image.Pt(sidebarW, size.Y))
			off.Pop()
		}
		writeRender(t, g.Ops, size, filepath.Join(dir, "tray-"+key+".png"))
	}
}

// writeRender draws ops in a headless GPU window of size and saves it as a PNG at path.
func writeRender(t *testing.T, ops *op.Ops, size image.Point, path string) {
	t.Helper()
	gpu, e := headless.NewWindow(size.X, size.Y)
	if e != nil {
		t.Fatal(e)
	}
	defer gpu.Release()
	if e = gpu.Frame(ops); e != nil {
		t.Fatal(e)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if e = gpu.Screenshot(img); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
}
