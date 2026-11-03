# Docked agent panel (issues #106 and #103)

Status: design and mockups approved by the owner on 2026-11-02 (issue #106). The three
open details were settled on 2026-11-03 (section 3). This document is what to build.

Mockup: `docs/design/agent-pane-dock-mockup.html` (clickable) and the three PNGs beside it
(`agent-pane-dock-running.png`, `-waiting.png`, `-report.png`). Where this text and the
mockup disagree on behaviour, this text wins; on look (spacing, colours, type), the mockup wins.

## 1. Why

Today each subagent opens as a floating card over the terminal. With three or four agents the
cards pile up, cover the session you are working in, and are hard to read. Their text is raw
markdown (`**bold**`, backticks, `- ` bullets), because the panes never use the markdown
renderer that the doc reader already has (#103).

The panel has to answer one question at a glance: **what has this agent been doing?**

## 2. Goals, non-goals, anti-goals

Goals
- One agent at a time, docked **under** the terminal, opened from its sidebar line.
- An **Activity** tab: a timeline of tool steps with the agent's own messages between them.
- A **Report** tab: the report rendered as markdown, with clickable links.
- Prompt, messages and report render markdown (#103).
- The floating agent cards are removed.

Non-goals
- More than one agent panel open at once.
- Animating the running ring (#107, separate).
- Floating cards as an option. They go away entirely.

Anti-goals (a failure even if it works)
- Idle cost. A closed panel, or an open panel on a finished agent, must cost nothing per frame
  beyond today's terminal. No tickers, no polling when nothing can change. Same rule as the
  existing pane: transcript reads happen only while the panel is visible.
- Re-parsing or re-wrapping the whole transcript on every frame. Layout is cached and only
  redone when content, width or metrics change.
- Stealing keys from Claude Code. `Esc` and the arrow keys belong to the terminal unless the
  panel has the keyboard (section 5).
- Drawing anything over the terminal.

## 3. Settled details (owner, 2026-11-03)

1. **Switching tabs:** the open panel belongs to its session (tab). Switching to another session
   hides it; switching back shows the same agent's panel again, on the same tab (Activity or
   Report) and scroll position. Each session remembers at most one open agent. Closing the
   session forgets it. If the remembered agent is gone from the tracker, nothing opens.
2. **Floating cards are replaced completely.** Everything that opened an agent's floating pane
   (sidebar agent line, "needs you · Go ›" toast, palette if it does) opens the dock instead.
   Floating panes for **workspace members** are unrelated and stay exactly as they are.
3. **Height is freely adjustable and remembered.** Drag the handle between terminal and panel to
   any height. One height for all sessions, saved to `config.toml` (new key, e.g.
   `agent_panel_height`, in dp) the same way `sidebar` is saved, written when a drag ends (not on
   every pointer move). Limits: the panel is at least about 120 dp tall, and the terminal keeps
   at least about 8 rows. If the window is too small for both, the terminal's minimum wins and
   the panel shrinks; the saved value is not changed by a small window. Default when unset:
   about 40% of the terminal area.

## 4. Layout

- The panel spans the full width of the terminal area (not under the sidebar), below the
  terminal, above the status bar. The terminal area shrinks by the panel height, so the PTY
  gets the smaller size (one resize when the panel opens, closes or a drag ends; during a drag,
  resizing the PTY on every move is not required, but the drawn split follows the pointer).
- **Drag handle:** a thin bar between terminal and panel with a resize cursor.
- **Header:** state mark, agent type, task, elapsed ("<1m", "4m", "took 6m", "waiting for you"),
  the **Activity / Report** switch, then **Copy report**, **Open in reader** and **×**.
  Copy and Open in reader appear only once the agent is done. The Report button can be pressed
  while running; it shows "No report yet" until there is one.
- **Prompt row:** one muted line, "▸ Prompt: <first line>". Click toggles it open to the full
  prompt rendered as markdown; click again collapses. Starts collapsed.
- **Body:** the selected tab, scrollable.

## 5. Opening, closing and keys

- Click an agent line in the sidebar: opens its panel (switching to its session first if
  needed). Click the same line again: closes it. Click another agent line: switches the panel
  to that agent. The open agent's sidebar line is highlighted.
- **×** closes. Closing does not change the remembered height.
- **Keyboard (builder's call, flagged for the owner):** the terminal keeps the keyboard when the
  panel opens, as today's floating agent pane does. `Esc`, `↑` and `↓` reach the panel only
  after the user clicks into the panel body; then `Esc` closes the panel and `↑`/`↓` move to the
  previous/next agent of the same session in sidebar order. Clicking back in the terminal gives
  it the keyboard again. This keeps `Esc` (Claude's interrupt) and arrows safe in the terminal.
- **Panels in other sessions:** state is per session (decision 3.1). With agent X open in
  session A, clicking agent Y under session B switches to B and opens Y there; A still
  remembers X and shows it again when you go back to A.
- **"needs you · Go ›" toast (builder's call, flagged for the owner):** alerts stay per session;
  no agent key is added to the alert path. At click time, Go switches to the session as today,
  then looks in the tracker for that session's subagents in the Waiting state: one opens its
  panel; several opens the one that started waiting first (oldest `Started` among them is fine);
  none (the main conversation is the one waiting) just switches, as today.
- **Workspace member sessions (builder's call, flagged for the owner):** the dock always sits
  under the main view's terminal and is owned by the session shown as the main view. Clicking an
  agent line under a member row first shows that member as the main view (as Expand does today),
  then opens the dock under it, owned by the member. Floating member panes keep their geometry
  relative to the terminal area; since that area now ends above the dock, they are clamped by the
  existing fitting and never cover the dock.
- Settings, documents and the board cover the panel the same way they cover the terminal.

## 6. Activity tab (default while running)

- Rows: time (clock, e.g. "14:02"), tool (Read, Grep, Bash…), target, the same look as today's
  pane rows, in the mono face. Source: `Agent.Steps` (bounded by `maxSteps`, with "… N earlier
  steps" at the top when `Dropped > 0`).
- The agent's own messages appear **between** the rows, in time order, as notes: sans font,
  accent bar on the left, rendered markdown. Source: the transcript reader
  (`subagents.Transcript`), same caps as today (128 entries / 128 KiB, 256 KiB per read, read
  once a second only while visible). To interleave, transcript messages need a timestamp: read
  the record's `timestamp` field into `TranscriptMessage` (builder's call how; a message with no
  usable time goes after the last step before it in file order).
- Codex agents and agents with no readable transcript show steps only.
- While running, a live last row "… working" follows the bottom unless the user scrolled up
  (same follow rule as `agentScroll` today).
- Waiting for approval: an amber box, "Wants to run `<command>`. Approve it in the session
  above." (command from the latest step's summary when it is a Bash step; otherwise "Waiting
  for your approval in the session above.").
- Finished: a final row "✓ Finished in Nm · read the report"; "read the report" switches to the
  Report tab.

## 7. Report tab

- The report rendered with `internal/mdoc` and the doc reader's layout (`doc_layout.go`):
  bold, italic, code, strike, headings, lists (bullet, numbered, task), quotes, fenced code,
  rules and tables, at sizes that fit the panel (smaller than the reader's).
- Tables wider than the panel: the builder may wrap cells or clip with sideways scroll; it must
  not overflow the panel or break layout.
- Links: web links open in the browser; `file.go:12` and `.md` paths open the same way they do in
  the terminal (same modifier rule as today's pane links, `panes_agent_links.go`).
- When the agent finishes, the panel switches to Report automatically, **unless the user picked
  a tab themselves** for this agent.
- Report text follows today's fallback chain (hook, SubagentHandback, last text, "No report").
- **Copy report** copies the original markdown, never the rendered text.
- **Open in reader** opens the report in the existing markdown reader as a read-only document
  nested under the session, titled "<type> · <task>". It has no file on disk, so reload checks
  do not apply.

## 8. Markdown everywhere (#103)

- Prompt (expanded), Activity notes and the Report all render through `mdoc.Parse`.
- Parsing is cached per text: a note or report is parsed and laid out once, and again only when
  its text, the width or the metrics change. A growing transcript re-parses only new or changed
  messages, never all of them.
- Bench check: a long report (about 100 KB of markdown) must open and scroll without dropped
  frames; record the numbers in a new audit note, `docs/audits/2026-11-03-agent-dock.md`
  (same style as the other notes in `docs/audits/`).
- Prompt whitespace (comment on #106): `cmd/kaze/notify.go` must stop trimming the prompt's
  leading whitespace. Use `notify.FitReport(prompt, ...)`, and let the panel trim blank lines
  only (`trimBlock`), so a prompt that opens with an indented block keeps its indent.

## 9. What goes away

- The floating agent pane (`paneAgent` in `internal/ui/panes_*`): its open path, drawing, wrap
  caches and tests move to the dock or are deleted. Workspace member panes keep working.
- No config for floating agent cards remains.

## 10. Constraints

- Strict TDD, RED tests fail safely (bounded sizes, no unbounded loops).
- No global keyboard or mouse input in tests.
- All five themes: text in the panel meets 4.5:1 contrast like the rest of the chrome.
- Windows and macOS from the same code.
- No new dependencies (goldmark is already in via `internal/mdoc`).

## 11. Left to the builder

- Exact sizes, paddings and fonts, following the mockup.
- Where the dock code lives (a new `internal/ui/dock_*.go` is fine) and how much of the pane code
  it reuses.
- How the doc reader's layout is generalised to take in-memory markdown at a panel-sized scale.
- Whether the PTY resizes live during a drag or once at the end.
