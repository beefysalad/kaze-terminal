# kaze: agent browser, headless in a panel tab, in four phases

Date: 2026-11-04. Issue: #30. Supersedes the parts of `2026-10-30-kaze-agent-browser-design.md` named in
section 2; everything else there still holds (intent, goals 1, 3 to 6, anti-goals, constraints).

Evidence: `docs/audits/2026-11-01-agent-browser-feasibility.md` (macOS) and
`docs/audits/2026-11-04-agent-browser-windows.md` (Windows, PR #144). Approved mockup:
`docs/design/agent-browser-tab-mockup.html` (owner, 2026-11-04, "looks good", premium pass).

## 1. In one paragraph

Each kaze session can have its own **headless** Chromium (the owner's installed Chrome, else Edge), with
its own clean profile. Claude Code and Codex drive it through a `kaze-browser` MCP tool that kaze registers
for them. Because it has no OS window, it can never take focus. Kaze shows the page as a **Browser tab in
the docked panel** under the terminal (the 0.27 panel, next to server and agent tabs): the owner watches it
live, and later clicks in it, reads its console and network, and sends what they see to the agent. It
ships in four phases, each its own release.

## 2. What changed from the 2026-10-30 design, and why

| Topic | Was | Now | Why |
|---|---|---|---|
| Visibility | A background browser window; the row mark brings it to the front | **Headless, no window.** Kaze draws the page in a panel tab | On Windows, page popups, new tabs and alerts brought the browser to the front whatever launched it (plain spawn, no-activate show flag, WMI outside kaze's tree). Headless never did and passed every phase. |
| Processes | One Chromium process, one window per session | **One headless browser per session** | A shared browser let sessions see each other's pages; separate instances isolated pages, targets and cookies (macOS and Windows). |
| Profile | One kaze profile shared by all sessions (shared sign-ins) | **A clean profile per session** | Follows from the owner's rule "isolation wins". No shared sign-ins; a per-repo saved login is a later option (section 9). |
| Browser | Edge on Windows, Chrome on macOS | **Chrome if installed, else Edge** on both | Chrome measured about 30% lighter (275 MB vs 400 MB private per browser on Windows). |
| Embedding | Not embedded | Still not embedded: kaze draws Chromium's **screencast** frames | No CEF (150 to 200 MB, C++ build, crash coupling) and no system web views (different engines, no agent control on macOS, native views inside Gio). |

## 3. What is reused, what kaze builds

**Reused (open source):** the installed Chromium (Chrome or Edge, `--headless=new`); the Chrome DevTools
Protocol (CDP); the agent tool server **Playwright MCP** (Apache-2.0; Chrome DevTools MCP stays the
alternative, both passed 27 of 27 calls); on the Go side **chromedp/cdproto** (MIT) for CDP.

**Kaze builds:** the per-session browser lifecycle; the `kaze browser-mcp` launcher; setup registration;
the Browser tab (screencast drawing, address bar, controls); input forwarding; the inspector; Send to agent
and Pick element.

## 4. Decisions (owner)

1. Headless browser shown as a panel tab (2026-11-04).
2. One browser and one clean profile per session; isolation over shared sign-ins (2026-10-30, kept).
3. Lazy: nothing runs until the agent's first browser call or the owner opens a page in the tab (kept).
4. Closes with its session, when the owner closes the tab's browser, or after idle (kept).
5. Agent tools wrap an existing Node MCP tool; Node is an accepted dependency (kept).
6. Released in four phases, each tried by the owner before the next (2026-11-04).

## 5. Calls made in this document (for the owner to check)

- **Idle close after 10 minutes** with no agent command and the tab not showing (`browser_idle_minutes`,
  `0` = never). One one-shot timer per running browser, armed when it goes quiet; no ticker.
- **Send to agent fills the prompt but does not press Enter.**
- **Chrome first, then Edge;** on a Mac with neither, the tool and the tab say so plainly. Downloading
  Playwright's Chromium on request is left for Phase 1 to decide.
- **The idle MCP cost:** registered MCP servers start Node at Claude's session start (100 to 130 MB each,
  measured). Phase 1 makes `kaze browser-mcp` a small Go shim that answers MCP discovery itself and only
  starts the Node tool server on the first tool call. If that proves unreliable, the fallback is the
  stock server and the measured cost, reported to the owner before shipping.
- **Repos with their own browser tool win there:** the tool's description and an optional line kaze can
  add to the owner's `~/.claude/CLAUDE.md` ("in Kaze, use kaze-browser for web pages unless the project or
  a skill names its own browser tool"). Plus a per-repo switch to hide `kaze-browser`. Example: Samurai's
  `.mcp.json` Playwright MCP (saved login, visible browser for `/run-test-plan`) keeps being used there.

## 6. How it works

### 6.1 The browser per session

- Started by kaze on demand with `--headless=new`, a fresh profile under kaze's temp area for that session,
  `--remote-debugging-port=0` bound to 127.0.0.1, and the flags the spike used (no sync, no component
  updates, no first-run). Kaze reads `DevToolsActivePort`, keeps the CDP connection, and is the only one
  that knows the browser's address.
- Each session gets **its own kaze-owned address** (`KAZE_BROWSER_CDP`, 127.0.0.1, random port) that
  forwards to that session's browser and starts it on the first connection. Agents never get another
  session's address.
- Closing: session closes, owner closes, idle (5), crash. The profile folder is deleted on close (the
  spike's cleanup check: no leftover process, profile removed, on both OSes).
- The status bar shows the total memory of running browsers; the session row shows a small browser mark
  while that session has one.

### 6.2 `kaze browser-mcp` and setup

- A `kaze` subcommand registered once at user level for Claude Code (and in Codex's config) by the
  Settings setup check, with the usual show-the-change, confirm and `.bak` backup.
- It reads `KAZE_TAB_ID` / `KAZE_BROWSER_CDP`. Outside kaze it exposes no tools (or one that says it only
  works inside kaze) and costs nothing.
- It serves MCP discovery itself and starts Playwright MCP (`--cdp-endpoint $KAZE_BROWSER_CDP`) on the
  first tool call, then relays. Node missing is reported plainly by the setup check.

### 6.3 The Browser tab (from the mockup)

- A tab in the 0.27 docked panel, opened from the row's browser mark, the palette, or automatically
  listed (not shown) when the agent starts a browser. It follows the panel rules: one tab shows, × closes
  the tab (the browser keeps running until idle or "Close browser"), Hide folds the panel.
- Toolbar: back, forward, reload; the address (dim host, bright path, editable); a "who's driving" pill
  ("Agent browsing" while agent commands arrive, "You" after the owner's input); Pick; Send to agent; Open
  in my browser.
- The page: Chromium's screencast (`Page.startScreencast`) frames drawn into the tab, **only while the tab
  is showing**; hidden or folded, kaze stops the screencast and receives nothing. A thin meta line shows
  "Headless · Chrome" and the viewport size. The viewport follows the tab's size (`Emulation`), debounced.
- Where the agent last clicked or typed: a violet outline with "Agent clicked", from the agent's CDP input
  commands seen by kaze's address, cleared on the next frame change after a short time (no ticker).
- Inspector (right, about 300 dp): Console, Network, Steps, with counts; errors tinted; failed requests
  as method, path, status chip, time. Kaze records the last N console messages and failed requests per
  browser only while it runs (N = 50, my call), capped in bytes.
- Empty states: "No browser yet" (address field to open a page) and "Closed after 10 quiet minutes"
  (Reopen now, Open in my browser).

### 6.4 Send to agent and Pick element

- Send to agent: a menu (screenshot, page address, console errors, failed requests, page text) with sizes
  and counts. Kaze writes the chosen items to a per-session temp folder (PNG plus a short Markdown
  summary), then types **one reference line** into the session's prompt, without Enter, for example
  `@/…/kaze-browser/login-form/capture-1/summary.md @/…/screenshot.png`. Note: Claude Code's prompt is a
  terminal, so these appear as text paths there, not as the chips drawn in the mockup.
- Pick element: CDP's inspect mode highlights under the pointer; a click records selector, text, outer
  HTML (capped, 4 KB) and a cropped screenshot, and writes them the same way.
- Kaze writes into its own session's PTY, which is not global input.

## 7. Phases

Each phase: its own GitHub issue under #30, plan, lanes, review, release. The owner tries it before the next
phase starts. Later phases may adjust after what earlier ones teach.

**Phase 1 (0.29): the agents get their browser.** 6.1 and 6.2: per-session headless browser, kaze-owned
address, lazy start, idle and session close, cleanup, Chrome-then-Edge; `kaze browser-mcp` shim with lazy
Node; setup check for Claude Code and Codex; the CLAUDE.md line and per-repo switch; the row mark and status
bar memory. **Gate:** real Claude Code and Codex turns that pick `kaze-browser` and complete navigate, type,
click, screenshot, console and network on a local app, on Windows (macOS on the owner's MacBook); idle
CPU zero; nothing running after the session closes.

**Phase 2 (0.30): the Browser tab, watch mode.** 6.3 without owner input and without the inspector: the
tab, toolbar (back, forward, reload, address, Open in my browser), the driving pill, the agent-click
outline, screencast only while showing, viewport follows the tab, empty states.

**Phase 3 (0.31): use it yourself, and the inspector.** Mouse, wheel and keyboard forwarded into the page
(`Input.dispatch*`) when the owner clicks into the tab (the tab takes the keyboard like a member tab does);
the inspector (Console, Network, Steps).

**Phase 4 (0.32): Send to agent and Pick element.** 6.4.

## 8. Constraints

- Idle CPU zero: no ticker; screencast only while visible; one one-shot idle timer per running browser.
- Never take focus: headless only; no OS browser windows. "Open in my browser" hands a URL to the default
  browser, which is the owner's explicit action.
- Addresses on 127.0.0.1, random ports, given only to their own session (accepted local-machine trust).
- No secrets committed; profiles are temporary and deleted on close.
- Strict TDD with bounded RED tests; never inject global input; GUI checks through code paths or headless
  captures.

## 9. Later, not in these phases

- A saved login per repo (one persistent profile per project), for apps like the Samurai admin.
- Downloading Playwright's Chromium on a machine with neither Chrome nor Edge, if Phase 1 does not.
- Redirecting a project's own `playwright test` suite to kaze's browser.
