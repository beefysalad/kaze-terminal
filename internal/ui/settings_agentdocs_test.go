package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/agentdocs"
)

func TestPeerInstructionsSettings(t *testing.T) {
	f := newClaudeFixture(t, "", false)
	win := claudeWindow(t, f, 800, 600)
	win.codexDir = filepath.Join(f.dir, "codex")
	win.loadAgents()
	for _, id := range []string{"claude_peer_docs", "codex_peer_docs"} {
		win.drawSettings(pageContext(800, 600), pageRect)
		rows := agentsRows(win)
		var row *settingRow
		for i := range rows {
			for _, c := range rows[i].ctrls {
				if c.id == id {
					row = &rows[i]
				}
			}
		}
		if row == nil {
			t.Fatalf("missing %s row", id)
		}
		if row.label != "Peer messaging instructions" || row.ctrls[0].on {
			t.Fatalf("initial row %+v", row)
		}
		path := filepath.Join(filepath.Dir(win.claudePath), "CLAUDE.md")
		if id == "codex_peer_docs" {
			path = filepath.Join(win.codexDir, "AGENTS.md")
		}
		pressAny(t, win, id, 1)
		if st, e := agentdocs.State(path); e != nil || st != agentdocs.Installed {
			t.Fatalf("on %v %v", st, e)
		}
		// Rendering must use the cached state, even if the file changes externally.
		if e := os.WriteFile(path, []byte("owner"), 0600); e != nil {
			t.Fatal(e)
		}
		win.drawSettings(pageContext(800, 600), pageRect)
		if len(controlsOf(win, id)) != 1 || controlsOf(win, id)[0].arg != 0 {
			t.Fatal("state read during frame")
		}
		win.loadAgents()
		win.drawSettings(pageContext(800, 600), pageRect)
		pressAny(t, win, id, 1)
		pressAny(t, win, id, 0)
		b, e := os.ReadFile(path)
		if e != nil || string(b) != "owner" {
			t.Fatalf("off %q %v", b, e)
		}
		if e = os.WriteFile(path, []byte(agentdocs.Block), 0400); e != nil {
			t.Fatal(e)
		}
		if e = os.Chmod(path, 0400); e != nil {
			t.Fatal(e)
		}
		win.loadAgents()
		win.drawSettings(pageContext(800, 600), pageRect)
		pressAny(t, win, id, 0)
		if !pageHas(win, "not writable") {
			t.Fatalf("error missing: %v", win.set.texts)
		}
	}
}

func TestPeerSettingsNeverFallbackToRealHome(t *testing.T) {
	// Capture only the path string. Never read or write anything there.
	realHome, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("USERPROFILE", temp)
	t.Setenv("CODEX_HOME", filepath.Join(temp, "custom"))
	win, _ := sideWindow(t, true)
	// Empty injected setup paths must not fall back to the process home.
	win.home = realHome
	win.loadAgents()
	for _, state := range []*peerDocsState{&win.claudeDocs, &win.codexDocs} {
		if state.path != "" {
			t.Fatalf("empty injection resolved %s", state.path)
		}
	}
	win.claudePath = filepath.Join(temp, "claude", "settings.json")
	win.codexDir = filepath.Join(temp, "codex")
	win.loadAgents()
	for _, state := range []*peerDocsState{&win.claudeDocs, &win.codexDocs} {
		if realHome != "" && strings.HasPrefix(state.path, realHome+string(os.PathSeparator)) {
			t.Fatalf("resolved real home: %s", state.path)
		}
		if !strings.HasPrefix(state.path, temp+string(os.PathSeparator)) {
			t.Fatal(state.path)
		}
	}
}

func TestPeerDocsCodexHomeOverride(t *testing.T) {
	win, _ := sideWindow(t, true)
	win.codexDir = filepath.Join(t.TempDir(), "hooks")
	win.codexDocsDir = t.TempDir()
	win.loadPeerDocs()
	want := filepath.Join(win.codexDocsDir, "AGENTS.md")
	if win.codexDocs.path != want {
		t.Fatalf("path %s want %s", win.codexDocs.path, want)
	}
	win.codexDocs.set(true)
	if st, e := agentdocs.State(want); e != nil || st != agentdocs.Installed {
		t.Fatal(st, e)
	}
	if _, e := os.Stat(win.codexDir); !os.IsNotExist(e) {
		t.Fatal("changed hook directory")
	}
}
