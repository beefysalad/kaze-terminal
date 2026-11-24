package ui

import (
	"hash/fnv"
	"image/color"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// shellSpaceKey is the key of the one space that holds tabs with no repo.
const shellSpaceKey = "\x00shell"

// spacePalette is the badge colours; a space's colour is picked from its name.
var spacePalette = [...]color.NRGBA{
	{0xA7, 0x8B, 0xFA, 0xFF}, {0x60, 0xA5, 0xFA, 0xFF}, {0x34, 0xD3, 0x99, 0xFF}, {0xF4, 0x72, 0xB6, 0xFF},
	{0xFB, 0xBF, 0x24, 0xFF}, {0x2D, 0xD4, 0xBF, 0xFF}, {0xFB, 0x92, 0x3C, 0xFF}, {0x81, 0x8C, 0xF8, 0xFF},
}

// spaceEntry is one space: a repo's sessions, or the Shell space.
type spaceEntry struct {
	Key, Name string
	Badge     rune // upper-cased first letter of Name
	Color     color.NRGBA
	Tabs      []tabs.TabID // its sessions in tab order, run members excluded
	Count     int          // sessions
	Waiting   int          // sessions with markNeeds
	Selected  bool         // holds the active tab
	Docs      []sideDoc    // documents opened from its sessions, listed under its line
}

// spaceTabs is the tab strip's contents for s, as a copy.
func (s spaceEntry) spaceTabs() []tabs.TabID { return append([]tabs.TabID(nil), s.Tabs...) }

type spaceModel struct{ Spaces []spaceEntry }

// selectedSpace is the space holding the active tab, if any.
func (m spaceModel) selectedSpace() (spaceEntry, bool) {
	for _, s := range m.Spaces {
		if s.Selected {
			return s, true
		}
	}
	return spaceEntry{}, false
}

func spaceBadge(name string) rune {
	r, _ := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError {
		return '?'
	}
	return unicode.ToUpper(r)
}

func spaceColor(name string) color.NRGBA {
	h := fnv.New32a()
	h.Write([]byte(name))
	return spacePalette[h.Sum32()%uint32(len(spacePalette))]
}

func newSpace(key, name string, rows []sideRow, active tabs.TabID) spaceEntry {
	s := spaceEntry{Key: key, Name: name, Badge: spaceBadge(name), Color: spaceColor(name), Count: len(rows)}
	for _, r := range rows {
		s.Tabs = append(s.Tabs, r.ID)
		s.Docs = append(s.Docs, r.Docs...)
		if r.Mark == markNeeds {
			s.Waiting++
		}
		if r.ID == active {
			s.Selected = true
		}
	}
	return s
}

// buildSpaces derives the spaces from a side model built with an empty Filter:
// repo groups in tab order, then Shell for the repo-less tabs. A space with no
// session does not exist.
func buildSpaces(side sideModel, active tabs.TabID) spaceModel {
	var m spaceModel
	for _, g := range side.Groups {
		m.Spaces = append(m.Spaces, newSpace(g.Key, g.Name, g.Rows, active))
	}
	if len(side.Loose) > 0 {
		m.Spaces = append(m.Spaces, newSpace(shellSpaceKey, "Shell", side.Loose, active))
	}
	return m
}

// agentGroup is an agent row's section, in display order.
type agentGroup int

const (
	agentNeeds agentGroup = iota
	agentWorking
	agentIdle
)

// agentRow is one session that has a known agent.
type agentRow struct {
	ID       tabs.TabID
	Name     string // agent name, also the logo key
	Subtitle string // the running command, else the branch or folder
	Since    time.Time
	State    agentGroup
	Space    string // the name of the space holding its session
	Mark     mark   // the session's alert mark, for the state dot
	Running  bool
}

type agentsModel struct{ Rows []agentRow }

// buildAgents lists every session with a known agent: Needs you, then Working,
// then Idle, each in space then tab order. Build side with an empty Filter.
func buildAgents(side sideModel) agentsModel {
	var by [3][]agentRow
	add := func(space string, rows []sideRow) {
		for _, r := range rows {
			if r.Agent == "" {
				continue
			}
			st := agentIdle
			switch {
			case r.Mark == markNeeds:
				st = agentNeeds
			case r.Running:
				st = agentWorking
			}
			sub := r.Command
			if sub == "" && r.Title != r.Agent {
				sub = r.Title
			}
			if sub == "" {
				sub = r.Detail
			}
			by[st] = append(by[st], agentRow{ID: r.ID, Name: r.Agent, Subtitle: sub, Since: r.Since, State: st,
				Space: space, Mark: r.Mark, Running: r.Running})
		}
	}
	for _, g := range side.Groups {
		add(g.Name, g.Rows)
	}
	add("Shell", side.Loose)
	var m agentsModel
	for _, rows := range by {
		m.Rows = append(m.Rows, rows...)
	}
	return m
}

// controlSpaceName uses the same repo membership rule as buildSide/buildSpaces.
// Workspace members also get a space name, although the tab strip excludes them.
func controlSpaceName(loc locEntry) string {
	if loc.cwd == "" || loc.repo == "" {
		return "Shell"
	}
	return loc.repoName
}
