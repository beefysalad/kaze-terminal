package ui

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// boardScene gives session a dockScene's agent "x" in xState plus a running
// and two finished agents, and session b one waiting, one running and one
// finished agent; it returns x.
func boardScene(t testing.TB, win *window, a, b tabs.TabID, xState string) subagents.Key {
	k := dockScene(t, win, int(a), xState)
	add := func(tab tabs.TabID, session, id, typ, task, step, end string) {
		send := func(m notify.Msg) { m.Tab, m.Session = int(tab), session; win.onNotify(m) }
		send(notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: typ, Description: task})
		send(notify.Msg{Kind: notify.KindAgentStart, AgentID: id, AgentType: typ})
		send(notify.Msg{Kind: notify.KindTool, AgentID: id, AgentType: typ, Tool: "Grep", Summary: step})
		switch end {
		case "wait":
			send(notify.Msg{Kind: notify.KindNeeds, AgentID: id})
		case "done":
			send(notify.Msg{Kind: notify.KindAgentStop, AgentID: id, Report: "Found it in internal/store/tenant.go.\n\nMore detail."})
		}
	}
	add(a, "s", "y", "Plan", "Map the checkout flow", "\"checkout\" internal/cart", "")
	add(a, "s", "d1", "general-purpose", "Draft the refund tests", "refund_test.go", "done")
	add(a, "s", "d2", "Explore", "List the cache keys", "\"cache\" internal", "done")
	add(b, "t", "w", "general-purpose", "Read the tenant scoping", "\"tenantID\" internal/store", "wait")
	add(b, "t", "r", "Explore", "Trace webhook retries", "\"retry\" internal/hooks", "")
	add(b, "t", "d", "Plan", "Check 3D Secure helper", "\"3ds\" internal/pay", "done")
	return k
}

// Opt-in headless captures of the agents board from the actual Gio frame, for
// the audit; no native window, global input or owner configuration.
// Development captures, not product screenshots.
func TestBoardRenders(t *testing.T) {
	dir := os.Getenv("KAZE_BOARD_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_BOARD_RENDER_DIR for headless visual evidence")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"sumi", "midnight"} {
		for _, state := range []string{"all", "done", "search", "empty", "narrow"} {
			win, _, r, a, b, _, _ := memberWindow(t)
			win.km, _ = buildKeymap(runtime.GOOS, nil)
			win.binds, _ = resolveBindings(runtime.GOOS, nil)
			th, ok := theme.ByKey(key)
			if !ok {
				t.Fatalf("no theme %q", key)
			}
			win.applyTheme(th)
			xState := "running"
			if state == "done" {
				xState = "report"
			}
			k := boardScene(t, win, a.ID, b.ID, xState)
			twoFrames(t, win, r)
			size := image.Pt(1400, 860)
			switch state {
			case "all", "narrow":
				win.openBoard(0)
				win.boardSelect(k)
			case "done":
				win.openBoard(0)
				win.board.filter = boardDone
				win.boardSelect(k)
			case "search":
				win.openBoard(a.ID)
				win.board.ed.SetText("split")
				win.boardSelect(k)
			case "empty":
				win.openBoard(0)
				win.board.ed.SetText("no such agent")
			}
			// The test window has no Claude settings, so the board asks for the
			// hooks; only the empty capture keeps that hint.
			win.board.missing = state == "empty"
			if state == "narrow" {
				size = image.Pt(1000, 600)
			}
			var g = pageContext(size.X, size.Y)
			for range 2 { // the second frame has the transcript read and the layout settled
				g = pageContext(size.X, size.Y)
				g.Now = time.Now()
				if err := win.frame(g); err != nil {
					t.Fatal(err)
				}
			}
			writeRender(t, g.Ops, size, filepath.Join(dir, "board-"+key+"-"+state+".png"))
		}
	}
}
