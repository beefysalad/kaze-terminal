package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Development evidence only. Fake PTYs, headless Gio, no native input.
func TestWorkerPresentationRenders(t *testing.T) {
	dir := os.Getenv("KAZE_WORKER_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_WORKER_RENDER_DIR for captures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"midnight", "washi"} {
		for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
			for _, fixture := range []string{"attention", "long-parent-done"} {
				for _, mode := range []string{"full", "rail"} {
					win, _ := sideWindow(t, true)
					win.opt.Config.SidebarMode = mode
					th, _ := theme.ByKey(key)
					win.applyTheme(th)
					parent := openIn(t, win, osPath("/code/kaze/app"))
					openIn(t, win, osPath("/code/kaze/web"))
					c := openIn(t, win, "")
					c.Parent, c.Role = parent.ID, "worker"
					d := openIn(t, win, "")
					d.Parent, d.Role = parent.ID, "worker"
					win.names = map[tabs.TabID]string{parent.ID: "orchestrator", c.ID: "review", d.ID: "tests"}
					win.workers = map[tabs.TabID]*worker{c.ID: {parent: parent.ID, agent: "codex", firstMessage: "unconfirmed"}, d.ID: {parent: parent.ID, agent: "claude", firstMessage: "delivered"}}
					win.agents = map[tabs.TabID]agentState{parent.ID: {claude: true, running: true, shellAt: shellSeq(parent), since: t0.Add(-2 * time.Minute)}, d.ID: {claude: true, running: true, shellAt: shellSeq(d), since: t0}}
					if fixture == "long-parent-done" {
						win.names[parent.ID] = "Kaze worker walkthrough with an exceptionally long parent name"
						win.workers[c.ID].firstMessage = "delivered"
						win.alerts.marks = map[tabs.TabID]mark{c.ID: markDone}
					}
					win.mgr.SetActive(parent.ID)
					var router input.Router
					for _, popover := range []bool{false, true} {
						win.stat.workers.open = popover
						if mode == "rail" {
							win.side.st.hover = c.ID
						}
						for range 3 {
							g := pageContext(size.X, size.Y)
							g.Now = t0
							g.Source = router.Source()
							if err := win.frame(g); err != nil {
								t.Fatal(err)
							}
							router.Frame(g.Ops)
							name := fmt.Sprintf("workers-%s-%s-%s-%dx%d-popover-%t.png", fixture, mode, key, size.X, size.Y, popover)
							if _, err := writeShot(t, g.Ops, size, filepath.Join(dir, name)); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			}
		}
	}
}
