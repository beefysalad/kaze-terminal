# Docked agent panel (#106, #103): verification

Branch `feat/agent-dock`. Design: `docs/superpowers/specs/2026-11-03-kaze-agent-dock-design.md`.
Plan: `docs/superpowers/plans/2026-11-03-kaze-agent-dock.md`.

## What changed

- Subagents open in one panel docked under the terminal instead of floating cards. The floating
  agent pane code (`paneAgent`, its wrap caches and link code) is gone; workspace member panes and
  the scratch shell are unchanged.
- Activity tab: tool steps (time, tool, target) with the agent's own messages between them as
  markdown notes, in time order (transcript records now carry their timestamp). Waiting box,
  live row and "Finished in Nm · read the report".
- Report tab: the reader's markdown layout, shared through `layoutMarkdown` / `drawMarkdown` at a
  smaller scale (`panelStyle`, body 12 sp). Narrow tables shrink their columns instead of
  overflowing (this also applies to the reader).
- Per-session memory of the open panel; one global height, dragged freely, saved as
  `agent_panel_height` on release. Only the docked session's PTY gets the smaller grid.
- Keyboard: the terminal keeps it; a click in the panel body gives it Esc (close) and ↑/↓
  (previous/next subagent). A click in the terminal takes it back.
- Entry points: sidebar agent line (toggles, highlighted while open), board card, and the
  "needs you" toast's Go (opens the earliest waiting subagent of that session).
- Open in reader: the report as an in-memory document under the session; it never polls a file.
- Prompt indent: `kaze notify tool` no longer trims a subagent prompt's leading whitespace.

## Measurements

`KAZE_DOCK_BENCH=1 go test ./internal/ui/ -run TestDockFrameTiming -v` on the owner's Windows
laptop, 1000x600 test frame, a 100 KB generated report (larger than the 24 KB hook cap, as the
worst case):

| Frame | Time |
|---|---|
| First frame (parse + layout) | 6.95 ms |
| Scrolled frame | 0.41 ms |
| Still frame, panel open | 0.42 ms |
| Frame with the panel closed | 0.11 ms |
| Parses over 201 frames | 1 |

`TestDockLongReportScrollNoReparse` (always on) checks a scrolled report parses nothing new;
`TestDockNotesParseOnce` checks a growing transcript parses only new notes; `TestDockIdleNoWake`
checks a finished agent's panel stops polling once its transcript was read after the end.
A closed panel does no work: no reads, no wakes.

## Development captures

Headless Gio renders of the real frame (`KAZE_DOCK_RENDER_DIR=<dir> go test ./internal/ui/ -run
TestDockRenders`). Development evidence only, not product screenshots.

| Theme | Running | Waiting | Report |
|---|---|---|---|
| Midnight | [capture](agent-dock/midnight-running.png) | [capture](agent-dock/midnight-waiting.png) | [capture](agent-dock/midnight-report.png) |
| Washi | [capture](agent-dock/washi-running.png) | [capture](agent-dock/washi-waiting.png) | [capture](agent-dock/washi-report.png) |

Text colours in the panel are nudged to at least 4.5:1 on both the header and the body in all
five themes (`TestDockContrast`).

## Checks

- `go test ./...` and `go vet ./...` pass on Windows (Smart App Control needed `GOTMPDIR` under
  the Desktop and fresh build ids for some test binaries).
- macOS: build and tests run in PR CI; not run by hand.

## Owner checks still open

- Real Claude sessions with several subagents: the timeline order, notes and the report switch.
- Dragging the handle on macOS and Windows; the saved height after a restart.
- Esc in the terminal with the panel open (Claude's interrupt) on both platforms.
- The lander and README screenshots that show floating agent cards need retaking
  (`docs/site-assets/SCREENSHOTS.md`).
