# Agent-to-agent control (design)

Date: 2026-11-24. Owner chose approach A (extend the existing local socket) on 2026-11-24.
Builder's calls are marked **(call)**; open items for the owner are at the end.

## Intent and why

The owner runs several agents (Claude Code, Codex, others) in kaze tabs all day and wants one agent to talk
to another, as herdr does with `hc`. The terminal is the shared surface: kaze can read a tab's screen and
type into it, so the receiving agent needs no special support and vendors can talk across tabs (Claude Code
to Codex). Success: from a Claude Code tab, `kaze send t3 "..."` arrives as a prompt in the Codex tab, the
sender can `kaze wait t3` until it is done and `kaze read t3` to get the answer, and the receiver can tell the
message is from a peer and not from the owner typing.

## Goals

- Stable addresses for tabs (`t3`) with optional names, shown with their space in `kaze ls`.
- Commands matching herdr's set: `ls`, `read`, `send`, `keys`, `interrupt`, `wait`.
- `working` / `idle` status from kaze's existing hook-based detection, which `wait` relies on.
- Bracketed paste for `send`, so multi-line text arrives as one paste.
- A message tag so the receiver can tell a peer message from the owner.
- Works on Windows (the daily driver) and macOS from one code path.

## Non-goals and anti-goals

- No core/window split now (PTYs stay in the window process; sessions die with the window).
- No remote machines, no cross-user access, no new settings UI, no new tabs or spawning agents from the CLI.
- Not a permission system between agents: any process holding the token acts as the owner.
- Anti-goals: idle CPU above 0%, a timer or poll that runs while no request is pending, anything that
  wakes the render loop when nothing changed, any control request touching Gio or UI state from a socket
  goroutine (CLAUDE.md: native and foreign callbacks must not run UI code inline).

## Approaches considered

1. **Extend the existing `internal/notify` listener (chosen by the owner).** One transport on both OSes
   (loopback TCP plus the per-run token every shell already has), same trust model as the hooks, smallest
   change. Weaker than a user-locked socket: the token in a tab's environment is the only gate.
2. A second Unix socket (mode 0600) and Windows named pipe (user ACL). Stricter, but two platform paths and
   Windows runtime I cannot test from the Mac. Kept possible: the protocol is written against a small
   transport interface so this can replace the listener later.
3. MCP server per tab. Only helps agents with MCP; loses "anything with a prompt".

## Design

### Packages and threading

- New package `internal/control`: the protocol (request and reply structs), the handlers, and a `Host`
  interface the window implements: tab list with name, space, agent and status; screen text for a tab; submit
  input; add an output or status observer. The handlers know nothing about Gio or `window`.
- `internal/notify`'s server decodes one JSON object per connection. A request carries `op`; the server
  routes it to the control handler, which writes one JSON reply and closes. Existing hook messages (no `op`)
  are untouched. Caps: request 64 KiB, reply 1 MiB, the existing 32 connections and 2 s read deadline; `wait`
  extends the deadline to its own timeout (default 120 s, max 600 s). **(call)**
- **Registry.** `win.agents` is UI-goroutine-only, so the window publishes an immutable snapshot (tab id,
  name, space, folder, agent, status, since) into an `atomic.Pointer` whenever `noteAgent`, tab open/close,
  rename or selection already runs on the UI goroutine. Handlers read the snapshot; no lock on the UI path and
  no extra frame requested. `read` and `send` use `term.Session` directly, which is already safe from any
  goroutine (screen under `Session.Lock`, input through `Submit`).
- **Wait without polling.** The registry keeps a version counter and a broadcast channel closed on each
  snapshot change; `wait` blocks on it with a timeout. `wait --for "<text>"` and the no-hook fallback need
  output wakes: `term.Session` gets an observer list (like `Manager.stateObserver`) so a waiting request is
  woken on output; with no waiters registered nothing runs.

### Addresses and registry

- Address = the existing `TabID` (`t3`); ids restart each launch and are never reused within one.
  Names come from the tab's rename (`FixedTitle`) and can address a tab (`kaze send review ...`) when unique.
  Labels (`k=v`) are set with `kaze label t3 k=v`, in memory only. **(call: phase 2)**
- Workspace-run members are addressable like any tab (they are tabs), and `ls` marks them.

### Commands (CLI is the `kaze` binary; subcommands handled in `cmd/kaze/main.go` before any window code)

| Command | Behaviour |
|---|---|
| `kaze ls [--json]` | id, name, space, agent, status, folder. Plain text for agents, JSON on request. |
| `kaze read <t> [--lines N]` | The visible screen as plain text; with `--lines N` also the last N scrollback lines (cap 2000). Wrapped rows are joined, trailing spaces trimmed. On the alternate screen it returns the alternate screen. **(call: decided as asked, visible by default)** |
| `kaze send <t> <text>` | Bracketed paste (when the program enabled it, else raw with newlines as CR), then Enter after the echo settles, using the existing `EchoMark`/`WaitEcho` and the way branch-session and workspace recipes send text. Text from stdin with `-`. Cap 32 KiB. |
| `kaze keys <t> <key...>` | Named keys from an allowlist: enter, esc, tab, up, down, left, right, backspace, ctrl+c, ctrl+d, ctrl+l. |
| `kaze interrupt <t>` | Agents: Esc (Claude Code and Codex stop the turn on it). Plain shell: Ctrl+C. `--ctrl-c` forces Ctrl+C. **(call)** |
| `kaze wait <t> [--for TEXT] [--timeout S] [--quiet MS]` | Returns when status becomes idle, or the screen prints TEXT, or (no-hook program) the tab was quiet for MS (default 1500). Exit 0 on match, 124 on timeout. |

### Status

`working` (an agent turn is running: `agentState.running`), `idle` (an agent is present and not running),
`needs` (the tab is waiting on the owner: an alert mark), `shell` (no agent known). `wait` treats `idle` and
`needs` as stopped. The status is only as good as the hooks; a plain shell has none, hence `--for` and `--quiet`.

### Busy target

`send` sends anyway and the reply carries `status` and `warning: "busy"` which the CLI prints on stderr.
`--if-idle` refuses (exit 3) when the status is `working`. Queueing is not done: it hides that the text may
land mid-turn. **(call, as discussed)**

### Message tag

The server wraps the text as `<kaze from="t1 name">text</kaze>` when the request carries a `from` that is an
existing tab (the CLI fills it from `KAZE_TAB_ID`); a literal `</kaze>` inside the text is stripped. Sent from
outside kaze, or with `--raw`, nothing is added. Config key `agent_tag` (default true) turns it off globally.
Any holder of the token can forge `from`; this is not authentication, only a hint for the receiving agent.
**(call)**

### Environment and discovery

Every tab already gets `KAZE_TAB_ID`, `KAZE_NOTIFY` (address) and `KAZE_NOTIFY_TOKEN`. Add `KAZE_SOCKET`
(same address, the name the owner asked for) and `KAZE_BIN` (path of the running kaze binary), since `kaze`
is not on PATH inside the Mac bundle. The CLI reads `KAZE_SOCKET`, falling back to `KAZE_NOTIFY`. **(call)**
A short snippet for the owner's CLAUDE.md or AGENTS.md ("use `"$KAZE_BIN" send ...`") goes in the README.

### Security

Loopback only, per-run random token compared in constant time (as today), request and reply caps, connection
cap and deadlines, a key allowlist, text caps. `[control] enabled` (default true) turns the ops off while hooks
keep working **(call)**. Known limit: the token sits in every child's environment, so any process in a tab can
type into any other tab. Approach 2 would narrow that to the same user; it does not fix a hostile process
running as the owner.

### Door left open for a core/window split

The protocol is tab-id based and the handlers use only the `Host` interface (list, screen, submit, observe),
which a future core could implement over its own PTYs. What would still need work: tab ids restart each
launch, so a core would own stable ids, and the status registry would move with it.

### Idle CPU

No timers, polls or goroutines run without a pending request. Snapshots are published only when the UI
already handled a hook message or tab change. Tested with a bounded test that an idle control server schedules
no wake-ups and an unchanged frame still allocates what it did before.

## Phasing

1. **Spike**: `ls`, `read`, `send` over the socket, `KAZE_SOCKET`/`KAZE_BIN`, tested between two fake sessions
   and by hand between two real tabs.
2. `keys`, `interrupt`, `wait` (`--for`, `--quiet`), status registry hardening, the message tag, names and labels.
3. README, the settings key, release.

## Left to the builder

Exact JSON field names, the wire error codes, how `send` splits paste and Enter per agent (the first
real-agent test decides the delay), file layout inside `internal/control`, test fixtures.

## Needs the owner

1. Is "any process in a tab can type into any other tab" acceptable for now (it is today's hook trust model)?
2. Ship the spike (`ls`, `read`, `send`) as its own release, or hold until phase 2 is done?

## Addendum 2026-11-25: teach the agents from Settings

Owner request: instead of pasting a paragraph into each agent, a Settings switch writes the instructions into the
global Claude Code and Codex files, so every agent in every tab knows how to use `"$KAZE_BIN"`.

- Settings > Agents gets a "Peer messaging instructions" switch on the Claude Code page and on the Codex page. **Off
  by default**; the files themselves are the state (no new config key). Turning it on writes a marked block into
  `~/.claude/CLAUDE.md` or `$CODEX_HOME/AGENTS.md` (default `~/.codex/AGENTS.md`); turning it off removes exactly that
  block. Markers `<!-- kaze:peer-agents:begin ... -->` / `<!-- kaze:peer-agents:end -->` make it idempotent, and
  everything outside the block is left byte for byte as it was. **(call)**
- The text uses `"$KAZE_BIN"` (no path), applies only when `KAZE_BIN` is set, and tells the agent that `<kaze from=...>`
  messages come from a peer, not the owner. One constant in the new package is the single source; the README quotes it.
- Safety: atomic write (temp file and rename) through symlinks to the real file, keep the file mode, one-time `.kaze-bak`
  copy if the existing files do it elsewhere in this repo (follow `internal/claudesetup`), refuse and show an error
  when the file is unreadable or unwritable, never touch the real home in tests.
