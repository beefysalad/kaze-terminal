# Task 15 report: benchmark harness and first milestone run

**Status:** DONE_WITH_CONCERNS. Two goals missed: keypress latency and launch time.

## Commits (branch feature/plan-1-terminal-core, author patrick-gogo, no AI attribution)
- `efee450` feat(bench): Add an in-app benchmark mode
- `384372f` chore(bench): Add benchmark harness and first milestone results

## What was built
- `internal/bench` (no Gio import): a `Recorder` built from `KAZE_BENCH_*` env vars (all clamped). `FromEnv` returns nil when `KAZE_BENCH` is unset or `0`, and every method is a no-op on nil. The window checks `win.bench != nil` before every hook, and the manager's notify callback is only wrapped when bench is on.
  - Launch: process creation (`GetProcessTimes`) to the first drawn frame with non-blank text, split into first frame and first shell output.
  - Keys: each key is timestamped when it is routed to the shell and again at the end of the first drawn frame whose cursor moved. The log also records kaze's own share (last echo byte to that frame) and how many frames were drawn while the echo arrived in pieces. `KAZE_BENCH_KEYS=N` injects x/Backspace through `route`, the same path a real key takes, paced by `KAZE_BENCH_PACE_MS` (default 50).
  - Frames: count, drawn, late (>16.7ms), rows rasterized (render.Stats deltas), and p50/p99/max frame time, reported for the whole run, after `KAZE_BENCH_TYPE`, and for the scroll phase.
  - `KAZE_BENCH_TABS`, `KAZE_BENCH_TYPE` (a command written into every tab after the prompt), `KAZE_BENCH_SCROLL` (pages to the top through wheel events once history is full and still), `KAZE_BENCH_SECONDS` (the window closes itself; the process exits 15s later as a backstop), `KAZE_BENCH_IGNORE_INPUT` (drops real input), and a dump of the screen text at exit.
  - Tests: 15, all written failing first (build-fail RED, then GREEN). They cover percentiles, the env-off nil, the nil no-ops, clamps, the echo matching and its timeout, held frames, late frames, launch logged once, key pacing, type-once, the scroll state machine, the latency split, ignore-input and the screen bound.
- `bench/flood`: `-lines N` (max 1M) mixed ASCII/Japanese/SGR colors. `-claude <capture>` plays the Task 7 capture with its queries and mode switches removed, then streams a Claude-style reply for `-seconds` (max 300). The capture has no timestamps, so it can't be played at original speed. The capture read is capped at 16MB.
- `bench/run.ps1`: builds into kaze-wt\bench, starts kaze through WMI so it is outside the foreground app's process tree, gives each config its own APPDATA dir (default pwsh / pwsh with history off / cmd.exe), samples private bytes and CPU, and enforces a deadline. It stops only PIDs it started, plus their shells, and only if they still have the expected name and a start time after the run began. Also `bench/run.sh` (macOS, not run) and `bench/README.md`.
- `docs/benchmarks/2026-10-28-milestone-1.md`: the results table, each missed goal with what was measured, the methods and the run notes.

## Results (Windows)
| Goal | kaze | Met? |
|---|---|---|
| Keypress to glyph <10ms | p50 24.6 / p99 38.3ms (kaze's own share p50 8.4 / p99 24.1ms) | No |
| Streaming frames <16.7ms | 879 frames, 0 late, p99 5.1ms, max 7.9ms (flicker/tearing not measured) | Yes (frame time) |
| Launch <500ms | median 1,266ms pwsh; 605ms cmd.exe; first frame 415-485ms | No |
| Scroll 10,000 lines | 5.67s, 334 frames, 0 late, p99 7.3ms | Yes |
| Idle CPU ~0 | 0.0% of one core over 30s, 4 tabs | Yes |
| Memory | 147.4MB (1 idle tab), 148.8MB (4); -0.3MB over 60s; +0.5MB per idle tab; +30.2MB per tab with full 10k history | Yes (no growth) |
| Warp | not measured: already running as the owner's live terminal; single instance, and 4 tabs would need input | - |

## Concerns
1. **Focus.** The bench window took the foreground in 4 of about 30 WMI launches, and in 1 earlier direct launch. Gio calls SetForegroundWindow, and Windows allows it after a period with no input. In the first full run, 11 real keystrokes reached an idle bench window's pwsh. No command ran (no Enter in its history). That run was discarded. `KAZE_BENCH_IGNORE_INPUT=1` (set by the harness) now drops all real input. In the second run a 4-tab idle window closed early, probably by hand; idle was rerun cleanly.
2. **History.** The first full run wrote 3 bench `flood.exe` lines into the owner's real PSReadLine history (`%APPDATA%\...\ConsoleHost_history.txt`), because Store pwsh ignores the APPDATA override. I left them in place, since it is his file. Later runs use pwsh with `HistorySaveStyle SaveNothing`, and I confirmed the history did not grow.
3. Launch misses 500ms even with cmd.exe (~605ms). kaze's first frame alone is ~450ms and was not broken down further (window/GPU/fonts). Every run's first frame is also its slowest (115-385ms).
4. Key latency is dominated by pwsh/ConPTY echo. kaze's ~8ms share is frame pacing (frame build is 0.5ms). Drawing as soon as output arrives could cut it. On-screen latency is higher still: OS input and compositor are excluded.
5. Memory counts only kaze.exe. Each tab's pwsh and conhost are not included, and full history costs ~30MB per tab.
6. `kaze-app\kaze.exe` (pid 8588, running at the start) is no longer running. The harness never targets kaze.exe by name and only stops its own PIDs and their shells, so I don't believe I closed it, but I can't prove what did.
7. A pre-existing flake: `internal/term` TestOnChangeNeverRunsConcurrentlyAcrossExit timed out once under a parallel `./...` run and passed on rerun. I did not touch term.
8. macOS: darwin vet passes on all packages except internal/ui, cmd/kaze and spike/gio (they need cgo). run.sh is untested; on macOS `os.UserConfigDir` ignores XDG, so the owner's real config applies there.

## Cleanup
No kaze-bench or flood processes are left. Build outputs, logs and results.json are in C:\Users\john\Desktop\kaze-wt\bench\. Nothing in kaze-app was touched, and tabbar.go was not edited.

## Fix round 1

Commit `fa875cb` fix(bench): Drop real input by default and keep bench shells out of history (author patrick-gogo, no AI attribution). On top of e10e9a6. No kaze, bench runs or shells were launched this round; unit tests only, no re-measuring.

1. **run.sh safeguards (critical).** It writes a config.toml in the run's own config dir, both `$out/home/Library/Application Support/kaze` (macOS; HOME is pointed there for kaze, since Go's UserConfigDir ignores XDG on darwin) and `$out/config/kaze` (XDG). The config's shell is `/bin/sh -c "... HISTFILE=/dev/null exec zsh -f"`, falling back to `bash --noprofile --norc`. Every kaze also gets `HISTFILE=/dev/null`, and `env -u KAZE_BENCH_ALLOW_INPUT` so real input stays dropped. A bare binary can't be launched without activating it (`open -g` needs an .app), so the script says so and relies on the input drop. The dead HOME_CONFIG_NOTE is gone. Untested on macOS (`sh -n` only).
2. **Input drop is the default.** Whenever KAZE_BENCH is set, real key, pointer, paste and tab-bar input is dropped; `KAZE_BENCH_ALLOW_INPUT=1` opts out. `KAZE_BENCH_IGNORE_INPUT` is removed. The drop moved into `window.benchBatch`, and a window-level test proves real input is dropped while the synthetic key is still routed.
3. **run.ps1.** It launches with `ShowWindow = 4` (SW_SHOWNOACTIVATE). The defaults are derived from `$PSScriptRoot` (`<parent of checkout>\kaze-wt\bench` and `...\captures\claude-7.bin`), and GoTmp comes from `$env:GOTMPDIR` (empty keeps Go's default). It forces `KAZE_BENCH_ALLOW_INPUT` empty. The script parses with 0 errors.
4. **kaze's share.** Each pending key now keeps its output stamps (bounded at 64), and `last_out` uses the latest stamp no later than the echo frame's *start*. That output was parsed before the frame copied its rows. I could not filter Output by active tab at the callback (the manager's onChange has no tab), so the comment and the README now say the stamp is active-tab output or, rarely, a background tab's title or exit change. The milestone doc says its 8.4/24.1ms were measured under the old definition (last signal before frame end, which can only understate), and that this was not re-measured.
5. **Milestone doc.** It now names the phases (`after_type`, `scroll`) and gives drawn vs total for each (all drawn). It says that no frame built in over 16.7ms does not prove no refresh was dropped later. It replaces "out of reach whatever kaze does" with what was measured, and defines "ready prompt" as the first drawn frame with non-blank text. The run notes are updated.
6. **Truncation.** Each phase summary has `truncated` (frames > kept samples) and the keys summary has `truncated`. The key, draw, between and last lists are now capped at 200,000 too.
7. **flood.** It strips every OSC (BEL or ST ended; OSC 52 included) and every DCS string, plus the queries and mode switches.

### Tests (RED, then GREEN; all bounded with `-timeout`, waits have 3s deadlines)
- `internal/bench`: TestRealInputDroppedByDefault, TestLastOutputCountsOnlyOutputBeforeTheFrame, TestSummaryFlagsTruncatedSamples. The RED output had 3 failures: "bench mode should drop real input unless KAZE_BENCH_ALLOW_INPUT=1", "last_out_p50_ms = 0.5, want 2", and frames with no `truncated` key.
- `internal/ui`: TestBenchDropsRealInputButRoutesSyntheticKeys. RED was a build failure (benchBatch undefined). I also mutation-checked it: with the drop disabled it fails with `shell got "\rrm -rfpastedx", want "x"`.
- `bench/flood`: TestStripRemovesQueriesModesAndEveryOSC. RED was the broken regexp panicking at init; GREEN after the fix.

Commands and output:
```
sh kaze-wt/gt.sh ./internal/bench/ -count=1 -timeout 120s  ->  ok  internal/bench 4.168s
sh kaze-wt/gt.sh ./internal/ui/    -count=1 -timeout 120s  ->  ok  internal/ui    4.875s
sh kaze-wt/gt.sh ./bench/flood/    -count=1 -timeout 120s  ->  ok  bench/flood    3.012s
go vet ./...  -> clean;  GOOS=darwin CGO_ENABLED=0 go vet ./internal/bench ./bench/flood -> clean;  gofmt -l -> none
```
