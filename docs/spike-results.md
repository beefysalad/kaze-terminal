# Gio speed spike results (Task 0)

**Decision: neither mode passes. Per spec 10.1 this goes to Patrick: reopening the
language or toolkit is his call.** No mode meets the one-cell p99 < 4ms target, none
comes near the 40MB memory target, and none keeps up with `-stream`. Idle CPU is the
only criterion every mode passes.

If Gio is kept anyway, **text mode is the better path**: it is the only mode whose
one-cell frame stays inside a 60Hz frame (p99 7.3ms), and atlas mode is slower on every
frame-time measure.

## Setup

- Date: 2026-10-28. Windows 11, Intel i7-1255U, Intel Iris Xe (integrated), 1920x1080
  at 100% scale. Go 1.27.0, gioui.org v0.10.3 (D3D11 backend).
- Window 1500x1000dp; grid 200x60 cells of ASCII plus `日本語テスト` (about 1/3 of each
  line), Consolas 13sp with MS Gothic as CJK fallback.
- Code: `spike/gio/main.go`. Each line is recorded into its own cached op macro and only
  re-recorded when its content changes, in both modes.
- Frame time = QueryPerformanceCounter around layout + `FrameEvent.Frame` (op build and
  GPU submit). First 3 frames dropped. "late" = frames over 16.7ms.
- One-cell change: `-autotype` (one random cell in the ASCII prefix of a random line,
  every 50ms), 20s run.
- Stream: `-stream` (5 random lines rewritten every 2ms), 20s run.
- Memory: `Get-Process` `PrivateMemorySize64` after 10s idle (no flags).
- Idle CPU: `(Get-Process).CPU` sampled at 10s and 40s of an idle run (30s apart).

Measurement script (scratch, not committed) ran, per mode:
`spike-gio.exe -mode=<m> -duration=45s` (idle, memory, CPU), then
`-autotype -duration=20s -log=...`, then `-stream -duration=20s -log=...`.

## Results

| Measure | Target | text | atlas (clip, default) | atlas (sub) | empty window |
|---|---|---|---|---|---|
| One-cell frame p50 | | 4.56ms | 26.95ms | 14.97ms | |
| One-cell frame p99 | < 4ms | **7.32ms** FAIL | **48.66ms** FAIL | **24.26ms** FAIL | |
| One-cell late frames | | 0 / 389 | 386 / 386 | 77 / 390 | |
| One-cell layout-only p99 | | 1.53ms | 0.13ms | 0.05ms | |
| Stream frame p50 | | 93.55ms | 19.82ms | 16.55ms | |
| Stream frame p99 | < 16.7ms (R5) | **140.91ms** FAIL | **31.18ms** FAIL | **25.95ms** FAIL | |
| Stream late frames | | 191 / 191 | 899 / 904 | 498 / 1074 | |
| Stream layout-only p50 | | 52.10ms | 0.60ms | 0.29ms | |
| Private memory, 10s idle | < 40MB | **354.7MB** FAIL | **165.2MB** FAIL | **221.7MB** FAIL | 159.8MB |
| Working set, 10s idle | | 310.3MB | 152.6MB | 205.3MB | 148.6MB |
| Go heap in use at exit (idle run) | | 158.1MB | 24.9MB | 16.0MB | 72.1MB* |
| Idle CPU over 30s | ~0 | 0.000s PASS | 0.000s PASS | 0.000s PASS | |
| Visual tearing under `-stream` | none | to be checked by Patrick | to be checked by Patrick | to be checked by Patrick | |

\* The empty window still parses both fonts at startup; most of that 72MB is garbage
not yet collected.

Raw `-duration` output and per-frame CSVs were kept in the session scratchpad.

## What the numbers say

- **Memory is not reachable with Gio on this machine.** An empty Gio window with nothing
  drawn already uses 160MB private (about 70MB outside the Go heap: the D3D11 driver
  and window buffers on Iris Xe). That alone is 4x the 40MB target, whatever render path
  is chosen. Text mode adds the most: Gio's GPU vertex cache for glyph outlines (about
  25MB live) plus go-text's parsed MS Gothic glyph tables (about 15MB live).
- **Text mode:** one-cell frames are cheap to build (layout p99 1.5ms) but the submit
  side brings the total to p99 7.3ms. Under `-stream` every line changes with new content
  each frame, so every line is reshaped: about 52ms of shaping per frame, 94ms per frame
  in total, which is about 10fps.
- **Atlas mode:** building ops is nearly free (layout p99 under 0.2ms) but Gio's GPU
  renderer re-processes every one of the ~12,000 cell paint ops on every frame, even
  when only one cell changed, so even a one-cell change costs 15 to 27ms. The
  one-texture `clip` variant pays for a clip per cell; the `sub` variant avoids the clip
  but gives every glyph its own texture and is still over 16.7ms on a fifth of frames.
- **Idle CPU is fine** in every mode: Gio only redraws on invalidate, and the spike
  only invalidates on a change.

## Pass/fail per criterion (spec 11.3 targets)

| Criterion | text | atlas (clip) | atlas (sub) |
|---|---|---|---|
| One-cell p99 < 4ms | FAIL (7.32ms) | FAIL (48.66ms) | FAIL (24.26ms) |
| Memory < 40MB, one grid | FAIL (354.7MB) | FAIL (165.2MB) | FAIL (221.7MB) |
| Idle CPU ~0 | PASS | PASS | PASS |
| Stream p99 < 16.7ms (stand-in for tearing, R5) | FAIL (140.91ms) | FAIL (31.18ms) | FAIL (25.95ms) |
| Visual tearing | not checked (Patrick) | not checked (Patrick) | not checked (Patrick) |

## Caveats

- One machine, integrated GPU, laptop power plan as found. macOS numbers come at the
  first milestone run on the MacBook (spec 10.3).
- Frame time covers op building plus `Frame` submit on the CPU side. It does not
  include GPU completion or display latency.
- During one earlier text-mode idle run (discarded) the window received 123 frames
  and used 1.16s CPU in 30s, most likely from mouse or focus activity over the window;
  the recorded run had 1 frame and 0s CPU.
- Visual tearing was not judged (ruling R5): the stream p99 frame time is the proxy.
- No tuning beyond the brief was attempted (for example dirty-rect drawing, a custom
  GPU glyph renderer, `GOGC` or memory limits, or a smaller CJK font). The spike measures
  Gio as specified.
