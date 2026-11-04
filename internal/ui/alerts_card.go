package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// alertCard is what one alert card shows (issue #9): the mark, a context line
// (branch and slash command), the session's sidebar name, the body and a status.
type alertCard struct {
	tab  tabs.TabID
	run  uint64
	kind notify.Kind
	mark string // "claude", "codex", "workspace" or "" (no agent known)

	ctx, title, body, status string
}

// cardOf is the card for one toast. A session's title follows the sidebar row's
// rule (never the bare shell title); a run summary keeps its own.
func (win *window) cardOf(t toast) alertCard {
	c := alertCard{tab: t.Tab, run: t.Run, kind: t.Kind, body: t.Text, mark: t.Agent}
	c.status = cardStatus(t.Kind, t.Text, t.Ran, t.Run != 0)
	if t.Run != 0 {
		c.title, c.ctx = t.Title, "workspace"
		if c.mark == "" {
			c.mark = "workspace"
		}
		return c
	}
	title, _, branch := win.sessionMeta(t.Tab)
	c.title = title
	if c.mark == "" {
		c.mark = win.rowAgent(t.Tab).agentName()
	}
	var parts [2]string
	n := 0
	if branch != "" {
		parts[n], n = "⎇ "+branch, n+1
	}
	if cmd := win.agentOf(t.Tab).command; cmd != "" {
		parts[n], n = cmd, n+1
	}
	c.ctx = strings.Join(parts[:n], " · ")
	return c
}

// cardStatus is the card's status line: why a session waits, how long a done
// turn ran, or that a workspace run finished.
func cardStatus(kind notify.Kind, reason string, ran time.Duration, run bool) string {
	switch {
	case run:
		return "Run finished"
	case kind == notify.KindNeeds:
		r := strings.ToLower(reason)
		if strings.Contains(r, "permission") || strings.Contains(r, "approve") {
			return "Needs your permission"
		}
		return "Waiting for your answer"
	case ran >= time.Second:
		return "Done · ran " + ranText(ran)
	}
	return "Done"
}

// ranText is a turn's length the way a person says it: "45s", "6m 40s", "1h 2m".
func ranText(d time.Duration) string {
	s := int64(d / time.Second)
	h, m := s/3600, s/60%60
	switch {
	case h > 0:
		return strconv.FormatInt(h, 10) + "h " + strconv.FormatInt(m, 10) + "m"
	case m > 0:
		return strconv.FormatInt(m, 10) + "m " + strconv.FormatInt(s%60, 10) + "s"
	}
	return strconv.FormatInt(s, 10) + "s"
}
