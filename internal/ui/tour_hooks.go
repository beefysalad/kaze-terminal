package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/claudesetup"
	"github.com/beefysalad/kaze-terminal/internal/codexsetup"
	"os"
)

type tourHookState int

const (
	hookReady tourHookState = iota
	hookNeedsFix
	hookNotInstalled
)

type tourHook struct {
	Agent string
	State tourHookState
}

func (w *window) tourHookStates() []tourHook {
	out := []tourHook{{"Claude Code", hookNotInstalled}, {"Codex", hookNotInstalled}}
	if _, e := os.Stat(w.claudePath); e == nil {
		w.loadClaude()
		out[0].State = hookNeedsFix
		r := w.claude.report
		if w.claude.readErr == "" && r.Needs.State == claudesetup.OK && r.Stop.State == claudesetup.OK && r.StatusLine.State == claudesetup.OK && r.Prompt.State == claudesetup.OK && r.AgentStart.State == claudesetup.OK && r.AgentStop.State == claudesetup.OK && r.Tools.State == claudesetup.OK && r.Session.State == claudesetup.OK {
			out[0].State = hookReady
		}
	}
	w.loadCodex()
	if w.codex.installed {
		out[1].State = hookReady
		for _, e := range w.codex.report.Entries() {
			if e.State != codexsetup.OK {
				out[1].State = hookNeedsFix
			}
		}
		if w.codex.readErr != "" {
			out[1].State = hookNeedsFix
		}
	}
	return out
}

// The Settings dialog remains the only route to a write.
func (w *window) tourFix(agent int) {
	w.closeTour()
	w.openSettings()
	for i, s := range w.sections() {
		if s.title == "Agents" {
			w.set.section = i
			break
		}
	}
	w.loadAgents()
	if agent == 1 {
		w.openFix()
	} else {
		w.openCodexFix()
	}
}
