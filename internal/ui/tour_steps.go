package ui

type tourTarget int

const (
	tourNone tourTarget = iota
	tourSidebar
	tourWorkspaces
	tourStatusBar
	tourAgentsButton
)

type tourStep struct {
	Label, Title, Body   string
	Target               tourTarget
	Board, Hooks         bool
	SkipLabel, NextLabel string
}

func tourSteps() []tourStep {
	return []tourStep{
		{"Welcome", "Welcome to Kaze.", "Thank you for trying it. A quick look around, about a minute. Skip any time. This shows once.", tourNone, false, false, "Skip tour", "Show me around"},
		{"1 of 5", "Spaces and agents", "Each project is a space with its own tabs. The Agents list sorts every Claude Code and Codex agent into Needs you, Working and Idle. Click one to jump to its tab. Kaze alerts you when something wants you.", tourSidebar, false, false, "Skip tour", "Next"},
		{"2 of 5", "Workspaces", "A workspace is a saved launch recipe: a few tabs, each in its folder with its commands typed for you. Make one in Settings, press Run. Running ones show here with status.", tourWorkspaces, false, false, "Skip tour", "Next"},
		{"3 of 5", "Bottom bar", "Folder and branch with lines changed, workspace members up, CPU and RAM for Kaze and what it runs, the model, then Claude usage: 5H is the 5 hour window, WK the week, CTX the context window of the tab in front.", tourStatusBar, false, false, "Skip tour", "Next"},
		{"4 of 5", "Agents board", "Cmd+Shift+A (Ctrl+Shift+A on Windows) opens a full-area board of every subagent Claude Code and Codex start, by space, with what needs you first. Pick one to read its steps and report.", tourAgentsButton, true, false, "Skip tour", "Next"},
		{"5 of 5", "Live status", "Status, alerts and usage come from small hooks in Claude Code and Codex. Kaze can set them up. Nothing changes until you press Fix.", tourNone, false, true, "Do this later", "Finish"},
	}
}
