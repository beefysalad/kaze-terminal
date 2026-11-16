# Sidebar side and rail: design

Date: 2026-11-16. Branch: `feat/sidebar-right-rail`.
Mockups: `docs/design/sidebar-v3-mockup.html` (all options),
`docs/design/sidebar-right-prototype.html` (the clickable prototype the owner asked for).

## Intent

The owner wants the sidebar on the right, and a way to keep session status in view
while giving the terminal more room. Brainstormed on 2026-11-16: the owner skipped the
questions, reviewed six layouts, asked for the rail to sit on the same side as the
sidebar, and asked for a clickable version of "right, full" combined with "rail". This
design is that prototype, made real.

## Goals

- A setting for the sidebar's side: **left** or **right**.
- Three widths, cycled by the existing `toggle_sidebar` key (Cmd+B / Ctrl+Shift+B):
  **full** (today's sidebar), **rail** (52 dp of session tiles), **hidden** (today's hidden).
- The rail always sits on the same side as the sidebar. Every side and width combination
  behaves the same way, mirrored.
- The full sidebar's content does not change: filter, repo groups, two-line rows, agent
  tray, Workspaces, docs rows. Only its position and the mirrored details move.

## Non-goals

- No restyle of rows, groups, the tray or Workspaces.
- No flyout (the rail opening the full sidebar over the terminal) and no floating panels.
  Both were shown in the mockup and not picked.
- No resizable sidebar width.
- No change to the dock, floating panes, toasts or the palette, beyond following the new
  terminal rectangle they already take.

## Anti-goals

- The rail must not cost idle CPU: no ticker, no per-frame shaping, no allocations on an
  unchanged frame (the full sidebar already holds this line; the rail must too).
- Existing `config.toml` files must keep working with no warning: `sidebar = true|false`
  still loads.
- Nothing about the left layout may change for someone who never touches the new setting.

## Constraints

- One font, JetBrains Mono; no emoji or Unicode-symbol icons. Rail tiles use the existing
  agent logos and state marks (`stateDot`, the agent logo drawing in `agentlogo.go`).
- Strict TDD; RED tests bounded.
- Both platforms: macOS (traffic lights, native title bar area) and Windows (caption
  buttons at the top right).

## Approaches considered

1. **Side setting plus a three-state width (picked).** Mirror the existing sidebar and add
   a rail drawing that reuses the sidebar model. Small surface, one look to maintain,
   covers both asks.
2. **Rail with flyout.** The rail opens the full list over the terminal. No reflow, but an
   overlay layer with focus and click-away rules. Not picked.
3. **Split panels** (sessions one side, agents and servers the other) and **floating
   panels**. Bigger change or cosmetic only. Not picked.

## Design

### Settings and config

- New key `sidebar_side = "left" | "right"`. Default `"left"` (today's layout; the owner
  sets right once). Any other value warns and falls back to left.
- New key `sidebar_mode = "full" | "rail" | "hidden"`. Default `"full"`.
  When `sidebar_mode` is absent, the old `sidebar` bool decides: `true` gives full,
  `false` gives hidden. Kaze writes only `sidebar_mode` from now on; `sidebar` stays a
  quiet, accepted key. An invalid `sidebar_mode` warns and falls back to the old rule.
- Settings, Appearance gets a **Sidebar** heading with two segmented rows:
  - **Side**: Left / Right. Hint: "Which edge of the window the sidebar sits on."
  - **Width**: Full / Rail / Hidden. Hint: "Cmd+B cycles these." (Ctrl+Shift+B on Windows;
    the hint shows the bound key, like other hints do.)
  Changes apply at once and are saved with `saveSetting`.
- The palette gets "Sidebar on the left" and "Sidebar on the right" commands next to
  "Toggle sidebar". "Toggle sidebar" keeps its id and name (`toggle_sidebar`); its action
  becomes the cycle.

### The cycle

`toggle_sidebar` steps full, rail, hidden, full. It saves `sidebar_mode` each time.

Narrow windows: today, under 560 dp (`minSideWindow`) the sidebar hides without saving.
New rule: under 560 dp a **full** sidebar shows as the **rail** (not saved); rail stays
rail; hidden stays hidden. Under the rail's own minimum (300 dp window width) it hides.
While the window's width overrides the mode, the toggle still cycles the saved mode, so
the owner sees the effect when the window grows. (Today's no-op rule in `toggleSidebar`
goes away.)

### Window layout (`window.go`)

- `sideWidth` returns the width for the effective mode: 260 dp full, 52 dp rail, 0 hidden.
- `sideRect` sits at `x = 0` for left, at `x = size.X - w` for right. `termRect` takes the
  rest. Everything that already lays out from `termRect` (terminal grid, dock, floating
  panes, find bar, toasts and alert cards, settings, board, docs, palette) follows with no
  change of its own.
- `drawSide` draws the full sidebar or the rail into `sideRect`.

### Full sidebar, mirrored for the right

- The divider line moves from the sidebar's right edge to its left edge
  (`sidebar.go`, the 1 px border fill).
- The active row's 2 dp accent line moves to the row's outer edge, the window side.
- Everything inside the rows stays left to right as today (text does not mirror).

### Title bar (`titlebar.go`, `layoutTitle`)

The title bar's column above the sidebar (the "lead" strip, with its divider) follows the
sidebar's side and width.

- **Left** (today): unchanged. Rail mode: the lead strip is the width the lights, mark and
  toggle need (it already grows past the sidebar width with `max`), and the + button is not
  shown.
- **Right**: the macOS lights, the K mark and the "Kaze" word stay at the left, then the
  crumb and search. The lead strip moves to the right edge, its divider over the sidebar's
  left edge. Inside it, right to left: the Windows caption buttons (Windows only), the
  toggle, the gear, the agents button, and in full mode the + at the strip's left end.
  The strip is at least the sidebar's width and grows to fit its buttons (so in rail mode
  it is wider than the rail; the mockup shows about 120 dp).
- The toggle icon mirrors with the side (panel line on the left or the right).
- Hidden, right side: the toggle, gear and agents stay at the top right with no divider.

### The rail

A new drawing, `layoutRail`, from the same `sideModel` the full sidebar uses, so order,
grouping, folding and states match.

- 52 dp wide, `Chrome` background, divider on the inner edge.
- Per repo group: a short label (the group name cut to fit 44 dp, Muted, 9.5 sp) then one
  36 dp tile per session, then a 24 dp divider before the next group. Folded groups still
  show their tiles (folding is a full-sidebar affordance).
- A tile: the session's agent logo (18 dp), and its state mark in the bottom-right corner
  on a `Chrome` disc (running ring, needs you, done, idle: the same marks the rows use).
- The shown session's tile has the `Active` fill and the 2 dp accent line on the window
  edge, as rows do.
- Hover: a tooltip beside the tile, toward the terminal: state mark, title, and the row's
  status word ("needs you", "done", "2 agents"), using the agent tray tooltip's style.
  Hover changes only redraw when the hovered tile changes.
- Click a tile: show that session (same call as a row click). Middle click closes it
  (same as a row). No rename and no filter in the rail.
- Workspaces: at the bottom, one 4x16 dp colour bar per running member, in member order;
  a stopped member's bar is drawn in the stopped red. Click a bar: show that member (same
  as clicking a member line). Hover: tooltip with the member's title and status.
- Docs rows and the agent tray are not in the rail. When a doc or agent panel of a session
  is shown, that session's tile is the highlighted one.
- Settings: while Settings is open, a gear tile sits at the top of the rail, highlighted
  when Settings is shown; click shows Settings.
- Too many sessions: the tile list scrolls with the wheel, like the full list;
  the Workspaces bars stay pinned at the bottom.
- Retained drawing with a cache key, like `sidebar_cache.go`, so an unchanged frame does
  no shaping and no allocation.

## Testing

- Config: load and save of `sidebar_side` and `sidebar_mode`, the old `sidebar` fallback,
  invalid values warn.
- Cycle: full, rail, hidden order and saving; the narrow-window rules.
- Layout: `sideRect` and `termRect` for each side and width; `layoutTitle` for right and
  rail on macOS and Windows (lead strip position, button order, no overlap with crumb and
  search, the + only in full).
- Mirroring: divider and accent line on the right side.
- Rail: tile order matches `sessionIDs`, hits for tiles and bars, hover tooltip hit,
  scroll bounds, zero allocations on an unchanged frame.
- Opt-in headless renders (like `TestSidebarRedesignRenders`) of both sides and widths,
  for review before the PR; not product screenshots.
- Real-app check by the owner on both platforms (macOS title bar especially).

## Release

User-facing feature: after merge, a minor release (0.43) per CLAUDE.md: version, changelog,
lander "What's new", README sidebar section. Screenshots with the sidebar need no retake
for the default (left) layout.

## Left to the builder

- Exact tooltip offset, label cut, and the rail's tile spacing within the sizes above.
- How `layoutRail` shares code with `layoutSidebar` (shared helpers versus a separate file).
- Test file layout and names.

## Calls made in this design (owner approved the spec on 2026-11-16; default side confirmed left)

1. Default side stays **left**; the owner switches to right in Settings once.
2. A narrow window shows the rail instead of hiding a full sidebar.
3. The rail shows tiles of folded groups too.
4. Two new palette commands for the side.
5. Config uses a new `sidebar_mode` key and keeps reading the old `sidebar` bool.
