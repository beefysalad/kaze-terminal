package ui

import (
	"time"

	"github.com/beefysalad/kaze-terminal/internal/config"
)

func (win *window) orchestratorSettingRow() settingRow {
	hint := "Allow agents to start Claude Code or Codex workers in new tabs."
	if !win.opt.Config.Control.Enabled {
		hint = "Turn on agent control first"
	}
	return settingRow{label: "Let agents start workers", hint: hint, ctrls: []ctrl{{
		kind: ctlToggle, id: "orchestrator_enabled", on: win.orchestratorEnabled,
		dead: !win.opt.Config.Control.Enabled,
	}}}
}

func (win *window) setOrchestratorEnabled(on bool) {
	if !win.opt.Config.Control.Enabled {
		return
	}
	win.orchestratorEnabled = on
	win.opt.Config.Orchestrator.Enabled = on
	if win.cfgPath != "" {
		if err := config.SetOrchestratorEnabled(win.cfgPath, on); err != nil {
			win.notice = "settings not saved: " + err.Error()
			win.noticeUntil = time.Now().Add(noticeFor)
		}
	}
	win.w.Invalidate()
}
