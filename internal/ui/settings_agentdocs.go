package ui

import (
	"path/filepath"

	"github.com/beefysalad/kaze-terminal/internal/agentdocs"
)

// Cached on page entry and after an action. Rendering never accesses disk.
type peerDocsState struct {
	path   string
	status agentdocs.Status
	err    string
}

func (s *peerDocsState) load(path string) {
	*s = peerDocsState{path: path}
	if path == "" {
		s.err = "Agent instructions path unavailable"
		return
	}
	status, err := agentdocs.State(path)
	s.status = status
	if err != nil {
		s.err = err.Error()
	}
}
func (s *peerDocsState) row(id, displayPath string) settingRow {
	r := settingRow{label: "Peer messaging instructions", hint: "Writes peer instructions to " + displayPath + ".", ctrls: []ctrl{{kind: ctlToggle, id: id, on: s.status != agentdocs.NotInstalled}}}
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
func (s *peerDocsState) set(on bool) {
	if s.path == "" {
		s.err = "Agent instructions path unavailable"
		return
	}
	var err error
	if on {
		err = agentdocs.Install(s.path)
	} else {
		err = agentdocs.Remove(s.path)
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
}
