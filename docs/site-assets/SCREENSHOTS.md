# Product captures

The folded terminal K branding update (2026-11-16, local, unreleased) changes
the title bar, Settings footer and About logo. Retake all captures with these
areas visible after native owner review, especially `kaze-about.png`. Existing
product captures stay unchanged; development renders under `docs/audits/branding/`
are verification evidence, not replacement product screenshots. Deferred retakes and Discord
artwork are tracked in [issue #205](https://github.com/beefysalad/kaze-terminal/issues/205).

Release 0.41 adds a Discord section at the bottom of Settings > Agents. No published capture shows
that page, so nothing needs retaking for it. A capture of the card itself would come from the owner's own
Discord profile, so none is published.

Release 0.40 removes the paired-machine Devbox Settings entry. Any private capture
showing that entry needs a native retake. No published Devbox captures are reused
or fabricated for this release.

Subagent pane reading (built on `feat/pane-reading`, not released): Readable activity now opens
edits inline with line numbers and code colours, folds quiet commands and opens failures. Retake
`kaze-agents.png` and `kaze-board.png` with a subagent that has edited a few files, so the panel or
the board preview shows a diff. Any other shot with the old one-line edit rows or the grey detail
box needs the same. The headless renders in `docs/audits/pane-reading/` are development evidence,
not product captures.

Issue #172 adds System notifications to Settings > Agents > Alerts. Any capture
of that page needs a native retake after owner review. Real OS notification
captures require permission and delivery checks on each platform; do not
substitute a mockup or headless render for a system banner.

Issue #101 changes the session rows, group headers and bottom launcher. All
sidebar-visible product captures need retaking after native owner review. Headless
Gio verification images are development evidence, not replacement product captures.


Native macOS window captures (2200 × 1440 px, Retina) retaken November 1, 2026, from Kaze 0.15 with the redesigned title bar and status bar. Kaze draws its own title bar now, so the captures have no system title bar above it.

- `kaze-sessions.png`: the session sidebar with three demo repos (one a git worktree), the "Todo API"
  workspace running, and its API member floating over the `todo-api` session.
- `kaze-palette.png`: the palette opened over the same session. The skills are five demo
  `SKILL.md` files in the throwaway home folder.
  **Needs retaking:** the palette changed in `feat/palette`. It now opens on sessions first, then
  recent skills. If the lander wants one, take a second shot of the `/` Skills view. Do not fake either shot.
- `kaze-scratch.png` (removed in the 0.27 docs): the scratch shell over the `todo-api` session.
  The scratch shell is gone, so the shot and its gallery tab were dropped.
- `kaze-workspaces.png`: the redesigned Workspaces section of Settings with the Todo API workspace card
  and its API, Web and Health members running.
- `kaze-about.png`: the redesigned About section displaying the shipped logo and byline, version 0.15,
  platform, chip and license facts.
- `kaze-settings.png`: Appearance settings with ten theme cards (two rows of five), Midnight selected, and the live
  terminal preview.
- Settings > Shortcuts has a new row layout (editable keys, a reset button and a "changed" tag). If the
  lander or README uses a Shortcuts screenshot, it needs retaking. Do not fake it.
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

## 0.28.2 retake (November 4, 2026)

Every lander capture was retaken from a demo build of `main` at
Kaze 0.28.2 (2200 × 1440 px, Retina). The pending retakes listed under 0.19 to 0.28 above are done,
apart from the Windows battery shot, which still needs a real Windows laptop.

- `kaze-sessions.png`: the `token-expiry` session (the `todo-api-auth` worktree) running `/fix-tests`
  with three subagents, `dark-mode` waiting on you with its alert card at the bottom right,
  `search-index` done, and the Todo API workspace all up. The panel under the terminal has a subagent
  tab and the API member's tab open, showing the server's log as the Health member polls it.
- `kaze-agents.png`: the same session with the subagent's tab open in a taller panel, on the
  Readable view: its notes with the coral bar, "Read 3 files", an edit with its line counts and
  "Ran the session tests · failed · 1 test".
- `kaze-board.png`: the agents board with Needs you first, then each session's agents.
- `kaze-palette.png`: the palette over `token-expiry`: waiting first, then sessions, servers and skills.
- `kaze-settings.png`, `kaze-workspaces.png`, `kaze-about.png`: Appearance, Workspaces and About.
  `kaze-workspaces.png` also replaces the HTML example table in the Workspaces section.
- `kaze-moment-needs.png`, `kaze-moment-servers.png`, `kaze-moment-done.png` (new): the "See what needs
  you" moments, which used to be an HTML-drawn window. Needs you: the `dark-mode` card over
  `token-expiry`. Your servers: the API member's tab in the panel with its log. Done: the
  `search-index` done card (done cards set to stay until seen for the take). "A subagent working"
  uses `kaze-agents.png`.

How they were made:

- A throwaway home folder with demo projects (`todo-api` with its `todo-api-auth` worktree on
  `feature/auth`, `web-app` on `feature/dark-mode`, `notes-cli` on `fix/search`), a demo git author,
  and a `~/.claude/settings.json` holding kaze's hooks. The owner's settings were not touched.
- The terminal output and the API log are real (git, `go test`, `python3 -m http.server`, a `curl`
  loop). Claude Code was not run: demo shell scripts sent Claude Code's hook input to Kaze's real
  `kaze notify` commands, and the subagents' transcripts were written in Claude Code's JSONL format,
  so the Readable view parsed them the same way it parses Claude's.
- A temporary helper in the demo build (never committed) opened the tabs, the workspace, the panel,
  the board, the palette and Settings, confirmed the workspace's commands, named the session rows,
  and saved the window to a PNG from inside the app. No global keyboard or mouse input was injected.
  No generated or retouched UI.

## Find (#155)

No existing screenshot changes: none shows the find bar. A find shot would be worth adding to the
lander later: the `member` state of the find mockup (`docs/design/find-mockup.html`), the API
member's tab in the panel with the bar open on "warn" and its matches marked. It needs a real run
of the app like the other shots, so none is faked here. The development captures in
`docs/audits/find/` are not for the lander.
- 0.33 put the Kaze mark and name at the left of the title bar on both platforms. Every capture that shows the title bar now shows the old one and needs retaking. Do not fake it.

## 0.46 orchestrator demo captures (November 26, 2026)

The Settings capture was taken at 0.46; the new Update/Edited pills are not shown in any capture.

Four captures retaken from a running macOS demo build of `release/0.46`
at release commit `7638d0e8e0577fbdeb4b77b6a12e991051744160`, at 2200 x 1440 px (1100 x 720 logical, Retina 2x), in Midnight.
Only the Midnight set was retaken for this release.

- `kaze-workers.png`: Claude parent in todo-api with reviewer (Codex, working),
  tests (Claude, idle, green), docs (Codex, waiting, amber), three nested workers,
  the real run/wait output and tagged reports, and the 3 workers status counter.
- `kaze-workers-popover.png`: the same running state with the workers popover
  open, names reviewer, tests and docs visible, and Stop all workers available.
- `kaze-workers-consent.png`: a separate run with confirm = "first", captured
  while the real spawn request was pending, before its 60 second deadline.
- `kaze-workers-settings.png`: Settings > Agents scrolled to show Workers enabled,
  the reviewer template and the Worker instructions control.

How they were made:

- A detached demo worktree and a throwaway home contained only todo-api, web-app
  and notes-cli demo projects, example.com modules and a demo git author. The
  owner's home, settings, agent instructions and installed Kaze were untouched.
  HOME and the launch folder used the same canonical `/private/tmp` prefix, so
  shell-reported folders abbreviate correctly to `~/todo-api`; every status bar
  shows that folder and branch `feature/token-expiry`.
- Claude Code and Codex were not run. PATH-first demo executables named claude
  and codex accepted the planner's real flags, printed illustrative agent output,
  and called Kaze's real notify session/prompt/tool/needs/done and report commands.
  The parent was a demo script in an ordinary owner-role startup tab, running real
  run --wait-start, wait --children, read and ls --children commands. Spawn,
  first-message confirmation, reports, notes, worker states and counts were real
  code paths. The displayed test-result sentences are demo content.
- Orchestration and control were enabled. Main shots used confirm = "never";
  consent shots used confirm = "first". A demo reviewer Markdown template and
  valid throwaway Claude hook settings were loaded from the demo home.
- A temporary helper, never committed, opened the background sessions, dismissed
  alert cards without clearing their marks, opened the workers popover and
  Settings > Agents, and scrolled the real Settings list. It dropped native
  input. An env-gated patch to the private Gio dependency suppressed activation
  and key-window ordering, so the demo stayed behind the owner's applications.
- The helper captured each running window's actual frame ops in process with
  Gio's headless GPU renderer at 2x, then encoded the resulting PNG. This was
  not a fixture render or a macOS screen capture. No UI drawing code or pixels
  were generated, composited or retouched. Parent note lines were filtered from
  the terminal presentation; complete real wait output was retained in the kit.
- Each PNG was inspected for cut-off feature text, stray content and personal
  data. Native demo processes and their children were stopped after capture.
  The helper patch, private Gio patch, scripts, configs and CLI evidence remain
  in `/private/tmp/kaze-release-046-R2-captures/shots-kit/` outside the repository.
  The four final PNGs were copied into `docs/site-assets/`, each under 500 KB.

These captures document a native demo build, not a real-agent acceptance test.
The existing Windows and real-agent owner gates remain open. The Settings footer
shows the compiled 0.46 version; no version was fabricated.

The detached worktree is `/private/tmp/kaze-release-046-R2-demo/`. The demo helper and fresh
private Gio copy were patched there only; no helper or dependency change is part
of the release branch. No global input was injected and no screen capture tool ran.

All other captures remain historical. `kaze-sessions.png`, `kaze-palette.png`,
`kaze-workspaces.png`, `kaze-about.png`, `kaze-settings.png`, `kaze-agents.png`,
`kaze-board.png`, `kaze-moment-needs.png`, `kaze-moment-servers.png` and
`kaze-moment-done.png` still predate the current title bar, project spaces,
sidebar and top tab strip. About still shows its older version. Those shots
need separate retakes; none was changed for 0.46.
