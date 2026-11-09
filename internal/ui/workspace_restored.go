package ui

import (
	"context"
	"strconv"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/sessionstate"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

func planRun(p wsPlan, now time.Time) workspaces.TabRun {
	return workspaces.TabRun{Line: p.line, Wait: p.tab.Wait, Port: p.tab.Port, Started: now}
}

// restoredHasCommands reports whether a restored member has commands to wait
// for. A tab the recipe no longer lists counts: nothing proves it needs none.
func restoredHasCommands(doc workspaces.Document, e sessionstate.Entry) bool {
	ws, ok := doc.ByID(e.Workspace)
	if !ok {
		return true
	}
	for _, t := range ws.Tabs {
		if t.Title == e.FixedTitle {
			return len(t.Commands) > 0
		}
	}
	return true
}

// waitingForRun lists run's restored members whose shells are still open.
func (win *window) waitingForRun(run *wsRun) []*wsMember {
	var out []*wsMember
	for _, m := range run.members {
		if t := win.mgr.Lookup(m.tab); m.restored && t != nil && !t.Exited {
			out = append(out, m)
		}
	}
	return out
}

// startRestored sends the saved commands into run's restored tabs, the one way
// they start after a restart. It reports whether there was anything to start.
func (win *window) startRestored(ws workspaces.Workspace, run *wsRun) bool {
	waiting := win.waitingForRun(run)
	if len(waiting) == 0 {
		return false
	}
	plan, err := win.preflight(ws)
	if err != nil {
		win.wsNotice(ws.Name + ": " + err.Error())
		return true
	}
	// A settled run cancelled its context; waits for ports and Docker need a live one.
	run.cancel()
	run.ctx, run.cancel = context.WithCancel(context.Background())
	run.probes, run.summarized = nil, false
	now, missing := win.wsNow(), 0
	for _, m := range waiting {
		i := 0
		for i < len(plan) && plan[i].tab.Title != m.title {
			i++
		}
		if i == len(plan) {
			missing++
			continue
		}
		p := plan[i]
		plan = append(plan[:i:i], plan[i+1:]...)
		m.restored, m.cmds, m.run = false, p.tab.Commands, planRun(p, now)
		m.run.Event(workspaces.StartedEvent{})
		if !m.run.Settled() {
			win.mgr.Watch(m.tab, true) // so a prompt or dependency wakes the window
		}
	}
	win.startProbes(run)
	if missing > 0 {
		win.wsNotice(ws.Name + ": " + strconv.Itoa(missing) + " restored tab(s) are no longer in this workspace")
	}
	if win.w != nil {
		win.w.Invalidate()
	}
	return true
}
