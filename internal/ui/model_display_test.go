package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

func TestStatusBarPlacesModelBeforeUsage(t *testing.T) {
	m := busyModel()
	m.Model = "Opus 5.5"
	l := statusAt(m, 1600)
	if !shown(l, grpModel) || l.ModelText != "Opus 5.5" {
		t.Fatalf("%+v", l)
	}
	if l.Groups[grpModel].Max.X > l.Groups[grpUsage].Min.X {
		t.Fatalf("model %v usage %v", l.Groups[grpModel], l.Groups[grpUsage])
	}
}

func TestStatusBarHidesModelWhenUnknown(t *testing.T) {
	if shown(statusAt(busyModel(), 1600), grpModel) {
		t.Fatal("empty model took a slot")
	}
}

func TestStatusBarShortensThenDropsModelFirst(t *testing.T) {
	m := busyModel()
	m.Model = "Opus 5.5"
	sawShort, sawGone := false, false
	for w := 1600; w >= 600; w-- {
		l := statusAt(m, w)
		if !shown(l, grpModel) {
			sawGone = true
			continue
		}
		if l.ModelText == "Opus" {
			sawShort = true
		}
		if !shown(l, grpTabs) || sawGone {
			t.Fatalf("width %d: something else moved before the model: %+v", w, l)
		}
	}
	if !sawShort || !sawGone {
		t.Fatalf("short=%v gone=%v", sawShort, sawGone)
	}
}

func TestStatusCacheNoticesModelChange(t *testing.T) {
	a, b := busyModel(), busyModel()
	a.Model, b.Model = "Opus 5.5", "Fable 5.1"
	if sameStatus(a, b) {
		t.Fatal("model change ignored")
	}
}

func modelMsg(tab int, model string) notify.Msg {
	pct := 10.0
	return notify.Msg{Tab: tab, Kind: notify.KindUsage, Usage: &notify.Usage{Ctx: &pct, Model: model}}
}

func TestStatusModelFollowsTheActiveTab(t *testing.T) {
	win, _ := alertWindow(t, 2)
	win.liveMgr.Store(win.mgr)
	a, b := win.mgr.Tabs()[0].ID, win.mgr.Tabs()[1].ID
	win.onNotify(modelMsg(int(a), "Opus 5.5"))
	win.onNotify(modelMsg(int(b), "Fable 5.1"))
	win.drainUsage()
	win.mgr.SetActive(a)
	if got := win.statusModel(time.Now()).Model; got != "Opus 5.5" {
		t.Fatalf("got %q", got)
	}
	win.mgr.SetActive(b)
	if got := win.statusModel(time.Now()).Model; got != "Fable 5.1" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusModelStaysWithoutContextYet(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.liveMgr.Store(win.mgr)
	win.onNotify(notify.Msg{Tab: int(win.mgr.Tabs()[0].ID), Kind: notify.KindUsage, Usage: &notify.Usage{Model: "Opus 5.5"}})
	win.drainUsage()
	if got := win.statusModel(time.Now()).Model; got != "Opus 5.5" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusModelHidesWhenClaudeLeaves(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.liveMgr.Store(win.mgr)
	tab := win.mgr.Tabs()[0]
	win.onNotify(modelMsg(int(tab.ID), "Opus 5.5"))
	win.drainUsage()
	report(t, tab, t.TempDir())
	if got := win.statusModel(time.Now()).Model; got != "" {
		t.Fatalf("still showing %q in a plain shell", got)
	}
}

func TestClosedTabModelIsDropped(t *testing.T) {
	win, _ := alertWindow(t, 2)
	win.liveMgr.Store(win.mgr)
	gone := win.mgr.Tabs()[1].ID
	win.onNotify(modelMsg(int(gone), "Opus 5.5"))
	win.drainUsage()
	win.mgr.Close(gone)
	win.pruneLocs(win.mgr.Tabs())
	if _, ok := win.models[gone]; ok {
		t.Fatal("closed tab model kept")
	}
}

func TestSubagentPaneShowsItsModel(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	main := filepath.Join(t.TempDir(), "main.jsonl")
	p := subagents.TranscriptPath(main, "x")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"assistant","message":{"model":"claude-haiku-5-5","content":[{"type":"text","text":"hello"}]}}` + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	k := dockAgent(t, win, int(a.ID), main)
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	if !rdHas(win.dock.texts, "Haiku 5.5") {
		t.Fatalf("no model in %q", win.dock.texts)
	}
}
