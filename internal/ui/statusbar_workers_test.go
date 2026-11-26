package ui

import (
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"image"
	"testing"
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
