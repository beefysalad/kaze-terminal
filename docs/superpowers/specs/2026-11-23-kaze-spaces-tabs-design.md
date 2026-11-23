# Spaces in the sidebar, tabs on top (design)

Date: 2026-11-23. Approved by the owner from `docs/design/spaces-tabs-mockup.html` ("do the ui rework").
Builder's calls are marked **(call)**.

## Intent

The owner likes herdr's layout: the sidebar lists spaces, only the selected space's terminals show as tabs on
top, and agents have their own section. Kaze today lists every session in the sidebar, grouped by repo, with
a subagent tray under each, and has no tab strip. This rework changes the layout, not what sessions are.

## Model

- **Space = today's repo group** (`buildSide`, key `loc.repo`). Tabs with no repo go to one space named
  "Shell" **(call)**. A space exists while it has a session; closing its last tab removes it and selects the
  neighbouring space **(call)**.
- **Selected space** = the space of the active tab. Selecting a space activates its last-used tab.
- **Tabs in the strip** = that space's sessions (not workspace-run members; those stay in the Runs section
  and the dock panel as today).
- **Agents section** = every session with a known agent, grouped Needs you, Working, Idle, from the existing
  `rowAgent` and `alerts.markOf` state. A click selects that session's space and tab.

## Layout

- Sidebar (full mode): header row (traffic-light gap handled by the title bar as today), then `SPACES` label
  with search and + icons at the right, the space list (one 34 dp line each: badge, name, count, amber count
  when a session waits), the Runs section as today, the `AGENTS` section, the Settings row.
- Tab strip: 36 dp, carved from the top of `termRect` before the dock panel is fitted. Tab = agent logo, title,
  state mark, close x. Active tab is raised, 2 dp accent line on its top edge. A + at the end opens a new tab
  in the space's folder. Tabs scroll horizontally when they overflow **(call: shrink to a minimum width, then
  scroll)**.
- Sidebar + opens a new session (today's `newTab`), which starts a new space when the folder is elsewhere.
- Title bar: Plus and the search field leave the bar while the sidebar is in full mode (search, plus live in
  the sidebar header). In rail or hidden mode they stay in the bar as today **(call)**.
- Rail mode keeps one tile per session in this release **(call, follow-up: one tile per space)**.
- The subagent tray under rows is removed from the sidebar; subagents stay in the dock panel and the board.

## Goals, non-goals

Goals: the mockup's layout, no new idle wakeups, 0 allocations per unchanged sidebar frame, JetBrains Mono,
drawn icons, no emoji. Non-goals: renaming or persisting spaces, drag to reorder, the agent-to-agent control
socket (parked), rail per space.

## Approaches considered

1. **Derive spaces from the existing grouping (chosen).** No new state; the strip is a filter of sessions.
   Smallest change and keeps session restore, workspaces and alerts untouched.
2. First-class space objects in `tabs.Manager` (named, persisted). More power, but changes session restore and
   workspace code; deferred until spaces need names of their own.
3. Keep the sidebar and only add a strip. Does not give the herdr look the owner asked for.
