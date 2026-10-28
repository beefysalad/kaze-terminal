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

Every number below comes from a single run per mode and measure.

Measurement script (scratch, not committed) ran, per mode:
`spike-gio.exe -mode=<m> -duration=45s` (idle, memory, CPU), then
`-autotype -duration=20s -log=...`, then `-stream -duration=20s -log=...`.

## Results

| Measure | Target | text | atlas (clip, default) | atlas (sub) | empty window (fonts parsed) |
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
| Go HeapAlloc at exit, no GC (includes garbage) | | 158.1MB | 24.9MB | 16.0MB | 72.1MB |
| Idle CPU over 30s | ~0 | 0.000s PASS | 0.000s PASS | 0.000s PASS | |
| Visual tearing under `-stream` | none | to be checked by Patrick | to be checked by Patrick | to be checked by Patrick | |

The Go heap row above was read without a forced GC, so it counts garbage as well as
live memory. The spike now runs `runtime.GC()` before reading it; the corrected numbers
are in the review-fix section below.

Raw `-duration` output and per-frame CSVs were kept in the session scratchpad.

## What the numbers say

- **Memory is not reachable with Gio on this machine.** The "empty" window above still
  parses both fonts, so it is not a true baseline. A bare Gio window (no fonts, nothing
  drawn; measured later, see the review-fix section) uses 83.6MB private, of which the Go
  runtime holds only about 13MB, so roughly 70MB sits outside the Go runtime (the
  graphics driver, D3D11 and the window; not attributed further). That alone is 2x the
  40MB target, whatever render path is chosen. Text mode adds the most memory; which
  parts of it (glyph caches, parsed font tables) was not measured.
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
- Frame time covers op building plus `FrameEvent.Frame`, which runs Gio's GPU renderer
  (including texture uploads) and `Present` with vsync (sync interval 1). `Present` can
  block when the GPU queue is full, so a slow GPU shows up in frame time. It does not
  include GPU completion or scan-out. There is no vblank floor inside the measured span:
  a bare window under `-stream` (nothing drawn) measures p50 0.10ms, p99 0.26ms at 60fps,
  because Gio waits for vsync before delivering the next `FrameEvent`, not inside `Frame`.
  So frames over 16.7ms are real work, not vsync waiting.
- During one earlier text-mode idle run (discarded) the window received 123 frames
  and used 1.16s CPU in 30s, most likely from mouse or focus activity over the window;
  the recorded run had 1 frame and 0s CPU.
- Visual tearing was not judged (ruling R5): the stream p99 frame time is the proxy.
- No tuning beyond the brief was attempted (for example dirty-rect drawing, a custom
  GPU glyph renderer, `GOGC` or memory limits, or a smaller CJK font). The spike measures
  Gio as specified.

## Task 0 review fixes

Two review findings on the numbers above, fixed in the spike and re-measured on
2026-10-28 (same machine, single runs).

1. **Go heap without GC.** `HeapAlloc` and the heap profile were read without
   `runtime.GC()`, so they included garbage. The spike now forces a GC first. The "empty"
   mode also parsed both fonts. A new `-mode=bare` skips font loading and draws nothing.

   | Mode | Private, 10s idle | Working set | Go HeapAlloc after GC | Go HeapSys | Go Sys |
   |---|---|---|---|---|---|
   | bare (no fonts) | 83.6MB | 72.2MB | 0.9MB | 8.1MB | 13.1MB |
   | empty (fonts parsed) | 159.9MB | 148.6MB | 0.9MB | 83.7MB | 91.3MB |
   | text | 347.2MB | 305.7MB | 56.1MB | 213.6MB | 222.3MB |
   | atlas (clip) | 161.1MB | 150.4MB | 16.1MB | 83.6MB | 91.6MB |
   | atlas (sub) | 230.6MB | 191.6MB | 12.5MB | 87.8MB | 95.8MB |

   Parsing the fonts leaves almost no live heap but about 75MB of heap the Go runtime has
   not returned to the OS (empty minus bare). The earlier per-component attributions
   (GPU vertex cache, MS Gothic tables) were not backed by a committed profile and are
   withdrawn.

2. **Frame time and vsync.** `Frame` includes `Present`, so the scope note in the caveats
   was wrong and is corrected above. The Present floor was measured with `-mode=bare
   -stream`: frame p50 0.095ms, p99 0.255ms, max 2.7ms, 1158 frames in 20s (59.9fps). The
   floor is near zero, so stream frame times are not inflated by vsync, and the 16.7ms
   threshold measures real work.
