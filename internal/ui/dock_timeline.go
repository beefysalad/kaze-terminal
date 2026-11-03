package ui

import (
	"strconv"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

type dockItemKind uint8

const (
	itemDropped dockItemKind = iota
	itemStep
	itemNote
	itemWaiting
	itemLive
	itemFinished
)

// dockItem is one row of a panel's Activity tab.
type dockItem struct {
	kind         dockItemKind
	at           time.Time
	tool, target string
	text         string
	msg          int // the note's index in the transcript's Messages, else -1
}

// dockTimeline is a's Activity: its tool steps with the agent's own messages
// between them in time order, then the waiting box, the live row or the
// finish row. Tool-only transcript messages are left out: the steps show them.
// A message with no time sorts with the one before it in the file.
func dockTimeline(a *subagents.Agent, tr *subagents.Transcript) []dockItem {
	var out []dockItem
	if a.Dropped > 0 {
		out = append(out, dockItem{kind: itemDropped, msg: -1, text: "… " + strconv.Itoa(a.Dropped) + " earlier steps"})
	}
	var notes []dockItem
	if tr != nil && tr.Available {
		if tr.Dropped {
			out = append(out, dockItem{kind: itemDropped, msg: -1, text: "… earlier messages hidden"})
		}
		var last time.Time
		for i, m := range tr.Messages {
			if m.Text == "" {
				if !m.At.IsZero() {
					last = m.At
				}
				continue
			}
			at := m.At
			if at.IsZero() {
				at = last
			}
			last = at
			notes = append(notes, dockItem{kind: itemNote, at: at, text: m.Text, msg: i})
		}
	}
	steps := a.Steps
	for len(steps) > 0 || len(notes) > 0 {
		if len(notes) == 0 || (len(steps) > 0 && steps[0].At.Before(notes[0].at)) {
			s := steps[0]
			steps = steps[1:]
			out = append(out, dockItem{kind: itemStep, at: s.At, tool: s.Tool, target: s.Summary, msg: -1})
			continue
		}
		out = append(out, notes[0])
		notes = notes[1:]
	}
	switch a.State {
	case subagents.Waiting:
		text := "Waiting for your approval in the session above."
		if n := len(a.Steps); n > 0 && a.Steps[n-1].Tool == "Bash" && a.Steps[n-1].Summary != "" {
			text = "Wants to run " + a.Steps[n-1].Summary + ". Approve it in the session above."
		}
		out = append(out, dockItem{kind: itemWaiting, text: text, msg: -1})
	case subagents.Done:
		out = append(out, dockItem{kind: itemFinished, text: "Finished in " + minutes(a.Ended.Sub(a.Started)), msg: -1})
	default:
		out = append(out, dockItem{kind: itemLive, text: "working", msg: -1})
	}
	return out
}
