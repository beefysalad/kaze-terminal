package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// sideAgent is one subagent line in the sidebar tray.
type sideAgent struct {
	Key                         subagents.Key
	Type, Task, Action, Elapsed string
	State                       subagents.State
	Depth                       int
}

// sideAgentOf is a tracker subagent as its sidebar line at now.
func sideAgentOf(a *subagents.Agent, now time.Time) sideAgent {
	s := sideAgent{Key: a.Key, Type: a.Type, Task: a.Task, State: a.State, Depth: a.Depth, Elapsed: agentElapsed(a, now)}
	switch {
	case a.State == subagents.Done:
		s.Action = reportLine(a.Report)
	case len(a.Steps) > 0:
		st := a.Steps[len(a.Steps)-1]
		s.Action = strings.TrimSpace(st.Tool + " " + st.Summary)
	default:
		s.Action = "Starting"
	}
	return s
}

// reportMarks drops inline markdown marks from a report line.
var reportMarks = strings.NewReplacer("**", "", "__", "", "`", "")

// reportLine is a report's first line of prose for a one-line summary: it
// skips headings, code blocks, tables and rules, drops list and quote marks
// and inline marks; a report of headings only gives its first heading.
func reportLine(report string) string {
	heading, fence := "", false
	for rest := report; rest != ""; {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "```"):
			fence = !fence
			continue
		case fence || line == "" || strings.HasPrefix(line, "|") || strings.Trim(line, "-*_ ") == "":
			continue
		case isHeading(line):
			if heading == "" {
				heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
			}
			continue
		}
		for _, m := range [...]string{"> ", "- ", "* ", "+ "} {
			line = strings.TrimPrefix(line, m)
		}
		return reportMarks.Replace(line)
	}
	return reportMarks.Replace(heading)
}

// isHeading reports whether a trimmed line is a markdown heading: #s then a
// space or tab, or #s alone; "#125 landed" is prose.
func isHeading(line string) bool {
	rest := strings.TrimLeft(line, "#")
	return len(rest) < len(line) && (rest == "" || rest[0] == ' ' || rest[0] == '	')
}

// agentElapsed is "<1m" or "4m" while it runs, "took 6m" once done.
func agentElapsed(a *subagents.Agent, now time.Time) string {
	if a.State == subagents.Done {
		return "took " + minutes(a.Ended.Sub(a.Started))
	}
	return minutes(now.Sub(a.Started))
}

func minutes(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	return strconv.Itoa(int(d/time.Minute)) + "m"
}

// agentChip is the row's "3 agents" chip; "" for none.
func agentChip(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}

// trayCap is a tray's most agent lines; the rest are counted in its "+N more" chip.
const trayCap = 3

// sideTray is one session's subagent tray as the sidebar model sees it.
type sideTray struct {
	Shown          []sideAgent          // the lines to draw; nil while folded
	Hidden         subagents.TrayCounts // agents past the lines, by state; zero while folded
	Total, Waiting int                  // every agent in the tray, and those waiting
	Active         int                  // running and waiting, for the group chip
}

// sideAgents is the sidebar input's view of the tracker for one session.
func (win *window) sideAgents(id tabs.TabID) sideTray {
	tr := sideTray{Active: win.subs.Active(int(id))}
	shown, hidden := win.subs.Tray(int(id), trayCap)
	tr.Total, tr.Waiting = len(shown)+hidden.Total(), hidden.Waiting
	for _, a := range shown {
		if a.State == subagents.Waiting {
			tr.Waiting++
		}
	}
	tr.Hidden = hidden
	now := time.Now()
	tr.Shown = make([]sideAgent, 0, len(shown))
	for _, a := range shown {
		tr.Shown = append(tr.Shown, sideAgentOf(a, now))
	}
	return tr
}
