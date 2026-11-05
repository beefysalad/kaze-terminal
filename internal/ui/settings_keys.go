package ui

import (
	"errors"
	"runtime"

	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/input"
)

// applyKeys sets win.km, win.binds, win.opt.Config.Keys and the reserved keys
// together from [keys] text, and drops cached key labels. Start-up and every
// save go through it, so a save gives what a restart would. Returns the warnings.
func (win *window) applyKeys(keys map[string]string) []string {
	km, warns := buildKeymap(runtime.GOOS, keys)
	binds, _ := resolveBindings(runtime.GOOS, keys)
	win.km, win.binds = km, binds
	win.opt.Config.Keys = keys
	input.SetReserved(runtime.GOOS, km.shortcuts())
	// The launcher caches its label; the title bar, palette and Settings read
	// keyText every time, and the alert caps are keyed by the shortcut itself.
	win.side.launch.work.shortcutReady = false
	if win.w != nil {
		win.w.Invalidate()
	}
	return warns
}

// reloadKeys re-Loads win.cfgPath and applies only its [keys] (spec 8.2). On a
// load error it returns the error and changes nothing.
func (win *window) reloadKeys() error {
	if win.cfgPath == "" {
		return errors.New("kaze has no config file")
	}
	cfg, _, err := config.Load(win.cfgPath)
	if err != nil {
		return err
	}
	win.applyKeys(cfg.Keys)
	return nil
}
