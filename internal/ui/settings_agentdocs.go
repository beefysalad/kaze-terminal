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
	if s.status == agentdocs.Different {
		r.pill = "Modified"
		r.tone = "warn"
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
func (win *window) loadPeerDocs() {
	// Derive from the existing injectable agent paths. Empty paths never resolve
	// the real home, including for headless/test windows.
	claude, codex := "", ""
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
	win.claudeDocs.load(claude)
	win.codexDocs.load(codex)
	win.claudeDocs.workers.worker = true
	win.codexDocs.workers.worker = true
	win.claudeDocs.workers.load(claude)
	win.codexDocs.workers.load(codex)
}
