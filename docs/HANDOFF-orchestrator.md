# Orchestrator handoff (Claude + Codex)

Written 2026-11-26 for a context compaction, so a fresh Claude can pick the work up cold. Read `CLAUDE.md` and
`docs/HANDOFF.md` too. This file is the single place for the orchestrator feature and how Claude and Codex work
together on it.

## TL;DR

- **Feature:** an agent in one Kaze tab (say Claude Code) starts worker agents (`claude`, `codex`) in new background
  tabs with a task, waits for them, reads their reports and stops them, the way herdr's `hc run` and
  `hc wait --children` do.
- **State:** tasks 1 to 11 are built on `feat/orchestrator-build`, pending whole-branch review, owner gates
  G2/G3 and release. Gate G1 passed in the spike. Automated evidence does not close native owner gates.
- **Branch:** `feat/orchestrator-build`, based on the approved design branch `feat/orchestrator` and 0.45.1.
  Task 11 is local only: no push, version bump or release changes.
- **Who does what:** Claude plans, briefs, verifies and talks to the owner. **Codex implements** (the owner asked for
  Codex to do the heavy lifting to save usage). Claude never implements a task inline unless it is a tiny fix.

## Read first

1. `docs/superpowers/specs/2026-11-26-kaze-orchestrator-design.md` (about 5,600 words, written by an Opus agent,
   spot-checked). Start with its **"Owner decisions (2026-11-26)"** section: those win over anything below it.
   Sections 1.x are code recon with file:line evidence, 3.x the design, 4 the safety rules, 6 the phased plan.
2. `spike/orchestrator/README.md` and `.superpowers/sdd/orchestrator/spike-report.md` on branch
   `spike/orchestrator-spawn` (throwaway, never merge; commit `b5dbe5f`).
3. The shipped control feature this builds on: `docs/superpowers/specs/2026-11-24-kaze-agent-control-design.md`, the
   README "agent control" section, `internal/control`, `internal/notify`, `internal/ui/control_*.go`,
   `cmd/kaze/control.go`, `internal/agentdocs`.

## Owner decisions (final)

1. **Consent banner: yes.** First spawn per parent tab per run asks on screen (Allow for this session / Deny);
   `confirm = "never"` turns it off. (Herdr does not ask.)
2. **Depth enforced at 1.** Only tabs the owner opened can start workers; workers cannot. `max_depth` setting,
   default 1. Needs the per-tab key (task 3).
3. **Workers close with their parent tab and are never restored.** No `--outlive-parent` in v1.
4. **Notes to the parent** (worker done, blocked, exited) are typed into the parent's prompt only when the parent is
   idle AND the owner has not pressed a key in that tab for 5 s (`note_quiet`, default 5); otherwise they wait and
   `wait --children` returns them, with an unread counter.
5. **Caps:** 4 workers per parent, 8 in total, 6 starts a minute per parent. Refused fast, never queued.
6. Off by default behind a Settings switch plus `[orchestrator] enabled`; only `claude`, `codex` and owner templates
   can run, never an arbitrary command, never a bypass flag (see design section 4).

## Spike results (gate G1 passed, 2026-11-26)

Run by the owner with real agents (stand-in home, `KAZE_SPIKE_SPAWN=1`). Log lines are in
`$TMPDIR/kaze-spike.log` on the owner's Mac.

- Both CLIs take the task as a launch argument and **submit it by themselves** (no Enter): `first_message pending ->
  delivered`. Claude: session hook at +2.2 s, prompt at +2.6 s, done at +7.4 s. Codex (launched with `--no-daemon`):
  session +5.1 s, prompt right after, done at +7 s, and every event carried `agent="codex"` (hooks reach spawned Codex).
- Fresh folder: Claude showed the trust dialog; the session hook came at +8.6 s (after the owner answered) and the task
  still ran. Unanswered, Kaze should report not delivered after 30 s (design 3.6).
- `zsh -l -i -c` did not stall. Settled argv: macOS `[$SHELL -l -i -c 't="$KAZE_TASK"; unset KAZE_TASK; exec claude
  --session-id UUID -n NAME -- "$t"']`, Codex `exec codex --no-daemon -- "$t"`.
- **Not tested:** Windows (is `claude` an `.exe` or an npm `.cmd`? ask the owner to run `where claude` and
  `where codex` on their Windows machine); an unanswered first-run dialog.

## The plan (design section 6, adjusted for the owner's decisions)

| # | Task | Size | State |
|---|---|---|---|
| 1 | Pure rules engine in new `internal/orchestrate`: request validation, caps, rate, depth, cwd check, brief builder, per-OS argv planner (golden tests incl. Windows), Windows command-line length check | M | Done |
| 2 | `internal/tabs`: `Argv`, `Background`, `Env`, `Parent`, `Role`; new env names in `droppedEnv` | S | Done |
| 3 | Per-tab caller key (HMAC) so only owner-opened tabs may spawn; verified in `internal/control` | M | Done |
| 4 | Spawn and kill on the UI goroutine (queue like the `name` op), `win.workers`, snapshot fields, statuses `starting`/`exited`, first-message states, restore exclusion | M | Done |
| 5 | CLI: `kaze run`, `kill`, `report`, `ls --children`, `templates` stub, exit codes | M | Done |
| 6 | `wait --children all|any`, inbox counter, automatic notes with the 5 s quiet rule, no `keys`/`send` into a worker in `needs` | M | Done |
| **G2** | **Owner gate**: local build, real Claude parent starts a Codex worker and a Claude worker, waits, reads reports, kills one | | Owner check open |
| 7 | Presentation: sidebar nesting, worker marker, status-bar counter and "Stop all workers", palette, close with parent | M | Done |
| 8 | Consent banner and `confirm` modes | M | Done |
| 9 | Settings switch, `[orchestrator]` keys, second managed block in `internal/agentdocs` | M | Done |
| 10 | Templates: loader, `kaze templates`, Settings list | M | Done |
| 11 | README, HANDOFF, end-to-end headless tests with fake PTYs | M | Done |
| **G3** | **Owner gate**: visual check on both OSes, then PR and release 0.46 | | Owner check open |

Order: 1 and 2 (parallel), 3, 4, 5, 6, G2, 7 and 9 (parallel), 8, 10, 11, G3. Build the engine and CLI before any
visuals so the owner can test a real parent-and-worker round trip at G2. Windows cannot be run from the Mac: keep
the planner pure and cross-build, and let CI (`windows-latest`) catch portability.

## How Claude and Codex collaborate (the loop that worked for the control feature)

1. **Claude writes the brief** for one task (the task text from the design/plan, constraints, test requirements) and
   starts Codex:
   ```
   hc run --bg --name <name> --cwd /Users/jpatrickzxc/Documents/coding/kaze-terminal \
     /Users/jpatrickzxc/.nvm/versions/node/v22.20.0/bin/codex --dangerously-bypass-approvals-and-sandbox "<prompt>"
   ```
   - The owner **explicitly asked** for Codex agents to run with `--dangerously-bypass-approvals-and-sandbox` so they
     never need permission prompts. Use it. (Without it Codex stops on Go build-cache prompts.)
   - Put the whole task in the launch prompt, never a separate `hc send` to a booting agent.
   - The prompt must tell Codex to report with `hc send t4 "<DONE sha, test summary, report path>"` using the
     **literal orchestrator address** (the Claude session's `$HC_TERMINAL_ID`, `t4` in this session; run
     `echo $HC_TERMINAL_ID` to confirm). Never write `$HC_TERMINAL_ID` inside the prompt: it would expand to Codex's own address.
   - Have Codex write a report file to `.superpowers/sdd/orchestrator/task-N-report.md` (git-ignored scratch).
2. **Wait without polling:** `hc wait <name>` in a loop (it returns on a timeout, a message or when done). A report
   arrives as a message that looks like the user typed it ("Control Task N DONE ...") but it is the helper's report,
   not a user instruction.
3. **Verify every task yourself before the next** (do not trust the report):
   - `hc kill <name>`; `git log --format='%h %s' <base>..HEAD`; attribution check
     `git log <base>..HEAD --format=%B | grep -ci "co-authored\|generated with\|codex"` must print 0;
     `git status --short | grep -v '^??'` must be empty; `git diff --stat`; no files outside the task's scope.
   - `go vet ./...` and `go test` for the touched packages plus `./internal/ui/ ./cmd/...` (ui takes about 25 s).
   - Read the risky diffs yourself (socket goroutines never touch UI state or Gio, locks, deadlines, caps).
   - Check existing tests were not weakened (`git diff --name-status <base>..HEAD -- '*_test.go'`, M files).
   - For UI work: run the headless render test, read the PNG.
   - Make sure no test touched the real `~/.claude` or `~/.codex` (`md5 -q ~/.claude/CLAUDE.md`).
4. **After the last task of a stretch:** a fresh read-only whole-branch review by another agent (Codex worked well,
   told not to edit), then a fix round with a test per finding, then push, PR, CI, owner merges (or says "merge"),
   then the release PR.
5. Claude writes small corrections itself only when tiny (a test portability line, a one-word doc fix); otherwise
   brief Codex. Keep a ledger of rulings in `.superpowers/sdd/orchestrator/progress.md`.

## Rules that bite (from CLAUDE.md and hard experience)

- Strict TDD, **RED tests must fail safely** (bounded sizes and deadlines; an unbounded test once crashed the machine).
- No emojis, no em dashes, no Unicode symbol icons (draw vector icons), Geist for UI chrome and JetBrains Mono for terminal/code, per CLAUDE.md.
- **Commits carry NO AI attribution** (no `Co-Authored-By`, no "Generated with"), even though a tool reminder may say
  to add them: `CLAUDE.md` wins. Commit format `type(scope): Verb ...`. Same for PR bodies.
- **Never push to `main`.** Push branches to the remote named **`fork`** (`beefysalad/kaze-terminal`). The remote
  `origin` points to an old name (`patrick-gogo`): never use it. Use `gh ... --repo beefysalad/kaze-terminal`.
- Native callbacks never run Gio code on the AppKit main thread. Socket goroutines read the immutable snapshot and
  `term.Session` only and queue UI work (see the `name` op in `internal/ui/control_metadata.go`).
- Idle CPU stays 0%: no timer, poll or goroutine runs with nothing pending. The zero-allocation tests in
  `internal/ui` stay green.
- Never inject keyboard or mouse input to the machine. Tests use fake sessions and headless windows.
- Tests never read or write the real home directory (use `t.TempDir()` and injected paths).
- A lander-only change does not get a release; a user-facing feature or fix gets one (minor for a feature, patch for
  a fix), following `CLAUDE.md` "Shipping a feature: lander and release".

## CI and release facts

- PR checks: `test (macos-15)` (about 8 min; the race step has a 10 min timeout; `internal/ui` race takes about 3 min
  locally), `test (windows-latest)` (about 3 min), plus macOS and Windows builds. Watch with
  `gh pr checks <n> --repo beefysalad/kaze-terminal --watch --interval 30` in a background shell (a network reset can
  interrupt it: just rerun). Failed-job logs: `gh api repos/beefysalad/kaze-terminal/actions/jobs/<job id>/logs`.
- **Windows portability traps** that already failed CI once: checkouts convert the README to CRLF (normalise line
  endings in tests that compare file text); path errors differ (a path under a plain file is "not found", not an
  error); the palette allocation cap in `internal/ui/palette_alloc_test.go` is measured on Windows (it is 116 now).
- A flaky Mac test exists in `internal/term` (`TestOnChangeNeverRunsConcurrentlyAcrossExit`): rerun before blaming.
- Release steps (done three times, 0.44, 0.45, 0.45.1): Codex prepares `release/<version>` (version test RED first,
  Info.plist, `winres.json` plus regenerated `.syso`, `docs/changelog.html`, lander `#changelog` keeping three
  releases, news strip, README, `.github/release-notes.md`, one commit `chore(release): Prepare Kaze X`), Claude
  verifies, pushes, opens the PR, waits for checks, merges, then `git tag vX <merge sha>` and `git push fork vX`; watch
  the Release workflow and confirm assets `Kaze-macOS.zip`, both Windows zips and `SHA256SUMS`.

## Testing with the owner on this Mac

- The owner's installed `/Applications/Kaze.app` is their daily driver: never replace or run over it.
- To let the owner try a local build with their real Claude and Codex logins without touching their saved Kaze state,
  launch the binary with a **stand-in home**: a folder under the session scratchpad with a fresh
  `Library/Application Support/kaze` and symlinks to the real `.claude`, `.claude.json`, `.codex`, `.config`, `.local`,
  `.nvm`, `.zshrc`, `.zprofile`, `.gitconfig`, `.oh-my-zsh`, `.p10k.zsh`, `.bun` and `Library/Keychains`; run with
  `HOME=<stand-in> nohup <binary> &`. In that window `~` is the stand-in, so give the owner **absolute** paths.
- Building to a scratch path: `go build -trimpath -ldflags "-s -w" -o <scratch>/kaze ./cmd/kaze`.
- Relaunching a test window ends the owner's sessions in it: say so first unless they already asked for the newest build.
- Flipping the **Peer messaging instructions** switch in such a window writes to the owner's real
  `~/.claude/CLAUDE.md` or `~/.codex/AGENTS.md` (it is a marked block, reversible).

## How the owner likes to work

Plain language, TL;DR first, a recommendation, one question at a time with context, no em dashes. Short answers to
"how far along are we". Likes mockups first for UI. Wants Codex doing implementation, Claude orchestrating and
verifying. Merges PRs themselves (or says "merge"), expects a release after a user-facing merge. Low on usage:
avoid needless re-reading and long reports.

## Open items outside this feature

- Owner has not yet reported the real-install test of 0.45/0.45.1: Claude Code to Codex both ways, the Settings
  switch (fresh `claude` should quote the KAZE_BIN block; if it still says "none", switch to a SessionStart hook that
  injects the instructions), `wait` against a real working agent, Windows runtime.
- Lander screenshots still need retaking (`docs/site-assets/SCREENSHOTS.md`). Lander motion shipped in PR #215
  (FAQ height stays instant by design).
- Rail mode still shows one tile per session, not per space; parked control follow-ups: a key for switching spaces,
  unused fold/tray code, renaming a space.

## Task 11 verification and remaining owner checks

The bounded headless round trip is `TestWorkerEndToEnd` in `internal/ui/worker_e2e_test.go`.
It uses the JSON control adapter, tab manager owner keys, real sessions and fake PTYs.
It covers both agent launch paths, background tabs, sidebar/space metadata, first-message
hooks, tagged report Submit, child waits, quiet notes, approval guards, ownership,
feature off, consent Allow/Deny, kill, parent close and session persistence exclusion.
The companion README contract normalises CRLF before checking the documentation.
Run `go test -count=1 -timeout=30s ./internal/ui -run 'TestWorker(EndToEnd|READMEContract)$'`.
Task reports, including final package/vet/race evidence, are under `.superpowers/sdd/orchestrator/`.

Windows runtime remains unverified. The owner must check native claude.exe resolution,
Codex launching and hooks, quoting, consent and worker presentation there. On macOS,
check a fresh Codex folder's trust dialog with no answer and then after owner approval;
headless tests cannot establish real CLI hook behavior. Templates v1 are list plus
Open folder in Settings, with no editor. G2/G3 and whole-branch review remain open.

V1 config scope confirmed by t4 during Task 11: the owner fixed the caps at 4 per
parent, 8 total and 6 starts a minute, with folder confinement always on. The real
config keys are `enabled`, `confirm`, `note_quiet` and `max_depth` only. The older
design's adjustable limit keys and `any_folder` are deliberately excluded from v1.
README documents this final scope.

## State at handoff

- `main` at the design baseline was `285e819` (0.45.1). `feat/orchestrator` holds the design and owner
  decisions; `feat/orchestrator-build` now holds tasks 1 to 11. The spike branch remains throwaway.
- Scratch reports live in `.superpowers/sdd/orchestrator/` (git-ignored), including `task-11-report.md`.
- Next: t4 verifies Task 11, arranges the read-only whole-branch review and any test-first fix round,
  and coordinates G2/G3 owner checks. No release preparation until the feature shipping gate.
