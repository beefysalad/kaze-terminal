package ui

import (
	"image/color"
	"strconv"
	"strings"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/render"
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

// cardInk is the alert cards' colours for one theme: the surface from the
// theme's toast tokens, every text nudged to at least 4.5:1 on what it sits on.
type cardInk struct {
	bg, line, tile, capBG, capLine                             color.NRGBA
	title, body, muted, branch, cmd, go_, needs, done, badgeFG color.NRGBA
}

func (c *chrome) cardInk() cardInk {
	th := c.th
	k := cardInk{
		bg:      th.ToastBG,
		line:    th.ToastLine,
		tile:    mix(th.ToastBG, th.ToastLine, 0.6),
		capBG:   th.Active,
		capLine: th.ToastLine,
	}
	on := func(fg color.NRGBA, bgs ...color.NRGBA) color.NRGBA {
		for range 2 { // each nudge moves away from the surfaces, so a second pass settles
			for _, bg := range bgs {
				fg = render.Readable(fg, bg)
			}
		}
		return fg
	}
	k.title = on(th.ToastTitle, k.bg)
	k.body = on(th.ToastFG, k.bg, k.capBG) // also the key caps' text
	k.muted = on(th.Muted, k.bg)
	k.branch = on(th.Done, k.bg)
	k.cmd = on(th.ToastGo, k.bg)
	k.go_ = k.cmd
	k.needs = on(th.Needs, k.bg)
	k.done = on(th.Done, k.bg)
	k.badgeFG = on(th.ToastBG, k.needs, k.done)
	return k
}

// cardInk is cardInk for the current theme, worked out once per theme.
func (win *window) cardInk() cardInk {
	a := &win.alerts
	if !a.inkOK || a.inkOf != win.ui.th {
		a.ink, a.inkOf, a.inkOK = win.ui.cardInk(), win.ui.th, true
	}
	return a.ink
}
