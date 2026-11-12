package discord

import (
	"fmt"
	"strings"
	"time"
)

// Activity is the presence card shared with the local Discord app.
type Activity struct {
	Details, State        string
	Start                 time.Time
	LargeImage, LargeText string
}

// Inputs contains only the names and counts intended for the presence card.
type Inputs struct {
	Project       string // repo name, else folder base name; "" when unknown or home
	ShowProject   bool
	Claude, Codex int
	Start         time.Time
	Version       string
}

// Build formats the card without performing I/O.
func Build(in Inputs) Activity {
	details := "In the terminal"
	if in.ShowProject && in.Project != "" {
		details = "Working on " + in.Project
	}
	state := "No agents running"
	switch {
	case in.Claude > 0 && in.Codex > 0:
		state = fmt.Sprintf("%d Claude, %d Codex", in.Claude, in.Codex)
	case in.Claude > 0:
		state = sessionCount(in.Claude, "Claude")
	case in.Codex > 0:
		state = sessionCount(in.Codex, "Codex")
	}
	return Activity{
		Details: clipText(details), State: clipText(state), Start: in.Start,
		LargeImage: "kaze", LargeText: clipText("Kaze " + in.Version),
	}
}

func sessionCount(n int, agent string) string {
	suffix := "sessions"
	if n == 1 {
		suffix = "session"
	}
	return fmt.Sprintf("%d %s %s", n, agent, suffix)
}

func (a Activity) payload() map[string]any {
	return map[string]any{
		"details":    clipText(a.Details),
		"state":      clipText(a.State),
		"timestamps": map[string]any{"start": a.Start.Unix()},
		"assets": map[string]any{
			"large_image": clipText(a.LargeImage),
			"large_text":  clipText(a.LargeText),
		},
	}
}

func clipText(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	n := 0
	for i := range s {
		if n == 128 {
			return s[:i]
		}
		n++
	}
	return s
}
