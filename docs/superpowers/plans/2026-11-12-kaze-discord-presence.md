# Discord Rich Presence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a "Playing Kaze" card on the owner's Discord profile with the active tab's project, the time since Kaze started, and the Claude/Codex session count.

**Architecture:** A new `internal/discord` package owns everything Discord: the frame codec, the activity text, the per-platform dial, a single-owner lock, and a `Client` whose one goroutine connects, rate-limits and reconnects. `internal/ui` computes plain inputs each frame and calls the non-blocking `Client.Set` only when the activity changes. Two config keys and two toggles on the Agents settings page turn it on and off.

**Tech Stack:** Go 1.27 standard library (`encoding/binary`, `encoding/json`, `net`), `golang.org/x/sys/unix` and `golang.org/x/sys/windows` (already in `go.mod`). No new module.

**Spec:** `docs/superpowers/specs/2026-11-12-kaze-discord-presence-design.md`. Issue #55.

## Global Constraints

- No new module dependency; `go.mod` keeps its current `require` list.
- Config keys: `discord_presence` (default `true`), `discord_show_project` (default `true`).
- Frame header: opcode then length, both little-endian `uint32`. Opcodes 0 handshake, 1 frame, 2 close, 3 ping, 4 pong. Frames over 64 KiB (`64 << 10`) are rejected.
- Handshake payload `{"v":1,"client_id":"<id>"}`; expect op 1 with `"evt":"READY"`.
- At most one `SET_ACTIVITY` every 15 s; at most one connection attempt every 15 s.
- Asset key `kaze`; `large_text` = `Kaze <version.Number>`.
- `ClientID` constant starts empty; with it empty the client never dials.
- Nothing on the UI goroutine blocks on Discord; `Client.Close` never waits on the peer.
- No user-visible errors, toasts or status messages about Discord.
- Copy rules: JetBrains Mono only (settings kit already does this), no emojis or symbol icons, no em dashes in UI copy.
- Commits `type(scope): Verb ...`, no AI attribution. Never push to `main`.
- Every RED test is bounded: fixed sizes, deadlines or `time.After` guards of at most 2 s, no unbounded reads.
- No release in this plan. The 0.41 release waits for the owner's Application ID (spec, "Shipping").

## Review Focus

1. **Discord running but frozen (accepts the socket, never answers the handshake).** Kaze must keep running, frames must not stall, and quitting must be instant. Test: `TestClientHandshakeTimeout` in Task 4.
2. **Discord restarts while Kaze is open.** The card must come back without the user doing anything. Test: `TestClientResendsAfterReconnect` in Task 4.
3. **A burst of tab switches.** Only the latest activity is sent, and only after the 15 s spacing; no queue grows. Test: `TestClientCoalescesBurst` in Task 4.
4. **Repo names with odd characters or very long names** (quotes, backslashes, 300 characters). JSON must stay valid and Discord's 128-character field limit must hold. Test: `TestBuildClipsLongProject` in Task 2, plus JSON round trip in `TestActivityJSON`.
5. **Turning the switch off while Discord is absent or mid-backoff.** No dial happens, and no clear is sent to a socket that doesn't exist; turning it on again resumes. Test: `TestClientDisableWhileDisconnected` in Task 4.

---

### Task 1: Config keys

**Files:**
- Modify: `internal/config/config.go` (struct near line 47, `Default()` near line 83, load switch near line 231)
- Create: `internal/config/discord_presence_test.go`

**Interfaces:**
- Produces: `Config.DiscordPresence bool` (`toml:"discord_presence"`), `Config.DiscordShowProject bool` (`toml:"discord_show_project"`), both `true` in `Default()`.

- [ ] **Step 1: Write the failing tests**, mirroring `system_notifications_test.go` (reflection lookup so the RED run compiles):
  - `TestDiscordPresenceConfiguration`: for each key, `Default()` is `true`; bodies `""` -> true/0 warnings, `key = false` -> false/0, `key = true` -> true/0, `key = 3` -> true/1 warning.
  - `TestDiscordPresenceSetPreservesComments`: `Set(p, "discord_presence", false)` keeps `# keep` and an inline comment, and reloads as false with no warnings.
- [ ] **Step 2: Run** `go test ./internal/config/ -run Discord`. Expected: FAIL, "missing discord_presence boolean".
- [ ] **Step 3: Add the two fields, defaults and `loadBool` cases** with one-line comments like the neighbours.
- [ ] **Step 4: Run** `go test ./internal/config/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(config): Add the Discord presence settings`.

### Task 2: Activity text and JSON

**Files:**
- Create: `internal/discord/activity.go`, `internal/discord/activity_test.go`

**Interfaces:**
- Produces:
  ```go
  type Activity struct {
      Details, State        string
      Start                 time.Time
      LargeImage, LargeText string
  }
  type Inputs struct {
      Project       string // repo name, else folder base name; "" when unknown or home
      ShowProject   bool
      Claude, Codex int
      Start         time.Time
      Version       string
  }
  func Build(in Inputs) Activity
  func (a Activity) payload() map[string]any // the "activity" object for SET_ACTIVITY
  ```

- [ ] **Step 1: Write the failing tests.**
  - `TestBuildDetails`: `Project "kaze-terminal", ShowProject true` -> `"Working on kaze-terminal"`; `Project ""` -> `"In the terminal"`; `Project "kaze-terminal", ShowProject false` -> `"In the terminal"`.
  - `TestBuildState` (table): (0,0) `"No agents running"`; (1,0) `"1 Claude session"`; (3,0) `"3 Claude sessions"`; (0,1) `"1 Codex session"`; (0,2) `"2 Codex sessions"`; (3,1) `"3 Claude, 1 Codex"`.
  - `TestBuildAssets`: `LargeImage == "kaze"`, `LargeText == "Kaze 0.41"` for `Version "0.41"`, `Start` copied.
  - `TestBuildClipsLongProject`: a 300-rune project gives `Details` of at most 128 runes, valid UTF-8, starting `"Working on "`.
  - `TestActivityJSON`: `json.Marshal(a.payload())` for a project `a"b\c` round-trips; it has `details`, `state`, `timestamps.start` equal to `Start.Unix()`, `assets.large_image`, `assets.large_text`, and no other top-level keys.
- [ ] **Step 2: Run** `go test ./internal/discord/`. Expected: FAIL, undefined `Build`.
- [ ] **Step 3: Implement** `Build` and `payload`. Clip any field to 128 runes on a rune boundary.
- [ ] **Step 4: Run** `go test ./internal/discord/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(discord): Build the presence text`.

### Task 3: Frames, dial and the single-owner lock

**Files:**
- Create: `internal/discord/frame.go`, `frame_test.go`
- Create: `internal/discord/dial_darwin.go`, `dial_windows.go`, `dial_other.go` (`//go:build !darwin && !windows`), `dial_darwin_test.go`
- Create: `internal/discord/lock_unix.go` (`//go:build !windows`), `lock_windows.go`, `lock_test.go`

**Interfaces:**
- Produces:
  ```go
  const (opHandshake, opFrame, opClose, opPing, opPong uint32 = 0, 1, 2, 3, 4)
  const maxFrame = 64 << 10
  var errFrameTooBig = errors.New("discord: frame too big")
  func writeFrame(w io.Writer, op uint32, payload []byte) error
  func readFrame(r io.Reader) (op uint32, payload []byte, err error)

  // Dial tries discord-ipc-0..9 and returns the first that connects.
  // Unsupported platforms return errUnsupported.
  func Dial() (io.ReadWriteCloser, error)
  var errUnsupported = errors.New("discord: unsupported platform")

  var ErrLocked = errors.New("discord: another Kaze owns the card")
  func lockFile(path string) (release func(), err error) // non-blocking; ErrLocked when held
  ```

- [ ] **Step 1: Write the failing tests.**
  - `TestFrameRoundTrip`: write op 1 with `{"x":1}` to a `bytes.Buffer`; the first 8 bytes are `01 00 00 00 07 00 00 00`; `readFrame` returns op 1 and the same payload.
  - `TestFrameRejectsTooBig`: a header claiming `maxFrame+1` bytes returns `errFrameTooBig` without reading the body (use a reader that fails the test if more than 8 bytes are read).
  - `TestFrameShortRead`: a header claiming 10 bytes followed by 3 bytes returns an error wrapping `io.ErrUnexpectedEOF`.
  - `TestDialFindsSocket` (darwin): set `TMPDIR` to a short temp dir (Unix socket paths are limited to 104 bytes on macOS; use `os.MkdirTemp("/tmp", "kd")` and clean up), listen on `discord-ipc-1` only, `Dial()` connects to it. With no socket, `Dial()` returns an error within 1 s.
  - `TestLockIsExclusive`: `lockFile(p)` succeeds; a second `lockFile(p)` returns `ErrLocked`; after `release()`, a third succeeds. (On Unix, flock locks are per open file, so this works in one process.)
- [ ] **Step 2: Run** `go test ./internal/discord/`. Expected: FAIL, undefined `writeFrame`.
- [ ] **Step 3: Implement.** Darwin dial: directories `$XDG_RUNTIME_DIR`, `$TMPDIR`, `$TMP`, `$TEMP`, `/tmp` (first set one), `net.DialTimeout("unix", ..., 1*time.Second)`. Windows dial: `\\.\pipe\discord-ipc-<n>` via `windows.CreateFile` (`GENERIC_READ|GENERIC_WRITE`, `OPEN_EXISTING`), wrapped in `os.NewFile`; how reads get a timeout is the implementer's call (spec, "Left to the builder"), but `Close` must never wait. Locks: `unix.Flock(LOCK_EX|LOCK_NB)` / `windows.LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)`, as in `internal/sessionstate/files_*.go`.
- [ ] **Step 4: Run** `go test ./internal/discord/ && GOOS=windows go vet ./internal/discord/ && GOOS=linux go vet ./internal/discord/`. Expected: PASS, no vet output.
- [ ] **Step 5: Commit** `feat(discord): Talk to the local Discord app`.

### Task 4: The client

**Files:**
- Create: `internal/discord/client.go`, `client_test.go`, `fake_test.go` (a fake Discord on `net.Pipe`)

**Interfaces:**
- Consumes: `Activity`, `payload`, `writeFrame`, `readFrame`, `Dial`, `lockFile`, `ErrLocked` from Tasks 2 and 3.
- Produces:
  ```go
  const ClientID = "" // the owner's Discord Application ID; empty disables the client
  type Options struct {
      ClientID string
      LockPath string                                 // "" skips the lock (tests)
      Dial     func() (io.ReadWriteCloser, error)     // default Dial
      Now      func() time.Time                       // default time.Now
      After    func(time.Duration) <-chan time.Time   // default time.After
      Pid      int                                    // default os.Getpid()
      Spacing  time.Duration                          // default 15 s, between sends and between dials
      Reply    time.Duration                          // default 5 s, wait for READY
  }
  func New(o Options) *Client         // starts the goroutine; nil-safe methods
  func (c *Client) Set(a *Activity)   // never blocks; nil means clear and disconnect
  func (c *Client) Close()            // never waits on the peer; releases the lock
  ```
  Behaviour (spec, "The client"): connect only when a non-nil activity is wanted; take the lock before each dial (`ErrLocked` counts as a failed attempt); handshake, then wait up to `Reply` for READY; send the latest wanted activity, at most once per `Spacing`, skipping one equal to the last accepted; answer ping with pong; on any error drop the socket and resend after reconnecting; `Set(nil)` while connected sends a clear right away (ignoring `Spacing`), then closes and releases the lock; empty `ClientID` never dials.

- [ ] **Step 1: Write the failing tests** (fake clock: `After` returns channels the test fires; every wait in a test is guarded by `time.After(2*time.Second)` that fails the test):
  - `TestClientHandshakeAndSet`: fake receives op 0 `{"v":1,"client_id":"id"}`, replies READY, then receives op 1 with `cmd` `SET_ACTIVITY`, `args.pid` = `Pid`, `args.activity.details` = the set value, and a non-empty `nonce`.
  - `TestClientEmptyIDNeverDials`: with `ClientID ""`, `Set` then `Close`; the dial func is never called.
  - `TestClientCoalescesBurst`: after the first send, call `Set` with A, B, C; before the spacing fires nothing more is sent; after it fires exactly one frame with C.
  - `TestClientSkipsSameActivity`: `Set(a)` twice with equal values sends once.
  - `TestClientBacksOff`: a dial func that fails; the second attempt happens only after the spacing channel fires, never before.
  - `TestClientResendsAfterReconnect`: the fake closes after the first send; after the backoff fires, a new connection gets the handshake and the same activity.
  - `TestClientHandshakeTimeout`: the fake accepts but never answers; after `Reply` fires the client closes that socket and backs off; `Close` returns within 100 ms.
  - `TestClientDisableClears`: connected, `Set(nil)` sends op 1 `SET_ACTIVITY` with no `activity` key at once (no spacing tick needed), then the fake sees EOF.
  - `TestClientDisableWhileDisconnected`: dial failing, `Set(nil)`: no further dial attempts; a later `Set(a)` resumes dialing.
  - `TestClientPong`: fake sends op 3 `{"n":1}`; client answers op 4 with `{"n":1}`.
  - `TestClientRespectsLock`: two clients with the same `LockPath` in `t.TempDir()`; only the first dials; after the first `Close`, the second dials on its next tick.
  - `TestClientCloseNeverWaits`: the fake never reads; after a `Set`, `Close` returns within 100 ms.
- [ ] **Step 2: Run** `go test -race ./internal/discord/ -run Client`. Expected: FAIL, undefined `New`.
- [ ] **Step 3: Implement** `Client`. One goroutine; `Set` stores the wanted value under a mutex and does a non-blocking send on a `chan struct{}` of size 1. A second goroutine per connection reads frames (pongs, READY, drops the rest) and reports errors; `Close` closes the socket to unblock it.
- [ ] **Step 4: Run** `go test -race -count=3 ./internal/discord/`. Expected: PASS, no races.
- [ ] **Step 5: Commit** `feat(discord): Keep the presence card up to date`.

### Task 5: Wire it into the window and Settings

**Files:**
- Create: `internal/ui/discord.go`, `internal/ui/discord_test.go`
- Modify: `internal/ui/window.go` (field; `initDiscord()` after `initSystemNotifications` near line 269; `win.discord.Close()` in `shutdown()` next to `win.battery.close()` near line 446; call `win.updatePresence()` once per `frame`)
- Modify: `internal/ui/settings_claude.go` (`sharedAgentRows` gets a `Discord` heading and two toggles at the end; `agentsAct` gets two cases)
- Modify: `internal/ui/settings_test.go` or a new `settings_discord_test.go`

**Interfaces:**
- Consumes: `discord.New`, `discord.Options`, `discord.ClientID`, `(*discord.Client).Set/Close`, `discord.Build`, `discord.Inputs`, `discord.Activity` (Tasks 2 and 4); `Config.DiscordPresence`, `Config.DiscordShowProject` (Task 1); existing `win.locOf(t).repoName`, `win.rowAgent(id).agentName()`, `win.mgr.Tabs()`, `win.mgr.Active()`, `version.Number`, `config.Path()`.
- Produces:
  ```go
  type presenceSink interface{ Set(*discord.Activity); Close() } // *discord.Client; tests use a fake
  // window fields: discord presenceSink; startedAt time.Time; presenceSent *discord.Activity
  func (win *window) initDiscord()                 // skips when win.bench != nil; LockPath = filepath.Join(filepath.Dir(config.Path()), "discord.lock"), "" if Path is ""
  func (win *window) presenceInputs() discord.Inputs
  func (win *window) updatePresence()              // nil-safe; Set only when the built activity differs from presenceSent
  ```
  `presenceInputs`: project from the active tab: `repoName` when set, else `filepath.Base(cwd)`, and `""` when the cwd is empty or equals `os.UserHomeDir()`. Counts over `win.mgr.Tabs()` by `rowAgent(id).agentName()`. `Start` = `win.startedAt`. With `DiscordPresence` off, `updatePresence` sends `Set(nil)` once and then nothing.

- [ ] **Step 1: Write the failing tests** with a fake sink and the existing window test kit (`testkit_test.go`):
  - `TestPresenceInputsProject`: active tab with repoName `kaze-terminal` -> `"kaze-terminal"`; a non-git cwd `/x/notes` -> `"notes"`; cwd = home -> `""`.
  - `TestPresenceInputsCounts`: three tabs whose `rowAgent` is claude, claude, codex plus one plain shell -> `Claude 2, Codex 1`.
  - `TestUpdatePresenceSendsOnlyChanges`: two frames with no change -> one `Set`; switching the active tab to another project -> a second `Set` with the new details.
  - `TestUpdatePresenceOff`: `DiscordPresence` false -> exactly one `Set(nil)` across several frames.
  - `TestSettingsDiscordToggles`: the Agents page shows `Show Kaze on your Discord profile` and `Show the project name` with their hints (exact copy from the spec); acting on `discord_presence` off sets the config false, saves it, and the next frame sends `Set(nil)`; acting on `discord_show_project` off makes the next activity's details `"In the terminal"`.
  - `TestInitDiscordSkipsBench`: with `win.bench` set, `initDiscord` leaves `win.discord` nil.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Presence|Discord'`. Expected: FAIL, undefined `presenceInputs`.
- [ ] **Step 3: Implement** the file, the window hooks and the two settings rows. Toggle ids `discord_presence` and `discord_show_project`; save with `win.saveSetting(id, on)`. Grey out the second row only if the settings kit already has a disabled control; otherwise always show it.
- [ ] **Step 4: Run** `go test ./... && go vet ./... && GOOS=windows go vet ./...`. Expected: PASS, no vet output.
- [ ] **Step 5: Commit** `feat(ui): Show Kaze on the Discord profile`.

### Task 6: Docs for the feature PR (no release)

**Files:**
- Modify: `README.md` (a short "Discord" section; a Roadmap line), `docs/HANDOFF.md` (the new package, the empty `ClientID`, the owner's portal steps and hand checks from the spec)

- [ ] **Step 1: README section.** What the card shows, that it goes only to the local Discord app, how to turn it off (`discord_presence = false` or Settings, Agents), how to hide the name (`discord_show_project = false`). No version number.
- [ ] **Step 2: HANDOFF.** Add the status line and the owner's to-do: create the `Kaze` application, upload the wind K as `kaze`, send the Application ID, then the Mac and Windows hand checks listed in the spec. Note that 0.41 waits for this.
- [ ] **Step 3: Run** `go test ./...`. Expected: PASS (docs only, nothing should change).
- [ ] **Step 4: Commit** `docs(discord): Explain the presence card and the owner's setup`.
- [ ] **Step 5: Push the branch and open a PR** referencing issue #55 ("Part of #55", not "Closes", since the release and the hand checks remain).

After the PR merges: the owner sends the Application ID; a short follow-up PR sets `ClientID`; the owner checks the card; then the usual 0.41 release (version bump, changelog, lander row, news strip). Those are outside this plan.
