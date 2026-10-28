// Command kaze is a lightweight terminal with tabs.
package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/patrick-gogo/kaze/internal/bench"
	"github.com/patrick-gogo/kaze/internal/config"
	"github.com/patrick-gogo/kaze/internal/ptyx"
	"github.com/patrick-gogo/kaze/internal/ui"
)

// r is set at link time (-X main.r=...) only to give a rebuild a new hash when
// Smart App Control blocks a fresh binary.
var r string

func main() {
	_ = r
	rec := bench.FromEnv(os.Getenv) // nil, and free, unless KAZE_BENCH is set
	cfg, warns := loadConfig()
	shell := cfg.Shell
	if len(shell) == 0 {
		shell = ptyx.DefaultShell()
	}
	shell = ptyx.Resolve(shell) // once, so opening a tab never searches PATH
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("kaze"), app.Size(unit.Dp(1100), unit.Dp(720)))
		if err := ui.Run(w, ui.Options{Config: cfg, Warnings: warns, Shell: shell, Dir: home, Bench: rec}); err != nil {
			log.Printf("kaze: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

// loadConfig never fails: a broken file gives defaults and says so in the first tab.
func loadConfig() (config.Config, []string) {
	path := config.Path()
	if path == "" {
		return config.Default(), nil
	}
	cfg, warns, err := config.Load(path)
	if err != nil {
		return config.Default(), append(warns, err.Error()+"; using defaults")
	}
	return cfg, warns
}
