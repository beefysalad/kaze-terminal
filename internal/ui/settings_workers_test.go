package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/agentdocs"
	"github.com/beefysalad/kaze-terminal/internal/config"
)

func workerSetting(t *testing.T, win *window, id string) ctrl {
	t.Helper()
	for _, r := range agentsRows(win) {
		for _, c := range r.ctrls {
			if c.id == id {
				return c
			}
		}
	}
	t.Fatalf("missing %s", id)
	return ctrl{}
}

func TestWorkerInstructionsSettings(t *testing.T) {
	f := newClaudeFixture(t, "", false)
	win := claudeWindow(t, f, 800, 600)
	win.codexDir = filepath.Join(f.dir, "codex")
	win.loadAgents()
	for _, agent := range []string{"claude", "codex"} {
		id := agent + "_worker_docs"
		if workerSetting(t, win, id).on {
			t.Fatal("default on")
		}
		agentsAct(win, id, 1)
		p := filepath.Join(filepath.Dir(win.claudePath), "CLAUDE.md")
		if agent == "codex" {
			p = filepath.Join(win.codexDir, "AGENTS.md")
		}
		if st, err := agentdocs.WorkersState(p); err != nil || st != agentdocs.Installed {
			t.Fatal(st, err)
		}
		if err := agentdocs.Install(p); err != nil {
			t.Fatal(err)
		}
		agentsAct(win, id, 0)
		if st, err := agentdocs.State(p); err != nil || st != agentdocs.Installed {
			t.Fatal("peer block changed", st, err)
		}
		if err := agentdocs.InstallWorkers(p); err != nil {
			t.Fatal(err)
		}
		win.loadAgents()
		if !workerSetting(t, win, id).on {
			t.Fatal("did not refresh from file")
		}
		if err := os.WriteFile(p, []byte("owner"), 0600); err != nil {
			t.Fatal(err)
		}
		if !workerSetting(t, win, id).on {
			t.Fatal("frame refreshed from disk")
		}
		win.loadAgents()
		if workerSetting(t, win, id).on {
			t.Fatal("stale state")
		}
		if err := os.Chmod(p, 0400); err != nil {
			t.Fatal(err)
		}
		agentsAct(win, id, 1)
		found := false
		for _, r := range agentsRows(win) {
			for _, c := range r.ctrls {
				if c.id == id && r.pill == "Error" && strings.Contains(r.hint, "not writable") {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("missing row error")
		}
	}
}

func TestOrchestratorSettingsToggle(t *testing.T) {
	win := sheetWindow(t, "midnight")
	win.opt.Config.Control.Enabled = true
	src := "# owner\n[orchestrator]\nenabled = false # switch\nconfirm = 'never'\n"
	if err := os.WriteFile(win.cfgPath, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	if workerSetting(t, win, "orchestrator_enabled").on {
		t.Fatal("default on")
	}
	for _, on := range []bool{true, false} {
		arg := 0
		if on {
			arg = 1
		}
		agentsAct(win, "orchestrator_enabled", arg)
		if win.orchestratorEnabled != on || win.opt.Config.Orchestrator.Enabled != on {
			t.Fatal("live flag")
		}
		c, _, err := config.Load(win.cfgPath)
		if err != nil || c.Orchestrator.Enabled != on {
			t.Fatal(c, err)
		}
	}
	b, _ := os.ReadFile(win.cfgPath)
	if !strings.Contains(string(b), "# switch") || !strings.Contains(string(b), "confirm = 'never'") {
		t.Fatal("lost config")
	}
	win.opt.Config.Control.Enabled = false
	if !workerSetting(t, win, "orchestrator_enabled").dead {
		t.Fatal("control off must disable")
	}
	agentsAct(win, "orchestrator_enabled", 1)
	if win.orchestratorEnabled {
		t.Fatal("enabled without control")
	}
	found := false
	for _, r := range agentsRows(win) {
		if r.label == "Let agents start workers" && r.hint == "Turn on agent control first" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing dependency hint")
	}
}
