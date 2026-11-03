# Empty window (#113), 2026-11-02

Implemented locally with the owner's approved design and plan. No release,
version bump, commit or push. Issue #114's Shift+Enter fix remains in the same
working tree for the owner's later bundle; unrelated original changes remain.

## Result

Closing the final terminal leaves Kaze open with the existing chrome and a simple
empty state: "No open sessions", a New session button and the configured shortcut.
The final plain shell exiting follows that path too. Exited workspace members
retain their existing stopped status and output. Explicit window close and app
quit remain native. The optional benchmark's automatic exit remains supported.

The normal frame pipeline now runs with zero tabs: Settings and Workspaces,
launch controls, status notices, cleanup dialogs and shared housekeeping continue
to work. Terminal-specific selection/focus work handles nil tabs. The cursor does
not schedule blink frames without a terminal. Input filters are installed before
giving a newly opened page or terminal the keyboard. Empty snapshots are saved
when sessions close, so the removed session is not restored later.

## Verification

Bounded regression tests were run failing before fixes, then passing:

- Close the final session by the configured shortcut and the hovered sidebar ×.
- Open a new session through the empty-state button, Enter on its focused button,
  configured shortcut and title-bar plus; confirm terminal keyboard focus.
- Open/close Settings and launch a saved workspace from Settings while empty.
- Return to the empty state after the final plain shell exits naturally.
- Keep a launch failure visible in the status bar and retry successfully.
- Preserve worktree cleanup refusal/dismissal and cancel a deferred check with Keep.
- Persist zero sessions after closing the final saved session.
- Preserve the explicit window-close action and existing shell shutdown tests.
- Stop empty-window cursor blinking; secondary text reaches at least 4.5:1 in all
  five themes. The initial contrast test caught Sumi (4.00) and Washi (4.16).

Final checks passed:

```sh
go test ./...
go vet ./...
go test -race ./internal/ui -run 'TestEmptyWindow|TestFinalSession|TestFinalShell|TestCleanupRefusal|TestDeferredClose|TestShiftEnter|TestClosingTheWindow' -count=1
go build -o /tmp/kaze-issues-113-114-macos ./cmd/kaze
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/kaze-issues-113-114-windows.exe ./cmd/kaze
```

## Visual evidence

Actual full-frame Gio headless renders at 1000×600 and 400×340:

- [Midnight, wide](empty-window/midnight-wide.png), [narrow](empty-window/midnight-narrow.png)
- [Washi, wide](empty-window/washi-wide.png), [narrow](empty-window/washi-narrow.png)

These are development evidence, with fixture caption controls and host-platform
shortcut labels. They are not native product captures or lander screenshots.
The independent UI review found secondary-text contrast below 4.5:1; a local
foreground adjustment resolved that finding. The same four renders were recaptured
and inspected, and the reviewer scored the contrast fix resolved.

To regenerate (absolute output path):

```sh
KAZE_EMPTY_RENDER_DIR="$PWD/docs/audits/empty-window" \
  go test ./internal/ui -run '^TestEmptyWindowRenders$' -count=1
```

## Remaining native checks

Installed macOS and Windows: close the final session, exit its plain shell, start
again from the empty state, open Settings/Workspaces, and explicitly close/quit.
Check cleanup choices with real linked worktrees. Headless tests and cross-builds
do not establish those native interactions. No global keyboard/mouse injection was
used and the installed app was not replaced.
