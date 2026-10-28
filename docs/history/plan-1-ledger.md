> **Note:** this is the raw progress ledger the coordinating session kept while building Plan 1 (2026-10-28).
> Commit hashes in it are from BEFORE the history was rewritten for the first push (authors moved to the GitHub
> noreply address and work references scrubbed), so they don't match `git log`. Match by commit subject instead.
> "Ruling" lines are decisions made on the owner's behalf; "minor (deferred)" lines are the later-fix backlog.

# SDD ledger — plan: docs/superpowers/plans/2026-10-28-kaze-plan-1-terminal-core.md

Spec: docs/superpowers/specs/2026-10-28-kaze-terminal-design.md
Branch: feature/plan-1-terminal-core (from main b15328c)
Env: Go 1.27.0 at %LOCALAPPDATA%\Programs\go\bin (user PATH). Module downloads need the shell sandbox off (network blocked inside it). proxy.golang.org reachable; go.dev direct downloads reset, dl.google.com works.

## Pre-flight scan

| Tasks | Shared file / interface | Finding |
|---|---|---|
| 1 → 2,3,5,7 | screen.Screen, Cell, Color | consistent: later tasks only add methods |
| 2 → 3 | LineFeed scroll-up → scrollback | consistent: 3 hooks into 2's scroll |
| 2 → 5 | LineFeed / scroll region | consistent: 5 makes scroll honor DECSTBM |
| 4 → 5,6,7 | vt.Handler / Parser | 6 says "parser treats ':' like ';'" but Task 4's text never asks for it. Ruling R1 |
| 5 → 6,7 | test helpers run/row (5), run2/applierFor (7) | same package test files; 7 defines run2/applierFor itself. OK |
| 5,7 → 9 | NewApplier(s, reply io.Writer) | consistent |
| 7 → 10,11 | Modes, SyncSince | consistent |
| 8 → 9,13 | ptyx.Session, Start, DefaultShell | consistent |
| 9 → 10,11,13 | term.Session Lock/Screen/Applier/Write | consistent |
| 10 ↔ 12 | Layout(gtx, s, sel *Selection) | Task 10 references Selection, which Task 12 defines. Ruling R2 |
| 13 ← 14 | config.Config | plan says 13 uses hard-coded defaults with same field names; 14 wires in. OK |
| 0 → 10 | winning text path | 10 must read docs/spike-results.md. OK |

| Task | Self-consistency |
|---|---|
| 0 | Step 1 asks to install Go: already done by controller (1.27.0). Ruling R3 |
| 1 | tests match signatures. OK |
| 2 | wrap test expects 'd' row 0 after 7 chars in 3x2: abc/def → scroll → def/g.. → row0 "def"? Ruling R4 |
| 3 | OK |
| 4 | OK apart from R1 |
| 5 | scroll-region test: rows top,A,B,bot; region 2-3; cursor row 3 then LF scrolls region: row1=B,row2=blank. OK |
| 6 | OK |
| 7 | OK |
| 8 | OK |
| 9 | OK |
| 10 | headless GPU may be unavailable; plan already allows Stats()-based tests. OK |
| 11 | OK |
| 12 | OK |
| 13 | OK |
| 14 | OK |
| 15 | doc filename date is the run date. OK |

## Rulings

- Ruling R1: Task 4's parser treats ':' as a param separator like ';' (Task 6 depends on it) — Task 6 states it as a parser behavior — if wrong, colon subparams for underline color are mis-split, cosmetic only.
- Ruling R2: Task 10 creates internal/render/selection.go holding only `type Point struct{Col, Row int}` and `type Selection struct{Start, End Point; Active bool}`; Task 12 adds the methods/functions to that file — keeps Task 10 compiling without reordering — cost if wrong: trivial file move.
- Ruling R3: Task 0 skips installing Go (controller installed 1.27.0); implementer only runs `go mod init` — tooling already present — cost if wrong: none.
- Ruling R4: Task 2 TestPrintAtBottomRightWrapsAndScrolls: "abcdefg" in 3x2 gives row0 "def", row1 "g". The plan's assertion (row0[0]=='d', row1[0]=='g') is correct. No change — checked, not a conflict.

## Progress
- Ruling R5: Task 0 'visible tearing' cannot be judged by an agent; implementer records frame-time p50/p99 and dropped/late frames under -stream as the proxy, visual tearing check goes to Patrick at the milestone run — cost if wrong: a tearing problem found later, at Task 15.
- Task 0: dispatched (BASE b15328c, model opus)
- Task 0: GATE FAILED (commit cf2c799). text p99 one-cell 7.32ms / stream 140.9ms / 354.7MB; atlas-clip 48.7ms / 31.2ms / 165.2MB; empty Gio window 160MB private. Warp on this machine right now: 420MB private. Stopped for Patrick's call per spec 10.1. Not reviewed yet (gate decision first).
- Task 0: Ruling R6: Patrick 2026-10-28: keep Go + Gio regardless; drop memory cap and beat-Warp bar; goals = keypress <10ms, stream <16.7ms, idle CPU 0. Spike becomes method picker. Spec s4/s5/s10 + plan updated (commit cf1ea40) — cost if wrong: none, owner's call.
- Task 0: review dispatched (b15328c..cf2c799)
- Task 0b: dispatched (BASE cf1ea40, model opus)
- Task 0: review: spec OK, Needs fixes (2 Important: memory claims unsupported/empty-mode loads fonts; frame time includes vsync Present). Ruling R7: fold Task 0 fixes into the running Task 0b implementer (it owns the same files now) instead of resuming the Task 0 implementer — avoids two agents editing spike/gio concurrently — cost if wrong: 0b report mixes two tasks; re-review covers both.
- Task 0: minor (deferred): pct floors p99 index; single run per number; rows may clip at 1000dp; text-mode CJK not cell-aligned; key handler path unmeasured; CJK fallback rendering unverified visually.
- Task 0b: DONE_WITH_CONCERNS (3287300 Task 0 fixes, c837f52 0b). Winner: image mode, single ImageOp: stream p99 3.51ms, one-cell p99 2.19ms, idle CPU 0, 179MB private (bare window 83.6MB). 8-strip variant also passes. Concerns: full 5.4MB texture upload per change; Task 0 idle CPU may have had the same measuring bug.
- Task 0+0b: combined review dispatched (cf1ea40..c837f52)
- Task 0: complete (commits b15328c..3287300, fix round folded into 0b per R7, review clean)
- Task 0b: complete (commits cf1ea40..c837f52, review clean)
- Task 0b: minor (deferred): idle CPU re-runs uneven across modes; run-to-run variance not discussed; Task 0 tables still show old 4ms target; Present-floor wording; measurement script not committed.
- Ruling R8: Task 10 uses the image method with a strip count constant, default 8 (not 1) — single vs 8 strips is within run-to-run noise on stream (3.5 vs 4.7ms), strips are 2x better on one-cell (1.0 vs 2.2ms) and upload only changed strips, which scales better on big/high-DPI windows — cost if wrong: change one constant.
- Task 1: dispatched (BASE c837f52, model sonnet)
- Task 1: DONE f16b0d5; review dispatched
- Task 1: complete (commits c837f52..f16b0d5, review clean)
- Task 1: minor (deferred): TakeDirty allocates a fresh slice per call (hot path, renderer may want reuse); no tests for out-of-range Cell/SetCell or MarkAllDirty; missing doc comments on Size/Cursor/MarkAllDirty.
- Task 2: dispatched (BASE f16b0d5, model sonnet)
- Ruling R9: Patrick asked for parallel execution (overrides the skill's one-implementer rule). Waves: [2,4,8,14] now; 3 after 2; 5-6-7 serial after 3+4; 9 and 11 parallel after 7; then 10,12,13,15. Parallel implementers run in isolated git worktrees off f16b0d5; controller merges each branch into feature/plan-1-terminal-core after its review (go.mod/go.sum conflicts resolved with go mod tidy) — cost if wrong: merge conflicts, caught at merge.
- Ruling R10: Task 14 runs before Task 8 lands, so Config.Shell defaults to empty, meaning 'use ptyx.DefaultShell()' resolved at startup in Task 13, instead of calling ptyx from config — avoids a cross-package dependency in parallel — cost if wrong: one-line change.
- Task 4, 8, 14: dispatched in parallel (worktrees, BASE f16b0d5, model sonnet)
- Task 2: DONE 4194079; review dispatched
- Ruling R11: parallel worktrees made by hand at C:\Users\john\Desktop\kaze-wt\task-{4,8,14} on branches plan1/task-{N} off 4194079 (the Agent tool's own worktree isolation would target the session's cwd repo, not kaze) — cost if wrong: none.
- Task 3: dispatched in main checkout (BASE 4194079, model sonnet), concurrently with Task 2 review; any Task 2 fixes go to the Task 3 implementer if they touch the same files.
- Task 2: complete (commits f16b0d5..4194079, review clean)
- Task 2: minor fixes sent to Task 3 implementer (orphan wide lead at last column; stale wrapPending on SetAutoWrap(false); go mod tidy). Remaining deferred: visible escape for combining mark in print_test.go; tests for wide char while wrap pending and scroll carrying wide cells.
- Task 14: DONE f5b58ad on plan1/task-14; review dispatched. Note: KnownActions guessed (9 actions); Task 13 owns the final list + default palette/keys.
- Task 14: complete (commits 4194079..f5b58ad on plan1/task-14, review clean). NOT merged yet: Ruling R12: merge parallel branches into feature/plan-1-terminal-core only when the main checkout is idle (after Task 3 commits), so the Task 3 agent's working tree isn't changed under it — cost if wrong: slight delay.
- Task 14: minor (deferred to Task 13): reject NaN/Inf font_size (+ upper bound); reject shell = [""]; no test for valid action with empty value.
- Task 8: DONE f5d06bc on plan1/task-8; review dispatched
- Task 8: review Needs fixes (close timeout returns nil; size validation; resize after close; Wait error). Fix round 1 sent to original implementer (FIX_BASE f5d06bc).
- Task 4: DONE e485af5 on plan1/task-4 (ASCII ~240MB/s, mixed ~110MB/s, 0 allocs); review dispatched. Env note: Windows App Control blocks test exes in default Go temp; set GOTMPDIR to a folder under Desktop for later tasks.
- Task 4: complete (commits 4194079..e485af5, review clean); merged into feature at 04098c6
- Task 4: minor (deferred): UTF-8 second-byte ranges (E0/ED/F0/F4) not validated; CAN in OSC vs ESC CAN inconsistency; overflow test asserts weakly; untested paths (CAN/SUB in OSC/DCS, split ST).
- Task 14: merged into feature at 5dcc370 (go.mod conflict resolved + tidy)
- Task 3: DONE 9e60163 (Task 2 fixes) + 53089d0; review dispatched (4194079..53089d0). Extra: view offset stays anchored when new lines arrive while scrolled back.
- Task 8: fix round 1 done (447ddc1); re-review dispatched (f5d06bc..447ddc1)
- Env: Smart App Control randomly blocks fresh test exes; workaround: GOTMPDIR under Desktop + re-run with -ldflags "-X main.r=$RANDOM".
- Task 5: dispatched in main checkout (BASE 04098c6, model sonnet)
- Task 8: fix round 1/5 (4 addressed, 0 open; commits f5d06bc..447ddc1)
- Task 8: complete (commits 4194079..447ddc1, review clean). Merge pending (main checkout busy with Task 5, per R12).
- Task 8: minor (deferred): raise TestWaitAfterCloseReturnsPromptly limit 3s->10s; Resize closed-check race window; Read/Write after Close hit closed pty.
- Task 3: review Needs fixes (anchoring bump is 0 when ring full -> view creeps; weak out-of-range test). Fix round 1 routed to Task 5 implementer (same package, main checkout busy) per R7 precedent. FIX_BASE 53089d0.
- Task 8: merged into feature at 5fdc8bc (go.mod/go.sum rebuilt with tidy). All 5 packages pass on the merged branch.
- Task 3: fix round 1 done in 672469a (by Task 5 implementer); re-review dispatched
- Task 5: DONE fb130c9; review dispatched (672469a..fb130c9)
- Task 6: dispatched in main checkout (BASE 5fdc8bc, model sonnet)
- Task 3: fix round 1/5 (2 addressed, 0 open; commit 672469a)
- Task 3: complete (commits 4194079..672469a, review clean)
- Task 3: minor (deferred): muddled comment in TestViewStaysAnchoredWhenRingIsFull; ring outer slice could preallocate.
- Task 5: review Needs fixes (DECSC loses wrap flag; ED 3 clears screen; CUU/CUD ignore margins; + minors). Fix round 1 routed to Task 6 implementer (same files, main checkout) per R7. FIX_BASE fb130c9.
- Task 6: DONE 36b186e; review dispatched. Concern: colon form with non-empty colorspace id unsupported (documented).
- Task 5: fix round 1 done in be5c617 (by Task 6 implementer); re-review dispatched
- Ruling R13: Task 7's Claude Code capture runs `claude` in a go-pty recorder for ~5s with no input, then kills it (startup UI shows which modes it sets; no prompt sent, so no usage) — cost if wrong: modes only used mid-conversation are missed; Task 15's replay capture can catch them.
- Task 7: dispatched in main checkout (BASE be5c617, model sonnet)
- Task 6: complete (commits 5fdc8bc..36b186e, review clean)
- Ruling R14: Task 6 review minor is a real bug from R1 (':' flattened like ';'): SGR 4:3 (undercurl) turns text italic and 4:0 wipes the pen. Scheduled as Task 7b after Task 7: parser marks which params were joined by ':' (bitmask, e.g. CSI gets a sub-param mask via an extra Handler-visible accessor), SGR treats 4:x as underline style and skips other colon groups whole. Not fixed inside Task 6 because it changes the vt interface while Task 7 is editing the Applier — cost if wrong: italic/garbled styling in vim/Claude until 7b lands.
- Task 6: minor (deferred): no table test for truncated 38/38;5/38;2;1; semicolon colorspace heuristic edge case.
- Task 5: fix round 1/5 (5 addressed, 0 open; commit be5c617)
- Task 5: complete (commits 672469a..be5c617, review clean)
- Task 5: minor (deferred): TestTabForwardStopsAtLastColumn doesn't guard the change; dead run in TestScrollRegionBottomClamped; weak DCH assertion; multi-param CSI T treated as SD.
- Plan: after Task 7, run 7b (vt+sgr, worktree), 9 (term/session.go, main) and 11 (input, worktree) in parallel.
- Task 7: DONE 99180ca; review dispatched. Claude Code startup emits: ?9001h ?1004h ?25 ?2004h ?2031h ?2026h/l, kitty kbd queries (CSI < u, > 5 u, ? u), CSI >4;2m, XTVERSION, OSC 7501, OSC 0, OSC 8. No alt screen, no mouse, no DSR/DA.
- Ruling R15: add DEC mode 1004 (focus events) to Modes as FocusEvents (Claude Code enables it); Task 11 encodes focus in/out as CSI I / CSI O when set; Task 13 sends them on window focus change. RIS also leaves the alt screen. Both go to whichever agent takes the Task 7 fix round — cost if wrong: Claude Code misses focus changes (cosmetic).
- Ruling R16: kitty keyboard, XTVERSION, OSC 7501/8, 9001, 2031 stay unanswered in Plan 1 (Claude Code falls back when unanswered) — cost if wrong: Claude shows slightly less rich key handling.
- Wave: Task 7b (worktree kaze-wt/task-7b), Task 9 (main), Task 11 (worktree kaze-wt/task-11), all from 99180ca.
- Task 7: complete (commits be5c617..99180ca, review clean). Follow-ups (R15 FocusEvents, Resize both buffers, separate 1049 cursor, RIS leaves alt + resets modes, region reset on switch, OSC 1 test escape, DA1 raw ESC) sent to Task 9 implementer.
- UI: Patrick asked for modern-look mockups; 3 directions (Midnight / Sumi / Tide) pushed to visual companion look.html.
- Task 7b: DONE 55700b2 on plan1/task-7b; review dispatched
- Task 11: DONE 733df7f on plan1/task-11; review dispatched. Merge note: modes.go FocusEvents field also added by Task 9 agent -> expect conflict, keep main's version.
- Task 7b: complete (commits 99180ca..55700b2 on plan1/task-7b, review clean). Merge pending (main busy with Task 9).
- Task 7b: minor (deferred, fold at next fix chance): '4:' (empty sub-param) should act as 4:0; tests for truncated colon groups and mask reset; comment on 32-bit mask width.
- Task 11: review Needs fixes (paste strip O(n^2); start marker not stripped; Cmd/Super combos leak letters). Fix round 1 sent to original implementer (FIX_BASE 733df7f).
- Ruling R17: Ctrl+Backspace sends 0x08 (word-delete in Windows Terminal/PSReadLine), plain Backspace 0x7f — cost if wrong: one table entry.
- Task 11: minor (deferred to Task 13 wiring): Option-as-Alt unconditional (needs config flag + suppress duplicate text event); AltGr arrives as Ctrl+Alt on Windows (guard needed); Gio scroll sign mapping.
- Task 11: fix round 1 done 8488a29; re-review dispatched
- Task 11: fix round 1/5 (9 addressed, 0 open; commits 733df7f..8488a29)
- Task 11: complete (commits 99180ca..8488a29 on plan1/task-11, review clean). Merge pending (main busy with Task 9).
- Task 11: minor (deferred): wall-clock 50ms bound in hostile-paste test may flake; Cmd+nav/Enter still encode; check Gio name for Ctrl+? at runtime (Task 13).
- Ruling/decision: Patrick picked look A 'Midnight'. Spec 6.7 tokens + plan Global Constraint added (uncommitted until main checkout is idle). Mockup copied to docs/design/look-midnight.html.
- Task 9: DONE 4d280df + da3cfdd (Task 7 follow-ups); review dispatched (99180ca..da3cfdd). No cgo -> no -race; -count=20 used.
- Merged 7b (db5a162) and 11 (a875f83) into feature; modes.go FocusEvents conflict resolved keeping main's commented field. Docs committed b0a8fb9. All 6 packages pass.
- Task 10: dispatched in main checkout (BASE a875f83, model opus)
- Task 9: complete (commits 99180ca..da3cfdd, review clean)
- Batch fix (worktree kaze-wt/fixes-9, branch plan1/fixes-9 off a875f83): Task 9 minors 1-4 (saved-cursor wrap on col change; onChange concurrency doc/single goroutine; alt-screen DECSC row shift; total-cell cap) + Task 7b minor ('4:' acts as 4:0). Task 9 minor 5 (drop policy/real ConPTY exit check) deferred to Task 13 runtime check.
- CRASH (session ended): Task 10 had only assets/ (fonts) untracked; fixes-9 had uncommitted edits in 5 files. Both agents resumed via SendMessage with state notes.
- fixes-9: DONE 8e63693; review dispatched. CRASH CAUSE: the RED run of the 65535x65535 clamp test allocated billions of cells and OOM-killed the machine/session.
- Ruling R18: every later brief carries 'RED tests must be bounded: never let a failing test allocate unbounded memory or loop forever (cap sizes, use timeouts)' — cost if wrong: none.
- fixes-9: review Needs fixes (clamp test still allocates 4.29G cells if clamp regresses). Fix round 1 sent: pure clampSize + bounded probes + total-cell cap 1<<22 + PTY resize in NewSession + sgr g[1] guard. FIX_BASE 8e63693.
- fixes-9: fix round 1 done f9606ab; re-review dispatched
- fixes-9: fix round 1/5 (4 addressed, 0 open; commits 8e63693..f9606ab)
- fixes-9: complete (commits a875f83..f9606ab on plan1/fixes-9, review clean). Merge pending (main busy with Task 10). No test can allocate a huge grid now.
- Task 10: DONE 8551c5c (fonts bundled: JetBrains Mono R/B + 2.3MB Noto Sans JP subset, OFL). Added SetFocused(bool). Merged fixes-9 at c84380a; all 7 packages pass. Task 10 review dispatched (a875f83..8551c5c, opus).
- Ruling R19: box-drawing (U+2500-257F) and block elements (U+2580-259F) are drawn procedurally to fill the cell, so Claude Code's input box borders join — Claude Code draws boxes around its prompt — goes into the Task 10 fix round — cost if wrong: gaps in borders.
- Ruling R20: Plan 1 window keeps the native OS title bar with the Midnight tab bar directly under it; an integrated custom title bar (as in the mockup) is later polish — custom decorations risk breaking Windows snap/resize — cost if wrong: looks slightly less like the mockup.
- Task 12: dispatched (worktree kaze-wt/task-12 off c84380a, sonnet). Task 13: dispatched (main, BASE c84380a, opus).
- Task 10: review Needs fixes (glyph cache unbounded incl. per-rune .notdef; box drawing gaps). Fix round 1 to original implementer in worktree kaze-wt/fix-10 (branch plan1/fix-10 off c84380a) because main is busy with Task 13 and task-12 edits renderer selection; includes minors 1-7 and a Release() for hidden tabs. FIX_BASE c84380a.
- Task 12: DONE 90d54b6 on plan1/task-12; review dispatched. Notes: Active=selection exists; absolute rows drift when ring evicts; clipboard/gestures belong to window wiring.
- Task 12: complete (commits c84380a..90d54b6 on plan1/task-12, review clean). Merge pending; expect renderer.go conflicts with fix-10 (snapshot/base block, rasterizeDirty loop, resize).
- Plan: Task 13b after 13 + merges of fix-10 and task-12: wire selection (drag, Shift+drag in mouse mode, double/triple click), clipboard copy, clear selection on alt switch/resize, adjust/clear on scrollback eviction (screen eviction counter), call renderer.Release() for hidden tabs.
- Task 12: minor (deferred): addSelectionRows queues whole ranges + allocs per change; no test for selection when returning to offset 0; ideographic space not a word stop.
- Task 10: fix round 1 done 4927420 (plan1/fix-10); re-review dispatched
- Ruling R21: the fix-10 implementer merges plan1/task-12 into plan1/fix-10 and resolves the renderer.go conflicts (it knows its own changes); the controller then merges the combined branch into main after Task 13 — cost if wrong: one more merge commit.
- Task 10: fix round 1/5 (8 addressed, 0 open; commits c84380a..4927420)
- Task 10: complete (commits a875f83..4927420, review clean)
- Task 10: minor (deferred): shared glyph cache/fonts unguarded across goroutines (safe single-window; needs mutex before multi-window); post-Release sync-hold blank frame; double-line junction overlap; shared cache never drops a size.
- plan1/fix-10 now = renderer fixes + task-12 (merge dcd1d81). Ready to merge into feature after Task 13 commits.
- Flake watch: internal/input had one unexplained FAIL in a batch run (passed 3/3 after). Suspect the wall-clock 50ms hostile-paste test (Task 11 minor). Fix in 13b or Task 15 by making it allocation/op-count based.
- Task 13: DONE dec3bc2, 6a82895, f949ca3. Manual run: 4 tabs, claude --version 2.1.312, git colors, Japanese aligned, Ctrl+Tab wrap, exit marker, close-all quits with no orphan shells, idle CPU 62ms/5s, kaze 25MB with 5 tabs. Fixed ptyx bare-name PATH lookup with dir set. Config warnings go to scrollback (ConPTY clears screen). Review dispatched (c84380a..f949ca3, opus).
- Merged plan1/fix-10 (renderer fixes + task-12) into feature at dc1162e; all 9 packages pass.
- Task 13b: dispatched in main (BASE dc1162e, opus). Task 13 review fixes will be routed to it.
- Task 13: review Needs fixes (input dropped after tab-switch key in same frame; bad shell exits silently in GUI build; + minors). Fix round 1 routed to Task 13b implementer (owns internal/ui now). FIX_BASE f949ca3 (fix lands on top of dc1162e).
- Task 13: minor (deferred): Ctrl+Alt+letter lost on US layout; macOS Option-as-Alt toggle; macOS wheel unit + RSS reading unverified; hidden-tab resize during drag; darwin never built (Task 15 must cross-compile).
- Patrick: first exe at C:\Users\john\Desktop\kaze-app\kaze.exe was blocked by Smart App Control; rebuilt + test-launched until one ran (try 1 ok, 1:18 PM).
- Patrick feedback 10-28 (first real use): (1) icons broken: likely symbols missing from bundled fonts (Claude Code's ⏺ ✻ ⎿ etc. -> .notdef); (2) pwsh directory names (ESC[44;1m) unreadable: default fg on Midnight ANSI blue #8ab4ff bg; (3) wants autocomplete.
- Ruling R23: (1)+(2) become Task 13c after 13b: system-font fallback chain for runes missing from bundled fonts (Windows: Segoe UI Symbol, Segoe UI Emoji as monochrome; macOS: Apple Symbols / Menlo) + a minimum-contrast rule (if fg/bg contrast < 4.5:1, flip fg to black or white) with a test on 44;1m. (3) shell-level first: PSReadLine prediction/menu-complete (his profile, his call); kaze-native completion goes to spec section 7 as a later feature.
- Patrick approved fixes 1+2 and PSReadLine. Profile edited (C:\Users\john\OneDrive\Documents\PowerShell\profile.ps1): prediction History+ListView, Tab MenuComplete, guarded for non-console/redirected runs; verified loads silently. Task 13c dispatched (worktree kaze-wt/task-13c off dc1162e, opus).
- Patrick 10-28: when Plan 1 is done, add remote https://github.com/patrick-gogo/kaze-terminal.git, write a full README, merge feature/plan-1-terminal-core into main, push main. Adapt his pasted snippet (no git init / 'first commit': repo already has history).
- INCIDENT 10-28: Task 13b manual-run synthetic keystrokes ('cls', 'echo hello-kaze-selection second-word') landed in Patrick's Claude Code window (kaze not foreground). Both GUI-testing agents told: verify foreground window == own kaze PID before every send, else abort; prefer non-global checks.
- Ruling R24: future briefs that run kaze for real must not inject global input without a per-send foreground-window check — cost if wrong: typing into the owner's live sessions.
- 13c checks requested: colors missing in its screenshot (contrast rule or plain-text pwsh?) and memory 25MB->123MB (emoji font load?).
- Task 13b: DONE ed98944..a323a49 (+ Task 13 review fixes in 0556b34). Manual GUI select/copy not run (global input banned; Gio uses WM_POINTER). Memory 107-132MB with 1 tab on this branch (13c NOT included) -> regression vs Task 13's 25MB; review told to find it. Review dispatched (dc1162e..a323a49, opus).
- Task 13c: DONE 9ce8510, 23db3dc. Missing colors were NO_COLOR=1 in the agent's env. Fonts add ~0.4MB; old and new builds both ~110MB -> memory regression predates 13c (13b review investigating). Review dispatched (dc1162e..23db3dc, sonnet).
- Ruling R25: accept 13c's contrast deviation: nudge fg toward #0f1117/#ffffff only until 4.5:1 instead of full replacement (full replacement turns dim text pure white) — cost if wrong: one line.
- Follow-up: tab title shows 'Ø Claude Code' (tab bar text uses Gio shaper without the symbol fallback; Claude puts ✳ in its OSC title) -> fix in ui.
- Task 13b: review Needs fixes (stuck drag after mid-gesture tab switch -> 25fps idle redraw). Memory: NOT a regression; Task 13 switched the status bar from Go heap to working set (f949ca3); real working set ~110-135MB, mostly D3D driver + mapped fonts. Fix round 1 sent to 13b implementer incl. status bar -> private bytes. FIX_BASE a323a49.
- Note for Task 15: real per-tab cost when scrollback fills ~ 10000 x cols x 20B (~40MB at 200 cols), never returned; measure and report, consider a smaller default or compact storage later.
- Task 13c: review Needs fixes (faceFor runs before cache hit -> file-backed system-font lookups per cell draw; no recover for malformed fonts). Fix round 1 sent to 13c implementer. FIX_BASE 23db3dc.
- Task 13c: fix round 1 done 3ac7053; re-review dispatched
- Task 13b: fix round 1 done 5491e00 (status bar now private bytes); re-review dispatched
- Task 13c: fix round 1/5 (3 addressed, 0 open; commits 23db3dc..3ac7053)
- Task 13c: complete (commits dc1162e..3ac7053, review clean)
- Task 13b: fix round 1/5 (5 addressed, 0 open; commits a323a49..5491e00)
- Task 13b: complete (commits dc1162e..5491e00, review clean)
- Task 13 (fixes routed via 13b): complete.
- Merged 13c at 4ed25e1; all 9 packages pass.
- Rebuilt C:\Users\john\Desktop\kaze-app\kaze.exe from 4ed25e1 (launch-tested OK).
- Ruling R26: Task 15 measures keypress-to-frame latency INSIDE kaze (KAZE_BENCH: timestamp the key event, timestamp after the frame is submitted, plus a synthetic in-process key injection path for the bench), never global SendInput; Warp is measured for memory and idle CPU only (no latency) — cost if wrong: latency excludes OS input/compositor time; note it.
- Task 15: dispatched (main, BASE 4ed25e1, opus). Task 13d (tab-title symbols) dispatched (worktree kaze-wt/task-13d, sonnet).
- Patrick: asked why copy is Ctrl+Shift+C; explained Ctrl+C = interrupt; offered smart Ctrl+C (copy when selection exists). Decision: leave as is for now (later idea).
- Task 13d: DONE dcebccc on plan1/task-13d; review dispatched
- Task 13d: review Needs fixes (shaper rebuilt per new rune from an already-loaded file; + fast path, 8MB file cap, Cf/bidi sanitizing). Fix round 1 sent. FIX_BASE dcebccc.
- Task 13d: fix round 1 done 2a9c540 (no separate RED run: tests written with code); re-review dispatched with instruction to judge whether each test fails against dcebccc.
- Task 13d: fix round 1/5 (3 addressed, 1 open: size-cap test passes even without the check). Fix round 2 sent (real font + shrinkable cap + real RED run). FIX_BASE 2a9c540.
- Task 13d: fix round 2 done b0bc5f3 (real RED captured); re-review dispatched
- Task 13d: fix round 2/5 (1 addressed, 0 open; commits 2a9c540..b0bc5f3)
- Task 13d: complete (commits 4ed25e1..b0bc5f3 on plan1/task-13d, review clean). Merge pending (main busy with Task 15).
- Patrick 10-28: Plan 3 = attention alerts + kaze's own window controls (custom title bar with close/minimize/maximize-or-hide, tabs inline, 'premium'); supersedes R20 for Plan 3. Spec edit deferred until main checkout is idle (Task 15 may git add).
- Clarified: buttons = close, minimize, maximize/restore (+ edge resize). No tray hide.
- Task 15: DONE_WITH_CONCERNS efee450, 384372f. Met: streaming (p99 5.1ms), scroll, idle CPU 0, memory flat (147MB/1 tab, +0.5MB idle tab, +30MB full history). Missed: keypress p50 24.6/p99 38.3ms (kaze part p50 8.4/p99 24.1), launch 1266ms pwsh (kaze first frame ~450ms). Warp not measured (owner's live terminal).
- INCIDENT 2: bench windows took focus in 4/~30 launches; 11 of Patrick's real keystrokes went into a bench pwsh (no command ran); 3 flood.exe commands landed in his real pwsh history. Agent now drops real input in bench mode and disables history saving.
- Task 15: review dispatched (4ed25e1..384372f).
- Merged 13d at e10e9a6; all 10 packages pass.
- Plan 3 scope (Patrick): + chime with the 'needs you' alert. Proposal: bundled short soft chime (open-licensed or self-made), Windows winmm PlaySound (no cgo), macOS afplay; config chime=on/off per alert kind; default: chime for 'needs you', silent for 'done'; never chime for the focused active tab; rate-limit.
- Task 15: review Needs fixes (Critical: run.sh has no input drop/history guard; input drop opt-in not default; ShowWindow=1 activates; own-share metric mislabeled; doc honesty). Fix round 1 sent (no launches this round). FIX_BASE e10e9a6 (agent commits on top of the 13d merge).
- Task 15: fix round 1 done fa875cb; re-review dispatched
- Task 15: fix round 1/5 (7 addressed, 0 open; commits e10e9a6..fa875cb)
- Task 15: complete (commits 4ed25e1..fa875cb, review clean)
- ALL PLAN 1 TASKS COMPLETE. Final whole-branch review dispatched (opus).
- FINAL REVIEW: Ready after fixes. Must-fix: TERM/COLORTERM + strip inherited TERM_PROGRAM/WT_*; README accuracy (macOS untested, Xcode CLT, shortcuts, Go version, license); font_family silently ignored -> warn; tab-switch blank frame during 2026 hold. Owner decisions before push: work email in commit authors, personal/work refs in spec/docs, module path vs repo name kaze-terminal, LICENSE. Repo patrick-gogo/kaze-terminal is PUBLIC and empty.
- Final fix wave dispatched (one agent, main, opus): TERM env, font_family warning, sync-hold-on-empty fix, .gitignore *.exe/capture.bin, Windows CJK system fallback (YuGothM/msgothic last) + macOS Hiragino, combining marks NOT in scope (later), first-shell-failure message box, README accuracy (license + module path left as placeholders pending owner).
- Final fix wave done fa875cb..60df039 (7 items; 12 packages pass).
- OWNER DECISIONS (10-28, AskUserQuestion): (1) rewrite commit authors to 283561876+patrick-gogo@users.noreply.github.com before first push; (2) make docs generic (no teammate names, work ticket keys, devbox/vault, local paths); (3) rename module to github.com/patrick-gogo/kaze-terminal; (4) MIT license. 1 = controller at push time; 2-4 sent to the final-fix agent.
- Owner decisions applied 60df039..354894d (module rename, MIT, install docs, generic docs). Final fix-wave re-review dispatched (fa875cb..354894d).
- Final re-review: all 10 findings ADDRESSED in tree; history still has personal data + work email on all 68 commits -> rewrite before push (owner chose noreply + generic docs).
