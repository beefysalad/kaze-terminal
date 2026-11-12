package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/bench"
	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/discord"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// fakePresence records what the window asked Discord to show.
type fakePresence struct {
	sets   []*discord.Activity
	closed int
}

func (f *fakePresence) Set(a *discord.Activity) {
	if a != nil {
		c := *a
		a = &c
	}
	f.sets = append(f.sets, a)
}
func (f *fakePresence) Close() { f.closed++ }

func (f *fakePresence) last(t *testing.T) *discord.Activity {
	t.Helper()
	if len(f.sets) == 0 {
		t.Fatal("no presence was set")
	}
	return f.sets[len(f.sets)-1]
}

func presenceWindow(t *testing.T) (*window, *fakePresence) {
	t.Helper()
	win, _ := sideWindow(t, true)
	win.home = osPath("/home/me")
	win.startedAt = time.Unix(1_700_000_000, 0)
	win.agents = map[tabs.TabID]agentState{}
	f := &fakePresence{}
	win.discord = f
	return win, f
}

func TestPresenceInputsProject(t *testing.T) {
	win, _ := presenceWindow(t)
	repo := openIn(t, win, osPath("/code/alpha/a"))
	plain := openIn(t, win, osPath("/notes/today"))
	home := openIn(t, win, win.home)
	none := openIn(t, win, "")
	for _, tc := range []struct {
		name string
		tab  *tabs.Tab
		want string
	}{
		{"repo name", repo, "alpha"},
		{"folder name outside a repo", plain, "today"},
		{"home folder", home, ""},
		{"no folder yet", none, ""},
	} {
		win.mgr.SetActive(tc.tab.ID)
		in, ok := win.presenceInputs()
		if !ok || in.Project != tc.want {
			t.Fatalf("%s: project %q ok=%v, want %q", tc.name, in.Project, ok, tc.want)
		}
		if !in.ShowProject || !in.Start.Equal(win.startedAt) || in.Version == "" {
			t.Fatalf("%s: %+v", tc.name, in)
		}
	}
}

func TestPresenceInputsCounts(t *testing.T) {
	win, _ := presenceWindow(t)
	a := openIn(t, win, osPath("/code/alpha/a"))
	b := openIn(t, win, osPath("/code/beta/b"))
	c := openIn(t, win, osPath("/code/gamma/c"))
	openIn(t, win, osPath("/code/delta/d")) // a plain shell
	win.agents[a.ID] = agentState{agent: "claude"}
	win.agents[b.ID] = agentState{claude: true} // old-style state still counts as Claude
	win.agents[c.ID] = agentState{agent: "codex"}
	in, ok := win.presenceInputs()
	if !ok || in.Claude != 2 || in.Codex != 1 {
		t.Fatalf("counts %+v ok=%v, want 2 Claude and 1 Codex", in, ok)
	}
}

func TestUpdatePresenceSendsOnlyChanges(t *testing.T) {
	win, f := presenceWindow(t)
	a := openIn(t, win, osPath("/code/alpha/a"))
	b := openIn(t, win, osPath("/code/beta/b"))
	win.mgr.SetActive(a.ID)
	win.updatePresence()
	win.updatePresence()
	if len(f.sets) != 1 || f.last(t).Details != "Working on alpha" || f.last(t).State != "No agents running" {
		t.Fatalf("after two identical updates: %+v", f.sets)
	}
	win.mgr.SetActive(b.ID)
	win.updatePresence()
	if len(f.sets) != 2 || f.last(t).Details != "Working on beta" {
		t.Fatalf("after switching tab: %+v", f.sets)
	}
	win.agents[b.ID] = agentState{agent: "claude"}
	win.updatePresence()
	if len(f.sets) != 3 || f.last(t).State != "1 Claude session" {
		t.Fatalf("after Claude started: %+v", f.sets)
	}
}

func TestUpdatePresenceHidesProjectOnRequest(t *testing.T) {
	win, f := presenceWindow(t)
	win.mgr.SetActive(openIn(t, win, osPath("/code/alpha/a")).ID)
	win.opt.Config.DiscordShowProject = false
	win.updatePresence()
	if got := f.last(t).Details; got != "In the terminal" {
		t.Fatalf("details with the name hidden: %q", got)
	}
}

func TestUpdatePresenceOff(t *testing.T) {
	win, f := presenceWindow(t)
	win.mgr.SetActive(openIn(t, win, osPath("/code/alpha/a")).ID)
	win.updatePresence()
	win.opt.Config.DiscordPresence = false
	for range 3 {
		win.updatePresence()
	}
	if len(f.sets) != 2 || f.sets[1] != nil {
		t.Fatalf("turning it off should clear exactly once: %+v", f.sets)
	}
	win.opt.Config.DiscordPresence = true
	win.updatePresence()
	if len(f.sets) != 3 || f.sets[2] == nil {
		t.Fatalf("turning it back on should show the card again: %+v", f.sets)
	}
}

func TestUpdatePresenceWithoutAClientIsQuiet(t *testing.T) {
	win, _ := presenceWindow(t)
	win.discord = nil
	win.updatePresence() // must not panic
	win.closeDiscord()
}

func TestFrameUpdatesPresence(t *testing.T) {
	win, f := presenceWindow(t)
	win.mgr.SetActive(openIn(t, win, osPath("/code/alpha/a")).ID)
	sideFrame(t, win, new(input.Router))
	if len(f.sets) == 0 || f.last(t).Details != "Working on alpha" {
		t.Fatalf("a frame should publish the active project: %+v", f.sets)
	}
}

func TestCloseDiscordClosesOnce(t *testing.T) {
	win, f := presenceWindow(t)
	win.closeDiscord()
	win.closeDiscord()
	if f.closed != 1 || win.discord != nil {
		t.Fatalf("closed %d times, client left %v", f.closed, win.discord)
	}
}

func TestInitDiscordSkipsBench(t *testing.T) {
	w := window{bench: new(bench.Recorder)}
	w.initDiscord()
	if w.discord != nil {
		t.Fatal("bench mode started the Discord client")
	}
}

func TestDiscordLockPath(t *testing.T) {
	if got := discordLockPath(""); got != "" {
		t.Fatalf("no config path should mean no lock, got %q", got)
	}
	cfg := filepath.Join("cfg", "kaze", "config.toml")
	if got, want := discordLockPath(cfg), filepath.Join("cfg", "kaze", "discord.lock"); got != want {
		t.Fatalf("lock path %q, want %q", got, want)
	}
}

func TestSettingsDiscordToggles(t *testing.T) {
	win, f := presenceWindow(t)
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(win.cfgPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	win.mgr.SetActive(openIn(t, win, osPath("/code/alpha/a")).ID)

	rows := map[string]settingRow{}
	for _, r := range sharedAgentRows(win) {
		for _, c := range r.ctrls {
			if c.id == "discord_presence" || c.id == "discord_show_project" {
				if !c.on {
					t.Fatalf("%s should start on", c.id)
				}
				rows[c.id] = r
			}
		}
	}
	if r := rows["discord_presence"]; r.label != "Show Kaze on your Discord profile" ||
		r.hint != "Your project, the time Kaze has been open and your session count. Needs the Discord app running." {
		t.Fatalf("presence row: %+v", r)
	}
	if r := rows["discord_show_project"]; r.label != "Show the project name" || r.hint != `Off shows "In the terminal" instead.` {
		t.Fatalf("project row: %+v", r)
	}

	agentsAct(win, "discord_show_project", 0)
	win.updatePresence()
	if win.opt.Config.DiscordShowProject || f.last(t).Details != "In the terminal" {
		t.Fatalf("hiding the name: config=%v last=%+v", win.opt.Config.DiscordShowProject, f.last(t))
	}
	agentsAct(win, "discord_presence", 0)
	win.updatePresence()
	if win.opt.Config.DiscordPresence || f.last(t) != nil {
		t.Fatalf("turning presence off: config=%v last=%+v", win.opt.Config.DiscordPresence, f.last(t))
	}
	saved, _, err := config.Load(win.cfgPath)
	if err != nil || saved.DiscordPresence || saved.DiscordShowProject {
		t.Fatalf("settings not saved: %+v err=%v", saved, err)
	}
}
