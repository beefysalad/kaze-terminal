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
| `KAZE_BENCH_IGNORE_INPUT` | `1` drops real keyboard, mouse and paste input (the harness sets it) | |

Log lines: `config`, `launch` (process creation to the first frame with text,
plus the first frame and first shell output), `typed`, `scroll_start`,
`screen` (the active tab's text at the end) and `summary` (frames drawn, late
frames over 16.7ms, rows rasterized, frame time percentiles; key latency;
scroll; private memory).

### What the numbers mean

- **Frame time** is from kaze receiving the frame event to `Frame` returning
  (building the frame and handing it to the GPU). Gio presents with vsync, so a
  frame time above 16.7ms means a refresh was missed.
- **Key latency** is from the key being routed to the shell to the end of the
  first drawn frame whose cursor moved. It includes the shell echoing the key
  through ConPTY or the pty. `last_out_*` is the part kaze owns: from the last
  shell output before that frame to the frame. `frames_between` counts frames
  drawn after the echo started arriving but before the cursor moved (the shell
  sent it in pieces).
- **Not included:** OS input delivery (keyboard to the window's message queue)
  and compositor latency (the frame being shown on the display). A camera or
  hardware probe is needed for those.
- **Launch** starts at process creation (`GetProcessTimes` on Windows, package
  init elsewhere, which misses a few ms of loading).

## Safety

- The harness starts kaze through WMI on Windows, so kaze isn't in the
  foreground app's process tree and usually can't take focus. When it does, the
  run notes `took_foreground` and `KAZE_BENCH_IGNORE_INPUT` keeps any typing out
  of its shells.
- Runs that type a command use pwsh with history saving off, so bench commands
  don't land in your PowerShell history.
- Every run has a deadline; the harness stops only the processes it started, by
  PID, including shells and flood left behind.
- `flood` caps lines at 1,000,000, replays at most 300s and reads at most 16 MB
  of capture.
