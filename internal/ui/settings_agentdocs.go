package ui

import (
	"path/filepath"

	"github.com/beefysalad/kaze-terminal/internal/agentdocs"
)

// Cached on page entry and after an action. Rendering never accesses disk.
type peerDocsState struct {
	agentDocsFileState
	workers agentDocsFileState
}

type agentDocsFileState struct {
	worker bool
	path   string
	status agentdocs.Status
	err    string
}

func (s *agentDocsFileState) load(path string) {
	*s = agentDocsFileState{path: path, worker: s.worker}
	if path == "" {
		s.err = "Agent instructions path unavailable"
		return
	}
	state := agentdocs.State
	if s.worker {
		state = agentdocs.WorkersState
	}
	status, err := state(path)
	s.status = status
	if err != nil {
		s.err = err.Error()
	}
}
func (s *agentDocsFileState) row(id, displayPath string) settingRow {
	r := settingRow{label: "Peer messaging instructions", hint: "Writes peer instructions to " + displayPath + ".", ctrls: []ctrl{{kind: ctlToggle, id: id, on: s.status != agentdocs.NotInstalled}}}
	if s.worker {
		r.label = "Worker instructions"
		r.hint = "Writes worker instructions to " + displayPath + "."
	}
	switch s.status {
	case agentdocs.Outdated:
		r.pill, r.tone = "Update available", "warn"
		r.ctrls = append(r.ctrls, ctrl{kind: ctlButton, id: id + "_update", text: "Update"})
	case agentdocs.Different:
		r.pill, r.tone = "Edited", "warn"
		r.hint += " Replaces your edits with Kaze's text; a backup is kept."
		r.ctrls = append(r.ctrls, ctrl{kind: ctlButton, id: id + "_reset", text: "Reset", ghost: true})
	}
	if s.err != "" {
		r.pill = "Error"
		r.tone = "err"
		r.hint += " " + cleanText(s.err)
	}
	return r
}
func (s *agentDocsFileState) set(on bool) {
	if s.path == "" {
		s.err = "Agent instructions path unavailable"
		return
	}
	install, remove := agentdocs.Install, agentdocs.Remove
	if s.worker {
		install, remove = agentdocs.InstallWorkers, agentdocs.RemoveWorkers
	}
	var err error
	if on {
		err = install(s.path)
	} else {
		err = remove(s.path)
	}
	s.load(s.path)
	if err != nil {
		s.err = err.Error()
	}
}
func (win *window) agentDocsPaths() (claude, codex string) {
	// Derive from the existing injectable agent paths. Empty paths never resolve
	// the real home, including for headless/test windows.
	if win.claudePath != "" {
		claude = filepath.Join(filepath.Dir(win.claudePath), "CLAUDE.md")
	}
	dir := win.codexDir
	if win.codexDocsDir != "" {
		dir = win.codexDocsDir
	}
	if dir != "" {
		codex = filepath.Join(dir, "AGENTS.md")
	}
	return claude, codex
}

func (win *window) loadPeerDocs() {
	claude, codex := win.agentDocsPaths()
	win.claudeDocs.load(claude)
	win.codexDocs.load(codex)
	win.claudeDocs.workers.worker = true
	win.codexDocs.workers.worker = true
	win.claudeDocs.workers.load(claude)
	win.codexDocs.workers.load(codex)
}

// agentInstructionsNotice performs the four read-only checks once, off the UI
// goroutine. Empty injected paths never fall back to the process home.
func agentInstructionsNotice(claude, codex string) <-chan string {
	result := make(chan string, 1)
	go func() {
		outdated := false
		for _, path := range []string{claude, codex} {
			if path == "" {
				continue
			}
			for _, state := range []func(string) (agentdocs.Status, error){agentdocs.State, agentdocs.WorkersState} {
				status, err := state(path)
				if err == nil && status == agentdocs.Outdated {
					outdated = true
				}
			}
		}
		message := ""
		if outdated {
			message = "Kaze's agent instructions changed. Update them in Settings > Agents."
		}
		result <- message
	}()
	return result
}
