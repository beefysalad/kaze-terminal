package ui

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// dockAgent starts subagent "x" under tab a's session "s" and returns its key.
func dockAgent(t *testing.T, win *window, tab int, transcript string) subagents.Key {
	t.Helper()
	win.liveMgr.Store(win.mgr)
	win.onNotify(notify.Msg{Tab: tab, Kind: notify.KindTool, Session: "s", Tool: "Agent", AgentType: "Explore", Description: "Find auth", TaskPrompt: "Find **auth**."})
	win.onNotify(notify.Msg{Tab: tab, Kind: notify.KindAgentStart, Session: "s", AgentID: "x", AgentType: "Explore", Transcript: transcript})
	return subagents.Key{Session: "s", ID: "x"}
}

func stopAgent(win *window, tab int, report string) {
	win.onNotify(notify.Msg{Tab: tab, Kind: notify.KindAgentStop, Session: "s", AgentID: "x", Report: report})
}

func TestDockDrawsReportMarkdown(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "**No binding** in `docs/notes.md`.")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	win.dockOf(a.ID).tab = dockReport
	twoFrames(t, win, r)
	all := strings.Join(win.dock.texts, "")
	if !strings.Contains(all, "No binding") || strings.Contains(all, "**") {
		t.Fatalf("report not rendered: %q", all)
	}
	if len(win.dock.hits.links) == 0 || win.dock.hits.links[0].url != "docs/notes.md" {
		t.Fatalf("file link not clickable: %+v", win.dock.hits.links)
	}
}

func writeTranscript(t *testing.T, path string, texts ...string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, s := range texts {
		f.WriteString(fixtureAssistant(s))
	}
}

func fixtureAssistant(text string) string {
	return `{"type":"assistant","message":{"content":[{"type":"text","text":"` + text + `"}]}}` + "\n"
}

func TestDockNotesParseOnce(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl")
	path := subagents.TranscriptPath(main, "x")
	writeTranscript(t, path, "one **a**", "two", "three")
	k := dockAgent(t, win, int(a.ID), main)
	win.openDock(a.ID, k)
	for range 3 {
		twoFrames(t, win, r)
	}
	s := win.dockOf(a.ID)
	if s.cache.parses != 3 {
		t.Fatalf("parses %d, want 3", s.cache.parses)
	}
	writeTranscript(t, path, "four")
	win.dockTick(s, win.subs.Get(k), time.Now().Add(2*time.Second))
	twoFrames(t, win, r)
	if s.cache.parses != 4 {
		t.Fatalf("parses %d after one new note, want 4", s.cache.parses)
	}
}

func TestDockIdleNoWake(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl")
	writeTranscript(t, subagents.TranscriptPath(main, "x"), "done")
	k := dockAgent(t, win, int(a.ID), main)
	stopAgent(win, int(a.ID), "ok")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	s := win.dockOf(a.ID)
	ag := win.subs.Get(k)
	now := ag.Ended.Add(time.Minute)
	if _, wake := win.dockTick(s, ag, now); wake {
		t.Fatal("a finished agent's panel keeps polling")
	}
	if s.tr == nil || !s.tr.Available {
		t.Fatal("finished agent's transcript never read once")
	}
}

func TestDockCodexNeverRead(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	win.liveMgr.Store(win.mgr)
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStart, Session: "s", AgentID: "x", Agent: "codex", Transcript: filepath.Join(t.TempDir(), "s.jsonl")})
	k := subagents.Key{Session: "s", ID: "x"}
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	s := win.dockOf(a.ID)
	if _, wake := win.dockTick(s, win.subs.Get(k), time.Now()); wake || s.tr != nil {
		t.Fatal("a Codex helper's transcript was read")
	}
}

func TestDockReportCopyIsMarkdown(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "**raw** report")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	var copied string
	win.toClipboard = func(s string) { copied = s }
	win.copyReport(pageContext(1000, 600), win.subs.Get(k))
	if copied != "**raw** report" {
		t.Fatalf("copied %q", copied)
	}
}

func TestDockContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		for where, bg := range map[string]color.NRGBA{"body": th.Term.BG, "header": c.th.Chrome} {
			cols := c.dockColors(bg)
			for label, fg := range map[string]color.NRGBA{"text": cols.text, "muted": cols.muted, "accent": cols.accent, "needs": cols.needs, "done": cols.done, "type": cols.kind} {
				if got := contrast(fg, bg); got < 4.5 {
					t.Errorf("%s %s %s contrast %.2f", th.Key, where, label, got)
				}
			}
		}
	}
}

// A member tab's dot uses the sidebar's state colours (down, needs you, up)
// and stands out at 3:1 on the tab row and on the shown tab, in every theme.
func TestDockMemberDotContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		for name, l := range map[string]dockTabLabel{
			"down": {member: true, down: true}, "needs": {member: true, mk: markNeeds}, "up": {member: true},
		} {
			for bgName, bg := range map[string]color.NRGBA{"row": th.UI.Chrome, "shown tab": th.Term.BG} {
				if got := contrast(c.memberDot(l), bg); got < 3 {
					t.Errorf("%s: %s dot on %s contrast %.2f, want at least 3", th.Key, name, bgName, got)
				}
			}
		}
	}
}
