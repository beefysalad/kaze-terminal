package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func TestRunningProjectRenders(t *testing.T) {
	dir := os.Getenv("KAZE_RUNNING_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_RUNNING_RENDER_DIR for headless development evidence")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, th := range theme.All() {
		for _, scale := range []int{1, 2} {
			size := image.Pt(620*scale, 620*scale)
			g := pageContext(size.X, size.Y)
			g.Metric = unit.Metric{PxPerDp: float32(scale), PxPerSp: float32(scale)}
			c := newChrome()
			c.setTheme(th)
			paint.Fill(g.Ops, th.UI.Chrome)
			model := sideModel{Groups: []sideGroup{
				{Key: "Stitch", Name: "Stitch", Count: 1, Rows: []sideRow{{ID: 1, Title: "Stitch", Agent: "codex", Branch: "main", Detail: "main", Add: 1, Del: 1}}},
				{Key: "ai-job-search", Name: "ai-job-search", Count: 1, Rows: []sideRow{{ID: 2, Title: "Profile onboarding", Agent: "claude", Branch: "master", Detail: "master", Running: true, Add: 358, Del: 236, Agents: []sideAgent{{Type: "Explore", Task: "Review profile flow", State: subagents.Running}}}}},
				{Key: "kaze-terminal", Name: "kaze-terminal", Count: 1, Rows: []sideRow{{ID: 3, Title: "Verify Kaze multiline shortcuts", Agent: "codex", Branch: "main", Detail: "main", Add: 171, Del: 39}}},
			}}
			c.layoutSidebar(g, model, sideState{shown: 3, settings: true, activeRepo: "kaze-terminal"}, image.Pt(sidebarW*scale, size.Y))
			// Actual shared markers at their shipped size, plus enlarged geometry.
			for i, mk := range []mark{markNone, markNeeds, markDone, markNone} {
				x, y := g.Dp(310), g.Dp(unit.Dp(60+i*86))
				slot := image.Rect(x, y, x+g.Dp(14), y+g.Dp(14))
				dot := centeredMarkBox(slot, g.Dp(7))
				c.stateDot(g, dot, mk, i == 0, false)
				names := []string{"running", "needs you", "done", "idle"}
				call, sz := c.label(g, names[i], 11, th.UI.ActiveFG)
				place(g.Ops, call, image.Pt(x+g.Dp(26), y+(slot.Dy()-sz.Y)/2))
				if i == 0 {
					c.runningMark(g, image.Rect(x+g.Dp(170), y-g.Dp(10), x+g.Dp(226), y+g.Dp(46)))
				}
			}
			win := &window{ui: c}
			win.drawCrumb(g, titleLayout{Crumb: image.Rect(g.Dp(290), g.Dp(440), size.X-g.Dp(12), g.Dp(482)), CrumbShow: crumbParts{Repo: "kaze-terminal", Title: "Working", State: "running"}})
			pill := image.Rect(g.Dp(300), g.Dp(512), g.Dp(430), g.Dp(542))
			paint.FillShape(g.Ops, tint(th.UI.Accent, 0x24), clip.UniformRRect(pill, g.Dp(8)).Op(g.Ops))
			c.runningMark(g, image.Rect(pill.Min.X+g.Dp(10), pill.Min.Y+g.Dp(9), pill.Min.X+g.Dp(22), pill.Min.Y+g.Dp(21)))
			call, sz := c.label(g, "2 agents", 11, th.UI.ActiveFG)
			place(g.Ops, call, image.Pt(pill.Min.X+g.Dp(30), pill.Min.Y+(pill.Dy()-sz.Y)/2))
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
			suffix := "1x"
			if scale == 2 {
				suffix = "2x"
			}
			f, err := os.Create(filepath.Join(dir, th.Key+"-"+suffix+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, img)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
