# Spaces and tabs rework: plan

Spec: `docs/superpowers/specs/2026-11-23-kaze-spaces-tabs-design.md`. Mockup: `docs/design/spaces-tabs-mockup.html`.
Branch `feat/spaces-tabs`. Strict TDD (bounded RED tests), JetBrains Mono, no emoji, no Unicode icons, commits
`type(scope): Verb ...` with NO AI attribution (CLAUDE.md wins over any tool reminder). Never push to main.
Tasks run in order; each ends green (`go test ./internal/ui/ ./internal/tabs/`, `go vet ./...`) and is committed.

Code map (internal/ui): model `sidebar_model.go` (`buildSide` 120, `sessionOrder` 204), draw `sidebar.go`
(`layoutSidebar` 197, `sideList` 338), window glue `sidebar_window.go` (`drawSide` 187, `sidePointer` 526,
`closeTab` 123, `afterClose` 137), cache `sidebar_cache.go` (`sideDrawKey` 32), rail `sidebar_rail.go`, title bar
`titlebar.go`/`titlebar_draw.go` (Plus, Search), layout `window.go` `frame` 646 (termRect carve before `dockFit`).

## Task 1: spaces and agents models (pure, no drawing)
Add `spaceModel` (list of spaces: key, name, badge letter and colour from name, tab ids, session count, waiting
count, selected) and `agentsModel` (Needs you / Working / Idle rows: tab id, logo, name, subtitle, age, state)
built from the same inputs as `buildSide`. Add `selectedSpace` and `spaceTabs(space)`. Tests: grouping, Shell
space for repo-less tabs, members excluded, selected follows active tab, closing the last tab removes the space,
agent grouping order. No UI change yet.

## Task 2: tab strip layout and hit model (pure)
`layoutTabStrip(width, tabs, active, scroll)`: tab rects (min width, max 230 dp), close-x rects, + rect, scroll
clamp and reveal-active. Tests: overflow scroll, active revealed, hit tests, bounded sizes.

## Task 3: draw and wire the tab strip
Carve 36 dp from the top of `termRect` in `window.frame` before `dockFit`, recompute `grid`. Draw the strip
(raised active tab with accent line, logo, title, state mark, x, +), pointer events (select, close, new tab in
the space folder, wheel scroll) registered before the terminal's. `closeTab`/`afterClose`/`cycleTab` stay inside
the selected space, then fall to the neighbouring space. Retained ops keyed so unchanged frames allocate 0.
Fix tests whose rects move (dock_render_test, window tests). Add a headless render test behind an env var
(`KAZE_SPACES_RENDER_DIR`).

## Task 4: sidebar redraw
Replace the group+row list with the space list (34 dp lines), add the `SPACES` header with search and + icons,
the `AGENTS` section (click selects space and tab), keep Runs and Settings. Remove the per-row tray. Update
`sideHits`, `sidePointer`, `sidePress`, keyboard order (`sessionOrder` is now spaces), filter field behaviour
(filters spaces and agents), rename (space name display only: rename tab stays on the tab). Extend
`sideDrawKey`/`sideTabInput` so the cache rebuilds only on change; keep the zero-allocation tests green.
Rail mode keeps per-session tiles. Rework or delete tests that asserted the old rows; every deleted test must be
replaced by one for the new behaviour.

## Task 5: title bar
While the sidebar is in full mode, drop Plus and the search field from the bar (the sidebar header has them);
keep them in rail and hidden mode. Update `titlebar` layout tests and caption hit tests (`captionEvent`).

## Task 6: verify and document
Headless renders of the whole window (Midnight and Washi, wide and narrow), compare with the mockup, contrast
checks for the new text, frame time and allocation numbers against `HANDOFF.md` baselines, idle wake count
unchanged. Update README shortcuts if any, `docs/HANDOFF.md` (what shipped, owner checks: macOS title bar,
Windows caption buttons, real agents), mockup link. Full `go test ./...` and `go vet ./...`, native macOS build,
Windows amd64 cross-build.
