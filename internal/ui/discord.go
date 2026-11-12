package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/beefysalad/kaze-terminal/internal/discord"
	"github.com/beefysalad/kaze-terminal/internal/version"
)

// presenceSink is what the window needs from the Discord client; tests replace
// it. Set never blocks and Close never waits on Discord.
type presenceSink interface {
	Set(*discord.Activity)
	Close()
}

// initDiscord starts the presence client. Bench mode never touches Discord. With
// no Application ID in the build the client dials nothing.
func (win *window) initDiscord() {
	if win.bench != nil {
		return
	}
	lock := discordLockPath(win.cfgPath)
	if lock != "" {
		// The lock file lives next to config.toml, whose folder may not exist yet.
		os.MkdirAll(filepath.Dir(lock), 0o700)
	}
	win.discord = discord.New(discord.Options{ClientID: discord.ClientID, LockPath: lock})
}

// discordLockPath is the file that lets only one running Kaze own the card.
func discordLockPath(cfgPath string) string {
	if cfgPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfgPath), "discord.lock")
}

// closeDiscord drops the card by closing the connection. It does not wait.
func (win *window) closeDiscord() {
	if win.discord != nil {
		win.discord.Close()
		win.discord = nil
	}
}

// presenceInputs gathers what the card says. It reports false while the active
// tab's folder is still being looked up, so a half-known name is never shown.
func (win *window) presenceInputs() (discord.Inputs, bool) {
	in := discord.Inputs{
		ShowProject: win.opt.Config.DiscordShowProject,
		Start:       win.startedAt,
		Version:     version.Number,
	}
	for _, t := range win.mgr.Tabs() {
		switch win.rowAgent(t.ID).agentName() {
		case "claude":
			in.Claude++
		case "codex":
			in.Codex++
		}
	}
	if t := win.mgr.Active(); t != nil {
		e := win.locOf(t)
		if e.pending {
			return in, false
		}
		in.Project = e.repoName
		if in.Project == "" && e.cwd != "" && !samePath(e.cwd, win.home) {
			if b := filepath.Base(e.cwd); b != "." && b != string(filepath.Separator) {
				in.Project = b
			}
		}
	}
	return in, true
}

// samePath compares two folders the way the OS does; Windows and macOS ignore case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// updatePresence hands the client the current card, once per change. Called every
// frame, it does no I/O; the client sends at most one update per 15 seconds.
func (win *window) updatePresence() {
	if win.discord == nil || win.mgr == nil {
		return
	}
	if !win.opt.Config.DiscordPresence {
		if !win.presenceCleared {
			win.discord.Set(nil)
			win.presenceCleared, win.presenceSent = true, nil
		}
		return
	}
	in, ok := win.presenceInputs()
	if !ok {
		return
	}
	a := discord.Build(in)
	if win.presenceSent != nil && *win.presenceSent == a {
		return
	}
	win.discord.Set(&a)
	win.presenceSent, win.presenceCleared = &a, false
}
