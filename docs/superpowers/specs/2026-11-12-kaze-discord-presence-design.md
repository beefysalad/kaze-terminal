# Discord Rich Presence: design

Status: draft for the owner's review, 2026-11-12. Nothing is built yet.

## Why

The owner spends the day in Kaze running Claude Code. They want their Discord profile to show what
they're doing: the project, how long they've been at it, and how many agent sessions are open. It also
shows Kaze to their Discord friends.

## Issue #55 and what changed

Issue #55 (2026-10-31) proposed this feature off by default with no repo names. On 2026-11-12 the owner
decided otherwise, and this design follows the owner:

| Issue #55 | This design (owner's call) |
|---|---|
| Off by default | On by default |
| Never publish repo names | The repo name shows by default; a switch hides it |
| Switches for elapsed time and counts | Always shown; only the project name has a switch |
| Show an example before enabling | The Settings hint describes what is shared |
| Clear on normal shutdown | Closing the socket clears it (Discord drops the card) |

Kept from the issue: Claude/Codex counts from known agent state (hooks or the program in front, never
terminal text), a generic fallback with no agents, bounded and coalesced updates, no busy retries, the
local Discord client only (no user token), connection work off the UI goroutine, macOS and Windows
behaviour checked by hand. Agent teams wording ("Codex leading, 2 helpers") is a later follow-up.

## What friends see

While Kaze is open and the Discord desktop app is running on the same machine, the owner's profile
shows a "Playing Kaze" card:

```
Playing Kaze
  [wind K logo]   Working on kaze-terminal
                  3 Claude, 1 Codex
                  02:14:37 elapsed
```

| Line | Discord field | Content |
|---|---|---|
| Name | the Discord application's name | `Kaze` (set in the Developer Portal, see "What the owner does") |
| Top line | `details` | `Working on <project>` |
| Second line | `state` | the session count (rules below) |
| Timer | `timestamps.start` | when Kaze started, as Unix seconds. Discord draws the counting-up timer itself |
| Large image | `assets.large_image` | the asset key `kaze` (the wind K), with `large_text` = `Kaze <version>` |

**The project** is taken from the tab the owner is looking at (the active tab):

- If its folder is in a git checkout: the repo name, the same `repoName` the status bar shows
  (`win.locOf(t).repoName`). Never the path, branch or remote.
- If it is not in a checkout: the folder's base name (`filepath.Base` of the cwd).
- If the cwd is unknown, empty or the home folder: the top line is `In the terminal`. (Builder's call,
  not confirmed by the owner.)
- With "Show the project name" off: the top line is always `In the terminal`.

**The session count** counts every tab the sidebar would show as an agent session, using the same rule
the rows use (`win.rowAgent(id).agentName()`, which is `"claude"` or `"codex"`, from the hooks or the
program in front of the tab), over every tab in `win.mgr.Tabs()`. Plain shells don't count. Wording:

| Claude | Codex | `state` |
|---|---|---|
| 0 | 0 | `No agents running` |
| 1 | 0 | `1 Claude session` |
| N | 0 | `N Claude sessions` |
| 0 | 1 | `1 Codex session` |
| 0 | M | `M Codex sessions` |
| N | M | `N Claude, M Codex` |

Claude comes first because it is the owner's priority. Codex only ever appears as a count.

**The timer** starts when Kaze starts and doesn't reset on tab or project switches (the owner's call).
It is the same value for the whole run.

## Goals

- On by default: anyone with Discord open sees the card after updating. Nothing happens without Discord.
- The card follows the active tab's project and the live session count within about 15 seconds.
- Turning it off in Settings clears the card from Discord at once and closes the connection.
- Kaze stays exactly as fast and responsive with Discord missing, closing, restarting or hung.

## Non-goals

- Time totals per project or per tab ("6 hours on kaze-terminal this week"). That needs saved history.
- Buttons or links on the card (repo link, website link).
- Showing the branch, file names, the model, the Claude prompt, or any Codex detail beyond a count.
- Linux. Kaze ships for macOS and Windows only.
- Join, spectate, invites, or reading anything back from Discord.
- Any network traffic. Kaze talks only to the Discord app on the same machine; Discord does the rest.

## Anti-goals (a failure even if it works)

- A frame, a keystroke or shutdown ever waits on Discord.
- Error pop-ups, toasts or status bar messages about Discord. When it can't connect, it stays quiet
  and tries again later.
- Full folder paths, usernames or remote URLs on the card.
- Busy reconnect loops: a missing Discord costs one connection attempt every 15 seconds at most.

## Settings and config

Two new keys in `config.toml`, both default `true`, loaded like `system_notifications` (a wrong type
warns and keeps the default) and written with `config.Set` so comments survive:

| Key | Default | Meaning |
|---|---|---|
| `discord_presence` | `true` | Show Kaze on the Discord profile |
| `discord_show_project` | `true` | Put the project name on the card. Off: `In the terminal` |

In Settings, the Agents page gets a `Discord` heading after the existing headings, with two toggles
(builder's call on placement, not confirmed by the owner):

- `Show Kaze on your Discord profile`, hint: `Your project, the time Kaze has been open and your session count. Needs the Discord app running.`
- `Show the project name`, hint: `Off shows "In the terminal" instead.` Greyed out while the first toggle is off, if the settings kit already supports a disabled row; otherwise always shown (builder's call).

The second switch is the builder's call, kept for private client work; the owner chose to show names by
default.

## How it works

### `internal/discord` (new package)

The only package that knows Discord's protocol.

**Transport.** Discord's desktop app listens on a local socket:

- macOS: a Unix socket `discord-ipc-<n>` for n = 0..9, in the first set directory of `$XDG_RUNTIME_DIR`,
  `$TMPDIR`, `$TMP`, `$TEMP`, else `/tmp`. Try each n in order; the first that connects wins.
- Windows: the named pipe `\\.\pipe\discord-ipc-<n>` for n = 0..9.

Each platform's dial lives in a `dial_darwin.go` / `dial_windows.go` file (with a `dial_other.go` that
returns "unsupported" so `go vet` and tests build everywhere). Prefer `net.Dial("unix", ...)` on macOS
and `golang.org/x/sys/windows` on Windows; no new module dependency. How to give the Windows pipe a
timeout (overlapped I/O, or a reader goroutine that is simply abandoned on close) is the builder's
call, as long as the anti-goals hold.

**Frames.** Every message is an 8-byte header, opcode then payload length, both little-endian
`uint32`, followed by that many bytes of UTF-8 JSON. Opcodes: 0 handshake, 1 frame, 2 close, 3 ping,
4 pong.

1. Send op 0 `{"v":1,"client_id":"<id>"}`.
2. Read one frame; expect op 1 with `"evt":"READY"`. Op 2 or anything else means close and retry later.
3. To set the card, send op 1:
   `{"cmd":"SET_ACTIVITY","args":{"pid":<os.Getpid()>,"activity":{...}},"nonce":"<unique>"}`.
   To clear it, the same with no `activity` key.
4. Answer op 3 with op 4 carrying the same payload. Read and drop other replies.

Reject frames over 64 KiB (close and retry), so a bad peer can't make Kaze allocate without bound.

**The client.** One goroutine owns the connection. The UI calls `client.Set(a Activity)` (or
`Set(nil)` to clear), which never blocks: it stores the latest wanted activity and wakes the
goroutine. The goroutine:

- connects when there is something to send; on failure waits 15 s before the next attempt;
- sends at most one `SET_ACTIVITY` every 15 s (Discord's limit is about 5 per 20 s), always the latest
  wanted value, and skips a send that equals the last one Discord accepted;
- after a write or read error, drops the connection and goes back to connecting; the wanted activity is
  resent after reconnecting (Discord drops the card when the socket closes);
- has `Close()`, which never waits: it tells the goroutine to stop and closes the socket. Discord drops
  the card when the socket closes, so no clear is sent on quit. `win.shutdown()` calls it alongside
  `win.battery.close()`. (Builder's call: a waiting close would let a hung Discord slow down quitting.)
- turning the setting off is different: there Kaze keeps running, so the goroutine sends a clear
  (ignoring the 15 s spacing) and then closes, all off the UI goroutine.

Time, the dialer and the backoff are injectable so tests run on a fake clock and a fake Discord.

**Only one Kaze owns the card.** The owner runs one Kaze with many tabs, but a second copy can be
started (for example `kaze --workspace <name>`). Without a rule, two copies would overwrite each other's
card. Before each connection attempt, the client takes an exclusive, non-blocking lock on
`discord.lock` in Kaze's config folder (the same `flock` / `LockFileEx` approach as
`internal/sessionstate`). A copy that can't get the lock stays quiet and tries again on its next 15 s
attempt, so it takes over when the first copy quits. The lock is released in `Close`. Counts are not
combined across copies (a non-goal). (Builder's call; the owner runs a single copy.)

**The client ID** is a constant in the package. It is public, not a secret. While it is empty the
client does nothing, so the code can merge before the Discord application exists.

### `internal/ui` wiring

- `win.startedAt` is recorded once when the window is created.
- A small pure function builds the activity from plain inputs (project name or "", the show-project
  flag, Claude count, Codex count, start time, version), so all the wording is table-tested without a
  window.
- Each frame, the window computes those inputs from the active tab and the tab list and calls
  `client.Set` only when the result differs from the last call. No I/O on the UI goroutine; `locOf`
  already does its git lookups in the background.
- Toggling `discord_presence` off calls `Set(nil)` then closes the connection; toggling it on starts
  the client again. Toggling `discord_show_project` changes the next activity.

## What the owner does (outside the code)

1. In the Discord Developer Portal, create an application named `Kaze`.
2. Under Rich Presence, Art Assets, upload the wind K (at least 512 by 512) with the key `kaze`.
3. Send the builder the Application ID; it goes in the client ID constant.
4. After a local build, check with Discord running, on the Mac and on Windows: the card appears, the
   project follows tab switches, the count changes when Claude starts and exits, the switch clears it,
   quitting Kaze clears it, and Kaze starts normally with Discord closed.

## Testing

Strict TDD; every RED test is bounded (fixed sizes, `t.Context` or explicit timeouts, no unbounded reads).

- Frame encoding and decoding, including the 64 KiB cap and short reads.
- The handshake and `SET_ACTIVITY` payloads against a fake Discord on `net.Pipe` (and a real Unix socket
  in `t.TempDir()` on macOS).
- The lock: a second client in the same test, pointed at the same lock path, sends nothing until the
  first one closes.
- Rate limiting, coalescing, skip-if-same, backoff and resend-after-reconnect on a fake clock.
- `Close` returning at once (bounded by a short test timeout) against a peer that never reads or never answers.
- Activity wording: the project rules and the count table above.
- Config: defaults, bad types warn, `Set` keeps comments (like `system_notifications_test.go`).
- Settings: the two toggles flip the config and call the client.

## Shipping

A feature, so it ships as 0.41 through the usual release PR. **The release waits for the client ID**
(builder's call): the feature PR can merge with an empty ID, but 0.41 is not cut until the ID is in and the
owner has seen the card once. Otherwise users would get a toggle that does nothing. Lander: a row in the feature list
(`#sessions`) and a "What's new" entry; no screenshot needed for the card itself unless the owner
wants one of their Discord profile. README: a short "Discord" section saying what is shared and how to
turn it off. HANDOFF: the new package and the owner's check list.

## Left to the builder

- Exact file split inside `internal/discord`, and the Windows pipe timeout approach.
- The nonce format (any unique string; a counter plus the pid is fine).
- Whether the second toggle greys out.
