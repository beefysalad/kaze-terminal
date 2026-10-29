# kaze Plan 4: Claude usage in the status bar

**Goal:** Show Claude's 5-hour and weekly plan usage and the active tab's context window in kaze's status bar.

**Architecture:** Claude Code runs one status line command and pipes it session JSON on every update. `kaze statusline` takes that slot: it prints a short line for Claude's own status row and, when run inside a kaze tab, sends the numbers to kaze over the existing alert listener as a new `usage` message. The UI keeps account-wide limits (latest from any tab) apart from per-tab context and draws them right-aligned before the memory text.

**Tech stack:** Go 1.27, Gio v0.10.3, the existing `internal/notify` listener.

**Spec:** `docs/superpowers/specs/2026-10-28-kaze-terminal-design.md` section 6.5 (with the 2026-10-29 note).

## Global constraints

- Owner's scope (2026-10-29): 5-hour, weekly and context only. No per-model (Fable) usage, no cost, no process tree in this plan.
- `kaze statusline` always exits 0, never blocks longer than about 300ms, and prints at most one line.
- Usage messages never chime, flash, mark a tab or replace a pending alert.
- Zero idle CPU: no ticker. One timed wake for the next window reset only.
- Times in 12-hour form (`3:40 PM`).

## Review focus

1. A usage message arriving right after a "done"/"needs" must not swallow that alert (the inbox keeps one message per tab).
2. `rate_limits` absent (API key users, first turn) or `context_window.used_percentage` null: show what exists, never "0%".
3. Window past its `resets_at`: hidden at once, even on an idle window.
4. Claude exits back to the shell: the tab's context must disappear (prompt OSC 7 after the last usage).
5. Hostile or odd values over the socket (negative, huge, NaN-free JSON floats): clamp to 0..100 before drawing.

---

### Task 1: Usage message and formatting

**Files:** `internal/notify/notify.go`, `internal/notify/notify_test.go`, create `internal/usage/usage.go`, `internal/usage/usage_test.go`

**Interfaces (produces):**
- `notify.KindUsage Kind = "usage"`; `notify.Window{Pct float64 "pct"; Resets int64 "resets"}`; `notify.Usage{FiveHour, Week *Window; Ctx *float64}` (json `five_hour`, `week`, `ctx`, all omitempty); `Msg.Usage *Usage "usage,omitempty"`.
- The server accepts `usage` only with a non-nil `Usage`; for `done`/`needs` it drops any `Usage`.
- `usage.Level` (`Normal`, `Warn` at >= 80, `High` at >= 95); `usage.Segment{Text string; Level Level}`.
- `usage.Segments(u notify.Usage, now time.Time) []Segment`: order 5h, wk, ctx; labels `5h 23%`, `wk 41%`, `ctx 34%`; percentages clamped to 0..100 and rounded down; a window with `Resets > 0` and `now >= Resets` is left out; a window at Warn or above gets ` · resets 3:40 PM` (weekday prefix `Mon 3:40 PM` when the reset is 24h or more away, local time).
- `usage.Line(u, now) string`: segment texts joined by `  ·  `, "" when none.
- `usage.NextReset(u, now) time.Time`: earliest future `Resets`, zero when none.

Tests: server round trip of a usage message; server drops `usage` without payload and strips payload from `done`; Segments ordering, clamping (-5 → 0%, 250 → 100%), expiry, Warn/High thresholds and reset text, absent fields; NextReset.

### Task 2: `kaze statusline`

**Files:** create `cmd/kaze/statusline.go`, `cmd/kaze/statusline_test.go`; modify `cmd/kaze/main.go`

**Interfaces:** `runStatusline(stdin io.Reader, stdout io.Writer, env func(string) string, now time.Time) int`. Reads stdin with `readStdinLimited` (300ms), parses `rate_limits.five_hour|seven_day.{used_percentage,resets_at}` and `context_window.used_percentage` (null = absent), writes `usage.Line` plus newline when non-empty, then sends `KindUsage` with the same env checks as `kaze notify` and whatever is left of a 300ms budget. Panics recovered, exit 0 always.

Tests: prints the line from a full sample; no `rate_limits` prints only ctx; null context omitted; sends one usage message to a test server from a kaze tab; outside kaze prints but does not dial; garbage stdin prints nothing, exits 0.

### Task 3: Status bar

**Files:** modify `internal/ui/alerts_ui.go` (route usage away from the alert inbox), create `internal/ui/usage.go` + `internal/ui/usage_test.go`, modify `internal/ui/status.go`, `internal/ui/tabbar.go`, `internal/ui/window.go`

**Interfaces:**
- `onNotify` sends `KindUsage` to `win.usageIn` (mutex, latest per tab plus arrival seq), invalidates, and returns before any chime/flash/inbox logic.
- `win.drainUsage()` (UI goroutine, next to `drainAlerts`): rate-limit windows from the newest message that carries them replace `win.acct`; a message's `Ctx` sets `win.ctx[tab] = ctxEntry{pct, cwdSeq}` with the tab's current `CwdSeq`; a message without `Ctx` deletes it.
- `statusParts` gains `Usage []usage.Segment`: account windows plus the active tab's ctx, but ctx only while the tab's `CwdSeq` equals the stored one. Closed tabs pruned with `pruneLocs`.
- The frame schedules `op.InvalidateCmd{At: usage.NextReset(...)}` when non-zero.
- `layoutStatus` draws the segments right-aligned left of `Right`, joined by a dim ` · `, colors `colStatusFG` / `colNeeds` / new `colUsageHigh = rgb(0xff6b6b)`; folder room shrinks by their width.

Tests: usage after done keeps the done mark and toast; usage never chimes; acct from any tab, ctx only for the active tab; prompt after usage hides ctx; expired window hidden; closed tab pruned.

### Task 4: Docs and install

README "Claude usage" setup (statusLine JSON for Windows and macOS), HANDOFF and spec 6.5 note. Owner install: `statusLine` in `~/.claude/settings.json` → `"C:\Users\john\Desktop\kaze-app\kaze.exe" statusline`, after a backup.
