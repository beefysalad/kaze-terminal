# kaze: notes for Claude Code

kaze is the owner's personal, lightweight terminal emulator, built around running Claude Code all day.
Go + Gio, Windows and macOS from one codebase, "Midnight" look. This repo is **public**.

**Start every session by reading `docs/HANDOFF.md`**: current status, decisions, known gaps, the Mac
checklist and what's next. The authority for behaviour is the spec
`docs/superpowers/specs/2026-10-28-kaze-terminal-design.md` (section 6.8 = the next plan).

## Layout

| Path | What lives there |
|---|---|
| `cmd/kaze` | main, fatal-error message box |
| `internal/vt` | escape-sequence parser (Williams state machine, allocation-free) |
| `internal/screen` | cell grid, dirty rows, scrollback ring, alt screen, resize |
| `internal/term` | Applier (vt.Handler → screen), modes/replies, Session (PTY + parser + screen, locking, notifier) |
| `internal/ptyx` | the only package that imports go-pty (ConPTY / Unix pty) |
| `internal/render` | CPU-drawn grid in 8 strips, glyph cache, box drawing, font fallback, contrast, selection |
| `internal/input` | key / paste / mouse / focus encodings |
| `internal/tabs` | tab manager, shell env (`KAZE_TAB_ID`, `TERM`, ...) |
| `internal/ui` | Gio window, tab bar, status bar, routing, selection gestures |
| `internal/config` | `config.toml` loading with warnings |
| `internal/bench`, `bench/` | in-app benchmark mode (`KAZE_BENCH`) and harness scripts |
| `spike/gio` | throwaway rendering spike; not product code |

## How we work here

- **Brainstorm before building** anything new; get the owner's yes on a written design, then a plan
  (specs in `docs/superpowers/specs/`, plans in `docs/superpowers/plans/`).
- **Strict TDD:** write the test, watch it fail (RED), then make it pass (GREEN).
- **RED tests must fail safely.** A failing test must never allocate unbounded memory or loop forever:
  bounded sizes, timeouts. (An unbounded RED test once crashed the machine.)
- **Never inject global keyboard or mouse input** (SendInput, SendKeys, posting to other windows) when
  testing the GUI. The owner works on the same machine; stray keystrokes have landed in their live
  sessions before. Test through the code paths, or use the bench mode, which drops real input by default.
- **Commits:** `type(scope): Verb ...` (e.g. `fix(render): Keep text readable on colored backgrounds`).
  **No AI attribution** of any kind (no `Co-Authored-By`, no "Generated with").
- **Never push to `main`.** Branch, push the branch, open a PR. The owner merges.
- **Public repo:** never commit personal or work details (names, employer, ticket keys, local user paths,
  emails, secrets). Use neutral examples like `feature/TICKET-123` and `%USERPROFILE%` / `~`.
- **Talk to the owner in plain language**, lead with a recommendation, keep it short.

## Build and test

- Go version: see `go.mod` (1.27). macOS also needs the Xcode Command Line Tools (`xcode-select --install`), because Gio uses cgo on darwin.
- macOS: `go build -o kaze ./cmd/kaze && ./kaze`
- Windows: `go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze`
- Tests: `go test ./...` and `go vet ./...`. Run one package at a time when chasing a failure.
- Windows only: Smart App Control can block freshly built test binaries or `kaze.exe`
  ("An Application Control policy has blocked this file"). Rebuild with a changed build id,
  e.g. `go test -ldflags "-s -w -X main.r=<random>" ./internal/<pkg>/`. Don't turn Smart App Control off.
