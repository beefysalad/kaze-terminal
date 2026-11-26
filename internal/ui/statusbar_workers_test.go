package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/font"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestWorkerCounterAndPalette(t *testing.T) {
	win, _ := workerWindow(t)
	if workerCounter(0) != "" || workerCounter(1) != "1 worker" || workerCounter(2) != "2 workers" {
		t.Fatal("counter text")
	}
	if shown(statusAt(win.statusModel(t0), 1200), grpWorkers) {
		t.Fatal("empty counter visible")
	}
	if hasWorkerAction(win.palActions(), "Stop workers of this tab") {
		t.Fatal("parent action without workers")
	}
	spawnPresentationWorker(t, win)
	if win.statusModel(t0).Workers != 1 || !shown(statusAt(win.statusModel(t0), 1200), grpWorkers) {
		t.Fatal("counter hidden")
	}
	if !hasWorkerAction(win.palActions(), "Stop all workers") || !hasWorkerAction(win.palActions(), "Stop workers of this tab") {
		t.Fatal("missing palette actions")
	}
	win.statusClick(grpWorkers)
	if !win.stat.workers.open {
		t.Fatal("counter did not open popover")
	}
	win.stopWorkerTabs(0)
	if len(win.workers) != 0 || win.mgr.Lookup(2) != nil || win.mgr.Lookup(1) == nil {
		t.Fatal("stop all must close only workers")
	}
	if win.statusModel(t0).Workers != 0 {
		t.Fatal("counter stayed visible")
	}
}

func hasWorkerAction(as []palAction, name string) bool {
	for _, a := range as {
		if a.Name == name {
			return true
		}
	}
	return false
}
func spawnPresentationWorker(t *testing.T, win *window) {
	t.Helper()
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
	if !r.OK {
		t.Fatal(r)
	}
}

func TestWorkerPaletteUpdatesWhileOpen(t *testing.T) {
	win, _ := workerWindow(t)
	spawnPresentationWorker(t, win)
	win.palette.actions = win.palActions()
	win.stopWorkerTabs(0)
	if hasWorkerAction(win.palInput().Actions, "Stop workers of this tab") {
		t.Fatal("stale parent action after last worker closed")
	}
}

func TestWorkerStopOneParentAndPopoverActions(t *testing.T) {
	win, _ := workerWindow(t)
	spawnPresentationWorker(t, win)
	parent := openIn(t, win, "")
	child := openIn(t, win, "")
	child.Role, child.Parent = "worker", parent.ID
	win.workers[child.ID] = &worker{parent: parent.ID, agent: "codex", firstMessage: "delivered"}
	win.mgr.SetActive(1)
	win.palAct(pageContext(1200, 800), palItem{Kind: palKaze, Key: "act:stop_tab_workers"}, false)
	if win.mgr.Lookup(2) != nil || win.mgr.Lookup(child.ID) == nil || win.mgr.Lookup(parent.ID) == nil {
		t.Fatal("stop parent closed unrelated tabs")
	}
	win.statusClick(grpWorkers)
	g := pageContext(1200, 800)
	win.drawStatus(g, 768, t0)
	win.drawWorkerPopover(g)
	if len(win.stat.workers.rows) != 1 {
		t.Fatal("worker popover list")
	}
	win.workerPopoverClick(win.stat.workers.rows[0].rect.Min.Add(image.Pt(2, 2)))
	if activeID(win) != child.ID || win.stat.workers.open {
		t.Fatal("worker row did not select and dismiss")
	}
	win.statusClick(grpWorkers)
	win.drawWorkerPopover(g)
	win.workerPopoverClick(win.stat.workers.stop.Min.Add(image.Pt(2, 2)))
	if len(win.workers) != 0 || win.mgr.Lookup(parent.ID) == nil || win.mgr.Lookup(1) == nil {
		t.Fatal("popover stop all")
	}
}

func TestWorkerPopoverNames(t *testing.T) {
	for _, tc := range []struct{ label, name, title, want string }{
		{"name", "review parser", "terminal title", "review parser"},
		{"title", "", "terminal title", "terminal title"},
		{"id", "", "", "t2"},
		{"long", strings.Repeat("review ", 80), "", ""},
	} {
		t.Run(tc.label, func(t *testing.T) {
			win, _ := workerWindow(t)
			spawnPresentationWorker(t, win)
			win.names = map[tabs.TabID]string{2: tc.name}
			shellTitle(win.mgr.Lookup(2), tc.title)
			win.stat.workers.open = true
			g := pageContext(560, 640)
			win.drawStatus(g, 608, t0)
			win.drawWorkerPopover(g)
			row := win.stat.workers.rows[0]
			if tc.label == "long" {
				if row.name == tc.name || !strings.HasSuffix(row.name, "…") {
					t.Fatalf("long name was not ellipsized: %q", row.name)
				}
			} else if row.name != tc.want {
				t.Fatalf("popover name = %q, want %q", row.name, tc.want)
			}
			_, sz := win.ui.labelW(g, row.name, 12, win.ui.th.ToastFG, font.Bold)
			if sz.X > row.rect.Dx() {
				t.Fatalf("name width %d exceeds row %d", sz.X, row.rect.Dx())
			}
		})
	}
}

func TestWorkerPopoverStillFrameAllocations(t *testing.T) {
	win, _ := workerWindow(t)
	spawnPresentationWorker(t, win)
	win.names = map[tabs.TabID]string{2: "review"}
	win.stat.workers.open = true
	g := pageContext(560, 640)
	win.drawStatus(g, 608, t0)
	draw := func() {
		g.Ops.Reset()
		win.drawWorkerPopover(g)
	}
	for range 3 {
		draw()
	}
	// The previous named-worker popover also used two allocations per draw.
	if n := testing.AllocsPerRun(20, draw); n > 2 {
		t.Fatalf("worker popover allocated %g, want at most 2", n)
	}
}
