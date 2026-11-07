# kaze handoff

Last updated: 2026-11-07, release 0.35 (the approved wind K as the app icon, website and README logo, PR #174; `docs/site-assets/kaze-about.png` still shows the old About page and needs retaking). Before that, release 0.34 (the branch launcher, its `new_branch_session` shortcut, the Kaze-made worktree cleanup and `internal/branchsession` removed, PR #171; old `sidebar_launcher*` keys load quietly, an old `[keys] new_branch_session` warns; next ideas filed as #167 smooth scrolling and #172 system notifications). Before that, release 0.33 (the Kaze mark and name in the title bar, after the traffic lights on macOS: **check the Mac bar by hand**, layout is unit-tested and Windows captures were inspected; the Windows exe details carry the real version, name and publisher, PR #168; `TestWinresMatchesVersion` keeps them in step, so the old "rerun go-winres" to-dos below are done). Before that, release 0.32 (five more themes and one UI font, PR #165). Before that, release 0.31 (the palette redesign, PR #163, and editable shortcuts, PR #162). Before that, release 0.30 (find in the terminal and docked tabs, #155 / PR #156). Before that, release 0.29 (battery on macOS, #139 / PR #158; the About page redesign, PR #160; the lander's `kaze-about.png` still shows the old page and needs retaking). Before that, release 0.28.2 (.md links open the file the agent touched, #143 / PR #153; uninstall page #112 designed and planned on `feat/uninstall`). Read this first when you pick up kaze on another machine.

## Where things stand

**Release 0.32 ships five more themes and one font (PR #165).** Matcha, Kōyō (`koyo`),
Hai, Fuji (dark) and Kiri (light) join the five, ten in all; the owner asked for fewer light ones, so only
Washi and Kiri are light. The Theme picker wraps at five per row and drops to fewer columns (none under
104 dp) in a narrow window; its hint wraps. All UI text is now JetBrains Mono: Manrope is gone from the
app (the website still uses it), and `CLAUDE.md` says not to add another font. Mono is wider, so: About's
buttons read "Report issue" and "GitHub"; the Settings menu subtitle is "Saved to config.toml"; the palette's
Skills view cuts more descriptions with "…" (106 allocations per still frame, cap raised from 105 to 112).
Every new theme passes the existing contrast tests. The lander screenshots all need retaking (new font).
Owner checks by hand: each new theme on the terminal, sidebar, palette and an alert card; Settings, About,
the palette and the Agents board for text that now looks cramped or cut.

**Release 0.31 ships the palette redesign (PR #163) with editable shortcuts.** The palette now
opens on Waiting on you, Sessions and up to 3 recent skills with a "Browse all N skills" row. Typing also
finds Kaze actions and up to 3 skills. `/`, Tab or the switch opens the Skills view, grouped Yours, This
project, then one group per plugin (4 shown, "N more in ..." for the rest). Servers are no longer listed.
Plan: `docs/superpowers/plans/2026-11-05-kaze-palette-redesign.md`; spec:
`docs/superpowers/specs/2026-11-05-kaze-palette-redesign-design.md`; mockup: `docs/design/palette-mockup.html`. The allocation
cost of a palette frame (still frame at 1000x700 on Windows, allocations per frame): empty field 281 before, 85 after; typed "re" 129 before, 111 after; "/" (Skills scope) 97 after (230 before the final-review fixes built the skill rows once per scan), bounded at 105 (the old palette had no "/" measure). The lander's
`kaze-palette.png` needs retaking (see SCREENSHOTS.md). No "What's new", changelog or version yet.
Rulings the owner may want to revisit (numbers are in the plan's Rulings): 5 (Browse row header), 6 ("Other"
folds at 4), 7 (pills for done and shell; no " · /command" text), 9 (which actions get icons), 10 (clicking
the chip leaves Skills), 11 (switch hidden under 440 dp). Owner checks by hand: open the palette, `/`, Tab,
a plugin group's "more" row, Shift+Enter on a skill, all five themes, and a narrow window.

**Release 0.31 also ships editable shortcuts (PR #162).** Settings >
Shortcuts now lets you change every shortcut: click, press the keys, saved at once. A clash asks first,
keys without Ctrl, Alt or Cmd are refused (F-keys are fine on their own), and a plain Ctrl+letter warns. Reset works per row or for all.
Plan: `docs/superpowers/plans/2026-11-05-kaze-editable-shortcuts.md`; spec:
`docs/superpowers/specs/2026-11-05-kaze-editable-shortcuts-design.md`. Next: release 0.31 (version,
changelog and What's new are not done yet). The page's colours, strips included, pass the contrast
test in all five themes: the lowest text contrast is 4.51:1 and the lowest mark 3.02:1. The page's
captures are in `docs/audits/shortcuts/`. Owner checks by hand: change Find to Ctrl+Alt+F and use it without a restart;
Replace a clash; the Ctrl+R warning; Reset all; the strips readable on Washi (the light theme); and on the
Mac, Cmd keys recorded as `cmd`.

**Release 0.30 ships find (#155, PR #156, merged 2026-11-05).** Ctrl+Shift+F (Cmd+F on macOS) opens a
find bar on the session's terminal or on a workspace member's tab in the panel. Every match is tinted,
the current one stronger, with marks on the right edge. Plan: `docs/superpowers/plans/2026-11-04-kaze-find.md`;
mockup `docs/design/find-mockup.html`; audit `docs/audits/2026-11-04-kaze-find.md`. README, the lander's
"Make it yours" card and the SCREENSHOTS note are updated; no "What's new", changelog or version yet (the
release PR does that). Full test run and `go vet` pass on Windows. Left: the whole-branch review and its
fixes, then mark the PR ready and release per CLAUDE.md. Delete the `kaze-find*` worktrees when done.
Owner checks by hand: Ctrl+Shift+F in Claude's terminal and in a member's log; Esc puts the view back;
Aa; the marks on the right edge; match text readable on Washi (the light theme); and idle CPU with the bar
open and closed (not measured yet).

**The repo moved to `beefysalad/kaze-terminal` (2026-11-04).** GitHub redirects the old repo, git and API
addresses, but not the website: it is now https://beefysalad.github.io/kaze-terminal/. The Go module, the
update checker (`internal/update.LatestURL`), the lander, README and release notes name the new owner.
Released in 0.28.1. Installed copies older than 0.28.1 still update through the redirect; never create a new
`patrick-gogo/kaze-terminal`, or that redirect breaks. Historical docs keep their old links.

**Release 0.28 ships PR #140: battery on Windows, readable agent activity and agent names (#111,
#134, #135).** Next: the macOS battery (#139, on the owner's MacBook). Built in three worktree lanes
(N, B, R) from `docs/superpowers/plans/2026-11-04-kaze-battery-activity-names.md`:

- **#111 battery (Windows only):** an icon that fills to the level and the percent at the far right of
  the status bar, a green bolt while charging below 100% (a full battery shows no bolt even if Windows
  says charging), amber at 20% or less, red at 10% or less, dropped first in a narrow bar, nothing on a
  desktop after the first check. Windows change notifications, no polling; if registering fails, a read
  at most once a minute on frames that happen anyway. Nothing while minimised, off in bench mode. One
  read about 560 ns and 0 allocations; an unchanged status frame still 3 allocations; idle 2 minutes
  visible = 2 wakes and 1 read, minimised = 0; the app grows about 25 KB. **macOS shows nothing until
  #139** (IOKit source, built on the MacBook).
- **#134 readable activity** in the dock panel and the board preview: the agent's messages lead (coral
  bar, time), each step is one sentence (Bash uses Claude's description in the past tense with passed /
  failed · N tests / done; "Edited <path> +N −M"; "Read 3 files"; "Searched the code N times";
  "Committed "<subject>"" with the hash), small runs fold, ToolSearch, Monitor, TaskStop and the like
  are hidden, a click opens a step's diff or command and output, and a window-wide **Readable | Every
  step** switch (not saved) keeps the old list. Codex helpers fall back to hook summaries. Caps: 200
  steps; 24 diff lines, 8 output lines and 4 KiB per step; 256 KiB in all. Frame (1000×600, 21 steps):
  Readable about 200-207 µs and 73 allocations, Every step 182-192 µs and 78, `main` before about
  199 µs and 91. Lowest readable contrast 4.51 (Sumi, Washi); diff colours lowest Washi add 5.95 /
  del 5.48.
- **#135 names:** agents of one type started together are matched to their own prompt (the first user
  record of their transcript, at most 64 KiB read, only when two could be mixed up, at most 3 tries). A
  wrong early guess swaps; a settled name never moves.

Audit with tests, numbers, captures and rulings: `docs/audits/2026-11-04-battery-activity-names.md`.
README and lander feature text are updated (no "What's new", changelog or version yet: the release PR
does that). `kaze-agents.png` and `kaze-board.png` show the old step list and need retaking; a battery
shot needs a real Windows laptop (`docs/site-assets/SCREENSHOTS.md`). Owner checks: the battery on the
Windows laptop (unplug, plug in, low states), the readable panel and board on a real Claude session
(click a Bash step, an Edit, a fold, switch to Every step), and three agents of one type started in one
message keeping their own names.

**Lander reworked (2026-11-04, branch `site/lander-rework`), inspired by herdr.dev:** a still, tilted wall of
the existing screenshots (click any to open a lightbox), an HTML-drawn Kaze window with four moments (needs you,
servers, a subagent, done) that shows the 0.27 panel and alert card until real captures exist, a README-style
feature index, an FAQ and a closing install section. Styles and script moved to `site-assets/lander.css` and
`lander.js`; `site.css` stays for `changelog.html`. Release markup in `#changelog` is unchanged. **When dropping
the oldest lander release, remove the whole `<li class="release">` up to `</div>
      </li>`**: a lazy
match to the first `</li>` stops inside the bullet list, which left orphaned 0.23 and 0.24 bullets on the lander
in the 0.26 and 0.27 releases (fixed here).

**Lander restyled (2026-11-07, branch `feat/lander-redesign`):** it had become a near copy of herdr.dev (even
"Every window here is Herdr"). The hero layout stays; its second line and the buttons are Kaze lilac (#c2b4eb,
from the mark). Below it the page is quiet and flat, reviewed with the impeccable skill: no gradients, glows,
eyebrow labels, section numbers or card grids. Ruled lists carry the moments, the hooks relay, the feature index
(drawn SVG icons), the FAQ and the changelog. Release markup (`ol.releases > li.release`) is unchanged, so the
release steps below still apply. Links drop `.html` (`changelog`, `./#install`); GitHub Pages serves them, and
`docs/vercel.json` sets `cleanUrls` for a move to Vercel.

**Release 0.27 ships PR #136: dock tabs, alert cards and path links (2026-11-04).** Next: #134
(readable agent activity, mockup approved) and #135 (subagents started together swap names). Three issues in one
bundle, built in three worktree lanes (L, A, D) from `docs/superpowers/plans/2026-11-04-kaze-dock-alerts-links.md`:

- **#133 dock tabs:** workspace members open as tabs in the panel docked under the terminal, the
  same panel as subagents. One panel per session, one tab shown, shared height, up to 6 tabs (a
  seventh replaces the oldest not showing), × closes a tab (the member keeps running), Hide folds
  the panel, Expand makes a member the main view. A click on a member tab gives it the keyboard,
  with a 2 dp accent line; a click on the terminal takes it back. Sidebar Workspaces rows, status bar
  server dots and the palette all open members as tabs. **The scratch shell and the floating panes
  are removed** (`scratch.go`, `panes*.go`, `win.fl`, `Tab.Parent`, the title bar button); an old
  `scratch_shell` line under `[keys]` is ignored with a warning.
- **#9 alert cards:** cards at the bottom right above the status bar (above the dock panel when it
  shows), newest nearest the corner, at most 3, one per session. Mark with a `!` or `✓` badge, branch
  and slash command, the sidebar row name as title (fixes "zsh needs you"), two lines (needs:
  Claude's message; done: the start of Claude's last reply), a status line ("Needs your permission",
  "Waiting for your answer", "Done · ran 6m 40s"), "Open session". Ctrl+Shift+J / Cmd+J opens the
  newest card's session. × closes a card and keeps the dot; done cards hide after `done_toast`.
- **#128 path links:** `.md` links stay whole across single spaces ("Patrick Obsidian") and when the
  folder was printed earlier on the line; quoted paths are taken whole. A click tries the longest
  candidate that exists; a miss says "Can't find X in <folder>". OSC 8 is still out of scope.
- **Frame timing** (headless 1000×600, Windows): no panel about 114-128 µs and 56 allocations, agent
  tab 188-198 µs and 86, member tab 117-142 µs and 63; the old floating pane was about 131 µs and 70.

Audit with tests, numbers, captures and rulings: `docs/audits/2026-11-04-dock-alerts-links.md`.
README and lander are updated (no "What's new" or changelog yet: the release PR does that).
`kaze-sessions.png` still shows a floating pane and must be retaken; `kaze-scratch.png` was removed
(`docs/site-assets/SCREENSHOTS.md`). Owner checks: a member tab with a real server, typing into it
and taking the keyboard back, the cards with real Claude alerts, and Ctrl+click on a spaced path, on
Windows and the Mac.

**Release 0.26 ships PR #131: the agents board redesign (filters with counts, search, session token,
pinned Needs you card, per-session cards with a state meter, finished-agent folds, a preview pane with
the live timeline or report) and the sidebar follow-ups (project rail removed, a click on an agent line
toggles its panel, "+N more" opens the board for that session).** Approved from the mockup on
2026-11-03, built in two parallel worktree lanes, then a fresh whole-branch review (8 minor findings,
all fixed test-first). Audit: `docs/audits/2026-11-03-agents-board.md`. Next: #128 (`.md` links split
on the line: a folder printed earlier, or a space in the path). Product screenshots need retaking.

**Agents board redesign and sidebar follow-ups (2026-11-03) are built on `feat/agents-board`, not yet
pushed or released. Release 0.26 is next.** The owner approved the clickable mockup
(`docs/design/agents-board-mockup.html`) and the sidebar highlight choice (option 3, "Session bar
only") on 2026-11-03. The sidebar drops 0.25's project rail; a click on an agent line opens its panel
again (a second click closes it), and "+N more" opens the board filtered to that session (the `›`,
safe click, the expand to 8 and "Show fewer" are gone). The board is now a list of session cards
(All / Needs you / Running / Done, search with `/` and Esc, a "<session> ×" token, an amber Needs you
card, a state meter per card, folded finished rows) with a live preview of the selected agent that
reuses the docked panel's timeline and report. Built in two parallel worktree lanes, then merged and
audited. 50 agents: about the same time per frame as 0.25, 244 allocations per frame instead of 636,
and still frames read no transcript. Tests, frame numbers, contrast, captures and rulings:
`docs/audits/2026-11-03-agents-board.md`. Lander and README screenshots need retaking per
`docs/site-assets/SCREENSHOTS.md`; real-session checks of the preview remain owner checks.

**Release 0.25 ships PR #129: #125 (slim capped subagent tray with safe click, fold and tooltip;
project rail; sidebar contrast), #124 (leading status symbols stripped, no missing-glyph boxes in
chrome text) and #127 (colour emoji: COLR/CPAL on Windows, sbix on macOS, `internal/colorfont`).**
The owner approved the mockup (PR #126, included in #129), the design and the plan on 2026-11-03,
then asked to merge and release. Built in two parallel worktree lanes, then a fresh whole-branch
review (2 important findings fixed test-first). Rulings, measurements, captures and deferred minors:
`docs/audits/2026-11-03-sidebar-tray-emoji.md`. Next: #128 (bare `.md` names next to a printed folder
can't be found). Product screenshots need retaking; native macOS emoji and real-session tray checks
remain owner checks.

**Sidebar agent tray, project rail, title symbols and colour emoji (#125, #124, #127) are built on
`feat/sidebar-tray-emoji`, not yet pushed or released.** The owner approved the mockup (PR #126)
and chose real colour emoji on 2026-11-03. Subagents sit in a slim tray under their session (one
22 dp line each, at most 3 then a "+N more" chip, up to 8 expanded, a fold control on the row); a
click on a line shows the session and only `›` opens the panel. The active project gets a 2 dp
accent rail instead of the tinted box. Title bars and the sidebar strip any leading status symbol
(`◐` no longer draws as a box). Emoji are drawn in colour from Segoe UI Emoji (COLR/CPAL) and
Apple Color Emoji (sbix), cached once per glyph. The chip keeps "+N more" and its caret at the
default 260 dp width. Tests, benchmarks, captures, rulings and owner checks:
`docs/audits/2026-11-03-sidebar-tray-emoji.md`. Open: the macOS sbix check on real hardware, U+FE0F
as a follow-up issue, and product screenshots need retaking (`docs/site-assets/SCREENSHOTS.md`).
One release covers all three issues.

**Release 0.24 bundles PRs #121 (docked agent panel, #106 and #103), #119 (Windows Ctrl+C/Ctrl+V
copy and paste with images, #117) and #120 (finished workspace members read Ready, #118).** The
owner asked on 2026-11-03 for a review subagent per PR, a fixer per PR for real findings, then merge
and release. All three reviews found issues; each was fixed test-first and passed macOS and Windows
CI, #121 together with the other two. Deferred minors are listed in PR #121. Product screenshots
need retaking (`docs/site-assets/SCREENSHOTS.md`); native installed-app checks remain owner checks.

**Docked agent panel (#106, includes #103) is built on `feat/agent-dock`.** The owner approved
the mockups (2026-11-02), the three open details and five builder calls, then the plan
(2026-11-03). Subagents open in one panel under the terminal (Activity timeline with markdown
notes, Report tab with rendered markdown, Copy report, Open in reader); floating agent cards are
removed. Height drags freely and is saved as `agent_panel_height`; each session remembers its
panel; Esc/↑/↓ reach the panel only after a click into it. Links in the panel are web and `.md`
like the terminal. Tests, vet, headless captures and timings: `docs/audits/2026-11-03-agent-dock.md`.
**Release:** the owner asked for ONE release that bundles this with #117 (PR #119), and #118
(PR #120) from another session, which were told not to release. Native checks remain open.

**Release 0.23 packages #107, #113 and #114.** The owner explicitly requested
opening a PR if needed, merging this bundle and publishing its release on
2026-11-03. This overrides the earlier release hold for this work. Feature PR
#115 passed macOS and Windows CI and is merged, with the approved behavior,
tests, audits and development captures.
Version, Mac metadata, download notes, README, full changelog and lander are
updated together on `release/0.23`. Product screenshots need native retakes as
recorded in `docs/site-assets/SCREENSHOTS.md`; none were fabricated. Installed-app
and native Windows interactions remain owner checks. Unrelated original changes
in the owner's checkout are preserved.

**Owner authorization (2026-11-03):** open a PR for #107, #113 and #114 if one
is not already open, merge it after checks and publish the combined release using
the shipping workflow. This overrides this work's earlier no-release/no-push
instructions and the general release hold for this bundle. Feature PR and release
0.23 are being prepared; unrelated original checkout changes are preserved.

**Running mark and active-project header (#107) are implemented locally.**
The owner approved the project-header design on 2026-11-03 after reviewing its
localhost mockup. The selected main session's common repository gets a soft
accent-tinted header, accent name/arrow and retained count or amber waiting count.
Context stays through Settings, documents, folded/filtered views and focused
floating panes (docked member tabs since 0.27); empty/plain-no-repo sessions highlight no group. The draw cache
includes the repo key; selection changes do not rebuild the model and unchanged
sidebar drawing still allocates zero times. Name/count/waiting text contrast is
at least 4.5:1 across all five themes, with local contrast adjustments on the tint.
The approved still closed ring and bright core replace the open arc in session
rows, subagents, title breadcrumb and status agents pill, without animation or
extra idle frames. Pixel tests cover bounds/core/closed top at 1x/2x in every theme.
Tests failed before changes and pass afterward. Full tests, vet, focused UI
races, macOS build and Windows amd64 cross-build pass. Independent UI review
returned ship with no material fixes. Native installed-app checks remain open.
See `docs/audits/2026-11-03-running-projects.md` for captures and final checks.
#107, #113 and #114 remain local and unreleased for the owner's later bundle.
No commit, push, version bump or release.

**Empty window (issue #113) is implemented locally for the next bundled release.**
Closing the last session leaves the existing chrome and a "No open sessions" view
with a New session button and configured shortcut. The final plain shell exiting
uses the same state; exited workspace members retain their stopped status/output.
Settings, Workspaces, palette and branch launcher stay reachable. Shared frame
housekeeping and status notices run without a terminal; cursor blink stops. New
views install their input filters before focus moves. Worktree cleanup choices,
refusals and explicit window close keep working. Empty session snapshots persist.
Tests failed before the changes and pass afterward; full tests, vet, focused UI
races, macOS build and Windows amd64 cross-build pass. Actual headless dark/light
wide/narrow captures were inspected; review's secondary-text contrast finding was
fixed locally and verified across all five themes. See
`docs/audits/2026-11-02-empty-window.md`. Installed macOS and Windows interactions
remain owner checks. The owner approved the design/plan and explicitly requested
no release, to bundle this with #114 and later issues. No commit or push.

**Shift+Enter (issue #114) is implemented locally for the next bundled release.**
Return and keypad Enter with Shift now send the existing Alt/Option+Enter sequence
(Escape + Return), which Claude Code uses for a newline. This is a compatibility
alias, not kitty keyboard protocol support; programs use their own bindings for
that sequence. Plain Enter and Ctrl/Cmd combinations keep their existing behavior,
and configured Kaze actions take precedence. Regression tests cover both platform
encoders, shortcut overrides, and delivery to the focused main terminal or floating
pane (a docked member tab since 0.27). The new tests failed before the fix and pass afterward. Full `go test ./...`,
`go vet ./...`, native macOS build and Windows amd64 cross-build pass. README
shortcuts are updated. Native installed-app checks remain open. The owner
explicitly requested no release and will bundle this with later issues.
No commit, push, version bump or release is part of this work.

**Release 0.22 ships the sidebar redesign (#101 / PR #104).** The owner explicitly
requested merging #104 and publishing this release on 2026-11-02, overriding the
implementation-stage no-release instruction for this work. The feature PR passed
macOS and Windows CI and is merged. Version, Mac metadata, release notes, full
changelog and lander are updated together on `release/0.22`. The sidebar's native
product captures still need retaking after owner review; headless development
renders are not used as product screenshots. Installed update/relaunch and native
launcher/cleanup interactions with real agents remain owner checks.

**Issue #101 merged in PR #104** from `feat/sidebar-rework`. Two-line rows use
the approved title priority,
agent logos and first-line statuses; group headers have no dots. Retained models,
session order and drawing operations avoid sidebar allocations or row shaping on
unchanged frames, including Workspaces and a retained filter. Editors stay live
over the cached chrome. The collapsible branch launcher supports the configurable
`new_branch_session` shortcut, previews, explicit remote tracking and shell-ready
agent starts. `internal/branchsession` remembers only created worktrees, bounds
Git/state I/O, and removes only registered linked worktrees with plain Git.
Cleanup checks are event-driven and minute-limited per repo. A close can schedule
one trailing check to satisfy that limit; Keep cancels it. The last window waits
for cleanup and preserves refusal reasons until dismissed. No recurring polling.
See `docs/audits/2026-11-02-sidebar-redesign.md` for measurements and verification.
Native installed-app interactions and Windows runtime remain owner checks.
Implementation-stage release work was deferred; release 0.22 is now explicitly
authorized above. Original checkout changes were preserved.


**Release 0.21 combines the markdown reader (#97), live subagent messages and
report recovery (#98 / #85), and the resource-card cleanup (#96 / #95).** The owner
explicitly authorized this agent to merge both feature PRs and publish the combined
release on 2026-11-02. Both features are on main. Version, Mac metadata, release
notes, full changelog and lander are updated together. The reader and Subagents
screenshots need native captures; no product screenshots were fabricated. Manual
checks with real Claude sessions and Windows interactions remain open. Earlier
"no release" notes below describe implementation-stage constraints.

**Issue #85 is implemented on `feat/subagent-live-messages`, stacked on the reader.**
Visible Claude subagent panes read their JSONL incrementally, show assistant text
and folded tool runs, omit thinking/tool output, follow the bottom and preserve a
scrolled-up position. Report and Copy report share hook, SubagentHandback, last-text,
then "No report" fallback. Missing/unreadable files and Codex helpers use the hook
view. Each pane keeps at most 128 entries / 128 KiB of message text; records and each
poll are capped at 256 KiB. Polls occur once a second only while the pane is visible;
closing or hiding it stops transcript reads and polling. Temporary partial records
are bounded too. Reader Task 11 is included here: modifier-click pane links, without
blue styling or wrapped-line joins. Full tests, vet, focused races and macOS/Windows
builds pass; actual headless Midnight/Washi renders were inspected. Real CLI sessions
and native Windows interactions still need owner checks. No release work.

**Markdown reader is implemented on `feat/markdown-reader`.** Cmd+click (Ctrl+click
on Windows) an `.md` path to open a read-only document nested under its session.
Includes GFM rendering, outline, keyboard/wheel scrolling, document and heading
links, and shown-only reload checks, with a 2 MB read cap. Docs are window state,
not terminal tabs or restored sessions. A workspace member shown as the main view
nests docs under the first sidebar session. Windows close is Ctrl+Shift+W. Italic
uses the existing fallback face; docs have no palette entry. Task 11 (subagent-pane
link clicks) moves to #85 at the owner's request. Full tests, vet, macOS build and
Windows cross-build pass; headless Midnight/Washi renders were visually inspected.
No release, version bump or changelog changes. Installed-app checks remain open.

**PR #93 review follow-up addresses all nine findings.** CPU baselines survive
title changes, tab churn and ordinary idle gaps. New processes warm up without
marking totals partial. Each native CPU read has its own timestamp; the hover card
shows the averaging interval. Alias-based Claude/Codex installs resume through a
bare allowlisted command word. Status separators and undersized-card hover cleanup
are corrected. Windows enumerates parent IDs before opening only candidate process
handles; unknown/reused PIDs cannot retain a closed session's row. Native Windows
1/4/16-root cost probes now run in CI. See the review section in
`docs/audits/2026-11-02-resource-usage.md`. No release or merge is part of this work.

**Resource usage (#92) is implemented locally** after owner approval of the
mockup and plan. The status group shows whole-machine CPU share and summed
process memory for Kaze plus observed descendants. Hover shows Kaze and session
rows, core count, units, scope and unavailable readings; scratch shells (removed in 0.27) counted
under their parent and workspace members have stable independent rows. Collection
runs off the UI thread, at most every 2 s on activity, without a new timer, and
pauses while minimized. No GPU measurement. macOS CPU Mach tick conversion is
verified against getrusage; the local 4-root collector benchmark averages about
0.4 ms per scan, projecting to 0.0213% of one core at a 2 s cadence. Tests, vet,
focused races and native macOS / Windows cross-builds pass. Native Windows runtime
and installed-app hover/minimize checks remain open. See
`docs/audits/2026-11-02-resource-usage.md` and the approved resource-usage plan.
The owner authorized committing and opening a review PR on 2026-11-02.
Changes are prepared on `fix/issues-88-90-89`; the release hold remains active.

**Issues #88, #90 and #89 are fixed for review** on `fix/issues-88-90-89`.
No release has been published from this work. Update confirmation text wraps within a growing strip,
with actions below it on narrow layouts. Closed workspace members show stopped
and count down; the status bar says `N/M UP`. Restored agents resolve through the
ready shell's PATH, avoiding the minimal PATH of Finder/updater launches. Native
zsh tests with isolated fake agents verify exact Claude/Codex resume arguments and
visible errors for missing commands. Full tests, vet, UI races and native macOS /
Windows cross-builds pass. Installed update/relaunch with real conversations and
Windows runtime checks remain open. See `docs/audits/2026-11-02-issues-88-90-89.md`.
The combined-release hold in CLAUDE.md remains active.

**Audit follow-up merged in PR #84.** The owner authorized merging and publishing
fix-only release **0.19.1** on 2026-11-01; the release PR prepares its version,
changelog, lander and download notes. Subagent
text wrapping shapes once without paint recording; pane caches track content and
layout; reopened helpers reset their turn's steps/report/time. Git discovery runs
on at most two background workers with stale-result checks and a bounded path
spelling cache. Input reserves chunks and bytes before copying, includes the active
write, and reports overload to the user. The queue budget is 64 MiB + 4 KiB so one
maximum paste fits. PR test/vet/build CI is added, and the concurrent notification
test uses race-safe callbacks. See `docs/audits/2026-11-01-main-followup.md` and the
matching audit-followup design/plan. No installed copy was replaced by this work.
Windows CI also exposed short-path and fixture assumptions, now corrected; the
lifetime-lock regression runs in a subprocess so Windows can clean up its files.

**macOS session shortcuts:** a native hidden-window probe reproduced AppKit
consuming Ctrl+Tab and Ctrl+Shift+Tab before Gio received them. The Mac view's
existing attachment now owns a window-scoped local key monitor that forwards only
those combinations to Gio's normal key path, once, retaining configured actions.
It detaches with the view/window. The native regression uses hidden, unfocused
windows and no global input; router tests cover both directions, wraparound,
workspace/scratch exclusion (no scratch shell since 0.27), Settings/board and overrides. Native CI runs the probe.

**Agent browser feasibility (issue #30) has macOS results**, with the report at
`docs/audits/2026-11-01-agent-browser-feasibility.md` and reproducible scripts plus
recorded evidence under `spike/agent-browser/`. Both MCP candidates passed the
tested tools, lazy Chrome startup and reconnection. Shared windows, contexts and
page-specific debugging endpoints still exposed other sessions. Separate browser
instances passed the assigned-connection checks. No test browser activated while
the focus guard blocked explicit activation requests; arbitrary popups/dialogs and
Windows remain unverified. Standard Node workers also run during tool discovery.
Separate per-session instances/profiles and the Node startup policy are proposals
to settle before a build plan, not approved changes to the draft design. No browser
feature has been implemented or released.

**Start folder (issue #77) shipped in 0.19** (PR #81). `start_dir` is an optional config
string; empty means home, `~` expands, invalid paths/types warn and use home. Settings >
Workspaces > Startup shows the path, Choose… and Use home, reusing the native folder picker.
Only new plain tabs use it; workspace/restored folders and startup precedence are unchanged.
Plain tabs record their folder at creation, before OSC 7. Windows picker runtime remains
unverified. See `docs/audits/2026-11-01-start-folder.md`. The subagent pane rework (#78,
PR #79) shipped in the same release.


**Session restoration (issue #75) is implemented** on `feat/session-restore`
(approved design and plan: `docs/superpowers/{specs,plans}/2026-11-01-kaze-session-restore*.md`).
Default-on `restore_sessions` has a Startup toggle in Settings > Workspaces, even when
there are no workspaces or their file is broken. `internal/sessionstate` saves bounded,
versioned metadata beside config.toml, with atomic replacement, an OS lifetime lock,
coalesced event-driven writes and a final pre-teardown flush. Background cwd and
origin-terminal hooks update it while minimized; scratch hooks could not replace the
main identity (the scratch shell is gone since 0.27). Restore maps names/order/selection and workspace grouping to fresh tabs,
queues exact UUID resume commands after shell readiness, and never replays prompts or
workspace recipes. Explicit --workspace wins; otherwise restoration precedes the
startup workspace. Ordinary extra instances neither restore nor write the owner snapshot.
Updater relaunch waits for the old owner to release its lock after shell teardown;
failed relaunch keeps ownership and restores input. Bench and ordinary test windows
never touch the owner's state. Merged in PR #76; the required 0.18 release updates the version, changelog,
lander and download notes. Installed-app update and Windows runtime checks remain open. See
`docs/audits/2026-11-01-session-restore.md` for verification and remaining platform checks.


**Performance fixes are built for PR #60** (2026-11-01), alongside its sidebar
flash, finished-subagent and long-command fixes. Title/status geometry is cached;
width measurement avoids paint recording; full histories and dirty rows reuse
storage; forgotten pane IDs are pruned; scrolling rotates row references; the
subagent inbox compacts obsolete tools in batches while preserving lifecycle
states. The matched applied-log benchmarks improve from 33.9 to 72.8 MB/s at
120x40 and 8.23 to about 52 MB/s at 240x80 on an Apple M3. These are CPU
microbenchmarks, not native latency results. Full tests, vet, focused races and
macOS/Windows amd64/arm64 builds pass; Windows runtime behavior is not manually
verified. See `docs/audits/2026-11-01-performance.md` and the performance-fixes plan.
No version bump until the release workflow after merge.

## Implementation history

The entries below describe the state at each milestone. Their branch/version and
setup notes are historical; use the current status above and Next below for work.

**Plan 1 (terminal core with tabs) is done**, reviewed task by task plus a whole-branch review, and on `main`.

**Plan 3 (the window frame) is built**: Plan 3a (kaze's own title bar) and Plan 3b (alerts, chime, folder and branch), plans in `docs/superpowers/plans/`. What it added:

- **Own title bar on Windows:** tabs at the top edge, kaze-drawn minimize / maximize-restore / close (close turns red on hover). The empty bar space moves the window (native drag, double-click maximize, edge snap, Win+Z), and at least 48dp of it always stays free however many tabs are open; the 7dp strip above the tabs also moves the window. **No Windows 11 snap-layout flyout** on hover over maximize: Gio has no way to answer `HTMAXBUTTON`. **macOS stays decorated** (the native title bar): an undecorated Gio window loses the traffic lights, so the inset look waits for the Mac milestone.
- **Alerts from Claude Code hooks:** `kaze notify needs|done` sends the tab's id to kaze over **TCP on 127.0.0.1 with a per-run token** (`KAZE_NOTIFY`, `KAZE_NOTIFY_TOKEN`), not named pipes or sockets: no dependency, same code on both OSes. The tab gets a **static** dot (amber = needs you, green = done; no pulse, which would cost idle CPU) and a toast with "Go ›". `notify` always exits 0 within about 1s. It reads `notification_type` from the hook JSON: `idle_prompt` and `auth_success` are dropped, anything else (or no type) is "needs".
- **Chime:** bundled soft chime on "needs you" (`chime_needs`, on), optional on "done" (`chime_done`, off), custom WAV via `chime_file`. No chime when that tab is already active and focused; bursts give one chime. Plays while minimized too.
- **Taskbar flash on Windows only** (FlashWindowEx) when an alert arrives while kaze is minimized or unfocused. The macOS Dock bounce is deferred to the Mac milestone.
- **Folder and branch in the status bar** from OSC 7 (pwsh and zsh snippets in the README) and `.git/HEAD` (worktrees handled, no git process). The branch is re-read on a new prompt report, on tab switch and on window focus gain.
- **Installed on the owner's Windows laptop:** the Claude Code hooks in `~/.claude/settings.json` (Notification with matcher `permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input` -> `kaze.exe notify needs`, Stop -> `kaze.exe notify done`, timeout 3) and the OSC 7 snippet in the pwsh profile. (Historical: the hooks then pointed at `kaze-app\kaze.exe`; since 2026-11-03 they point at the downloaded release, see "The Windows laptop setup".)

**Plan 4 (Claude usage) is built** on `feature/plan-4-usage`:

- `kaze statusline` is Claude Code's status line command. It prints `5h 23%  ·  wk 41%  ·  ctx 34%` for Claude's own status row and, inside a kaze tab, sends the numbers to kaze as a `usage` message on the alert listener. Usage messages never chime, flash or mark a tab, and travel apart from the alert inbox so they cannot replace a pending alert.
- The status bar shows them right-aligned before the memory text. Plan limits are account-wide (latest from any tab); context is per tab and hides once the tab's shell reports a new prompt (OSC 7), meaning Claude exited. Amber at 80%, red at 95%, with the reset time. A window past its reset disappears via one timed wake, no ticker.
- **Owner's scope (2026-10-29):** 5-hour, weekly and context only. No per-model (Fable) number: Claude Code's status line data has none, and the only other source is an undocumented endpoint behind the login token, which he declined. Cost and the process tree are left out.
- **Installed on the owner's laptop:** `statusLine` -> `kaze.exe statusline` in `~/.claude/settings.json`. The old orca status line and its 13 hooks were removed on 2026-10-29 (backups `settings.json.bak-20261029-*`).

**Plan 5a (settings page) is built** on `feature/settings-page` (spec `docs/superpowers/specs/2026-10-29-kaze-settings-page-design.md`, plan `docs/superpowers/plans/2026-10-29-kaze-plan-5a-settings.md`):

- **Settings page** opens from the gear (title bar on Windows, right end of the tab strip on macOS, where the system title bar stays) or Ctrl+, / Cmd+, (`open_settings` under `[keys]`), as a chip beside the tabs. Sections: Appearance (theme, font size, line spacing, cursor, blink) and Claude (chimes, taskbar flash, done-toast time, usage readout, Claude Code setup check). Workspaces is greyed out until Plan 5b.
- **Five themes** in `internal/theme`: Midnight (default), Sumi, Tide, Washi (light), Sakura. A theme sets the terminal palette and the chrome colors; `[colors]` still overrides the palette.
- **Config writes** (`internal/config/write.go`): one key is edited in place, other lines and comments kept. New keys: `theme`, `line_spacing`, `cursor_style`, `cursor_blink`, `taskbar_flash`, `done_toast`, `show_usage`, plus `open_settings` under `[keys]`.
- **Claude setup check** (`internal/claudesetup`): inspects the Stop hook, the Notification hook and the status line in `~/.claude/settings.json` (OK / missing / broken / taken). Fix and Replace only after a confirm dialog, with a timestamped `.bak` copy first; other entries are never touched.
- **`kaze --version`** (`internal/version`) prints the version and build commit without opening a window.
- Not done: the Windows app icon, the notification card redesign.

**The settings redesign is built** on `feat/settings-redesign` (mockup `docs/design/settings-redesign-mockup.html`, approved without its gradients; plan `docs/superpowers/plans/2026-10-31-kaze-settings-redesign.md`):

- **UI font:** Manrope Regular/Medium/SemiBold in `assets/fonts` (static instances cut from the OFL variable font, because Gio draws a variable font only at its default ExtraLight). `chrome.sans`, `page.textW`, `page.mono`; JetBrains Mono stays for keys, paths, commands and versions.
- **Frame** (`settings.go`, `settings_style.go`): menu with title, icons (`chrome.icon`), a "Kaze" group, badges and the update state in the footer; no "Later" line. Rows with `heading` start a card (`newCard` starts one without a label); rows can carry `icon`, `pill`/`tone` and `monoHint`; hints and lines wrap. Controls restyled; buttons can be `ghost`.
- **Pages:** Appearance (theme cards with previews, live preview, cursor chips), Workspaces (a card per workspace with a state pill), Claude (setup check first, with Connected/Missing/Broken/Taken pills), Shortcuts (was Commands: grouped key caps, search), Updates (new; the updater moved out of About), About (lockup, version pill, facts row, Report an issue).
- **Tests** click like a person: `pressAny` scrolls a control into view first, because the roomier pages scroll.

**Agents settings (issue #31) is built** on `feat/agents-settings` (design and plan
`docs/superpowers/{specs,plans}/2026-10-31-kaze-agents-settings*.md`):

- Settings **Agents** replaces Claude: shared alerts, Claude-only usage, then Claude Code and
  Codex setup cards. Existing config keys and installed Claude hook commands still work.
- `kaze notify <needs|done|prompt|interrupt> --agent codex` identifies Codex; no agent flag keeps
  Claude behavior. Codex Stop prints `{}` even outside Kaze. Other Codex commands are silent.
  Codex slash commands and `$skill` names are bounded and sanitized like Claude commands.
- Sidebar and palette sessions identify the agent. Interrupt ends running with no alert, chime
  or flash. The next shell prompt clears the agent. Claude's palette skill catalog is unchanged.
- `internal/codexsetup` checks hooks.json and read-only inline hooks. Fix confirms first,
  preserves other handlers, backs up existing files, checks for stale sources, and writes only
  hooks.json. Inline problems or duplicate Kaze handlers need manual repair. Codex's notify,
  config.toml and hook trust are never written. After Fix, the user must review `/hooks` in Codex;
  the dismissible reminder lasts until restart. The page says Configured, not Connected.
- The ordered JSON editor is shared with claudesetup; Claude's repair and Replace tests remain.
- Verified the prerequisite with the real Codex CLI 0.166.0 in a real Kaze PTY, isolated HOME,
  temporary input-blocked overlay and local Responses stub: env inheritance, loopback delivery,
  prompt delivery, Stop `{}`, one completed turn, and Interrupt on a probe-only SIGINT. No account credentials or external model calls were used.
- Baseline `go test ./...` and vet passed. Baseline UI race testing reproduces the already-known
  race in the fake chime/flash counters of TestOnNotifyNeverBlocksAndStaysBounded. Windows hooks
  remain unverified by hand. No global input was injected and no owner settings were changed.

**Downloads and updates are built** (spec `docs/superpowers/specs/2026-10-31-kaze-updates-design.md`, plan `docs/superpowers/plans/2026-10-31-kaze-updates.md`). Version 0.11:

- **Releases** (`.github/workflows/release.yml`, PR #35): pushing tag `v` + `version.Number` builds the universal Mac app (`KAZE_UNIVERSAL=1 ./tools/package-macos.sh`) and the Windows x64/ARM exes, and publishes them with `SHA256SUMS` and the first-launch notes in `.github/release-notes.md`. Unsigned. The lander's Download buttons point at `releases/latest/download/<zip>`.
- **Updater** (`internal/update`, `internal/ui/updates.go`): checks `releases/latest` 10 s after start and every 24 h (`check_updates`, on). Status bar "Update X" opens Settings, Updates (its own page since the settings redesign); it has Update and restart (asks first), What's new, Check now and the switch. The install downloads the zip and `SHA256SUMS` next to the installed app, verifies, unzips (no `..`, absolute paths or links), renames the old `Kaze.app`/`kaze.exe` aside (`Kaze.app.old`, `kaze-old.exe`, removed on the next start), moves the new one in (and back on failure), relaunches (`open -n` / the exe) and quits.
- **Source builds are never replaced:** a `.git` in the app's or exe's folder or its parent (a clone's root or `dist/`) means Download (release page) only.
- **Test windows never reach the network:** the updater is idle unless `initUpdates` (called from `Run`) set its URL; tests point it at `httptest` servers.
- **Not tested for real yet:** an actual update needs two releases with the updater (0.11, then 0.11.1 or 0.12). Check on the Mac and on Windows that the relaunch comes up and the old copy is removed.

**Plan 6d (floating panes and the scratch shell) is built** on `feat/floating-panes` (plan `docs/superpowers/plans/2026-10-30-kaze-plan-6d-floating-panes.md`, spec sections 7, 8, 11). Version 0.10:

- **Replaced in 0.27:** dock tabs (#133, `feat/dock-alerts-links`) took the place of the floating panes and the scratch shell; `panes*.go`, `scratch.go` and `Tab.Parent` are deleted.
- **Floating panes** (`internal/ui/panes*.go`): a workspace member opens over the shown session from its sidebar line, the server dots or the palette (`floatMember`). Drag the header, resize from edges and corners (6 dp bands, 14 dp corners, 280x140 dp minimum), click to raise and take the keyboard, Expand to make it the main view, × to hide it (the member keeps running). Up to 4 panes per session, the oldest member replaced first; one session at a time per member. Geometry is kept in dp relative to the terminal area and fitted to it every frame.
- **Own renderer and size per pane:** each visible pane has its own `render.Renderer` (a renderer keeps one tab's strips and dirty rows) and its own PTY size (`tabs.Manager.ResizeTab`, `Dock`; `ResizeAll` skips floated tabs). `SetVisible` makes a pane's output ask for frames. Hidden panes and other sessions' panes cost nothing.
- **Input through views** (`termView`): the session and every pane use the same key, paste, mouse and selection code. Esc goes to the program in a pane. Focus reports move with the keyboard.
- **Scratch shell** (`internal/ui/scratch.go`): `scratch_shell` (Ctrl+Shift+` / Ctrl+`), a title bar button on Windows, a sidebar and rail button on macOS. A tab with `Tab.Parent` set: hidden from the sidebar and next/prev, started in the session's folder, kept running while hidden, replaced if it exited, closed with its session. Its hook messages count on the parent row (`hookTab`); usage stays on the scratch tab.
- **To check on the Mac by hand:** the Ctrl+` key name and the scratch button in the rail. **To do by the owner:** the go-winres rerun so the exe details say 0.10.
- **Known failing before 6d** (also on `main` at 61a9229, macOS): the four `TestCount*` line-count tests, `TestRenameSurvivesClaudeLeaving`, and a data race reported by `go test -race` in `TestOnNotifyNeverBlocksAndStaysBounded`.

**Plan 6e (the palette) is built** on `feat/palette` (plan `docs/superpowers/plans/2026-10-30-kaze-plan-6e-palette.md`, spec sections 9.3 and 10), on top of 6b. Version 0.9:

- **Palette** (`internal/ui/palette*.go`): `palette` key (Ctrl+Shift+K / Cmd+K), the title bar search box on Windows, a magnifier next to the sidebar toggle (and in the rail) on macOS. Sections Waiting on you, Sessions, Skills, Kaze; contains-anywhere matching with name-prefix matches first; the list is rebuilt from live state each frame while open, the selection follows its item. No ticker: hover redraws only when the row under the pointer changes.
- **Skill catalog** (`internal/skills`): user and project `skills/*/SKILL.md` and `commands/**/*.md`, enabled plugins from `installed_plugins.json` (newest version, else the highest cache folder). Frontmatter read without a YAML library, 32 KB per file, 1000 entries, read only, on a goroutine when the palette opens, cached by modification time and size. A plugin's nested command keeps its folder in the name (`plugin:folder:name`), as Claude Code names it.
- **Starting a skill** (`internal/ui/prompt_start.go`, spec 9.3): typed into a Claude session in front (no Enter), else a new session in the same folder types `claude '<skill>'` on its first prompt marker; after 3s the workspace "Waiting for shell prompt" banner asks. `workspaces.QuoteArg` quotes for POSIX and PowerShell; other shells get a notice.
- **Settings, Commands:** every shortcut as bound on this machine, the mouse gestures, and the command line with Copy buttons. The nav's Later line is now "Terminal, Shell setup".
- **Not in 6e:** chips and `kaze status` (6c, parked); changing shortcuts from Settings.
- **To do by the owner:** rerun the go-winres command in the README so the exe details say 0.9. The Windows search box is untested by hand (built and tested on the Mac).
- **Known failing on macOS before 6e** (also on `main` at 7822b23): the four line-count tests (`TestCount*`) and `TestRenameSurvivesClaudeLeaving` in `internal/ui`.

**Workspaces UI round 2 is built** on `feat/workspaces-ui` (spec `docs/superpowers/specs/2026-10-30-kaze-workspaces-ui-design.md`, plan `docs/superpowers/plans/2026-10-30-kaze-workspaces-ui.md`): a two-pane workspace editor (tab list left, selected tab right, next unused color, red problem marks), a native folder picker (`internal/pick`: AppleScript `choose folder` on macOS, PowerShell `FolderBrowserDialog` on Windows; Windows not yet tried by hand), a list that shows each tab's color, folder and full command chain, a group dropdown that shows what each member runs and its status (Finished = a prompt report after the line was typed), and About / sidebar without the build line. Still owed: the Windows picker check on the laptop.

**Plan 6b (agent awareness) is built** on `feat/agent-awareness` (plan `docs/superpowers/plans/2026-10-30-kaze-plan-6b-agent-awareness.md`, spec `docs/superpowers/specs/2026-10-30-kaze-agent-sidebar-design.md`). Version 0.8:

- **Prompt hook:** `kaze notify prompt` runs from a Claude Code `UserPromptSubmit` hook. It sends the leading slash command and its first line of arguments, prints nothing, always exits 0, and cleans and caps the text (`internal/notify`, `cmd/kaze/notify.go`). Settings, Claude, Claude Code setup shows it as "Command each session runs"; Fix adds or repoints it with the usual show-the-change, confirm and back-up steps (`internal/claudesetup`).
- **Per-session state** (`internal/ui/agent.go`): running starts on a prompt and ends on done or needs. When the shell prompt comes back, Claude has left and the row is a plain shell. Esc interrupts fire no hook, so an interrupted row stays running until the next message (known limit).
- **Rows:** ` · /command` in magenta, a still `running` ring (no ticker, idle CPU stays zero; `needs you` and `done` win over it). Title rule: your name, then the command's first argument, then the folder name, then the shell title. Double-click a title to rename (Enter saves, Esc cancels, blank goes back to the rule; workspace members can't be renamed). The filter matches the command.
- **Line counts:** `+N -M` from `git diff --shortstat HEAD` (`internal/gitstat`, `internal/ui/linecount.go`), run only on events, one at a time per tab, no console window, 2s timeout, hidden outside a repo.
- **Next waiting:** `next_waiting` (Ctrl+Shift+J on Windows, Cmd+J on macOS, changeable under `[keys]`) and a "Next waiting" button at the bottom of the sidebar. With nothing waiting the status bar says "Nothing is waiting on you."
- **To do by the owner:** press **Fix** once in Settings, Claude, Claude Code setup to add the prompt hook (the installed settings.json predates it); rerun the go-winres command in the README so the exe details say 0.8 (the `.syso` files still carry older versions); then run `kaze.exe notify done` once after the rebuild for Smart App Control. The macOS look of the new rows and the Next waiting key are unchecked on a Mac.
- **What is next:** Plan 6d (floating panes and the scratch shell, `docs/superpowers/plans/2026-10-30-kaze-plan-6d-floating-panes.md`) and Plan 6e (the palette, `docs/superpowers/plans/2026-10-30-kaze-plan-6e-palette.md`). 6e is now built (above). **Plan 6c (`kaze status`, chips, timed checks) stays parked.**

**Plan 6a (the sidebar) is built** on `feat/agent-sidebar` (spec `docs/superpowers/specs/2026-10-30-kaze-agent-sidebar-design.md`). Version 0.7:

- **A left sidebar replaces the tab strip.** Sessions are grouped by git repo (worktrees count as the same repo), groups fold, rows show title, short folder, branch and `needs you` / `done`. Hover gives a ×, middle click closes, `+` opens a session. Settings is a pinned row at the top while open. A "Filter sessions…" field narrows by title, folder and branch (Esc clears, Enter shows the first match, folded groups open while filtering).
- **Workspaces section** at the bottom: a folding header per running workspace ("all up" / "N down"), one line per member (color bar, title, status, `stopped` in red). Clicking a member shows it as the main view. Running a workspace keeps the current session in front. The status bar has one dot per running member plus "servers"; clicking shows the down member.
- **Hiding:** `toggle_sidebar` (Ctrl+Shift+B / Cmd+B) saves `sidebar = true|false`; under 560 dp the sidebar hides without saving. The Windows title bar holds the toggle, K mark, gear and window buttons; macOS keeps the native title bar, with the toggle and gear at the top of the sidebar and a thin rail when hidden.
- **Removed:** the tab strip, the workspace group chip and dropdown, the Settings chip. Ctrl+Tab and Ctrl+Shift+Tab walk sessions in sidebar order and skip workspace members.
- **Plans 6b to 6e were next**, as in spec section 15 (6b is now built, see above): 6b agent awareness, 6c `kaze status`, chips and timed checks (**parked** by the owner, 2026-10-30), 6d floating panes and the scratch shell, 6e the palette, the skill catalog and the Commands settings section.
- **Still open from 6a:** the go-winres rerun is now for 0.8 (above). The macOS look of the sidebar (rail, toggle at the top of the sidebar) is still unchecked on a Mac.

**Plan 5b (Workspaces) is built** on `feature/workspaces` (decisions `docs/superpowers/specs/2026-10-29-kaze-workspaces-decisions.md`, mockups in `docs/design/`). Version 0.6:

- **Workspaces** are saved launch recipes (name plus ordered tabs: title, folder, commands, color, wait for nothing / Docker / a port) kept in `workspaces.toml` beside `config.toml` (`internal/workspaces`). kaze owns the layout; a hand-edited file is checked strictly and never overwritten when invalid. Edited in Settings > Workspaces with a Save / Cancel draft; Run, or Go to tabs when members are already open.
- **A run is one tab group** in the strip (chip with member dots, name and count, dropdown, alerts on the chip, Ctrl+Tab steps through members), not one tab per member. Real panes are a later plan and would replace groups.
- **Commands** are typed as one joined line (`; if ($?) { }` in PowerShell, `&&` in zsh/bash/sh) once the first prompt marker (OSC 7 or OSC 133 A) arrives; after 3s without one a banner offers Run commands / Use shell. Typing in a waiting tab cancels its commands. Docker (started once if needed) and port waits give up after 3 minutes.
- **Startup:** `startup_workspace` (a workspace ID) and `kaze --workspace "<name>"`, which wins.
- **To do by the owner:** the `.syso` files in `cmd/kaze` still carry version 0.5; regenerate with the go-winres command in the README. The macOS run of the shell-readiness and one-line tests is still owed. The `samurai` workspace is created only on his word.
- Known limits: no folder Browse button, commands can not be reordered, closing the window drops an unsaved draft.

What works (tested on Windows):

- Tabs (Ctrl+Shift+T / W, Ctrl+Tab), each with its own shell (pwsh on Windows, zsh on macOS) and `KAZE_TAB_ID`.
- Full xterm-style emulation for Claude Code, git, vim, less: colors (256 + truecolor), alt screen, scroll regions, bracketed paste, mouse modes, focus events (1004), synchronized output (2026).
- Fast renderer: CPU-drawn grid in 8 strips, only changed rows redrawn, nothing drawn when idle.
- Japanese: wide characters, bundled Noto Sans JP subset, system font fallback for rare kanji.
- Symbols Claude Code prints (`⏺ ⎿ ✻`, spinners) via system symbol fonts, and emoji in colour on Windows (Segoe UI Emoji COLR) and macOS (Apple Color Emoji sbix). Box-drawing is drawn by kaze so borders join.
- Readable text on colored backgrounds (minimum-contrast nudge).
- Mouse selection (drag, double-click word, triple-click line, Shift+drag in mouse-mode apps), Ctrl+Shift+C / V, optional `copy_on_select`.
- Status bar: kaze's private memory and tab count.
- Config file with warnings for bad values.

Measured (Windows, `docs/benchmarks/2026-10-28-milestone-1.md`):

| Goal | Result | Met |
|---|---|---|
| Claude Code streaming frames < 16.7ms | p99 5.1ms | yes |
| Smooth scroll through 10,000 lines | no late frames | yes |
| Idle CPU ~0 | 0.0% with 4 tabs | yes |
| Memory doesn't grow when idle | ~147MB, flat; +0.5MB per idle tab, ~+30MB per tab with full history | yes |
| Keypress to glyph < 10ms | ~25ms typical; kaze's own part ~8ms p50, ~24ms p99 | **no** |
| Launch to prompt < 500ms | ~1.3s with pwsh (kaze's first frame ~0.45s) | **no** |

**Window chrome redesign (feat/window-chrome):** one Kaze-drawn 42dp title bar on macOS and Windows (spec `docs/superpowers/specs/2026-10-31-kaze-window-chrome-design.md`) and the direction-A status bar (32dp). macOS is now frameless: `internal/ui/maclights_darwin.go` shows the native traffic lights again and keeps them centred; its callbacks never call Gio inline (that deadlocked once). Native double-click on empty bar space zooms or minimizes per `AppleActionOnDoubleClick`. Owner checked the Mac behaviour by hand on 2026-10-31. The Windows build compiles (amd64, arm64) but has not been run since the redesign: try it on the Windows laptop. Every lander capture still shows the old chrome.

## Next

- Owner: open the reader spec from terminal output, use the outline, return via the
  session row, edit the file and check reload, close with Cmd+W, and try all themes.
  On Windows verify Ctrl+click, backslash paths and drive letters. Native runtime
  checks remain open. New reader screenshots can be captured after owner review.

1. Verify the 0.20 update in an installed copy: the CPU/RAM status group, its
   hover card, minimize/restore, and resumed agents after the update relaunch.
   The status bar changed, so the lander screenshots need retaking
   (`docs/site-assets/SCREENSHOTS.md`). On Windows, check the resource numbers.
2. Verify installed-app update/relaunch and current window chrome, hooks and native
   folder pickers on Windows. A cross-build does not verify runtime behavior.
3. Follow up the existing combining-mark/NFD and Option-as-Alt gaps separately.
4. Re-run native keypress/first-frame benchmarks before claiming end-to-end gains.
   Incremental link scanning is lower priority than these measured fixes.
5. Subagent in-between messages require a transcript reader; hooks only provide
   the prompt, tool activity and final report. Other future ideas remain in specs.

## Mac milestone checklist

Native macOS builds, tests and owner chrome checks are complete. This original
setup/checklist is historical; unresolved platform checks are listed above.

1. Install Go (version in `go.mod`) and the Xcode Command Line Tools: `xcode-select --install`.
2. Clone and build: `git clone https://github.com/beefysalad/kaze-terminal && cd kaze-terminal && go build -o kaze ./cmd/kaze && ./kaze`.
3. Run the tests: `go test ./...` and `go vet ./...`. Native macOS tests now run locally; PR CI adds macOS and Windows checks.
4. Check by hand, in a kaze tab:
   - zsh starts; `echo $TERM $COLORTERM $TERM_PROGRAM` prints `xterm-256color truecolor kaze`.
   - `claude` renders with colors, its input box borders join, symbols show.
   - Japanese file names: macOS stores names in NFD, and kaze currently **drops combining marks**, so `ls` of Japanese names may lose dakuten (が → か). Known gap, fix it here.
   - Rare kanji draw (Hiragino fallback paths in `internal/render/fontpaths_darwin.go` are unverified).
   - Cmd+T / Cmd+W / Cmd+C / Cmd+V work.
   - **Option key:** kaze treats Option as Alt unconditionally. On layouts that type `@ { | [ ]` with Option this breaks; add a config toggle.
   - Trackpad/mouse wheel: scroll speed may be off (unit not verified on macOS).
   - macOS status memory now uses `proc_pid_rusage` physical footprint, including native allocations.
5. Plan 3 on macOS:
   - **Hooks setup:** add the two Claude Code hooks from the README ("Alerts") to `~/.claude/settings.json` on the Mac, with the Mac path to `kaze` (`"/path/to/kaze" notify needs` / `notify done`, timeout 3). Check that a permission prompt in a kaze tab marks it and chimes (`afplay`).
   - **zsh snippet:** add the OSC 7 snippet from the README to `~/.zshrc` and check folder and branch show, including a folder with spaces, `#`, `%` and Japanese. Unlike the pwsh one, the zsh snippet only escapes `%`, `#` and space and sends other characters as raw UTF-8 (kaze accepts that); it could not be run on Windows, so verify it here.
   - **Dock bounce:** alerts while kaze is in the background should bounce the Dock icon (the Windows taskbar flash has no macOS twin yet; `flashTaskbar` is a no-op off Windows).
   - **Title bar:** the native traffic lights are restored on the custom frame; owner checked the Mac behavior on 2026-10-31.
6. **Battery (#139, `feat/macos-battery`):** the IOKit source is built (`internal/battery/source_darwin.go`
   + `iops_darwin.c`; changes come from the `com.apple.system.powersources.percent` and
   `kIOPSNotifyPowerSource` notify alerts, not a run loop source). Check the far right of the status bar
   matches Windows: level, percent, the bolt while charging, amber at 20% or less, red at 10% or less,
   nothing on a Mac desktop, no wakes while idle or hidden.
7. Benchmarks: `bench/run.sh` (it uses a history-free shell and kaze drops real input in bench mode, so it can't type into your sessions). Add a "macOS" column to the milestone doc.

## Decisions so far (why the code looks the way it does)

- **Editable shortcuts:** `none` means an action has no shortcut. Two shortcuts clash whatever order the
  modifiers are written in. Settings always keeps a shortcut (a hand-written `open_settings = "none"` is
  ignored with a warning). The Shortcuts page writes the file only through `config.SetKeys`.
- **Go + Gio**, one codebase for Windows and macOS. Beating Warp is not a goal; "fast, not laggy, fits the workflow" is. Memory has no hard cap but must not grow when idle.
- **Renderer:** a CPU-drawn image of the grid, split into 8 horizontal strips; only strips with changed rows are re-uploaded. Chosen by two spikes (`docs/spike-results.md`): Gio's text path and per-glyph ops were far too slow.
- **Synchronized output:** frames are held while mode 2026 is on, up to 150ms, but never when nothing has been drawn yet (tab switch).
- **Parser** treats `:` like `;` but records which params were colon-separated, so SGR `4:3` (curly underline) and `38:2::r:g:b` work.
- **Live processes stop when Kaze closes.** Shells live in the Kaze process behind `ptyx.Session`. Issue #75 supersedes the earlier no-restoration decision: folders and exact supported agent conversations now reopen automatically from saved metadata; this does not keep PTYs alive.
- **Shell env:** each tab gets `KAZE_TAB_ID`, `TERM=xterm-256color`, `COLORTERM=truecolor`, `TERM_PROGRAM=kaze`; inherited `TERM_PROGRAM`/`WT_*` from the launching terminal are dropped.
- **Title bar:** custom chrome on both platforms, with restored native traffic lights on macOS (owner checked 2026-10-31).
- **Alert transport:** TCP 127.0.0.1 + random token (spec 6.4 note), not named pipes.
- **No pulsing dot, no idle reminders** (spec 6.8 note): a static dot keeps idle CPU at zero; `idle_prompt` and `auth_success` notifications are dropped.
- **Copy is Ctrl+Shift+C** (Ctrl+C must stay "interrupt"). Smart Ctrl+C is a later idea, not wanted yet.
- **Ctrl+Backspace** sends 0x08 (word delete in PSReadLine).
- **Claude Code features left unanswered** (kitty keyboard protocol, XTVERSION, some OSCs): Claude falls back fine. Shift+Enter now aliases Alt/Option+Enter for a newline (issue #114); full kitty keyboard support remains deferred.
- **Fonts:** JetBrains Mono is the only UI font (owner, 2026-11-05; Manrope dropped), Noto Sans JP subset bundled for Japanese (OFL); missing symbols and rare kanji come from system fonts, loaded lazily, memoized, malformed fonts skipped.
- **Low-contrast text** is nudged just enough to reach 4.5:1, not forced to white (keeps dim text dim).
- **Stable line numbers (find):** a match is stored as `screen.Evicted()` plus its row, a number that never
  changes while the line exists, so eviction only drops matches and never shifts them. Use the same pattern
  for anything that must survive the scrollback ring.
- **Step budget (find):** one search step reads at most `find.StepBudget` = 1,000,000 cells under the session
  lock (about 2 to 2.5 ms here, 2,000,000 took 7 ms, over the 5 ms a frame may spend), then asks for another
  frame. No background goroutine.
- **Benchmarks** measure keypress latency inside kaze (`KAZE_BENCH`), never with global input.

## Known gaps (deferred, not blocking)

- Shortcuts: no chords (two-step keys) and no second shortcut per action; no tooltip on the reset button;
  hover is not shown on the keys button (keyboard focus only); keys that `parseShortcut` cannot name
  (Insert, numpad keys, `+`) cannot be recorded.
- Combining marks dropped (NFD on macOS, see checklist).
- macOS: Option-as-Alt toggle, wheel units and rare-kanji fallback verification. Memory reporting uses physical footprint.
- Scrollback uses packed 16-byte cells, trims default blank tails and reuses evicted lines; retained memory still depends on line widths and contents.
- Ctrl+Alt+letter is dropped on a US Windows layout.
- Status bar lacks CPU and the per-tab process tree (spec 6.5; left out of Plan 4 by the owner).
- `font_family` is accepted but not supported yet (kaze warns).
- Glyph cache is safe only with one window / one UI goroutine (add a mutex before multi-window).
- Closing a tab whose ConPTY close times out can keep that screen in memory until ConPTY lets go.
- Links set with OSC 8 (hidden link targets) are not read (#128 item 3; needs its own design).
- Accepted link false positive (Ruling 21 of the 0.27 plan): in "cd /tmp then edit docs/notes.md" the underline starts at `/tmp`; the click still opens `docs/notes.md` through the shorter candidates.
- The macOS percent alert (`com.apple.system.powersources.percent`) is not named in the public SDK; powerd
  posts it on every percent change. If a future macOS stopped posting it, the percent would refresh only
  when the charger is plugged in or out (the power source alert is public and still fires).
- No battery tooltip (time remaining), left out to keep the bar's cost unchanged (Ruling 7 of the 0.28 plan).
- A battery plugged into a desktop after Kaze started shows only after a restart (Ruling 2). If the OS
  notifications fail to register, an idle window may show a stale percent until something redraws (Ruling 3).
- Agent names: two prompts identical in their first 2 KiB count as the same task, so the start order
  decides (Ruling 17).
- Readable activity: a tool result more than 64 calls after its step shows no outcome; "failed · N
  tests" counts only top-level Go `--- FAIL:` lines.
- Agent-touched `.md` links: a file written only from the shell (`echo > plan.md`, a script) is not
  reported by the hooks, so its link falls back to guessing from the folder and Git root. The list of
  touched files is in memory only (gone on restart, dropped with the tab). The Codex `apply_patch`
  payload shape the hook reads is unverified against a real Codex session (it could not be captured:
  no network in the build sandbox).
- Find: no regex; no find in the agent panel or the doc reader (the shortcut does nothing there); case is
  simple lowercase folding, so "ß" does not find "SS" (Ruling 3); only the newest 1,000 matches are kept
  (Ruling 9); no F3 or Cmd+G for next match, Enter and the arrows do it (Ruling 22).
- Smaller items: the full later-fix backlog is below, and every "minor (deferred)" line is in `docs/history/plan-1-ledger.md`.

## Where kaze came from

Patrick (the owner) started kaze on 2026-10-27 after seeing **sir Sat's** terminal. sir Sat is Patrick's senior/lead at work; his own terminal is written in **Swift** (likely macOS-only), very lightweight, memory-lean and fitted to his workflow. His key idea, which kaze copies: **only redraw what changed** (dirty tracking). sir Sat's terminal also has a **browser button** that opens a separate Chromium window; that's kaze's "browser button" later feature. Swift was ruled out for kaze because Swift on Windows has no usable UI toolkit; Go + Gio was chosen instead.

Patrick's daily terminal before kaze was **Warp** (Windows and Mac). He uses it like a plain terminal; the one Warp feature he relies on is the pane that says a tab's Claude session is done or needs him (that's Plan 3's alerts). He runs 3-4 tabs at once, flips between them, and wants side-by-side only for related things (a lead agent and its subagents: Plan 2). He doesn't speak or type Japanese, but his work output is full of it, so Japanese must display correctly (no IME needed).

He also considered Zellij; the answer was: it runs inside kaze, but kaze's own tabs are needed for the per-tab Claude features.

## Working with the owner

- **Plain language, short.** TL;DR first. No jargon; explain terms the first time. Use tables for status.
- **Recommend, don't survey.** When there's a choice, give one recommendation and why. End decision questions with the recommendation.
- **One question at a time, with context first** ("here's what I found, so: X or Y?").
- **No em dashes** in anything written for him. 12-hour times. Translate any Japanese you quote.
- **"how far along are we" / "update?" = report only**: re-check live state (git log, status) before any summary, show a progress bar plus a table, never stop or restart work to answer.
- **Brainstorm before building** anything new; he approves the design, then the plan. Once approved, **don't stop for check-ins**: run to the end, stop only for blockers, irreversible actions, or real decisions.
- **Never commit or push unless asked**; never push to `main` (his git hook blocks it: branch + PR, he merges). **Never AI attribution** in commits or PRs.
- He was fine with parallel agents ("can some tasks run in parallel?") when tasks don't share files: use separate git worktrees and merge when the main checkout is idle.
- **Smart App Control stays ON** on the Windows laptop (turning it off is irreversible). Live with the occasional "Part of this app has been blocked" pop-up during builds; rebuild with a new build id when an exe is blocked.

## The Windows laptop setup (for reference)

- Repo: `C:\Users\john\Desktop\kaze`. Go 1.27 installed in `%LOCALAPPDATA%\Programs\go` (zip install; winget/go.dev downloads failed, `dl.google.com` worked).
- His daily Kaze (since 2026-11-03): the downloaded release at `C:\Users\john\AppData\Local\Programs\Kaze\kaze.exe`, updated from Settings, Updates. The Claude Code and Codex hooks point there. The old `Desktop\kaze-app` source build was retired and deleted. Settings > Agents reports hooks that call any `kaze.exe` as Connected, even an old copy's path, so check the paths when Kaze moves.
- pwsh profile: `C:\Users\john\OneDrive\Documents\PowerShell\profile.ps1` (has the PSReadLine lines below).
- Test runs used `GOTMPDIR` under the Desktop because App Control blocks test binaries in the default temp dir.
- Backup of the pre-rewrite history: `C:\Users\john\Desktop\kaze-backup\kaze-before-rewrite-2026-10-28.bundle`.

## Incidents in Plan 1 (and the rules they produced)

1. **OOM crash.** A RED test for "clamp 65535x65535 to 4096" ran before the clamp existed and allocated billions of cells, crashing the machine and the session. Rule: RED tests must fail safely (bounded sizes, pure-function tests for limits).
2. **Stray keystrokes.** Two GUI-testing agents used global SendKeys/SendInput while kaze wasn't the foreground window; `cls` and `echo hello-kaze-selection second-word` landed in Patrick's live Claude Code window. Later, benchmark windows grabbed focus and ~11 of his real keystrokes went into a bench shell, and 3 `flood.exe` lines landed in his pwsh history (not yet removed; ask him). Rules: never inject global input; bench mode drops real input by default; bench shells don't save history; launch without activating.
3. **Smart App Control** blocks fresh unsigned binaries at random. Not an incident, a fact of life on that laptop.

## Later-fix backlog (from the final whole-branch review)

Remaining Mac checks: Option-as-Alt toggle; NFD combining marks; rare-kanji fallback paths; wheel units. Physical-footprint memory reporting is implemented.

Later:
- A few remaining screen/vt test gaps (combining-mark escape, wide char while wrap pending, CAN/SUB inside OSC/DCS, split ST, truncated colon groups, colon-mask reset).
- UTF-8 second-byte ranges (E0/ED/F0/F4) not validated (harmless: renders a replacement glyph).
- ptyx: 3s wait-test limit could flake on slow CI; small Resize-vs-Close race window; Read/Write after Close reach the closed pty.
- Multi-param `CSI ... T` treated as scroll-down.
- Cmd+nav/Enter still encode on macOS.
- Ctrl+Alt+letter dropped on US Windows layout.
- Hidden tabs resize when the grid size actually changes; the unchanged-size guard is implemented.
- Input overload is rejected whole with a status notice; the queue is bounded by chunks and bytes, including the active write.
- Glyph cache unguarded across UI goroutines (fine with one window); double-line box junction overlap. Shared font-size caches retain only the two most recent sizes.
- Selection uses retained dirty-row scratch. Ideographic space U+3000 is not a word stop (matters for Japanese double-click).
- Full-history memory varies with retained line widths; cells are packed to 16 bytes and blank tails are trimmed.
- Full kitty keyboard protocol support remains deferred. Shift+Enter aliases Alt/Option+Enter for Claude Code multiline input (issue #114, implemented locally).
- A ConPTY close timeout keeps that tab's screen alive until ConPTY releases.
- kaze's own keypress p99 (~24ms) and first frame (~0.45s).

## If this repo goes public

History was already scrubbed once (before the first push). Personal/work context was re-added afterwards in `docs/HANDOFF.md`, `docs/history/` and `CLAUDE.md`. Before going public again: remove or neutralize those (names, employer details, ticket keys like `V3-...`, local paths, the ledger), and check `git log --all -p` with a grep, because history keeps old versions. The scrub script used last time replaced terms with `git filter-repo` on text blobs only (fonts untouched) and moved authors to `283561876+patrick-gogo@users.noreply.github.com`.

## Optional shell setup

Autocomplete in pwsh (history suggestions + Tab menu) is shell-level, not kaze. In `$PROFILE`:

```powershell
if ($Host.Name -eq 'ConsoleHost' -and -not [Console]::IsOutputRedirected -and (Get-Module -ListAvailable PSReadLine)) {
    Set-PSReadLineOption -PredictionSource History -PredictionViewStyle ListView
    Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete
}
```

macOS equivalent: the `zsh-autosuggestions` plugin plus zsh's own `compinit` menu; the steps are in the README ("Optional shell setup", zsh).
