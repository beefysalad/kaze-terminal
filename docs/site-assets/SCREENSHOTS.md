# Product captures

Issue #101 changes the session rows, group headers and bottom launcher. All
sidebar-visible product captures need retaking after native owner review. Headless
Gio verification images are development evidence, not replacement product captures.


Native macOS window captures (2200 × 1440 px, Retina) retaken November 1, 2026, from Kaze 0.15 with the redesigned title bar and status bar. Kaze draws its own title bar now, so the captures have no system title bar above it.

- `kaze-sessions.png`: the session sidebar with three demo repos (one a git worktree), the "Todo API"
  workspace running, and its API member floating over the `todo-api` session.
- `kaze-palette.png`: the palette opened over the same session. The skills are five demo
  `SKILL.md` files in the throwaway home folder.
- `kaze-scratch.png`: the scratch shell over the `todo-api` session, showing `git status -sb` and
  `git diff --stat`.
- `kaze-workspaces.png`: the redesigned Workspaces section of Settings with the Todo API workspace card
  and its API, Web and Health members running.
- `kaze-about.png`: the redesigned About section displaying the shipped logo and byline, version 0.15,
  platform, chip and license facts.
- `kaze-settings.png`: Appearance settings with five theme cards, Midnight selected, and the live
  terminal preview.
- `kaze-agents.png` (Kaze 0.14, taken the same day): three subagents under the `TODO-142` session in
  the sidebar (two running, one done), one running under `dark-mode`, and the finished subagent's live
  pane with its steps and report.
- `kaze-board.png` (Kaze 0.14): the agents board with three running and two done subagents from three
  sessions.

How they were made:

- Everything ran in a throwaway home folder with demo projects (demo git author, example.com
  module). No personal or work data appears in the demo projects; the shipped About branding is
  unchanged. The owner's settings and workspace files were not changed. Demo folders: `todo-api`,
  its `todo-api-auth` worktree on `feature/auth`, `web-app` on `feature/dark-mode`, and `notes-cli` on `fix/search`, each with an uncommitted diff.
- The terminal output is real: `make check` (git log, Go tests, ls), git commands, and two
  `python3 -m http.server` processes with a `curl` loop that makes the API log. The Appearance
  live preview is Kaze's built-in sample, not output from an additional command.
- The subagents in `kaze-agents.png` and `kaze-board.png` come from demo shell scripts calling
  Kaze's real hook commands (`kaze notify tool`, `notify agent-start`, `notify agent-stop`) with
  the same input Claude Code's hooks send; the demo home has its own `~/.claude/settings.json` with
  kaze's hooks, so the board shows no setup hint. No subagent is shown waiting on you: Claude Code
  runs subagents non-interactively, so they don't ask for permission themselves.
- Claude Code was not run. The sidebar states (needs you, running, done), the slash commands and
  the alert toast come from demo shell scripts calling Kaze's real hook command (`kaze notify
  prompt`, `notify needs`, `notify done`) with the same input Claude Code's hooks send.
- A temporary build overlay (not part of shipped code) opened the demo sessions, floated the pane,
  showed the scratch shell, and opened the palette or a Settings page. It did not change the UI
  renderer. Each capture was inspected for stray input and cut-off text. No generated or retouched UI.
- Captured directly with macOS `screencapture`, without desktop shadows or surrounding
  applications. No global keyboard or mouse input was injected. The demo app, server and sleep
  processes were stopped after each take.

## 0.18 session restoration

Settings > Workspaces gains the Restore sessions on launch control in Startup, and
Updates confirmation explains that conversations/folders return while commands stop.
Retake captures showing those sections when needed. Existing terminal/board captures
are not replaced with generated UI; no screenshot was retouched for this release.

## 0.19 subagent pane and start folder

The subagent pane now shows the prompt, a one-line summary and the report instead of tool
calls, so the Subagents capture is out of date. Settings > Workspaces > Startup gains the
Start folder row. Retake those captures when needed; nothing was generated or retouched.

## 0.21 markdown reader and live subagent messages

Retake the Subagents capture to show live assistant messages and folded tool counts,
and add a native capture of the markdown reader and outline. The CPU/RAM hover card
also changed in #96. Existing captures remain historical; no generated or retouched
screenshots were substituted for this release. Headless QA renders are validation
artifacts, not lander product captures.

## 0.23 running marks, active project and empty window

Retake sidebar-visible captures to show the active-project header and closed-ring
running marks. Title-bar and status-bar running marks changed too. Add a native
capture of the empty window when useful. Existing screenshots remain historical;
headless audit galleries and the approved HTML mockup are development evidence,
not replacement product captures. No product screenshot was fabricated or retouched.

## 0.24 docked agent panel and Windows clipboard

`kaze-agents.png` shows the old floating subagent card; retake it with a subagent's panel docked
under the terminal (Activity with a note, or the formatted Report). Workspace members whose
commands finished now read Ready, so retake `kaze-workspaces.png` if it shows a finished setup tab.
The headless renders in `docs/audits/agent-dock/` are development evidence, not product captures.
No product screenshot was fabricated or retouched.
