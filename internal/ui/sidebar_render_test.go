package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in actual Gio GPU render; creates no native window and injects no input.
func TestSidebarRedesignRenders(t *testing.T) {
	dir := os.Getenv("KAZE_SIDEBAR_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_SIDEBAR_RENDER_DIR for headless visual evidence")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		size := image.Pt(sidebarW, 650)
		gpu, e := headless.NewWindow(size.X, size.Y)
		if e != nil {
			t.Fatal(e)
		}
		g := pageContext(size.X, size.Y)
		model := sideModel{Groups: []sideGroup{
			{Key: "repo", Name: "kaze-terminal", Count: 2, Rows: []sideRow{
				{ID: 1, Title: "Read Markdown Reader Plan", Agent: "claude", Branch: "subagent-live-messages", Detail: "subagent-live-messages", Add: 47, Del: 3, Running: true, AgentCount: 2, Agents: []sideAgent{{Type: "Explore", Task: "Find the sidebar model", Elapsed: "<1m", State: subagents.Running}, {Type: "Plan", Task: "Draft the row layout", Elapsed: "<1m", State: subagents.Running}}},
				{ID: 2, Title: "codex", Agent: "codex", Branch: "subagent-live-messages", Add: 47, Del: 3},
			}},
			{Key: "todo", Name: "todo-api", Count: 1, Waiting: 1, Rows: []sideRow{{ID: 3, Title: "Add login rate limit", Agent: "claude", Branch: "feature/auth", Detail: "⑂ todo-api-auth · feature/auth", Mark: markNeeds, Add: 7}}},
		}}
		c.layoutSidebar(g, model, sideState{shown: 1}, image.Pt(sidebarW, size.Y))
		if e = gpu.Frame(g.Ops); e != nil {
			t.Fatal(e)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		if e = gpu.Screenshot(img); e != nil {
			t.Fatal(e)
		}
		gpu.Release()
		f, e := os.Create(filepath.Join(dir, th.Key+".png"))
		if e != nil {
			t.Fatal(e)
		}
		e = png.Encode(f, img)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
}
