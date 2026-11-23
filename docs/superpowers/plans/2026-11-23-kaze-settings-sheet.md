# Settings sheet: plan

Spec: `docs/superpowers/specs/2026-11-23-kaze-settings-sheet-design.md`. Branch `feat/settings-sheet` (on top of
`feat/spaces-tabs`). Strict TDD with bounded RED tests, JetBrains Mono only, no emojis or Unicode icons, commits
`type(scope): Verb ...` with NO AI attribution. Never push, never touch main. Each task ends green
(`go test ./internal/ui/`, `go vet ./...`) and committed.

Code map (internal/ui): `settings.go` (`drawSettings` ~497, `set.open`, key routing ~208-270), `window.go` (~856
where `drawSettings(gtx, termRect)` is called and the terminal is skipped), `palette_window.go` (`drawPalette` ~523,
the overlay, dismiss and focus pattern to copy), `settings_*.go` (section content, rect-based).

## Task 1: the sheet
Add `sheetRect(window, metrics)` (centred, max 880 x 640 dp, falls back to the full window under 900 x 680 dp) as a
pure function with tests. Draw the terminal as normal while Settings is open, then the dimmed backdrop and the
sheet (border, rounded corners, shadow) with the existing section menu and content laid out in the sheet rect.
Esc and a backdrop click close it; the terminal takes no input while open; focus returns to the terminal that had
it. Keep a backdrop-click that lands inside the sheet from closing it. Frame: 0 allocations unchanged, no timers.

## Task 2: retune, test, document
Check every section (Look, Keyboard, Machines/Workspaces, Alerts, Updates, Privacy/About and any others present)
at sheet width and at the narrow fallback: no clipped text, scrolling works, editors keep focus inside the sheet.
Update tests that assumed the page rect. Headless renders (env `KAZE_SHEET_RENDER_DIR`) of the sheet over a
terminal in Midnight and Washi, wide and narrow; inspect them. Update `docs/HANDOFF.md` (a short paragraph, owner
checks: real Mac and Windows window, a real keyboard Esc, resize while open). No version bump, lander or changelog.
