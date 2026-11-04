# Product captures

Issue #101 changes the session rows, group headers and bottom launcher. All
sidebar-visible product captures need retaking after native owner review. Headless
Gio verification images are development evidence, not replacement product captures.


Native macOS window captures (2200 × 1440 px, Retina) retaken November 1, 2026, from Kaze 0.15 with the redesigned title bar and status bar. Kaze draws its own title bar now, so the captures have no system title bar above it.

- `kaze-sessions.png`: the session sidebar with three demo repos (one a git worktree), the "Todo API"
  workspace running, and its API member floating over the `todo-api` session.
- `kaze-palette.png`: the palette opened over the same session. The skills are five demo
  `SKILL.md` files in the throwaway home folder.
- `kaze-scratch.png` (removed in the 0.27 docs): the scratch shell over the `todo-api` session.
  The scratch shell is gone, so the shot and its gallery tab were dropped.
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

## 0.25 agent tray, project rail and colour emoji

Retake sidebar captures (`kaze-sessions.png`, `kaze-agents.png`): subagents are now a slim tray
(three lines, "+N more", the fold), and the project you're in has an accent rail instead of a tinted
header. Terminal captures with Claude's status tables now show colour emoji. The headless renders in
`docs/audits/sidebar-tray-emoji/` are development evidence, not product captures. No product
screenshot was fabricated or retouched.

## 0.26 agents board

Retake `kaze-agents.png` (the board now has filters, session cards and a preview pane) and
`kaze-sessions.png` (no project rail; agent lines have no ›). The headless renders in
`docs/audits/agents-board/` are development evidence, not product captures. No product screenshot
was fabricated or retouched.

## 0.27 dock tabs, alert cards and path links

Workspace members now open as tabs in the panel under the terminal, and the floating panes and
the scratch shell are gone. Alerts are cards at the bottom right.

- Retake `kaze-sessions.png`: it still shows the API member floating over `todo-api`. Retake it
  with the API member as a tab in the panel under the `todo-api` terminal (a subagent tab beside it
  is a plus), and ideally a "needs you" alert card at the bottom right. Its alt text and caption on
  the lander were made neutral ("with the todo-api session and its API server log") so they stay
  true until then; once retaken, they can say the log is in the panel under the terminal.
- `kaze-scratch.png` was deleted with its "Scratch shell" gallery tab and feature card. Nothing
  replaces it.
- The "Servers in the panel." card links to the README instead of showing a shot, because no
  product shot shows a member tab yet. Point it back at the gallery (`data-show="sessions"`) once
  `kaze-sessions.png` is retaken.
- `kaze-agents.png` should show the panel's tab row too when it is next retaken (it was already
  due since 0.24 to 0.26).
- Worth adding when convenient: a native shot of an alert card with a done reply.

The headless renders in `docs/audits/dock-alerts-links/` are development evidence, not product
captures. No product screenshot was fabricated or retouched.

## 0.28 battery, readable agent activity and agent names

- Retake `kaze-agents.png` and `kaze-board.png`: both show the old step list. The panel and the
  board preview now read like Claude Code's own transcript (the agent's words first with the coral
  bar, one sentence per step, folded reads and searches, a **Readable | Every step** switch). A good
  take shows a running agent with a note, a folded "Read 3 files", a Bash step with "passed" or
  "failed · 1 test", and ideally one Edit opened to its diff. Use a real Claude session (or demo
  transcripts written in Claude's format); hook-only demo agents fall back to hook summaries and
  won't show the readable view.
- New, Windows only: a status bar shot with the battery at the far right. It needs a real Windows
  laptop (desktops show nothing, and macOS shows nothing until #139). Charging (green bolt) and a low
  state (amber at 20% or less) are both worth a take. Don't fake one with a fake battery source.
- The existing status bar captures are macOS shots, so they stay true until #139 lands.

The headless renders in `docs/audits/battery-activity-names/` (`battery-*.png` from a fake battery
source, `activity-*.png` from fixture transcripts) are development evidence, not product captures.
No product screenshot was fabricated or retouched.
