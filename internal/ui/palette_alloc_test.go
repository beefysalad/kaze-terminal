package ui

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/skills"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Today's palette: allocations per still frame at 1000x700 on Windows,
// measured on commit 94f942c (before the redesign).
const (
	palAllocsEmpty = 281
	palAllocsTyped = 129
)

// palAllocsSlash bounds the Skills scope on "/": 97 measured after the
// final-review fixes (skill rows built once per scan, 230 before), then 106
// once all UI text moved to JetBrains Mono (wider, so more rows are cut with
// "…" each frame), plus a small margin.
const palAllocsSlash = 112

// palAllocScene is palWindow with five sessions (one needing you) and 40 fake
// skills: 15 user, 9 devbox, 6 superpowers, 5 v3-gauntlet, 5 project.
func palAllocScene(t *testing.T) (*window, *input.Router) {
	t.Helper()
	win, _, r, _ := palWindow(t)
	openIn(t, win, osPath("/code/gamma/ddd"))
	d := openIn(t, win, osPath("/code/gamma/eee"))
	var list []skills.Skill
	add := func(prefix string, n int, scope skills.Scope) {
		for i := range n {
			list = append(list, skills.Skill{Name: fmt.Sprintf("%sskill-%02d", prefix, i), Desc: "does a thing for the scene", Scope: scope})
		}
	}
	add("", 15, skills.User)
	add("devbox:", 9, skills.Plugin)
	add("superpowers:", 6, skills.Plugin)
	add("v3-gauntlet:", 5, skills.Plugin)
	add("local-", 5, skills.Project)
	win.skillsFn = func(skills.Sources) []skills.Skill { return list }
	win.alerts.apply(notify.Msg{Tab: int(d.ID), Kind: notify.KindNeeds}, 0, false, func(tabs.TabID) bool { return true }, time.Now())
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	return win, r
}

// palStillAllocs runs 3 warm frames of a w x h window, then measures a still one.
func palStillAllocs(t *testing.T, win *window, r *input.Router, w, h int) float64 {
	t.Helper()
	g := pageContext(w, h)
	g.Ops = new(op.Ops)
	g.Now = time.Unix(1000, 0)
	frame := func() {
		g.Ops.Reset()
		g.Source = r.Source()
		if err := win.frame(g); err != nil {
			t.Fatal(err)
		}
		r.Frame(g.Ops)
	}
	for range 3 {
		frame()
	}
	return testing.AllocsPerRun(20, frame)
}

func TestPaletteFrameAllocs(t *testing.T) {
	win, r := palAllocScene(t)
	openPal(t, win, r)
	if len(win.palette.skills) != 40 {
		t.Fatalf("scene: %d skills", len(win.palette.skills))
	}
	empty := palStillAllocs(t, win, r, 1000, 700)
	palType(t, win, r, "re")
	typed := palStillAllocs(t, win, r, 1000, 700)
	win.palette.ed.SetText("")
	sideFrame(t, win, r)
	palType(t, win, r, "/")
	slash := palStillAllocs(t, win, r, 1000, 700)
	t.Logf("palette allocs per still frame: empty %.0f, typed \"re\" %.0f, \"/\" (Skills) %.0f", empty, typed, slash)
	if runtime.GOOS != "windows" {
		return // Ruling 12: measured on Windows; other platforms only log
	}
	if empty > palAllocsEmpty || typed > palAllocsTyped || slash > palAllocsSlash {
		t.Fatalf("empty %.0f (want <= %d), typed %.0f (want <= %d), \"/\" %.0f (want <= %d)", empty, palAllocsEmpty, typed, palAllocsTyped, slash, palAllocsSlash)
	}
}
