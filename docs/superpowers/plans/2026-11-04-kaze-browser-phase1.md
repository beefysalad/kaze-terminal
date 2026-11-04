# Agent Browser Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship issue #145 as release 0.29: every Kaze session can have its own headless Chrome (else Edge) that Claude Code and Codex drive through a `kaze-browser` MCP tool, started on the agent's first browser call, closed with the session, after 10 quiet minutes or on a crash, with nothing left behind; a small browser mark on the session row and the browsers' memory in the status bar. No Browser tab yet (Phase 2).

**Architecture:** Four build lanes and a gate lane in separate worktrees off `feat/browser-phase1`, after one small shared task (T0, config keys). **Lane A** (`internal/browser`, new) finds Chrome or Edge, launches it headless with a fresh profile in a Windows job object or a Unix process group, and serves one Kaze-wide listener on `127.0.0.1` where each session has its own unguessable address (`KAZE_BROWSER_CDP`); the first connection to an address starts that session's browser, then Kaze pipes bytes to it, counts the agent's bytes as activity and closes the browser with one one-shot idle timer. Kaze speaks no CDP in Phase 1. **Lane M** (`internal/browsermcp`, new, plus `kaze browser-mcp` in `cmd/kaze`) is the MCP shim: it answers `initialize` and `tools/list` from a tool list captured from the pinned Playwright MCP and verified by a golden test, and starts Node (Playwright MCP with `--cdp-endpoint $KAZE_BROWSER_CDP`) only on the first `tools/call`, then relays; outside Kaze or in a hidden folder it lists no tools. It also finds Node and installs the pinned Playwright MCP into Kaze's cache folder from an embedded lockfile. **Lane S** (setup) registers `kaze-browser` for Claude Code (`~/.claude.json`, user scope) and Codex (`~/.codex/config.toml`), adds the optional steering line, and puts a "Browser for agents" block in Settings, Agents (browser found, Node, install, both registrations, idle minutes, hidden folders) plus a palette command to hide the tool in the current project. **Lane U** (UI) gives each tab its address, closes a session's browser when the tab closes or its shell exits, draws the row mark and the status bar item from the existing resource sampler, and wires the hub into the window. **Lane G** builds a local fixture app and a consent-gated driver that runs real Claude Code and Codex turns against it. F1 merges, measures and writes the docs.

**Tech Stack:** Go 1.27, Gio v0.10.3, `golang.org/x/sys/windows` v0.49.0 (job objects, `LazySystemDLL` for `NtResumeProcess`), `golang.org/x/sys/unix`, `net`, `net/http` (`ReadRequest` only), `embed`, existing `internal/orderedjson`, `internal/claudesetup` (`Write`, `Diff`), `internal/config` (`Set`), `internal/resources`, `internal/procs`, `internal/theme`, `render.Readable`. Test-only: `golang.org/x/net/websocket` (already in `go.sum`, never linked into `kaze`). External, not linked: Node.js 20 or later and `@playwright/mcp` 0.0.84 (Apache-2.0).

**Inputs:** issue #145; design `docs/superpowers/specs/2026-11-04-kaze-agent-browser-phases-design.md` (sections 5, 6.1, 6.2, 7, 8); what still holds of `docs/superpowers/specs/2026-10-30-kaze-agent-browser-design.md` (goals 1, 3, 5, 6, anti-goals, constraints, 6.5 "session = one `KAZE_TAB_ID`"); evidence `docs/audits/2026-11-01-agent-browser-feasibility.md`, `docs/audits/2026-11-04-agent-browser-windows.md` and `spike/agent-browser/` (flags in `probe.mjs` `launchBrowser`, the relay in `protocol.mjs`, `results/windows-x64-chrome-headless-node-default.json`: 25 Playwright tools, first call about 1.5 s, reconnect after a browser close succeeded on the first call, no leftover process, profile removed). On behaviour the issue and the 2026-11-04 design win; where this plan deviates it says so in a ruling.

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: every test that starts a process has a `time.After` or context deadline (helper processes exit by themselves after at most 10 s), every read of a pipe or socket has a deadline, fixed fixture sizes (at most 3 sessions, 8 concurrent connections, 1 MiB MCP lines), no unbounded loops, no unbounded buffers.
- **Tests that need a real browser, real Node or real Playwright MCP are opt-in** and skip cleanly otherwise: `KAZE_BROWSER_REAL=1` (Chrome or Edge), `KAZE_BROWSER_MCP_GOLDEN=1` and `KAZE_BROWSER_MCP_INSTALL=1` (Node and network). CI (`.github/workflows/test.yml`: `windows-latest`, `macos-15`) does not set them, although both images ship Chrome, Edge and Node (Ruling 24). Every other test uses fakes: a helper process re-executed from the test binary (`TestMain` with `KAZE_BROWSER_HELPER=<mode>`) that plays a browser or a Playwright MCP server.
- No global keyboard or mouse input, ever. The browser is driven only through its own CDP connection; UI tests call handlers or queue events on an `input.Router` (`testkit_test.go`).
- **Never take focus.** The browser always runs `--headless=new`. Every console program Kaze or the shim starts on Windows (`node`, `npm`) gets `SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}`; `kaze.exe` is a GUI-subsystem program, so without it each would open a console window (Ruling 21).
- **Idle CPU zero.** No ticker anywhere. The only timer is one pending `time.AfterFunc` per running browser (Ruling 9). With no browser running, Kaze has one goroutine blocked in `Accept` and nothing else. The shim blocks on stdin. Status memory comes from the existing resource sampler on frames that happen anyway (`memEvery`).
- **Addresses:** the listener binds `127.0.0.1` only; each session's address carries a 128-bit random token and is put only in that tab's environment; the browser's own port (also `127.0.0.1`) is never given out. Accepted local-machine trust (design 8).
- **Nothing runs until needed:** no browser until the first connection to an address; no Node until the first `tools/call`. **Nothing left after:** browser process tree gone and profile folder deleted on every close path.
- No secrets committed: the gate reports keep tool names, timings and booleans only, never prompts' answers, tokens, cookies or screenshots. Profiles are temporary.
- Colours only from the theme (`Chrome, Active, Muted, StatusFG, Folder`) with `render.Readable` nudges: status text at least 4.5:1 on `Chrome`, the row mark (non-text) at least 3:1 on both `Chrome` and `Active`, in all five `theme.All()` themes.
- Allocations: the sidebar keeps zero allocations on unchanged frames (`TestSidebarCache*`), and an unchanged status frame allocates no more than on `main` with the browser item shown.
- **Binary size measured** in F1 (stripped Windows build before and after); budget 300 KB (Ruling 27).
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`). Every `go test` in this plan means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off. The helper-process tests re-execute the test binary, which App Control already allowed.
- **No C compiler on this Windows machine.** Nothing here needs cgo. `GOOS=darwin go vet ./internal/browser/ ./internal/browsermcp/ ./internal/procs/` runs locally; `./internal/ui` on darwin is left to the macOS CI job.
- Never touch the owner's daily Kaze (`C:\Users\john\AppData\Local\Programs\Kaze\kaze.exe`), and never write the owner's real `~/.claude.json`, `~/.claude/CLAUDE.md`, `~/.codex/*` from a test or the gate: tests use temp homes; the gate registers the tool per run (Ruling 25).
- Commits: `type(scope): Verb ...`, **no AI attribution lines of any kind**. Never push to `main`; the owner merges. Release 0.29 (version, changelog, lander "What's new") is its own PR, not in this plan.

## Lanes, shared files and merge order

T0 runs first on `feat/browser-phase1`. Then each lane: `git worktree add ../kaze-browser-<lane> -b feat/browser-phase1-<lane> feat/browser-phase1`, merging back into `feat/browser-phase1`.

| Lane | Tasks | Owns |
|---|---|---|
| T0 (shared, first) | T0 | `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/write.go`, `internal/config/write_test.go` |
| **A** browser lifecycle and address | A1 to A8 | `internal/browser/*` (new package), `internal/procs/args.go` (new, exported `Args`), `internal/procs/args_test.go` (new) |
| **M** MCP shim | M1 to M5 | `internal/browsermcp/*` (new package, including `npm/package.json`, `npm/package-lock.json`, `tools.json`, `server.json`), `cmd/kaze/main.go` (one dispatch line), `cmd/kaze/browsermcp.go` (new), `cmd/kaze/browsermcp_test.go` (new) |
| **S** setup and settings | S1 to S5 | `internal/claudesetup/mcp.go`, `mcp_test.go`, `steer.go`, `steer_test.go` (new), `internal/codexsetup/mcp.go`, `mcp_test.go` (new), `internal/ui/settings_browser.go`, `settings_browser_test.go` (new), `internal/ui/settings_codex.go` (`agentsRows`, `agentsAct`, `agentsDialogBox` only), `internal/ui/commands.go`, `internal/ui/commands_test.go`, `internal/ui/window.go` (one field line: `bset browserSetup`) |
| **U** UI and wiring | U1 to U5 | `internal/tabs/manager.go`, `internal/tabs/browser_test.go` (new), `internal/ui/browser.go`, `browser_test.go` (new), `internal/ui/sidebar_model.go`, `sidebar_cache.go`, `sidebar.go`, `sidebar_browser_test.go` (new), `internal/ui/status.go` (`statusModel` only), `statusbar.go`, `statusbar_draw.go`, `statusbar_test.go`, `chrome_cache.go` (`sameStatus` only), `internal/ui/resources.go` (`resourceRoots` only), `internal/ui/window.go` (the `browsers` field, hub init in the `win.mgr == nil` block, `shutdown`) |
| **G** gate | G1 to G3 | `bench/browsergate/*` (new), `docs/audits/browser-phase1/*` (new, reports) |
| Final | F1 | `README.md`, `docs/index.html` (feature cards only), `docs/HANDOFF.md`, `docs/audits/2026-11-04-browser-phase1.md` (new) |

**Files two lanes touch** (each edit stays inside the named lines; the owner resolves any conflict):

| File | Lane S | Lane U |
|---|---|---|
| `internal/ui/window.go` | one field line `bset browserSetup` in the `window` struct | the `browsers browserHub` field (its own line, away from S's), the hub creation next to `SetNotify` in the `win.mgr == nil` block of `frame`, `win.browsers.Close()` in `shutdown` beside `win.srv.Close()`, the one-time `browser.Sweep` start |

No other file is shared. `internal/config` is edited only in T0; later lanes call `config.Set` and read the new fields.

**Cross-lane contracts (fixed here; nobody renames them):**

- T0: `Config.BrowserIdleMinutes int` (`browser_idle_minutes`, default 10, `0` = never, max 1440), `Config.BrowserHiddenFolders []string` (`browser_hidden_folders`), action name `toggle_project_browser` in `config.KnownActions`, `config.Set(path, key, []string{...})` writes a TOML string array.
- Lane A, `package browser`:
  - `type Exe struct{ Path, Name string }` (`Name` is `"Chrome"` or `"Edge"`), `var ErrNoBrowser`, `func Find() (Exe, error)`.
  - `type Options struct { Root string; Idle func() time.Duration; OnChange func(); Find func() (Exe, error) }` (`Root ""` = `DefaultRoot()`, `Idle` read at every arm, `<= 0` never; `OnChange` from any goroutine).
  - `func DefaultRoot() string` (`<os.TempDir()>/kaze-browser`), `func NewHub(Options) *Hub`, `func (*Hub) Address(session int) (string, error)`, `func (*Hub) Has(session int) bool`, `func (*Hub) Running() []Running` with `type Running struct{ Session, PID int; Name string }`, `func (*Hub) CloseSession(session int)`, `func (*Hub) Close()` (bounded 3 s), `func Sweep(root string) error`.
  - Address format: `ws://127.0.0.1:<port>/devtools/browser/<32 lowercase hex>`. Status endpoint: `GET http://127.0.0.1:<port>/kaze/status/<token>` answers `200 {"ok":true,"browser":"Chrome"}` or `200 {"ok":false,"error":"<plain sentence>"}`; an unknown token gets `404` (Ruling 15).
- Lane M, `package browsermcp`: `const Version = "0.0.84"`, `func FindNode(getenv func(string) string, home string) (string, error)`, `func NodeVersion(ctx, node string) (string, error)`, `func Dir() string`, `func Installed(dir string) (version string, ok bool)`, `func Install(ctx context.Context, node, dir string, log io.Writer) error`, `const MinNode = 20`. The served name is `kaze-browser`; the subcommand is `kaze browser-mcp` with no arguments.
- Lane U defines in `internal/ui/browser.go`: `type browserHub interface { Address(int) (string, error); Has(int) bool; Running() []browser.Running; CloseSession(int); Close() }` (`*browser.Hub` satisfies it), and `func browserOwner(id tabs.TabID) int { return -1000 - int(id) }` for resource rows.

**Merge order:** T0 → A → M → S → U → G (G1 may be written any time; G2 needs A and M merged). Before S4 and S5, merge `feat/browser-phase1` (with M) into the S worktree. Before U5, merge `feat/browser-phase1` (with A) into the U worktree. Then F1 on `feat/browser-phase1`.

## Review Focus

1. **A session's address reaches only its own browser,** and a stale or wrong token reaches nothing and launches nothing. (A5 `TestAddressReachesOnlyItsSession`, `TestUnknownTokenLaunchesNothing`; A8 `TestRealSessionsAreIsolated`.)
2. **Nothing runs before the first call:** no browser for `Address()` alone; no Node for `initialize`/`tools/list`/`ping`. (A5 `TestAddressStartsNothing`, M2 `TestDiscoveryStartsNothing`, G2 process check.)
3. **Nothing left after any close path:** tab close, shell exit, Kaze quit, idle, crash; process tree gone and profile removed. (A3, A6, U1 `TestEndCalledOncePerTab`, A8 `TestRealCloseLeavesNothing`, G2.)
4. **No timer without a browser and at most one pending timer with one.** (A6 `TestNoTimerWithoutBrowser`, `TestIdleTimerIsOneShot`.)
5. **No focus:** headless flag always present; `CREATE_NO_WINDOW` on every console child. (A2 `TestArgsAlwaysHeadless`, M4 `TestInstallHidesConsole`, M5 `TestChildHidesConsole`.)
6. **Config edits are safe:** diff shown, confirm, `.bak`, "changed meanwhile" re-check, Taken entries never overwritten. (S1, S2, S4.)
7. **The relay is faithful:** the agent sees the pinned list before start and the same server after; ids and server-to-client requests pass through untouched. (M1 golden, M3.)

---

### Task T0 (shared, first): Config keys for the browser

**Files:**
- Modify: `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/write.go`, `internal/config/write_test.go`

**Interfaces:**
- Produces: `BrowserIdleMinutes int` (`toml:"browser_idle_minutes"`, default 10); a negative or non-integer value warns and uses 10; above 1440 warns and caps at 1440. `BrowserHiddenFolders []string` (`toml:"browser_hidden_folders"`, default empty); a non-list or non-string entry warns and is dropped; blank entries are dropped. `KnownActions["toggle_project_browser"] = true`. `tomlLiteral` accepts `[]string` (`["a", "b"]`, each element through the string case).
- [ ] **Step 1: Write the failing tests:** `TestBrowserIdleDefault` (10), `TestBrowserIdleZeroMeansNever` (0 kept), `TestBrowserIdleBadValues` (table: -1, "ten", 99999 → 10, 10, 1440, each with a warning), `TestBrowserHiddenFolders` (list kept in order; `[1, "x", ""]` → `["x"]` and one warning), `TestSetWritesStringList` (`Set(path, "browser_hidden_folders", []string{`C:\a`, "~/b"})` round-trips through `Load`, backslashes escaped, other lines and comments kept), `TestToggleProjectBrowserIsKnown`.
- [ ] **Step 2: Run** `go test ./internal/config/` → FAIL (undefined fields).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/config/` → PASS.
- [ ] **Step 5: Commit** `feat(config): Add browser idle minutes and hidden folders`

---

## Lane A: browser lifecycle and address (`internal/browser`), Tasks A1 to A8

### Task A1 (Lane A): Find Chrome, else Edge

**Files:**
- Create: `internal/browser/find.go`, `find_windows.go`, `find_darwin.go`, `find_other.go` (`//go:build !windows && !darwin`, always `ErrNoBrowser`), `find_test.go`

**Interfaces:**
- Produces: `Find()` checks a fixed list with a package var `statFile = os.Stat` (regular file only). Windows, in order: `%ProgramFiles%\Google\Chrome\Application\chrome.exe`, `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`, `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`, then `%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`, `%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`. macOS: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, `~/Applications/Google Chrome.app/...`, then the same two for `Microsoft Edge.app/Contents/MacOS/Microsoft Edge`. `ErrNoBrowser` text: "Kaze found no Chrome or Edge. Install Google Chrome, then try again." (Rulings 13, 14). An internal `candidates(getenv, home) []Exe` is what the tests table.
- [ ] **Step 1: Write the failing tests:** `TestFindPrefersChrome` (both present → Chrome), `TestFindPerUserChrome` (only the LocalAppData one), `TestFindFallsBackToEdge`, `TestFindNoneSaysWhatToDo` (`errors.Is(err, ErrNoBrowser)` and the text), `TestFindSkipsDirectories` (a folder named `chrome.exe` is not a browser). OS-specific candidate tests carry the OS build tag.
- [ ] **Step 2: Run** `go test ./internal/browser/ -run TestFind` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browser/` → PASS; `GOOS=darwin go vet ./internal/browser/`.
- [ ] **Step 5: Commit** `feat(browser): Find Chrome, else Edge`

### Task A2 (Lane A): Launch flags and a fresh profile

**Files:**
- Create: `internal/browser/profile.go`, `profile_test.go`

**Interfaces:**
- Produces:
  - `func args(profile string) []string`: `--headless=new`, `--user-data-dir=<profile>`, `--remote-debugging-address=127.0.0.1`, `--remote-debugging-port=0`, `--no-first-run`, `--no-default-browser-check`, `--disable-extensions`, `--disable-background-networking`, `--disable-component-update`, `--disable-sync`, `--disable-default-apps`, `--metrics-recording-only`, `--disable-features=Translate,MediaRouter`, `--window-size=1280,800`, `about:blank` (the spike's set, Ruling 11).
  - `func newProfile(root string, session int) (string, error)`: `MkdirAll(root, 0o700)`, then `MkdirTemp(root, fmt.Sprintf("%d-%d-", os.Getpid(), session))`; writes `kaze-owner` (`{"kaze":<pid>}`; the browser PID is added by `writeOwner` after launch) and `Default/Preferences` with `{"download":{"default_directory":"<profile>/downloads","prompt_for_download":false},"profile":{"default_content_setting_values":{"notifications":2}}}` so downloads stay inside the profile (Ruling 12).
  - `func removeProfile(dir string) error`: `os.RemoveAll` with up to 20 retries 100 ms apart (Windows file locks for a moment after exit), refusing any path not directly under the hub's root.
- [ ] **Step 1: Write the failing tests:** `TestArgsAlwaysHeadless` (contains `--headless=new` and no other `--headless`), `TestArgsLoopbackOnly` (port 0, address 127.0.0.1, no `0.0.0.0`), `TestNewProfileLayout` (name prefix `<pid>-<session>-`, `kaze-owner` JSON, Preferences JSON parses and its download folder is inside the profile), `TestRemoveProfileRefusesOutsideRoot`, `TestRemoveProfileRetriesBounded` (a fake remover failing 3 times then working; and one always failing returns an error after 20 tries without sleeping in the test: the sleep is a package var).
- [ ] **Step 2: Run** `go test ./internal/browser/ -run 'TestArgs|TestNewProfile|TestRemoveProfile'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browser/` → PASS.
- [ ] **Step 5: Commit** `feat(browser): Give each browser a fresh profile and fixed flags`

### Task A3 (Lane A): Own the process tree (job object, process group)

**Files:**
- Create: `internal/browser/proc.go`, `proc_windows.go`, `proc_unix.go` (`//go:build !windows`), `proc_test.go`, `helper_test.go` (`TestMain` and helper modes, shared by later tasks)

**Interfaces:**
- Produces: `type proc struct` with `func start(exe string, args []string) (*proc, error)`, `PID() int`, `Done() <-chan struct{}` (closed when the main process exits; one goroutine in `cmd.Wait`, which is blocked, not polling), `Kill()` (idempotent).
  - Windows: `CreateJobObject` with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`; the child starts with `CREATE_SUSPENDED` and `HideWindow`, is assigned with `AssignProcessToJobObject`, then resumed with `NtResumeProcess` (`windows.NewLazySystemDLL("ntdll.dll")`); `Kill` is `TerminateJobObject` then `CloseHandle`. If Kaze itself dies, Windows closes the job handle and kills every browser (Ruling 10).
  - Unix: `SysProcAttr{Setpgid: true}`; `Kill` sends `SIGTERM` to `-pgid`, waits up to 2 s on `Done`, then `SIGKILL` to `-pgid`.
- Helper modes in `helper_test.go` (`KAZE_BROWSER_HELPER`): `tree` (starts itself as `sleep`, prints the child's PID, sleeps at most 10 s), `sleep` (sleeps at most 10 s), and later `fakebrowser` (A4).
- [ ] **Step 1: Write the failing tests:** `TestKillEndsTheWholeTree` (start `tree`, read the grandchild PID, `Kill`, both PIDs gone within 3 s, checked by a package helper `alive(pid)`), `TestDoneClosesOnExit` (`sleep` killed from outside → `Done` closes within 3 s), `TestKillIsIdempotent`; Windows only: `TestChildIsInJob` (`IsProcessInJob` true for the child and grandchild), `TestClosingJobHandleKills` (close the handle only, both PIDs gone within 3 s: the "Kaze crashed" case).
- [ ] **Step 2: Run** `go test ./internal/browser/ -run 'TestKill|TestDone|TestChildIsInJob|TestClosingJob'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browser/` → PASS; `GOOS=darwin go vet ./internal/browser/`.
- [ ] **Step 5: Commit** `feat(browser): Keep each browser's processes in one job or group`

### Task A4 (Lane A): Read the browser's port, bounded

**Files:**
- Create: `internal/browser/port.go`, `port_test.go`; Modify: `helper_test.go` (mode `fakebrowser`)

**Interfaces:**
- Produces: `func waitPort(profile string, p *proc, limit time.Duration) (port int, path string, err error)`: reads `<profile>/DevToolsActivePort` every 25 ms (only during a launch, never while idle) until two lines parse (port 1 to 65535, path starting `/devtools/browser/`), the process exits (error with "the browser closed while starting"), or `limit` passes (launch uses 10 s).
- `fakebrowser` helper: parses `--user-data-dir`, listens on `127.0.0.1:0`, writes `DevToolsActivePort`, answers one HTTP upgrade per connection with `101` and then echoes bytes prefixed by its own session label from `KAZE_FAKE_LABEL`; exits after 10 s or on stdin close.
- [ ] **Step 1: Write the failing tests:** `TestWaitPortReadsFile` (file written after 100 ms), `TestWaitPortProcessExit` (helper `sleep` killed → error well before the limit), `TestWaitPortMalformed` (`"x\n/devtools/browser/a"`, `"70000\n..."`, missing second line → keeps waiting, then times out at a 300 ms test limit), `TestWaitPortTimeoutBounded` (returns within limit + 200 ms).
- [ ] **Step 2: Run** `go test ./internal/browser/ -run TestWaitPort` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browser/` → PASS.
- [ ] **Step 5: Commit** `feat(browser): Read the browser's debugging port as it starts`

### Task A5 (Lane A): The hub and each session's address

**Files:**
- Create: `internal/browser/hub.go`, `forward.go`, `hub_test.go`

**Interfaces:**
- Produces: `NewHub`, `Address`, `Has`, `Running`, `Close` per the contracts. Unexported test hooks on `Options`: `launch func(session int, profile string) (*proc, error)` (default: `Find` then `start(exe.Path, args(profile))`) and `afterFunc func(time.Duration, func()) stopper` (A6).
  - `Address(session)` opens the listener on the first call (`net.Listen("tcp", "127.0.0.1:0")`, one accept goroutine), makes the session's token (`crypto/rand`, 16 bytes, hex) once and returns the same address every time. It never starts a browser.
  - Each accepted connection: 5 s deadline and a 16 KiB cap (`io.LimitReader` under `bufio.Reader`) for the request head, `http.ReadRequest`; `GET /kaze/status/<token>` → the status JSON (A6 fills in `ok:false` cases); `GET /devtools/browser/<token>` with `Upgrade: websocket` → `ensure(session)` (start if not running; concurrent callers wait on the same start; 15 s limit; a failure answers `503` with the error sentence and leaves nothing running), then dial `127.0.0.1:<browser port>`, rewrite `URL.Path` to the browser's own path and `Host` to `127.0.0.1:<port>`, `req.Write(upstream)`, flush any bytes `bufio` already read, then copy both ways. Client-to-browser reads go through a counting reader that calls `touch(session)` (A6). Anything else → `404`, connection closed, nothing launched.
  - The hub tracks every open client connection per session, so a close (A6) can end them.
- [ ] **Step 1: Write the failing tests** (fake `launch` = the `fakebrowser` helper with `KAZE_FAKE_LABEL=s<session>`; a raw `net.Dial` client writes an upgrade request and reads the `101` and an echo):
  - `TestAddressStartsNothing`: three `Address` calls → same string, matches `^ws://127\.0\.0\.1:\d+/devtools/browser/[0-9a-f]{32}$`, launch count 0, `Has(1) == false`.
  - `TestFirstConnectionStartsOneBrowser`: 8 concurrent upgrades on session 1 → launch count 1, every client echoed with label `s1`.
  - `TestAddressReachesOnlyItsSession`: sessions 1 and 2 → each echo carries its own label; session 1's token with session 2's path prefix never reaches `s2`.
  - `TestUnknownTokenLaunchesNothing`: wrong token, missing token, `POST`, no `Upgrade` header, a 20 KiB header → 404 (or closed), launch count 0.
  - `TestRequestIsRewritten`: the fake browser sees `GET /devtools/browser/<its path>` and `Host: 127.0.0.1:<its port>`.
  - `TestListenerIsLoopback`: the listener address is `127.0.0.1`.
  - `TestLaunchFailureAnswers503`: a `launch` returning `ErrNoBrowser` → `503` whose body has the sentence; the next connection tries again.
- [ ] **Step 2: Run** `go test ./internal/browser/ -run 'TestAddress|TestFirstConnection|TestUnknownToken|TestRequestIs|TestListenerIs|TestLaunchFailure'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/browser/` → PASS.
- [ ] **Step 5: Commit** `feat(browser): Give each session its own browser address`

### Task A6 (Lane A): Idle timer, close, crash and the status endpoint

**Files:**
- Create: `internal/browser/life.go`, `life_test.go`; Modify: `hub.go`

**Interfaces:**
- Produces:
  - `touch(session)`: an atomic store of `time.Now()` in the running browser's `last`. No timer work on the hot path.
  - Arming: when a browser starts, `arm(b)` with `d := Idle()`; `d <= 0` arms nothing. The timer callback: if `now - last >= d`, close the browser; else re-arm once for `d - (now - last)`. At most one pending timer per browser, and none when no browser runs (Ruling 9). `Idle` is read again at every arm, so a Settings change applies from the next arm.
  - `closeBrowser(b, reason)`: stop its timer, close its client connections, `Kill`, `removeProfile`, drop it from the map, call `OnChange`. Reasons: `idle`, `session` (`CloseSession`), `quit` (`Close`), `crash` (its `Done` closed without Kaze asking; the waiting goroutine calls `closeBrowser(b, "crash")`).
  - `CloseSession(session)` also forgets the token, so the old address answers 404 (a later `Address` gives a new one; Lane U only calls it for ended tabs).
  - `Close()`: closes every browser in parallel and the listener, returning within 3 s.
  - Status endpoint: `ok:true` with the running browser's name, or the result of `Find` when none runs (`ok:false` and the `ErrNoBrowser` sentence when neither exists). Never starts a browser.
- [ ] **Step 1: Write the failing tests** (fake clock: `afterFunc` records each arm and lets the test fire it):
  - `TestNoTimerWithoutBrowser`: `Address` for 3 sessions → 0 arms.
  - `TestIdleTimerIsOneShot`: a start → exactly 1 arm; fire with no traffic → closed, 0 pending, profile gone, `OnChange` called.
  - `TestActivityPostponesClose`: traffic at t+6 min of 10, fire at t+10 → not closed, 1 re-arm for 6 min; fire again → closed.
  - `TestIdleZeroNeverArms`, `TestIdleReadAtEachArm` (change `Idle` between two browser starts).
  - `TestCloseSessionEndsConnections`: an open client gets EOF within 1 s; the old address now 404; the profile is gone.
  - `TestCrashClearsAndRelaunches`: kill the fake browser from outside → `Has` false, profile gone, `OnChange` called within 3 s; the next connection launches again (count 2).
  - `TestCloseAllBounded`: 3 running → `Close` returns within 3 s, nothing alive.
  - `TestStatusEndpoint`: unknown token 404; no browser running and a `Find` returning Chrome → `{"ok":true,"browser":"Chrome"}` with launch count 0; `Find` failing → `ok:false` with the sentence.
- [ ] **Step 2: Run** `go test ./internal/browser/ -run 'TestNoTimer|TestIdle|TestActivity|TestCloseSession|TestCrash|TestCloseAll|TestStatusEndpoint'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/browser/` → PASS.
- [ ] **Step 5: Commit** `feat(browser): Close a session's browser when idle, on close or after a crash`

### Task A7 (Lane A): Sweep what a crashed Kaze left

**Files:**
- Create: `internal/browser/sweep.go`, `sweep_test.go`, `internal/procs/args.go`, `internal/procs/args_test.go`

**Interfaces:**
- Produces: `procs.Args(pid int) (exe string, args []string)` exported over the existing `processArgs` (darwin and linux; empty elsewhere). `Sweep(root)`: for each directory entry `^\d+-\d+-` (never following links), read `kaze-owner`; skip if the Kaze PID is alive and `procs.Args` names a `kaze` exe (on Windows: alive is enough); otherwise, on Unix, if the recorded browser PID is alive and its args contain this profile path, kill its process group; then `removeProfile`. Package vars `aliveFn`, `argsFn` for tests. Run once per Kaze start, in the background (Lane U).
- [ ] **Step 1: Write the failing tests:** `TestSweepRemovesDeadOwners`, `TestSweepKeepsLiveKaze`, `TestSweepIgnoresOtherNames` (a folder `notes` and a file stay), `TestSweepNeverFollowsLinks` (skip on Windows without symlink rights), `TestSweepKillsOrphanOnlyWhenArgsMatch` (Unix: fake args without the path → no kill); `TestArgsOfSelf` in `procs` (darwin, linux: own PID gives the test binary).
- [ ] **Step 2: Run** `go test ./internal/browser/ ./internal/procs/ -run 'TestSweep|TestArgsOf'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browser/ ./internal/procs/` → PASS; `GOOS=darwin go vet ./internal/browser/ ./internal/procs/`.
- [ ] **Step 5: Commit** `feat(browser): Clean up browsers a crashed Kaze left behind`

### Task A8 (Lane A): Opt-in checks with a real browser

**Files:**
- Create: `internal/browser/real_test.go` (skips unless `KAZE_BROWSER_REAL=1`; uses `golang.org/x/net/websocket`, test only)

- [ ] **Step 1: Write the tests** (each with a 60 s context; a small `cdpCall(ws, method, params)` helper in the test file):
  - `TestRealFirstCallStartsBrowser`: `Browser.getVersion` through the address → product starts with `Chrome/` or `Edg`; logs the start time in ms.
  - `TestRealSessionsAreIsolated`: session 1 `Target.createTarget` a `data:` page; session 2 `Target.getTargets` does not list it and `Target.attachToTarget` with its id fails.
  - `TestRealBrowserIsLoopbackOnly`: dialing the browser's port on the machine's first non-loopback IPv4 fails (skip when there is none).
  - `TestRealDownloadsStayInProfile`: an `httptest` server sends `Content-Disposition: attachment; filename=kaze-<rand>.txt`; navigate there → the file appears under the profile's `downloads` and not in `~/Downloads`.
  - `TestRealCloseLeavesNothing`: `CloseSession` → no live process from the run (job membership on Windows, `procs.Args` scan for the profile path on macOS) and the profile is gone; then the idle path with `Idle` 1 s does the same.
  - `TestRealNoWindows` (Windows): `EnumWindows` finds no visible top-level window owned by a browser PID while a page loads.
- [ ] **Step 2: Run** with `KAZE_BROWSER_REAL=1` on Windows with Chrome; then once with Chrome hidden (`statFile` override env `KAZE_BROWSER_REAL_EDGE=1`) for Edge. Without the env: all skip.
- [ ] **Step 3:** If any check fails, fix test first in A2 to A6 (for example: new headless ignores `--remote-debugging-address`; the loopback test then decides whether a flag change is needed).
- [ ] **Step 4: Commit** `test(browser): Check a real headless Chrome and Edge, opt-in`

---

## Lane M: `kaze browser-mcp` (`internal/browsermcp`, `cmd/kaze`), Tasks M1 to M5

### Task M1 (Lane M): Pin Playwright MCP and capture its tool list

**Files:**
- Create: `internal/browsermcp/npm/package.json`, `internal/browsermcp/npm/package-lock.json`, `internal/browsermcp/tools.json`, `internal/browsermcp/server.json`, `internal/browsermcp/pinned.go`, `pinned_test.go`, `golden_test.go`, `helper_test.go` (`TestMain`, helper `fakemcp`)

**Interfaces:**
- Produces: `package.json` `{"private":true,"dependencies":{"@playwright/mcp":"0.0.84"}}` and its lockfile made once by the lead with `npm install --package-lock-only --ignore-scripts` (network; the spike's lock already resolved this version). `tools.json`: the exact `tools` array of `tools/list` from the pinned server started with the child flags of M5. `server.json`: `{"serverInfo":…,"capabilities":…,"protocolVersions":[…]}`, the versions being every one the pinned server echoes back when asked (probe `2024-11-05`, `2025-03-26`, `2025-06-18`, `2025-11-25`). All embedded with `//go:embed`. `const Version = "0.0.84"`; `MinNode` taken from the package's `engines.node` at capture (20 if it says `>=18`, Ruling 20).
  - `golden_test.go` (opt-in `KAZE_BROWSER_MCP_GOLDEN=1`; needs Node and an install from M4 or `KAZE_BROWSER_MCP_DIR`): starts the real server over stdio, compares `tools/list` and `initialize` with the embedded files byte for byte after `json.Compact`; with `-update` it rewrites them.
- [ ] **Step 1: Write the failing tests:** `TestPinnedToolsParse` (25 tools, each with `name`, `description`, `inputSchema`; includes `browser_navigate`, `browser_type`, `browser_click`, `browser_take_screenshot`, `browser_console_messages`, `browser_network_requests`, `browser_snapshot`), `TestLockPinsVersion` (lockfile `packages["node_modules/@playwright/mcp"].version == Version`, and every package entry has `resolved` and `integrity`), `TestServerJSONHasVersions`.
- [ ] **Step 2: Run** `go test ./internal/browsermcp/` → FAIL (files missing).
- [ ] **Step 3:** Make the lock; install into a temp dir (M4's `Install` is not there yet: use the spike's `npm ci` command line from `spike/agent-browser/README.md` with `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`); capture with `KAZE_BROWSER_MCP_GOLDEN=1 go test ./internal/browsermcp/ -run TestGolden -update`.
- [ ] **Step 4: Run** `go test ./internal/browsermcp/` → PASS, and the golden test once without `-update` → PASS.
- [ ] **Step 5: Commit** `feat(browsermcp): Pin Playwright MCP 0.0.84 and its tool list`

### Task M2 (Lane M): Answer discovery without Node

**Files:**
- Create: `internal/browsermcp/serve.go`, `serve_test.go`

**Interfaces:**
- Produces: `type Env struct { CDP string; Hidden bool; Start func(ctx context.Context) (Child, error); Status func(ctx context.Context) error }`, `type Child interface { In() io.Writer; Out() io.Reader; Wait() error; Kill() }`, `func Serve(ctx context.Context, in io.Reader, out io.Writer, env Env) error`.
  - Newline-delimited JSON-RPC 2.0, lines capped at 1 MiB (an over-long line gets a `-32700` error and the reader resyncs at the next newline).
  - `initialize` → `protocolVersion`: the client's if it is in `server.json`'s list, else the newest there; `capabilities: {"tools":{}}` (no `listChanged`); `serverInfo: {"name":"kaze-browser","version":"<kaze version> (Playwright MCP 0.0.84)"}`; `instructions`: "A headless browser that belongs to this Kaze session only. Use it to open and check web pages, including local dev servers, unless the project or a skill names its own browser tool. It starts on the first call." Outside Kaze (`CDP == ""`): "kaze-browser only works inside a Kaze session; there are no tools here." Hidden: "kaze-browser is hidden in this folder by Kaze settings."
  - `tools/list` → the pinned list minus `browser_run_code_unsafe`, with "Kaze session browser (headless, private to this session). " prefixed to `browser_navigate`'s description (Ruling 5); empty when outside Kaze or hidden. `ping` → `{}`. Other requests → `-32601`. Notifications are ignored before start.
  - The client's `initialize` params are kept for the replay (M3).
- [ ] **Step 1: Write the failing tests** (pipes, 2 s deadline each): `TestInitializeEchoesKnownVersion`, `TestInitializeFallsBackToNewest`, `TestToolsListIsPinnedMinusUnsafe` (24 tools, navigate prefixed, others byte-equal to `tools.json`), `TestOutsideKazeListsNothing`, `TestHiddenFolderListsNothing`, `TestPingAndUnknown`, `TestOverlongLineRecovers`, `TestDiscoveryStartsNothing` (initialize, initialized, tools/list, ping, 10 more tools/list → `Start` called 0 times).
- [ ] **Step 2: Run** `go test ./internal/browsermcp/ -run 'TestInitialize|TestToolsList|TestOutside|TestHidden|TestPing|TestOverlong|TestDiscovery'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browsermcp/` → PASS.
- [ ] **Step 5: Commit** `feat(browsermcp): Answer MCP discovery from the pinned tool list`

### Task M3 (Lane M): Start Playwright MCP on the first call, then relay

**Files:**
- Create: `internal/browsermcp/relay.go`, `relay_test.go`; Modify: `serve.go`, `helper_test.go` (`fakemcp`: a scripted server that answers `initialize`, records every line it gets, echoes `tools/call` as a result naming the tool, and on `--ask-roots` sends a `roots/list` request first)

**Interfaces:**
- Produces: on the first `tools/call` (not for `browser_run_code_unsafe`, which gets an `isError` result "This tool is turned off in Kaze." without starting anything):
  1. `Status(ctx)` (2 s): an error → an `isError` result with its sentence (for example "Kaze found no Chrome or Edge…" or "This Kaze session's browser address is gone; restart the agent in a Kaze tab."), nothing started.
  2. `Start(ctx)`; an error → an `isError` result with the plain reason (Node missing: "kaze-browser needs Node.js 20 or later. Install it from nodejs.org, then open Kaze Settings, Agents."; not installed: "The browser tool is not installed. Open Kaze Settings, Agents, and press Install.") plus the last 2 KiB of the child's stderr when it exited early. 30 s limit.
  3. Replay `initialize` with the client's params and id `"kaze-init-<n>"`, wait for that response (swallowed), send `notifications/initialized`, then forward the queued call and every later client line; child output is copied to `out` line by line (no parsing after the swallowed response, so screenshots pass at full size).
  4. Requests arriving meanwhile queue in order (bounded at 64; beyond that `-32000` "busy"). `notifications/cancelled` for a queued id drops it.
  5. Child exit: every forwarded request without a response gets `-32000` "The browser tool stopped; try again." The next `tools/call` starts a new child (no restart loop).
- [ ] **Step 1: Write the failing tests** (`fakemcp` as the child; 5 s deadlines): `TestFirstCallStartsOnce` (3 calls → 1 start, 3 results in order), `TestReplayKeepsClientParams`, `TestInitResponseIsSwallowed` (the client sees exactly one `initialize` result, its own), `TestServerRequestsPassThrough` (`roots/list` from the child reaches the client and the client's answer reaches the child, ids untouched), `TestUnsafeToolRefused` (0 starts), `TestStatusErrorStartsNothing`, `TestStartErrorIsReadable` (stderr tail included, capped), `TestChildExitFailsPendingAndRestartsLazily`, `TestQueueBounded` (65th queued request → busy).
- [ ] **Step 2: Run** `go test ./internal/browsermcp/ -run 'TestFirstCall|TestReplay|TestInitResponse|TestServerRequests|TestUnsafe|TestStatusError|TestStartError|TestChildExit|TestQueue'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/browsermcp/` → PASS.
- [ ] **Step 5: Commit** `feat(browsermcp): Start Playwright MCP on the first tool call and relay`

### Task M4 (Lane M): Find Node and install the pinned tool

**Files:**
- Create: `internal/browsermcp/node.go`, `node_windows.go`, `node_other.go`, `install.go`, `node_test.go`, `install_test.go`, `install_real_test.go` (opt-in `KAZE_BROWSER_MCP_INSTALL=1`)

**Interfaces:**
- Produces:
  - `FindNode(getenv, home)`: `exec.LookPath`-style search of `getenv("PATH")` first, then fixed places (Windows: `%ProgramFiles%\nodejs\node.exe`, `%LOCALAPPDATA%\Programs\nodejs\node.exe`, the newest `%APPDATA%\nvm\v*\node.exe`; macOS: `/opt/homebrew/bin/node`, `/usr/local/bin/node`, `~/.volta/bin/node`, the newest `~/.nvm/versions/node/v*/bin/node`). The GUI on macOS does not have the shell's PATH, hence the fixed places (Ruling 20). Package var `statFile`.
  - `NodeVersion(ctx, node)`: `node --version` with a 5 s limit and `CREATE_NO_WINDOW`; "v22.14.0" → "22.14.0".
  - `Dir()`: `KAZE_BROWSER_MCP_DIR` if set, else `<os.UserCacheDir()>/kaze/browser-mcp`.
  - `Installed(dir)`: reads `node_modules/@playwright/mcp/package.json`; ok only when its version equals `Version` and `cli.js` exists.
  - `Install(ctx, node, dir, log)`: writes the embedded `package.json` and lockfile into `dir + ".tmp-<rand>"`, runs `node <npm-cli.js> ci --ignore-scripts --no-audit --no-fund --omit=dev` there (`npm-cli.js` beside Node: Windows `<nodeDir>\node_modules\npm\bin\npm-cli.js`, Unix `<nodeDir>/../lib/node_modules/npm/bin/npm-cli.js`; not found → an error naming Node), env plus `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`, `CREATE_NO_WINDOW`, 5 minute limit, log capped at 64 KiB; on success renames the old `dir` aside, the new one in, and removes the old; on failure removes the temp folder and leaves the old install as it was. Every argument is fixed: no user text reaches the command line. Runner is a package var.
- [ ] **Step 1: Write the failing tests:** `TestFindNodePathFirst`, `TestFindNodeFixedPlaces` (per-OS tags; newest nvm by version, not by name order: v9 < v20), `TestFindNodeNone` (error text), `TestNodeVersionParses` (fake runner), `TestInstalledVersionMismatch`, `TestInstallCommandIsFixed` (recorded argv and env exactly), `TestInstallHidesConsole` (Windows: recorded `SysProcAttr.CreationFlags` has `CREATE_NO_WINDOW`), `TestInstallSwapsOnlyOnSuccess` (failing runner → old install intact, no temp left), `TestInstallLogCapped`; opt-in `TestRealInstall` (temp dir, real Node, network) → `Installed` ok, reports the size in MB.
- [ ] **Step 2: Run** `go test ./internal/browsermcp/ -run 'TestFindNode|TestNodeVersion|TestInstall'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/browsermcp/` → PASS; `KAZE_BROWSER_MCP_INSTALL=1` once on Windows; `GOOS=darwin go vet ./internal/browsermcp/`.
- [ ] **Step 5: Commit** `feat(browsermcp): Find Node and install the pinned browser tool`

### Task M5 (Lane M): The `kaze browser-mcp` subcommand

**Files:**
- Create: `internal/browsermcp/child.go`, `child_test.go`, `hidden.go`, `hidden_test.go`, `cmd/kaze/browsermcp.go`, `cmd/kaze/browsermcp_test.go`
- Modify: `cmd/kaze/main.go` (one `if os.Args[1] == "browser-mcp"` line beside `statusline`, before any window code)

**Interfaces:**
- Produces:
  - `childArgs(dir, cdp, outDir string) []string`: `<dir>/node_modules/@playwright/mcp/cli.js --cdp-endpoint <cdp> --output-dir <outDir> --no-webmcp` (plus any flag the M1 capture used; the golden test runs with exactly these). `StartChild(ctx, node, args)`: stdio pipes, env plus `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`, `CREATE_NO_WINDOW` on Windows, stderr kept as a 2 KiB ring.
  - `Hidden(cwd string, folders []string, home string) bool`: `~` expanded, `filepath.Clean`, equal or inside on a separator boundary, case-insensitive on Windows and macOS (`samurai` does not match `samurai2`).
  - `cmd/kaze`: `runBrowserMCP(stdin, stdout, getenv, getwd) int` builds `Env`: `CDP = getenv("KAZE_BROWSER_CDP")`, hidden from `config.Load(config.Path())` and `getwd()`, `Status` = an HTTP GET of the status URL derived from the address (2 s timeout, loopback only: a non-`127.0.0.1` host is refused), `Start` = `FindNode` + `Installed(Dir())` + `StartChild` with an output dir from `os.MkdirTemp("", "kaze-browser-mcp-")` removed when the shim exits. Exit code 0 when stdin closes.
- [ ] **Step 1: Write the failing tests:** `TestChildArgs`, `TestChildHidesConsole` (Windows), `TestHiddenTable` (Windows and Unix cases, `~`, boundary, case), `TestStatusURLFromAddress` (and a non-loopback host refused), `TestBrowserMCPOutsideKaze` (`runBrowserMCP` with pipes: initialize and tools/list → empty list, exit 0 on EOF), `TestBrowserMCPRunsBeforeWindow` (in the style of `args_test.go`: the dispatch is before `workspaceArg`).
- [ ] **Step 2: Run** `go test ./cmd/kaze/ ./internal/browsermcp/ -run 'TestChild|TestHidden|TestStatusURL|TestBrowserMCP'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./cmd/kaze/ ./internal/browsermcp/` → PASS; build `kaze.exe` in a temp folder and check by hand with a pipe: `printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}' | ./kaze.exe browser-mcp` answers at once and no `node` process appears.
- [ ] **Step 5: Commit** `feat(cmd): Add kaze browser-mcp`

---

## Lane S: setup and settings, Tasks S1 to S5

### Task S1 (Lane S): Register `kaze-browser` for Claude Code

**Files:**
- Create: `internal/claudesetup/mcp.go`, `internal/claudesetup/mcp_test.go`

**Interfaces:**
- Produces: `func InspectMCP(data []byte, exists func(string) bool) Entry` over `~/.claude.json`'s top-level `mcpServers["kaze-browser"]`: OK when `command` is a kaze exe that exists and `args == ["browser-mcp"]` (and `type` absent or `"stdio"`); Broken when it is a kaze exe that is gone or has other args; Taken when the key runs anything else; Missing when absent; Unreadable for invalid JSON or a non-object `mcpServers`. `func FixMCP(data []byte, e Entry, exe string) (out []byte, summary []string, err error)`: Missing adds `{"type":"stdio","command":"<exe>","args":["browser-mcp"],"env":{}}` (creating `mcpServers` when absent), Broken repoints `command` and `args`, OK and Taken unchanged; everything else in the file keeps its order and bytes (orderedjson round trip). The file can be large: callers cap reads at 32 MiB.
- [ ] **Step 1: Write the failing tests:** `TestInspectMCPStates` (table: missing file content `{}`, no key, OK, gone exe, wrong args, another program, bad JSON, `mcpServers` a list), `TestFixMCPAddsOnlyItsKey` (a fixture with `projects`, `oauthAccount` stub values, other servers: only the new key differs in `Diff`), `TestFixMCPRepointsBroken`, `TestFixMCPLeavesTaken`, `TestFixMCPQuotesWindowsPath` (backslashes and spaces survive a parse).
- [ ] **Step 2: Run** `go test ./internal/claudesetup/ -run 'MCP'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/claudesetup/` → PASS.
- [ ] **Step 5: Commit** `feat(claudesetup): Register kaze-browser for Claude Code`

### Task S2 (Lane S): Register `kaze-browser` for Codex

**Files:**
- Create: `internal/codexsetup/mcp.go`, `internal/codexsetup/mcp_test.go`

**Interfaces:**
- Produces: `func InspectMCP(config []byte, exists func(string) bool) Entry` over `[mcp_servers.kaze-browser]` (decoded with BurntSushi/toml): OK when `command` is a kaze exe that exists, `args == ["browser-mcp"]` and `env_vars` contains both `KAZE_TAB_ID` and `KAZE_BROWSER_CDP`; Broken otherwise for a kaze command (Fixable only when the table is a plain `[mcp_servers.kaze-browser]` header block; inline or dotted definitions get Detail "Edit config.toml by hand"); Taken for another command; Missing when absent. `func FixMCP(config []byte, exe string, exists func(string) bool) ([]byte, []string, error)`: Missing appends, after one blank line,
  ```toml
  [mcp_servers.kaze-browser]
  command = '<exe>'
  args = ["browser-mcp"]
  env_vars = ["KAZE_TAB_ID", "KAZE_BROWSER_CDP"]
  tool_timeout_sec = 120
  ```
  (a TOML literal string, so Windows backslashes need no escaping; an exe containing `'` falls back to a basic string); Broken rewrites the `command`, `args` and `env_vars` lines inside that block only. The result must decode and `InspectMCP` it as OK, else an error and nothing is returned for writing. CRLF files stay CRLF.
- [ ] **Step 0 (before the tests): confirm Codex's format** on the installed Codex: `codex --version`, `codex mcp add --help`, and the Codex config docs for `env_vars`, `tool_timeout_sec`, `startup_timeout_sec`. If `env_vars` does not exist in the installed version, stop and report to the coordinator (Ruling 17's fallback needs the owner).
- [ ] **Step 1: Write the failing tests:** `TestCodexInspectMCPStates`, `TestCodexFixMCPAppends` (a config with `model`, `[projects."C:\\x"]`, `[mcp_servers.other]` → only appended lines differ), `TestCodexFixMCPKeepsCRLF`, `TestCodexFixMCPRepairsOwnBlockOnly`, `TestCodexFixMCPRefusesDotted`, `TestCodexFixMCPQuoteFallback`.
- [ ] **Step 2: Run** `go test ./internal/codexsetup/ -run 'MCP'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/codexsetup/` → PASS.
- [ ] **Step 5: Commit** `feat(codexsetup): Register kaze-browser for Codex`

### Task S3 (Lane S): The optional steering line

**Files:**
- Create: `internal/claudesetup/steer.go`, `internal/claudesetup/steer_test.go`

**Interfaces:**
- Produces: `const SteerLine = "- In Kaze, use the kaze-browser tools to open and check web pages, unless the project or a skill names its own browser tool."`; `func HasSteer(md []byte) bool` (any line containing `kaze-browser`); `func AddSteer(md []byte) []byte` (appends the line with a separating newline, keeps CRLF if the file uses it, creates content for an empty or missing file). Used for `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` (Ruling 18).
- [ ] **Step 1: Write the failing tests:** `TestAddSteerToEmpty`, `TestAddSteerKeepsCRLF`, `TestAddSteerNoTrailingNewline`, `TestHasSteerAfterAdd`, `TestAddSteerIsIdempotent`.
- [ ] **Step 2: Run** `go test ./internal/claudesetup/ -run Steer` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/claudesetup/` → PASS.
- [ ] **Step 5: Commit** `feat(claudesetup): Offer a line that steers agents to kaze-browser`

### Task S4 (Lane S): "Browser for agents" in Settings, Agents

**Before this task:** merge `feat/browser-phase1` (with Lane M) into the S worktree.

**Files:**
- Create: `internal/ui/settings_browser.go`, `internal/ui/settings_browser_test.go`
- Modify: `internal/ui/settings_codex.go` (`agentsRows` appends `win.browserRows()` under a "Browser for agents" heading; `agentsAct` and `agentsDialogBox` route browser ids first), `internal/ui/window.go` (field `bset browserSetup`)

**Interfaces:**
- Produces: `type browserSetup struct` holding the last check (browser found, Node path and version, installed version, the Claude entry and file bytes, the Codex entry and file bytes, steer states), an `installing bool`, the pending `*claudeDialog`, and a result channel. `win.loadBrowserSetup()` runs when the Agents page is shown (beside `loadAgents`), never per frame; `browser.Find`, file reads and `Installed` run inline (cheap), `NodeVersion` runs in a goroutine with its 5 s limit and wakes the window when done.
- Rows (one `settingRow` each, pills like `entryPill`):
  1. "Browser" · "Chrome" / "Edge" (ok) or "No Chrome or Edge found. Install Google Chrome." (warn). No button (Ruling 14).
  2. "Node.js" · version (ok), "Node 18 is too old; Kaze needs 20 or later" (warn), "Not found. Install Node.js 20 or later from nodejs.org" (warn).
  3. "Browser tool" · "Playwright MCP 0.0.84" (ok) or Missing / "Other version" → **Install** (`browser_install`, only with Node OK): a confirm dialog "Downloads Playwright MCP 0.0.84 and its packages from npm (about N MB) into <Dir()>. No browser is downloaded. Nothing else changes." → `Install` in a goroutine; the pill reads "Installing…" until done; the result goes to `win.note`.
  4. "Claude Code" · "kaze-browser" entry → **Fix** (`browser_claude_fix`): the usual dialog over `~/.claude.json` (title "Add kaze-browser to Claude Code", summary + `claudesetup.Diff`, "New Claude sessions get the browser; running ones after a restart."); Write re-reads, refuses on "changed meanwhile", `claudesetup.Write` (temp, rename, `.bak-<time>`), then re-reads and notes if the entry is not OK (a running Claude may rewrite the file; Ruling 16).
  5. "Codex" · same over `~/.codex/config.toml` (`browser_codex_fix`).
  6. "Tell Claude to prefer it" and "Tell Codex to prefer it" (optional) · Added / Not added → **Add** (`browser_steer_claude`, `browser_steer_codex`): dialog with the diff, `.bak`, Write.
  7. "Close an idle browser after" · segmented "5 min", "10 min", "30 min", "Never" → `config.Set(path, "browser_idle_minutes", n)` and `win.opt.Config.BrowserIdleMinutes = n` (the hub reads it at the next arm).
  8. "Hidden in" · each folder of `BrowserHiddenFolders` as a row with **Show** (`browser_unhide`, arg = index) that removes it; a hint "Hide it in a project with the palette: Hide or show kaze-browser in this project."
- [ ] **Step 1: Write the failing tests** (`liveWindow` with a temp home holding `.claude.json`, `.claude/CLAUDE.md`, `.codex/config.toml`; package vars for `browser.Find`, `browsermcp.FindNode`, `NodeVersion`, `Installed`, `Install`): `TestBrowserRowsStates` (each pill and text for found / not found), `TestBrowserClaudeFixShowsDiffAndBacksUp` (dialog lines, Write → file has the key, a `.bak-` file exists, other keys unchanged), `TestBrowserFixRefusesChangedFile`, `TestBrowserCodexFix`, `TestBrowserSteerAdd`, `TestBrowserInstallRunsInBackground` (fake `Install` blocking on a channel: the row says "Installing…", the frame is not blocked; release → note), `TestBrowserInstallNeedsNode` (no button without Node), `TestBrowserIdleSegmentWritesConfig`, `TestBrowserUnhideRemovesFolder`, `TestBrowserChecksNotPerFrame` (100 frames of the Agents page → `Find` called once).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBrowser(Rows|ClaudeFix|FixRefuses|CodexFix|SteerAdd|Install|IdleSegment|Unhide|ChecksNot)'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Settings|Agents|Claude|Codex|Browser'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Set up the agent browser in Settings, Agents`

### Task S5 (Lane S): Hide or show `kaze-browser` in this project

**Files:**
- Modify: `internal/ui/commands.go`, `internal/ui/commands_test.go` (and the action switch it routes to, in the same file or the file `actAgentsBoard` lives in if that is `commands.go`)

**Interfaces:**
- Produces: `kazeCommand{"toggle_project_browser", "Hide or show kaze-browser in this project", actToggleProjectBrowser}`. It takes the active session's repo root (`locOf(t).repo`, else its `cwd`); in `BrowserHiddenFolders` (by `browsermcp.Hidden` rules) → removed, note "kaze-browser shows again in <name> for new agent sessions"; else added, note "kaze-browser is hidden in <name> for new agent sessions". Written with `config.Set`; `win.opt.Config` updated. No active session → note "Open a session in the project first".
- [ ] **Step 1: Write the failing tests:** `TestToggleProjectBrowserHides`, `TestToggleProjectBrowserShows`, `TestToggleProjectBrowserUsesRepoRoot` (a session in a subfolder of a repo hides the repo root), `TestToggleProjectBrowserNoSession`, `TestToggleProjectBrowserListed` (palette and Settings, Commands list it).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestToggleProjectBrowser` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Palette|Commands|Toggle'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Hide kaze-browser in a project from the palette`

---

## Lane U: UI and wiring, Tasks U1 to U5

### Task U1 (Lane U): Each tab gets its address; ended tabs say so

**Files:**
- Modify: `internal/tabs/manager.go`; Create: `internal/tabs/browser_test.go`

**Interfaces:**
- Produces: `func (m *Manager) SetBrowserEnv(fn func(TabID) string)` (like `SetNotify`; `newTab` appends `KAZE_BROWSER_CDP=<fn(id)>` when `fn` is set and returns non-empty); `"KAZE_BROWSER_CDP"` added to `droppedEnv`; `func (m *Manager) OnEnd(fn func(TabID))`, called exactly once per tab: from `Close`, from `CloseAllReleased` for each tab, or the first time `notify` sees the shell exited (on the notifier goroutine, so `fn` must be safe from any goroutine). An `ended` atomic flag on `entry` makes it once.
- [ ] **Step 1: Write the failing tests** (`recStarter` fake PTYs): `TestBrowserEnvPerTab` (tabs 1 and 2 get their own values), `TestNoBrowserEnvWithoutProvider`, `TestInheritedBrowserEnvDropped` (a base env with `KAZE_BROWSER_CDP=x` → gone), `TestEndCalledOncePerTab` (shell exits, then `Close` → one call), `TestEndOnCloseAll`.
- [ ] **Step 2: Run** `go test ./internal/tabs/ -run 'TestBrowserEnv|TestNoBrowserEnv|TestInheritedBrowser|TestEnd'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/tabs/` → PASS.
- [ ] **Step 5: Commit** `feat(tabs): Give each tab its browser address and report ended tabs`

### Task U2 (Lane U): The browser mark on the session row

**Files:**
- Modify: `internal/ui/sidebar_model.go` (`sideRow.Browser bool`, `sideInput.Browser func(tabs.TabID) bool`), `internal/ui/sidebar_cache.go` (`sideTabInput.browser bool` so the cache sees a change), `internal/ui/sidebar.go` (`c.browserMark` and its place); Create: `internal/ui/sidebar_browser_test.go`

**Interfaces:**
- Produces: `func (c *chrome) browserMark(gtx layout.Context, box image.Rectangle, col color.NRGBA)`: a still globe outline (circle, one meridian ellipse, the equator) drawn with `clip.Stroke` paths in a 10 dp box, no animation, no frame request. Placed at the right end of the row's detail line, left of the `+N −M` counts; the detail text loses that width plus a gap. Colour `th.Muted`, nudged with `render.Readable` to at least 3:1 against both `th.Chrome` and `th.Active` (Ruling 22). Session rows only; workspace members and the dock carry none in Phase 1. Not clickable.
- [ ] **Step 1: Write the failing tests:** `TestSideRowBrowserFlag` (`buildSide` with a `Browser` func), `TestSidebarCacheSeesBrowser` (cache invalid after the set changes, valid after), `TestSidebarCacheZeroAllocsWithBrowser` (unchanged frames with a mark: 0 allocations), `TestBrowserMarkContrast` (all five themes, both surfaces, ≥ 3:1), `TestBrowserMarkDrawn` (headless render: the mark box holds mark-coloured pixels; the detail text ends before it).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestSideRowBrowser|TestSidebarCacheSees|TestSidebarCacheZeroAllocsWith|TestBrowserMark'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Sidebar|Side'` → PASS (every `TestSidebarCache*`).
- [ ] **Step 5: Commit** `feat(ui): Mark a session that has a browser`

### Task U3 (Lane U): Browser memory in the status bar

**Files:**
- Modify: `internal/ui/status.go` (`statusModel.Browsers browserStat` where `type browserStat struct{ N int; Bytes uint64; Valid bool }`), `internal/ui/statusbar.go` (`grpBrowser` after `grpBattery`; width; compaction), `internal/ui/statusbar_draw.go`, `internal/ui/chrome_cache.go` (`sameStatus`), `internal/ui/statusbar_test.go`

**Interfaces:**
- Produces: `func browserLabel(s browserStat, compact bool) string`: "Browser 275 MB" for one, "2 browsers · 550 MB" for more (`resources.Size`), "Browser" while no reading is valid yet; compact: "275 MB" after a small globe glyph (`browserMark`, reused). Shown only when `N > 0`, placed just left of the CPU/RAM group with a hairline. Dropped right after the battery when the bar is tight (a new compaction step after battery's). Text `StatusFG` nudged to 4.5:1 on `Chrome`. Not clickable in Phase 1.
- [ ] **Step 1: Write the failing tests:** `TestBrowserGroupHiddenWithoutBrowsers`, `TestBrowserLabel` (table: 1 and 2 browsers, invalid, compact), `TestBrowserGroupDropsAfterBattery` (narrowing widths), `TestBrowserStatusContrast` (five themes), `TestStatusBrowserAddsNoAllocations` (unchanged frame with the group shown allocates no more than without it), `TestStatusBrowserCacheHits` (`sameStatus` true for equal stats).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBrowserGroup|TestBrowserLabel|TestBrowserStatus|TestStatusBrowser'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Status'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Show the browsers' memory in the status bar`

### Task U4 (Lane U): Browsers as resource rows

**Files:**
- Modify: `internal/ui/resources.go` (`resourceRoots` only); Create: `internal/ui/browser.go` (the `browserHub` interface, `browserOwner`, `browserStatOf(snapshot, running) browserStat`), `internal/ui/browser_test.go`

**Interfaces:**
- Produces: `resourceRoots` appends, for each `win.browsers.Running()` entry, `resources.Root{PID: r.PID, Owner: browserOwner(id), Name: "Browser · " + <tab name>}` (no hub → nothing). The browser's helper processes are its descendants, so the sampler attributes them to that root; they no longer count as "Other Kaze processes". `statusModel` fills `Browsers` with `browserStatOf` (sum of rows whose ID is a browser owner; `Valid` when every such row has `RAMValid`). The resource hover card lists the rows by their names unchanged.
- [ ] **Step 1: Write the failing tests** (fake hub): `TestResourceRootsIncludeBrowsers`, `TestBrowserStatSumsOnlyBrowserRows`, `TestBrowserStatInvalidUntilRead`, `TestNoHubNoBrowserRoots`, `TestResourceCardNamesBrowser`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestResourceRootsInclude|TestBrowserStat|TestNoHubNoBrowser|TestResourceCardNames'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Resource|Status'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Count each browser's memory under its session`

### Task U5 (Lane U): Wire the hub into the window

**Before this task:** merge `feat/browser-phase1` (with Lane A) into the U worktree.

**Files:**
- Modify: `internal/ui/window.go` (field, init, shutdown, sweep), `internal/ui/browser.go`, `internal/ui/browser_test.go`

**Interfaces:**
- Produces: in the `win.mgr == nil` block, after `SetNotify` and before any tab: unless bench mode, `win.browsers = browser.NewHub(browser.Options{Idle: func() time.Duration { return time.Duration(win.opt.Config.BrowserIdleMinutes) * time.Minute }, OnChange: win.w.Invalidate})` (the config field is read under the same rules other settings use; the hub reads it only when arming), `win.mgr.SetBrowserEnv(func(id tabs.TabID) string { a, _ := win.browsers.Address(int(id)); return a })`, `win.mgr.OnEnd(func(id tabs.TabID) { win.browsers.CloseSession(int(id)) })`, and once `go browser.Sweep(browser.DefaultRoot())`. `sideInput.Browser` is `win.browsers.Has`. `shutdown` closes the hub in parallel with the alert server inside `shutdownWait`. Bench mode: no hub, so no env, no listener (Ruling 26).
- [ ] **Step 1: Write the failing tests** (`liveWindow` with an injected fake hub through a package var `newBrowserHub`): `TestTabsGetBrowserAddress` (the started shell's env has the fake address), `TestClosingTabClosesItsBrowser`, `TestShellExitClosesItsBrowser`, `TestShutdownClosesHub` (within `shutdownWait`), `TestBenchHasNoBrowserHub`, `TestRestoredTabsGetAddress` (session restore path), `TestRowMarkFollowsHub` (`Has` true → the row's `Browser` set).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestTabsGetBrowser|TestClosingTabCloses|TestShellExitCloses|TestShutdownClosesHub|TestBenchHasNo|TestRestoredTabsGet|TestRowMarkFollows'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` and `go test -race ./internal/ui/ ./internal/tabs/ ./internal/browser/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Run a browser per session for agents`

---

## Lane G: the gate, Tasks G1 to G3

### Task G1 (Lane G): A local fixture app

**Files:**
- Create: `bench/browsergate/fixture.go`, `bench/browsergate/fixture_test.go`

**Interfaces:**
- Produces: `func StartFixture() (*Fixture, error)` on `127.0.0.1:0`: `/` is "Kaze gate fixture" with a labelled `Message` textbox, an `Apply` button (sets `#result` to the text and `POST /apply` with it), `console.error("kaze-gate-console-error")` on load, a `fetch("/missing")` answered `500`, and a `fetch` to a closed local port (a failed request). `Fixture.Applied() []string` records `/apply` bodies (capped at 16 entries of 256 bytes). Same idea as the spike's fixture, written in Go.
- [ ] **Step 1: Write the failing tests:** `TestFixturePage` (elements and script present), `TestFixtureRecordsApply`, `TestFixtureMissingIs500`, `TestFixtureCaps`.
- [ ] **Step 2: Run** `go test ./bench/browsergate/` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./bench/browsergate/` → PASS.
- [ ] **Step 5: Commit** `test(browsergate): Add a local fixture app for the browser gate`

### Task G2 (Lane G): A consent-gated driver for real agent turns

**Before this task:** Lanes A and M are merged into `feat/browser-phase1`; merge it into the G worktree.

**Files:**
- Create: `bench/browsergate/main.go`, `bench/browsergate/report.go`, `bench/browsergate/report_test.go`, `bench/browsergate/README.md`

**Interfaces:**
- Produces: `go run ./bench/browsergate --agent claude|codex --consent [--kaze <path>] [--idle 1m]`. Without `--consent` it prints what it would run and exits 2. With it:
  1. Builds `kaze.exe` into a temp folder (or uses `--kaze`), sets `KAZE_BROWSER_MCP_DIR` to a temp folder and installs Playwright MCP there with `browsermcp.Install` (so the owner's cache is untouched), starts the fixture and a `browser.Hub` (idle from `--idle`), and takes session 7's address.
  2. Writes a per-run MCP registration only: Claude: a temp `mcp.json` passed with `--mcp-config` (the owner's own servers stay loaded, so the choice is realistic; their files are not edited); Codex: `-c` overrides for `mcp_servers.kaze-browser.command`, `.args`, `.env_vars`, `.tool_timeout_sec` (Ruling 25).
  3. Runs the agent once, non-interactive, in a temp project folder with `KAZE_TAB_ID=7` and `KAZE_BROWSER_CDP=<address>` in its env and a 6 minute limit: `claude -p <prompt> --output-format stream-json --verbose --allowedTools "mcp__kaze-browser__*"`; `codex exec --json <prompt>` with `-c approval_policy="never"`. The prompt (fixed, in `main.go`): "Check the local web app at <url> in a browser. Type <token> into the Message box, click Apply, and take a screenshot. Then tell me which errors the page logged in the console and which network requests failed." No tool is named, so the choice is the agent's.
  4. Checks and writes `docs/audits/browser-phase1/gate-<agent>-<goos>.json`: the ordered list of tool names used (the first browser-capable tool must be `kaze-browser`'s), `browser_navigate`, `browser_type`, `browser_click`, `browser_take_screenshot`, `browser_console_messages` (or the answer naming `kaze-gate-console-error`) and `browser_network_requests` (or the answer naming `/missing`) seen; the fixture received `<token>`; times (first call, whole turn); after the turn, no Node process left from the shim; then waiting up to `--idle` plus 15 s: the browser closed by idle, no browser process and no profile left. Nothing else is saved: no answer text, no screenshots, no account data.
- [ ] **Step 1: Write the failing tests:** `TestRefusesWithoutConsent`, `TestParseClaudeStream` (a canned stream-json with tool_use names → the ordered list), `TestParseCodexJSON` (canned `codex exec --json` events), `TestReportHasNoText` (a report built from canned input holds only names, numbers and booleans), `TestFirstBrowserToolRule` (a run whose first browser tool is `mcp__playwright__browser_navigate` fails the choice check).
- [ ] **Step 2: Run** `go test ./bench/browsergate/` → FAIL.
- [ ] **Step 3: Implement**, including `README.md` (what it does, what it costs: a few agent turns on the owner's accounts, what it never touches).
- [ ] **Step 4: Run** `go test ./bench/browsergate/` → PASS.
- [ ] **Step 5: Commit** `test(browsergate): Drive real agent turns against the fixture, with consent`

### Task G3 (Lane G, lead with the owner's consent): Run the gate

- [ ] **Step 1:** Ask the owner, in plain words, for consent to run two short turns on their Claude Code and Codex accounts (what runs, about how long, that no config file of theirs is edited). Do not run without a yes.
- [ ] **Step 2:** On Windows with Chrome: `go run ./bench/browsergate --agent claude --consent`, then `--agent codex`. Both must pass every check. A failed choice (another browser tool first) is fixed in the shim's `instructions` or the navigate description (M2) and rerun, at most twice, then reported to the owner. Codex not seeing `KAZE_BROWSER_CDP` (the tool says "only works inside a Kaze session") means Ruling 17's fallback: stop and report.
- [ ] **Step 3:** Idle CPU of Kaze with the hub: build the branch and `main` as local copies in temp folders (never over the owner's copy) and start each with `APPDATA` pointed at a temp folder (so `os.UserConfigDir` gives a throwaway `config.toml` and the owner's settings are not read or written). With one tab open and untouched, read `(Get-Process -Id <pid>).TotalProcessorTime` twice 60 s apart, after a 2 minute settle: the branch must not use more CPU than `main` (the hub adds one goroutine blocked in `Accept` and no timer). Browser idle close and cleanup are already proven at hub level by A6, A8 and G2.
- [ ] **Step 4:** Write the macOS steps into `bench/browsergate/README.md` for the owner's MacBook: run both agents; confirm no Dock icon and no focus change while the browser runs (headless on macOS was not measured by the spike).
- [ ] **Step 5: Commit** `test(browsergate): Record the Windows gate runs`

---

### Task F1 (after all lanes): Merge check, size, audit, README, lander, HANDOFF

**Files:**
- Create: `docs/audits/2026-11-04-browser-phase1.md`
- Modify: `README.md`, `docs/index.html` (feature cards only), `docs/HANDOFF.md`

- [ ] **Step 1: Full check** on `feat/browser-phase1`: `go test ./...`, `go vet ./...`, `go test -race ./internal/browser/ ./internal/browsermcp/ ./internal/tabs/ ./internal/ui/`, `GOOS=darwin go vet ./internal/browser/ ./internal/browsermcp/ ./internal/procs/ ./internal/claudesetup/ ./internal/codexsetup/ ./cmd/kaze/` (pure Go only), the opt-in suites once on Windows (`KAZE_BROWSER_REAL=1`, `KAZE_BROWSER_MCP_GOLDEN=1`). Push the branch and open the PR only when the coordinator says so; the macOS CI job is the macOS check for code.
- [ ] **Step 2: Binary size:** `go build -trimpath -ldflags "-s -w -H=windowsgui"` on `main` (a `git worktree add ../kaze-before main` copy) and on the branch, both in temp folders; record bytes and the difference (budget 300 KB, Ruling 27); if over, find the cause (`go tool nm -size -sort size`) before shipping.
- [ ] **Step 3: README:** a "Browser for agents" section (what it is; Chrome else Edge; headless, so it never takes focus; one per session with its own clean profile; starts on the agent's first browser call; closes with the session or after 10 quiet minutes, `browser_idle_minutes`; needs Node 20 or later; set up in Settings, Agents: Install, then Fix for Claude Code and Codex; the optional line; hiding it in a project; the row mark and status bar memory; no Browser tab yet). Roadmap: Phase 1 done, Phases 2 to 4 listed.
- [ ] **Step 4: Lander `docs/index.html`, feature cards only:** add or update one card in the feature grid (`#sessions`): "A browser for every agent" in two plain sentences. Do not touch `#changelog`, "What's new" or the release markup. Say in the audit that no screenshot shows the feature yet (there is nothing visible beyond the mark and the status item); do not fake one.
- [ ] **Step 5: HANDOFF:** a dated "Where things stand" entry (built on `feat/browser-phase1`, release 0.29 next, owner checks: Settings, Agents, Install and the two Fixes in the released app, a real Claude session using the browser, the mark and memory, idle close, the MacBook steps); "Next": Phase 2 (#30); "Mac milestone checklist": headless with no Dock icon or focus change, Node found from the GUI, the gate on macOS; "Known gaps": Rulings 14 (no Chromium download), 16 (a running Claude can undo the `~/.claude.json` edit), 22 (no mark on workspace members), the per-repo switch applying to new agent sessions only.
- [ ] **Step 6: Audit** `docs/audits/2026-11-04-browser-phase1.md`: per lane what changed and the tests per task with commits; the gate reports (G3) and the real-browser numbers (A8: start time, memory per browser from the status item); idle CPU (G3 step 3); binary size (step 2); the install size (M4); contrast minimums (U2, U3); and the rulings below with any the owner changed.
- [ ] **Step 7: Commit** `docs: Record agent browser Phase 1`

## Rulings made while planning

Each is cheap to overturn in its own task.

1. **No CDP library in Phase 1.** Kaze only forwards bytes and manages processes; it never speaks CDP. For Phase 2 (screencast, input, console), the plan of record is a small hand-written client for the dozen methods it needs over `golang.org/x/net/websocket` (already in `go.sum`), **not** the full `chromedp` and not `cdproto`: `cdproto` is a large generated tree (its domain packages pull each other in) and `chromedp` adds a runtime on top; both cut against the lean and size rules. This departs from the design's "chromedp/cdproto" in section 3; Phase 2's plan measures it. (A, F1)
2. **Playwright MCP is a Kaze-managed install**, not `npx`: `npm ci` from an embedded `package.json` and lockfile (every package pinned with an integrity hash) into `<UserCacheDir>/kaze/browser-mcp`, run as `node …/cli.js`. `npx` would add 0.3 to 1 s to every first call, could reach the network during a tool call, needs a `.cmd` launch on Windows, and leaves transitive versions floating. The install is an explicit Settings action with a confirm; nothing installs during a tool call. (M1, M4, S4)
3. **Pinned at `@playwright/mcp` 0.0.84**, the version measured on both OSes. A bump is its own PR that re-captures `tools.json` with the golden test. (M1)
4. **The tool list without Node:** captured once from the pinned server with Kaze's exact flags into `tools.json` and `server.json`, embedded, and checked by an opt-in golden test; the shim never starts Node for discovery. (M1, M2)
5. **`browser_run_code_unsafe` is hidden and refused** (it runs arbitrary code inside the tool's Node process; page JavaScript stays available through `browser_evaluate`). Every other tool is served as captured, except one sentence prefixed to `browser_navigate`'s description for steering. (M2, M3)
6. **Outside Kaze, or in a hidden folder,** the shim lists no tools, says why in `instructions`, and answers any call with that sentence. (M2, M5)
7. **One listener, one token per session:** a single Kaze-wide listener on `127.0.0.1:0`, each session's address `ws://127.0.0.1:<port>/devtools/browser/<128-bit token>`. The design says "random port per session"; a token gives each session its own unguessable address with one socket instead of one per tab, and a stale address never reaches another session's browser. Only WebSocket clients (`--cdp-endpoint`, `connectOverCDP`) are served; no `/json/version`. (A5)
8. **A byte pipe after one rewritten request:** Kaze reads the upgrade request, rewrites path and host to the browser's own, then copies bytes both ways. Activity = bytes from the agent's side. (A5, A6)
9. **One one-shot timer per running browser:** armed at start, re-armed at most once per expiry for the remaining quiet time, none without a browser; the idle minutes are read at each arm, so a Settings change applies from the next one. Default 10, `0` never. Phase 1 has no tab, so "not showing" is always true. (A6)
10. **Close paths:** the tab closes, its shell exits, Kaze quits, idle, or the browser crashes; the next connection starts a fresh one. A Kaze crash: Windows kills the job's processes when its handle closes; on macOS the next Kaze start sweeps orphans whose command line names their profile. (A3, A6, A7, U1, U5)
11. **Process control without CDP:** a Windows job object (`KILL_ON_JOB_CLOSE`; the browser starts suspended, joins the job, resumes) or a Unix process group (`SIGTERM`, then `SIGKILL` after 2 s). The temp profile is deleted anyway, so there is nothing to save with a graceful `Browser.close`. Flags are the spike's, plus `--headless=new`. (A2, A3)
12. **Profiles:** `<os.TempDir()>/kaze-browser/<kazePID>-<session>-<rand>`, seeded so downloads stay inside the profile, deleted on every close with bounded retries; a sweep at each Kaze start removes folders of dead Kaze processes. (A2, A7)
13. **Chrome first, then Edge,** from fixed install places on each OS; no config override in Phase 1. (A1)
14. **No Playwright Chromium download in Phase 1.** With neither browser installed, Settings, the status endpoint and the tool say "Kaze found no Chrome or Edge. Install Google Chrome, then try again." Every Windows machine has Edge; the download is about 150 MB more, Kaze would have to keep it patched (it does not auto-update), and it is one more thing to clean up. Revisit if a real machine without either shows up (design section 9 already lists it as later). (A1, S4)
15. **A status endpoint** on the same listener (`/kaze/status/<token>`) lets the shim give a plain error (no browser, stale address after a Kaze restart) before starting Node; it never starts a browser. (A6, M3)
16. **Claude Code registration edits `~/.claude.json` directly** (user scope `mcpServers.kaze-browser`) with the existing orderedjson round trip, diff, confirm, "changed meanwhile" re-check and `.bak`, rather than shelling out to `claude mcp add` (not on the GUI's PATH on macOS, and no safer). A running Claude Code that rewrites the file can still drop the entry; the check re-reads after writing and shows Missing again if so. Known gap. (S1, S4)
17. **Codex registration** appends a `[mcp_servers.kaze-browser]` block with `env_vars = ["KAZE_TAB_ID", "KAZE_BROWSER_CDP"]` (Codex starts MCP servers with a reduced environment) and `tool_timeout_sec = 120` (the first call starts Node and the browser). If the installed Codex lacks `env_vars`, S2 stops and the owner decides (fallback options: ask Codex upstream, or the shim finds its session another way in a later phase). (S2, G3)
18. **The steering line is optional and offered for both** `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` (the gate needs Codex to choose it too); Add only, the owner removes it by hand. (S3, S4)
19. **The per-repo switch is a Kaze setting,** `browser_hidden_folders`, never a file in someone else's repo; the shim matches its working folder (Claude Code and Codex start MCP servers in the project folder; G3 confirms), and the palette toggles the active session's repo root. It applies to agent sessions started afterwards. (T0, M5, S5)
20. **Node 20 or later,** found on PATH first, then in fixed places, because the macOS GUI does not inherit the shell's PATH. If the pinned package's `engines` says more, that wins. (M1, M4)
21. **No console windows:** `CREATE_NO_WINDOW` and `HideWindow` on every console child on Windows (`node`, `npm`); the browser is headless. (M4, M5, A3)
22. **The row mark** is one still state (no "agent connected" variant), on session rows only (not workspace members or the dock), not clickable until Phase 2 makes it open the Browser tab; `Muted` nudged to 3:1 on both row surfaces. (U2)
23. **The status item** shows only while a browser runs ("Browser 275 MB", "2 browsers · 550 MB"), fed by the existing resource sampler with each browser as its own root; no new sampling, same memory measure as the CPU/RAM group; dropped right after the battery. (U3, U4)
24. **Real-browser, real-Node and network tests are opt-in** and not in CI, although the runner images ship Chrome, Edge and Node: they take minutes, depend on image updates and are flaky in ways that are not Kaze's. The lead runs them on Windows; the owner runs the macOS ones. (A8, M1, M4)
25. **The gate uses the owner's accounts only with consent and never edits their config:** per-run registration (`--mcp-config`, `-c`) and a temp install. Settings' own Fix on the real files is covered by tests and checked by the owner in the released app. (G2, G3)
26. **Bench mode has no hub:** no address in the env, no listener, frames unchanged. (U5)
27. **Size budget 300 KB** on the stripped Windows build (the embedded tool list and lockfile are most of it). (F1)
28. **Idle choices in Settings:** 5, 10, 30 minutes, Never. Other values stay possible in `config.toml`. (S4)
29. **The shim's output folder** (`--output-dir`, where Playwright MCP saves files) is a temp folder per shim process, removed when the shim exits, not with the browser, so files outlive an idle close. (M5)
30. **Concurrent first connections share one launch** (15 s limit); a failed launch answers `503` with the sentence and leaves nothing behind; the next connection tries again. (A5)
31. **`KAZE_BROWSER_CDP` is dropped from the inherited environment** like `KAZE_TAB_ID`, so a Kaze started inside a Kaze tab gives clean addresses. (U1)
