# kaze benchmarks

`run.ps1` (Windows) and `run.sh` (macOS) build kaze and the flood tool into an
output folder, run each scenario in a fresh kaze, and collect the numbers.
Timings come from kaze's own bench mode; memory and CPU are read from the kaze
process by the harness. Nothing in the harness sends keyboard or mouse input to
any window.

```
pwsh -File bench/run.ps1 [-Out <dir>] [-Capture <claude capture>] [-Only keys,flood]
sh bench/run.sh [out-dir] [claude-capture]
```

Scenarios: `launch` (5 runs with the default shell, 3 with cmd.exe), `keys`
(200 synthetic keys), `flood` (10,100 colored lines, then a scroll through
10,000 lines of history), `claude` (the Claude Code capture plus a streamed
response), `idle` (1 and 4 idle tabs: memory, CPU, growth over 60s) and `full`
(1 and 4 tabs with full history: per-tab cost).

## Bench mode (`KAZE_BENCH`)

With `KAZE_BENCH` unset kaze builds no recorder; every hook is behind one nil
check. With `KAZE_BENCH=1` it writes JSON lines to `KAZE_BENCH_LOG`
(default `%TEMP%\kaze-bench.log`):

| variable | meaning | bounds |
|---|---|---|
| `KAZE_BENCH_SECONDS` | the window closes itself after this long; the process exits 15s later if it hasn't | 1-600, default 60 |
| `KAZE_BENCH_KEYS` | synthetic keys to type ("x", Backspace, ...) through the same routing as real keys | 0-2000 |
| `KAZE_BENCH_PACE_MS` | time between synthetic keys | 10-1000, default 50 |
| `KAZE_BENCH_TABS` | tabs to open at start | 1-16 |
| `KAZE_BENCH_TYPE` | a command written once into every tab, 2s after the first prompt | 1 KB |
| `KAZE_BENCH_SCROLL` | once history holds this many lines and has been still for 1.5s, scroll to the top a page per frame through the wheel path | 0-1,000,000 |
| `KAZE_BENCH_ALLOW_INPUT` | real keyboard, mouse, paste and tab-bar input is dropped whenever `KAZE_BENCH` is set; `1` lets it through, for manual debugging only | |

Log lines: `config`, `launch` (process creation to the first frame with text,
plus the first frame and first shell output), `typed`, `scroll_start`,
`screen` (the active tab's text at the end) and `summary` (frames drawn, late
frames over 16.7ms, rows rasterized, frame time percentiles; key latency;
scroll; private memory). `truncated` is true when a phase or the key list hit
the 200,000-sample cap; counts keep going, percentiles cover the first 200,000.

### What the numbers mean

- **Frame time** is from kaze receiving the frame event to `Frame` returning
  (building the frame and handing it to the GPU). A frame time over 16.7ms means
  kaze could not have made that refresh; frame times under it do not prove no
  refresh was dropped later (in the GPU or compositor), and nothing here sees
  those.
- **Key latency** is from the key being routed to the shell to the end of the
  first drawn frame whose cursor moved. It includes the shell echoing the key
  through ConPTY or the pty. `last_out_*` is from the last change signal kaze
  had before that frame's event (output parsed from the active tab, or rarely a
  background tab's title or exit) to the end of the frame: that output was
  already in the screen when the frame copied it, so this is mostly kaze's own
  share, waiting for the frame plus drawing it. `frames_between` counts frames
  drawn after the echo started arriving but before the cursor moved (the shell
  sent it in pieces).
- **Not included:** OS input delivery (keyboard to the window's message queue)
  and compositor latency (the frame being shown on the display). A camera or
  hardware probe is needed for those.
- **Launch** starts at process creation (`GetProcessTimes` on Windows, package
  init elsewhere, which misses a few ms of loading).

## Safety

- Bench mode drops all real input unless `KAZE_BENCH_ALLOW_INPUT=1`, so typing
  into a bench window that took focus reaches no shell. Both harnesses make sure
  that variable is off.
- On Windows the harness starts kaze through WMI with `SW_SHOWNOACTIVATE`, so
  kaze isn't in the foreground app's process tree and usually can't take focus;
  when it does anyway, the run records `took_foreground`. On macOS a bare binary
  can't be started without activating it, so the input drop is the safeguard.
- Bench shells keep no history: pwsh with history saving off on Windows (the
  Store pwsh ignores the APPDATA override), and `zsh -f` or `bash --norc` with
  `HISTFILE=/dev/null` on macOS, set through a config file in the run's own
  config dir.
- `flood -claude` removes queries, input-mode switches and every OSC and DCS
  string (titles, OSC 52 clipboard writes) from the capture before replaying it.
- Every run has a deadline; the harness stops only the processes it started, by
  PID, including shells and flood left behind.
- `flood` caps lines at 1,000,000, replays at most 300s and reads at most 16 MB
  of capture.
