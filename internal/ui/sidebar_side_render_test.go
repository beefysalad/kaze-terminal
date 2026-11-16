package ui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in actual Gio GPU render of whole window frames (title bar, sidebar or
// rail, terminal area) for the sidebar on each side in full and rail modes.
// Creates no native window and injects no input. Development captures only.
func TestSidebarSideRenders(t *testing.T) {
	dir := os.Getenv("KAZE_SIDE_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_SIDE_RENDER_DIR for headless visual evidence")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	size := image.Pt(1100, 540)
	for _, key := range []string{"midnight", "washi"} {
		th, ok := theme.ByKey(key)
		if !ok {
			t.Fatalf("no theme %q", key)
		}
		for _, side := range []string{"left", "right"} {
			for _, mode := range []string{"full", "rail"} {
				win, _ := sideWindow(t, true)
				win.opt.Config.Theme, win.opt.Config.SidebarSide, win.opt.Config.SidebarMode = key, side, mode
				win.applyTheme(th)
				a := openIn(t, win, osPath("/code/kaze-terminal/app"))
				b := openIn(t, win, osPath("/code/kaze-terminal/web"))
				c := openIn(t, win, osPath("/code/todo-api/api"))
				openIn(t, win, "") // a plain shell, for a tile with the terminal icon
				openMember(t, win, "api")
				openMember(t, win, "web")
				win.agents = map[tabs.TabID]agentState{}
				win.agents[a.ID] = agentState{agent: "claude", running: true, shellAt: shellSeq(a)}
				win.agents[b.ID] = agentState{agent: "claude", shellAt: shellSeq(b)}
				win.agents[c.ID] = agentState{agent: "codex", shellAt: shellSeq(c)}
				win.alerts.marks = map[tabs.TabID]mark{b.ID: markNeeds, c.ID: markDone}
				for i := range 3 {
					startAgent(win, a.ID, "s", string(rune('a'+i)))
				}
				stepAgent(win, a.ID, "a")
				win.mgr.SetActive(a.ID)
				var r input.Router
				sizedFrame(t, win, &r, size.X, size.Y)
				sizedFrame(t, win, &r, size.X, size.Y)
				shot := func(name string) {
					gtx := pageContext(size.X, size.Y)
					gtx.Source = r.Source()
					gtx.Now = time.Unix(1000, 0)
					if err := win.frame(gtx); err != nil {
						t.Fatal(err)
					}
					r.Frame(gtx.Ops)
					writeRender(t, gtx.Ops, size, filepath.Join(dir, name+".png"))
				}
				shot(key + "-" + side + "-" + mode)
				if mode == "rail" { // the hovered tile's tooltip, beside the rail toward the terminal
					sideMove(win, &r, pointer.Move, centre(rowHit(t, win, b.ID).rect))
					sizedFrame(t, win, &r, size.X, size.Y)
					shot(key + "-" + side + "-rail-tooltip")
				}
			}
		}
	}
}
