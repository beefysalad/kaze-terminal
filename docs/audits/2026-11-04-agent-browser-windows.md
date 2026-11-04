# Agent browser: Windows feasibility run (2026-11-04)

Follow-up to `2026-11-01-agent-browser-feasibility.md` (macOS). Issue #30. Harness: `spike/agent-browser`
on branch `spike/agent-browser-windows`. Machine: Windows 11 x64, Node 24.19, Edge and Chrome installed.
The owner was at the machine and occasionally typed; one burst of foreground changes to Arc and the
taskbar at 13:37:21 to 13:37:30 was the owner (confirmed) and is excluded below.

## Results

| Setup | Popups and alerts took the foreground | Tool calls | Notes |
|---|---|---|---|
| Edge, plain spawn (`node-default`) | yes: new tab, popup window, alert (3) | 27 of 27 ok | |
| Edge, `CreateProcess` with `SW_SHOWNOACTIVATE` | yes (4, one after the owner's Arc click) | 27 of 27 ok | the show flag only affects the first window |
| Chrome, plain spawn | yes (3) | 27 of 27 ok | |
| Chrome and Edge started by WMI (`Win32_Process.Create`), outside Kaze's process tree, default and no-activate | yes (3 each) | not rerun | rules out "inherited foreground right" |
| **Chrome and Edge, `--headless=new`, all phases** | **never (0), and no windows at all** | **27 of 27 ok** | popups, alerts, downloads and isolation all pass |

- The first runs failed the tool phase because of a harness bug (a local `tools` list shadowed the Windows
  helper); fixed, then the tool phase was rerun on all three windowed setups.
- Normal agent tool use (navigate, snapshot, type, click, screenshot, console, network) never took the
  foreground in any setup. Only page-initiated new windows/tabs and JavaScript dialogs did.
- Chromium brings its own new windows and dialogs forward whoever launched it, so no launch option fixes
  it while the browser has real windows.

## Other measurements (Windows)

- First tool call, browser start included: about 1.5 to 2.2 s. Separate-instance startup about 0.85 s.
- Idle MCP workers after discovery: 0 ms CPU over the sampled second; about 100 MB private (Playwright MCP)
  and 132 MB (Chrome DevTools MCP).
- Browser memory with one fixture page, private bytes: Chrome about 275 MB (10 to 11 processes), Edge
  about 400 MB (16 processes). Three instances scale linearly (Chrome about 820 MB, Edge about 1.2 GB).
  Headless costs the same as windowed.
- Isolation with separate instances: cookies separated, foreign targets absent, attaching to a foreign
  target rejected (as on macOS).
- Cleanup: no leftover browser process naming the run's profile root, and the profile root was removed,
  in every run.

## Decision (owner, 2026-11-04)

The agent browser runs **headless** (no OS window, so it cannot take focus) and Kaze shows the page itself
as a **Browser tab in the docked panel** under the terminal (the 0.27 panel), where the owner can watch,
click and scroll, use "Send to agent", and "Open in my browser" for anything heavier. This replaces the
2026-10-30 decision "background window; the session's browser mark brings it up". It draws only while
the tab is shown.

Still open for the design: real Claude Code and Codex turns through the tools, setup integration, the
idle cost of registered MCP workers (100 to 130 MB per session before any browser starts), per-session
profiles (no shared sign-ins), and Chrome versus Edge as the default (Chrome is about 30% lighter).
