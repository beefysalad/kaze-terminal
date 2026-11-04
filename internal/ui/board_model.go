package ui

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// The agents board's view model (spec 2026-11-03-kaze-agents-board-design, 3.1):
// a Needs you card under All, then one card per session in sidebar order.

// boardRow is one agent line on the board.
type boardRow struct {
	key                   subagents.Key
	tab                   tabs.TabID
	session               bool // a session wait with no subagent named
	state                 subagents.State
	typ, task, step, time string
}

// boardGroup is one session's card.
type boardGroup struct {
	tab                tabs.TabID
	title, meta, agent string               // agent names the logo
	meter              subagents.TrayCounts // every agent of the session, whatever the filter
	rows               []boardRow
	done               int  // matching finished rows
	folded             bool // the finished rows are hidden behind "✓ N finished ›"
}

// boardView is what the board draws this frame.
type boardView struct {
	needs            []boardRow // the Needs you card, under All only
	cards            []boardGroup
	counts           [4]int     // per segment: session token applied, search and segment not
	agents, sessions int        // every agent, for the summary
	order            []boardRow // selectable rows top to bottom
}

// boardSession is a session's names, looked up once per view.
type boardSession struct{ title, repo, branch, agent string }

// boardView builds the board from the tracker and the board's filter, search,
// session token and folds.
func (win *window) boardView() boardView {
	b := &win.board
	var v boardView
	now := time.Now()
	run, wait, done := win.subs.Board()
	lq := strings.ToLower(strings.TrimSpace(b.ed.Text()))
	names := map[tabs.TabID]boardSession{}
	name := func(id tabs.TabID) boardSession {
		s, ok := names[id]
		if !ok {
			s.title, s.repo, s.branch = win.sessionMeta(id)
			s.agent = win.rowAgent(id).agentName()
			names[id] = s
		}
		return s
	}
	meters := map[tabs.TabID]subagents.TrayCounts{}
	seen := map[tabs.TabID]bool{}
	byTab := map[tabs.TabID][]boardRow{} // card rows, in drawing order before folding
	segment := func(st subagents.State) boardFilter {
		switch st {
		case subagents.Waiting:
			return boardNeeds
		case subagents.Done:
			return boardDone
		}
		return boardRunning
	}
	// add files a row: counted, matched, then into the Needs you card or its session's card.
	add := func(r boardRow) {
		if b.tab != 0 && r.tab != b.tab {
			return
		}
		seg := segment(r.state)
		v.counts[boardAll]++
		v.counts[seg]++
		s := name(r.tab)
		if !boardMatch(r, s.title, s.repo, lq) || (b.filter != boardAll && b.filter != seg) {
			return
		}
		if b.filter == boardAll && r.state == subagents.Waiting {
			if !r.session {
				r.step = s.title + " · " + r.step
			}
			v.needs = append(v.needs, r)
			return
		}
		byTab[r.tab] = append(byTab[r.tab], r)
	}
	agentRow := func(a *subagents.Agent) boardRow {
		s := sideAgentOf(a, now)
		return boardRow{key: a.Key, tab: tabs.TabID(a.Tab), state: a.State, typ: a.Type, task: a.Task, step: s.Action, time: s.Elapsed}
	}
	for _, list := range [][]*subagents.Agent{wait, run, done} {
		for _, a := range list {
			id := tabs.TabID(a.Tab)
			v.agents++
			if !seen[id] {
				seen[id] = true
				v.sessions++
			}
			m := meters[id]
			switch a.State {
			case subagents.Waiting:
				m.Waiting++
			case subagents.Running:
				m.Running++
			default:
				m.Done++
			}
			meters[id] = m
		}
	}
	for _, a := range wait {
		add(agentRow(a))
	}
	for _, tab := range win.subs.SessionWaits() {
		id := tabs.TabID(tab)
		add(boardRow{tab: id, session: true, state: subagents.Waiting, typ: "Session", task: name(id).title, step: "a subagent may be asking"})
	}
	for _, a := range run {
		add(agentRow(a))
	}
	for _, a := range done {
		add(agentRow(a))
	}

	// Cards in sidebar order; a session the sidebar does not list goes last.
	ids := make([]tabs.TabID, 0, len(byTab))
	listed := map[tabs.TabID]bool{}
	for _, id := range win.sessionIDs() {
		listed[id] = true
		if len(byTab[id]) > 0 {
			ids = append(ids, id)
		}
	}
	for _, list := range [][]*subagents.Agent{wait, run, done} {
		for _, a := range list {
			id := tabs.TabID(a.Tab)
			if !listed[id] && len(byTab[id]) > 0 {
				listed[id] = true
				ids = append(ids, id)
			}
		}
	}
	for _, r := range v.needs {
		if !r.session {
			v.order = append(v.order, r)
		}
	}
	for _, id := range ids {
		s := name(id)
		g := boardGroup{tab: id, title: s.title, meta: joinMeta(s.repo, s.branch), agent: s.agent, meter: meters[id]}
		show := b.filter == boardDone || b.showDone[id]
		for _, r := range byTab[id] {
			if r.state == subagents.Done {
				g.done++
				if !show {
					continue
				}
			}
			g.rows = append(g.rows, r)
		}
		g.folded = !show && g.done > 0
		for _, r := range g.rows {
			if !r.session {
				v.order = append(v.order, r)
			}
		}
		v.cards = append(v.cards, g)
	}
	return v
}

// joinMeta is "repo · branch", either part left out when empty.
func joinMeta(repo, branch string) string {
	switch {
	case repo == "":
		return branch
	case branch == "":
		return repo
	}
	return repo + " · " + branch
}

// boardMatch reports whether r matches the lower-cased search: its type, task,
// session title or repo, case-insensitive. An empty search matches all.
func boardMatch(r boardRow, title, repo, lowerQuery string) bool {
	if lowerQuery == "" {
		return true
	}
	return containsLower(r.typ, lowerQuery) || containsLower(r.task, lowerQuery) ||
		containsLower(title, lowerQuery) || containsLower(repo, lowerQuery)
}

// containsLower reports whether s, lower-cased rune by rune, contains the
// lower-cased lowerQuery; it allocates nothing.
func containsLower(s, lowerQuery string) bool {
	if lowerQuery == "" {
		return true
	}
	for i := range s {
		j, k := i, 0
		for k < len(lowerQuery) && j < len(s) {
			sr, sn := utf8.DecodeRuneInString(s[j:])
			qr, qn := utf8.DecodeRuneInString(lowerQuery[k:])
			if unicode.ToLower(sr) != qr {
				break
			}
			j, k = j+sn, k+qn
		}
		if k == len(lowerQuery) {
			return true
		}
	}
	return false
}

// boardSummary is the header's "11 agents in 3 sessions"; "" with no agents.
func boardSummary(agents, sessions int) string {
	if agents == 0 {
		return ""
	}
	plural := func(n int, one string) string {
		s := strconv.Itoa(n) + " " + one
		if n != 1 {
			s += "s"
		}
		return s
	}
	return plural(agents, "agent") + " in " + plural(sessions, "session")
}

// meterSplit divides width px into the meter's needs, running and done bars in
// proportion, gap px between the bars that show. A zero count gets no bar and no
// gap; any other gets at least 1 px.
func meterSplit(c subagents.TrayCounts, width, gap int) (needs, run, done int) {
	n := [3]int{c.Waiting, c.Running, c.Done}
	total, parts := 0, 0
	for _, x := range n {
		total += x
		if x > 0 {
			parts++
		}
	}
	if total == 0 || width <= 0 {
		return 0, 0, 0
	}
	room := max(width-gap*(parts-1), parts)
	var w [3]int
	sum, big := 0, -1
	for i, x := range n {
		if x == 0 {
			continue
		}
		w[i] = max((x*room+total/2)/total, 1)
		sum += w[i]
		if big < 0 || x > n[big] {
			big = i
		}
	}
	w[big] += room - sum // the rounding goes to the biggest bar
	return w[0], w[1], w[2]
}
