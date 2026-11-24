# Agent-to-agent control: plan

Spec: `docs/superpowers/specs/2026-11-24-kaze-agent-control-design.md` (approved by the owner 2026-11-24:
approach A, token trust level accepted, release decision below). Branch `feat/agent-control` from main (0.44).

**Release call:** one PR and one release (0.45) after Task 9. The spike (Tasks 1-5) alone has `send` without
`wait`, which forces polling, so it is a gate for an owner hand-check, not a release.

## Global constraints (every task)

- CLAUDE.md applies in full: strict TDD with bounded RED tests (bounded sizes, deadlines on every socket and wait,
  no unbounded loops or allocation), one font (no UI is added here), no emojis, no em dashes in docs, no secrets.
- Never inject global keyboard or mouse input to the machine. Test through code paths, fake sessions and headless
  windows. A test that talks to the real socket uses its own listener on 127.0.0.1 with a test token.
- Native or socket goroutines never run Gio or `window` code. Control handlers read the published snapshot and
  `term.Session` only (Session is safe from any goroutine).
- Idle CPU stays 0: no timers, polls or goroutines run without a pending request; an unchanged frame allocates what
  it did before (the zero-allocation tests in `internal/ui` stay green).
- Commits `type(scope): Verb ...`, NO AI attribution, NO Co-Authored-By. Never push, never touch main, no version bump
  (the release PR does that).
- Each task ends green: `go test ./internal/control/ ./internal/notify/ ./internal/term/ ./internal/tabs/ ./internal/ui/ ./cmd/...`,
  `go vet ./...`. The two known-slow tests (`TestProxy*` in internal/codexlaunch, the version dispatch test in cmd/kaze)
  may time out on a busy Mac; rerun alone.

## Code map

- Socket: `internal/notify/notify.go` (`Listen` 120, `serve` 128, `handle` 210, `Send` 374, `Msg` 62, caps 54-60).
  Window glue: `internal/ui/window.go` (~287 `notify.Listen(win.onNotify)`, ~696 `mgr.SetNotify`).
- Tabs and env: `internal/tabs/manager.go` (`Tab` 21, `newTab`, env at ~153, `SetNotify` 115, `droppedEnv` 102).
- Session: `internal/term/session.go` (`Submit` 188, `EchoMark`/`WaitEcho` 133-160, `Lock`, `Screen()`, `Applier()`, `OutputSeq` 387,
  `notifyLoop` 364). Screen: `internal/screen` (`Cell`, `Size`, `ScrollbackLen`, `ScrollbackLine`, `OnAltScreen`, wrapped marks in print.go).
  Paste: `internal/input/paste.go` (`EncodePaste(s, term.Modes)`), keys in `internal/input/keys.go`.
- Agent state: `internal/ui/agent.go` (`agentState`, `noteAgent` 52), `alerts.markOf`, names via `tabs.Tab.FixedTitle`, spaces via
  `internal/ui/spaces_model.go`.
- CLI entry: `cmd/kaze/main.go` (subcommands dispatched before any window code, lines ~26-41; `notify.go` is the client pattern).

## Task 1: control protocol, Host interface, ls/read/send handlers (pure)
New `internal/control`: request/reply structs (`op`, `tab` address or name, args), error codes, `Host` interface
(`Tabs() []TabInfo`, `Screen(id, lines) (string, error)`, `Submit(id, []byte) error`, `Echo`/observe hooks as needed later),
address resolution (`t3` or a unique name; unknown and ambiguous are distinct errors), and handlers for `ls`, `read`, `send`.
`read`: visible screen as plain text, wrapped rows joined, trailing spaces trimmed, `--lines N` scrollback capped at 2000,
alt screen returns the alt screen. `send`: cap 32 KiB. Tests with a fake Host: resolution, caps, error codes, wrapped-row join,
scrollback cap, alt screen. No sockets, no UI.

## Task 2: serve control requests on the existing listener
`notify.Server` routes a JSON object with an `op` to a registered `control` handler, writes exactly one JSON reply, closes.
Hook messages (no `op`) behave exactly as today (their tests must stay green untouched). Caps: request 64 KiB, reply 1 MiB, the
existing connection cap, constant-time token check for control too, a longer per-request deadline only for `wait` later.
Registration is by an optional handler so the package stays free of UI imports. Tests with a real listener on 127.0.0.1: round
trip, bad token, oversize request, oversize reply truncated with a flag, slow client times out, hook messages unaffected.

## Task 3: the window Host, snapshot registry, env
Implement `control.Host` for the window: an immutable snapshot (`atomic.Pointer`) of tab id, name, space, folder, agent, status,
since, published on the UI goroutine from `noteAgent`, tab open/close/rename and selection, without requesting a frame. `Screen`
reads the session under `Session.Lock`; `Submit` writes bracketed paste via `input.EncodePaste` then Enter after `WaitEcho` (use the
same pattern as branch-session and workspace recipe sends; the delay between paste and Enter must be a named constant). Add
`KAZE_SOCKET` (same address as `KAZE_NOTIFY`) and `KAZE_BIN` (`os.Executable()`) to every tab's env in `tabs.Manager` and to
`droppedEnv` so a kaze started inside a kaze tab gets clean values. Tests: snapshot content after hook messages, publish does not
request a frame (bounded wake-count test), env values, Submit to a fake session records paste then Enter, closed tab error.

## Task 4: the CLI client for ls, read, send
`kaze ls [--json]`, `kaze read <t> [--lines N]`, `kaze send <t> <text|->` in `cmd/kaze`, dispatched before any window code like
`notify`. Reads `KAZE_SOCKET` (fallback `KAZE_NOTIFY`) and `KAZE_NOTIFY_TOKEN`, 5 s connect and reply deadlines, exit codes
(0 ok, 1 error, 2 usage, 3 refused, 124 timeout), plain text for agents, JSON with `--json`. Errors go to stderr. Tests against a
fake server: output formats, exit codes, stdin text, missing env, oversize text refused client side.

## Task 5: spike gate, end to end
A test with two real `term.Session`s on a fake PTY behind a real listener: `send` from A's address to B arrives as one
bracketed paste then Enter, `read` returns B's screen, `ls` shows both. A bounded idle test: with the server running and no
requests, no goroutine wakes and a still frame allocates what it did before. Add a short section to `docs/HANDOFF.md` with the
owner's hand-check (two real tabs, Claude Code to Codex, multi-line text, a busy target). STOP here for the owner's check; the
controller reports and continues only on the owner's word or at Task 6 if told to continue.

## Task 6: keys and interrupt
`kaze keys <t> <key...>` with the allowlist (enter, esc, tab, up, down, left, right, backspace, ctrl+c, ctrl+d, ctrl+l), encoded with
the existing `internal/input` encoders and the target's cursor-key mode. `kaze interrupt <t>`: Esc for a known agent, Ctrl+C for a
shell, `--ctrl-c` to force. Tests: every key's bytes, application-cursor mode, unknown key refused, interrupt per kind.

## Task 7: status and wait
Add the status to the snapshot (`working`, `idle`, `needs`, `shell`) from `agentState.running`, `alerts.markOf` and the foreground
check. `wait`: block on a registry broadcast channel (closed on every snapshot change) with a timeout; `--for TEXT` and `--quiet MS`
(default 1500) use a new `term.Session` output observer list (registered only while a waiter exists, removed on return); exit 0 on
match, 124 on timeout. Request deadline for `wait` is its timeout (default 120 s, max 600 s). Tests with a fake clock where possible:
idle after working, needs counts as stopped, `--for` matches output arriving later, quiet fallback, timeout, waiter cleanup (no leaked
observers or goroutines), and no wakeups with no waiters.

## Task 8: tag, names, labels, config
Server wraps `send` text as `<kaze from="t1 name">text</kaze>` when `from` is an existing tab (CLI fills it from `KAZE_TAB_ID`; `--raw`
and an outside caller add nothing; a literal `</kaze>` in the text is stripped). `--if-idle` refuses (exit 3) when `working` and the reply
carries `warning: "busy"` otherwise. `kaze name <t> <name>` (sets the rename) and `kaze label <t> k=v` (in memory). Config: `agent_tag`
(default true) and `[control] enabled` (default true; off returns a refusal for every op while hooks keep working); unknown keys keep
warning as today. Tests: tag shape and stripping, forged `from` for a missing tab ignored, busy warning, if-idle, config on/off, config
warnings.

## Task 9: docs, verification, race
README section (commands, env vars, the CLAUDE.md/AGENTS.md snippet using `"$KAZE_BIN"`, the trust note), `docs/HANDOFF.md`, mention in
the lander's feature list only if the release PR agrees. Run the full suite, `go vet ./...`, the macOS race command from
`.github/workflows/test.yml` (including `internal/control`, add the package to that list), native macOS build and Windows amd64/arm64
cross-builds. Record frame and allocation numbers and the idle wake-up result in the report. No version bump, no changelog (release PR).

## After Task 9

Controller review of the whole branch with a fresh read-only reviewer, fixes test first, push, PR, wait for CI, owner merges, then the
release PR (0.45) per CLAUDE.md.
