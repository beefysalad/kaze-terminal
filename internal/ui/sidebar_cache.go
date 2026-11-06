package ui

import (
	"image"
	"maps"
	"slices"
	"time"

	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

type sideTabInput struct {
	id                        tabs.TabID
	title, fixed, color, name string
	exited                    bool
	run                       uint64
	loc                       locEntry
	agent                     agentState
	mark                      mark
	lines                     lineCount
}
type sideDocInput struct {
	session tabs.TabID
	doc     sideDoc
}
type sideDrawKey struct {
	activeRepo            string
	openAgent, hoverAgent subagents.Key
	launchVersion         uint64
	launcher              bool
	size                  image.Point
	metric                unit.Metric
	locale                system.Locale
	shown, hover, docked  tabs.TabID
	renaming              tabs.TabID
	doc, hoverDoc         docID
	scroll                int
	settings, settingsOn  bool
	theme                 theme.Chrome
	shaper                *text.Shaper
	generation            uint64
}
type sideCache struct {
	valid             bool
	inputs            []sideTabInput
	docs              []sideDocInput
	folded            map[string]bool
	runFolded         map[uint64]bool
	trayFolded        map[tabs.TabID]bool
	filter, home, def string
	revision          uint64
	until             time.Time
	base, model       sideModel
	order             []tabs.TabID
	builds, draws     uint64
	ops               op.Ops
	call              op.CallOp
	drawKey           sideDrawKey
	drawValid         bool
}

func sameRuns(a, b []sideRun) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.Run != y.Run || x.Name != y.Name || x.Down != y.Down || x.Mark != y.Mark || !slices.Equal(x.Members, y.Members) {
			return false
		}
	}
	return true
}

// cachedSide compares small raw values. It never builds rows, copies strings,
// enumerates subagent trees or shapes text on an unchanged frame.
func (win *window) cachedSide(now time.Time) sideModel {
	s, c := &win.side, &win.side.cache
	s.tabs = win.mgr.TabsInto(s.tabs)
	dirty := !c.valid || len(c.inputs) != len(s.tabs) || c.home != win.home || c.def != win.defTitle || c.revision != win.subs.Revision() || (!c.until.IsZero() && !now.Before(c.until))
	if cap(c.inputs) < len(s.tabs) {
		c.inputs = make([]sideTabInput, 0, len(s.tabs))
	}
	oldLen := len(c.inputs)
	c.inputs = c.inputs[:len(s.tabs)]
	for i, t := range s.tabs {
		run, _ := win.isMember(t)
		add, del := win.linesOf(t.ID)
		v := sideTabInput{id: t.ID, title: t.Title, fixed: t.FixedTitle, color: t.Color, name: win.names[t.ID], exited: t.Exited, run: run, loc: win.locOf(t), agent: win.rowAgent(t.ID), mark: win.alerts.markOf(t.ID), lines: lineCount{add, del}}
		if i >= oldLen || c.inputs[i] != v {
			dirty = true
			c.inputs[i] = v
		}
	}
	if len(c.docs) != len(win.docs.list) {
		dirty = true
	}
	for i, d := range win.docs.list {
		v := sideDocInput{d.session, sideDoc{d.id, d.name, d.folder}}
		if i >= len(c.docs) || c.docs[i] != v {
			dirty = true
		}
	}
	if !maps.Equal(c.folded, s.folded) || !maps.Equal(c.runFolded, s.st.runFolded) ||
		!maps.Equal(c.trayFolded, s.trayFolded) {
		dirty = true
	}
	runs := win.curRuns()
	if !sameRuns(c.base.Runs, runs) {
		dirty = true
	}
	filter := s.filterText()
	if dirty {
		in := win.sideIn()
		in.Tabs = s.tabs
		in.Runs = runs
		c.base = buildSide(in)
		c.base.Runs = cloneSideRuns(runs)
		c.order = sessionOrder(c.base)
		c.folded = maps.Clone(s.folded)
		c.runFolded = maps.Clone(s.st.runFolded)
		c.trayFolded = maps.Clone(s.trayFolded)
		c.docs = c.docs[:0]
		for _, d := range win.docs.list {
			c.docs = append(c.docs, sideDocInput{d.session, sideDoc{d.id, d.name, d.folder}})
		}
		c.revision = win.subs.Revision()
		c.home = win.home
		c.def = win.defTitle
		c.until, _ = win.subTimer(now)
	}
	if dirty || filter != c.filter {
		c.model = c.base
		if filter != "" {
			in := win.sideIn()
			in.Tabs = s.tabs
			in.Runs = runs
			in.Filter = filter
			c.model = buildSide(in)
			c.model.Runs = c.base.Runs
		}
		c.filter = filter
		c.builds++
		c.valid = true
	}
	return c.model
}

func cloneSideRuns(runs []sideRun) []sideRun {
	out := slices.Clone(runs)
	for i := range out {
		out[i].Members = slices.Clone(out[i].Members)
	}
	return out
}
