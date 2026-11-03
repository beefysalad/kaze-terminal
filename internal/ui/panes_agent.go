package ui

import (
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// agentTranscriptPoll is how often a shown panel rereads its transcript.
const agentTranscriptPoll = time.Second

// agentTranscriptPath is the JSONL file subagent a writes, "" for Codex
// helpers and agents whose session's transcript kaze never learned.
func (win *window) agentTranscriptPath(a *subagents.Agent) string {
	if a.Engine == "codex" {
		return ""
	}
	main := a.Transcript
	if main == "" {
		state := win.agentOf(tabs.TabID(a.Tab))
		if state.agentName() == "codex" || state.session != a.Key.Session {
			return ""
		}
		main = state.transcript
	}
	return subagents.TranscriptPath(main, a.Key.ID)
}

// trimBlock drops blank lines around s and trailing spaces, but keeps the
// first line's indent: a prompt that opens with a code block stays indented.
func trimBlock(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	for {
		line, rest, ok := strings.Cut(s, "\n")
		if !ok || strings.TrimSpace(line) != "" {
			return s
		}
		s = rest
	}
}

// showAgent opens subagent k's panel under session tab, showing that session
// first (a floating workspace member becomes the main view, as Expand does).
// From the sidebar, toggle closes the panel when it already shows k.
func (win *window) showAgent(tab tabs.TabID, k subagents.Key, toggle bool) {
	if win.subs.Get(k) == nil || win.mgr.Lookup(tab) == nil {
		return
	}
	if win.fl.owner(tab) != 0 {
		win.fl.remove(tab)
		win.mgr.Dock(tab)
	}
	if win.dockOwner() != tab {
		win.selectTab(tab)
		win.side.reveal = true
	}
	if toggle {
		win.toggleDock(tab, k)
	} else {
		win.openDock(tab, k)
	}
}

// waitingAgent is session tab's subagent that has waited for approval the
// longest (the earliest started among the waiting ones).
func (win *window) waitingAgent(tab tabs.TabID) (subagents.Key, bool) {
	shown, _ := win.subs.Tree(int(tab))
	var best *subagents.Agent
	for _, a := range shown {
		if a.State == subagents.Waiting && (best == nil || a.Started.Before(best.Started)) {
			best = a
		}
	}
	if best == nil {
		return subagents.Key{}, false
	}
	return best.Key, true
}

// toastGo is a toast's "Go ›": show the session, and the panel of the
// subagent waiting there, if one is.
func (win *window) toastGo(tab tabs.TabID) {
	win.selectTab(tab)
	if k, ok := win.waitingAgent(tab); ok {
		win.openDock(tab, k)
	}
	win.invalidate()
}

// dropGoneAgents closes panels whose subagent the tracker forgot.
func (win *window) dropGoneAgents() {
	for id, s := range win.dock.by {
		if win.subs.Get(s.agent) == nil {
			win.closeDock(id)
		}
	}
}

// copyReport puts a's report, as markdown, on the clipboard.
func (win *window) copyReport(gtx layout.Context, a *subagents.Agent) {
	if win.toClipboard != nil {
		win.toClipboard(win.dockReport(a))
	} else {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(win.dockReport(a)))})
	}
	win.note("Report copied")
}
