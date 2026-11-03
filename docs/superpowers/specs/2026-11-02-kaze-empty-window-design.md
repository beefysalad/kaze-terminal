# Keep Kaze open without sessions (#113)

Approved by the owner in this session on 2026-11-02.

## Behavior

Closing the last terminal, or its plain shell exiting, leaves the window open.
Stopped workspace members keep their existing output and status. Explicit
window close and app quit still exit. Worktree cleanup, Keep/Remove choices and
refusal dialogs keep their existing behavior and remain usable without terminals.
No release, version change, commit or push is authorized for this task.

## Empty state

Keep the existing title bar, sidebar and status bar. In the terminal area show
"No open sessions", a short hint and a "New session" action using existing theme,
typography and button styles. Show the actual configured new-session shortcut.
Settings, Workspaces, the palette and branch launcher remain accessible through
the existing controls. New session works from the empty state using the button,
title/sidebar controls or configured shortcut. Starting a shell can fail visibly
in the status bar and be retried without relaunching the app.

Reuse the existing UI identity. No animation, polling, new persisted setting or
full application redesign. The empty state must fit narrow windows and both dark
and light themes. Closing Settings returns to this state when there is no terminal.

## State and lifecycle

Remove the implicit close-on-empty policy. Run shared housekeeping, input routing
and chrome drawing even with no active tab. Guard terminal-only work and stale
terminal/pane focus. Stop cursor blink invalidations while empty. Drain background
cleanup, updater, workspace and restoration state as usual. Persist an empty
session snapshot so deliberately closed sessions do not return after restart.

## Validation

Bounded regression tests cover shortcut/sidebar final-close, natural exit, opening
Settings and Workspaces, new-session UI and configured shortcut, launch failures,
cleanup refusal/dismissal and explicit window shutdown. Inspect actual headless
Gio renders of the empty state at wide and narrow sizes in Midnight and Washi.
Run full tests and vet, native macOS build and Windows cross-build. Native
installed-app runtime verification is reported separately.
