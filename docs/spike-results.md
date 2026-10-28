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

# Task 0b: one CPU-drawn grid image

**Decision: `-mode=image` with one `ImageOp` for the whole grid is the fastest method and
Task 10 uses it.** It meets both frame goals with a wide margin: stream p99 3.51ms (goal
16.7ms), one-cell p99 2.19ms (goal 10ms), idle CPU 0. The 8-strip variant also meets
both goals and is faster for one-cell changes (p99 1.02ms) but a little slower under
stream (p99 4.68ms), so by the brief's order (stream p99 first) the single image wins.
Memory is reported, not judged.

## What `-mode=image` does

- Glyphs are rasterized once with the atlas code, then composited onto the background
  into opaque cell-sized images (two cells wide for CJK): the glyph cache.
- One `image.RGBA` holds the whole grid (1400x960px, 5.4MB here). Each frame, rows whose
  version changed are compared cell by cell with what the image shows, and only the
  changed cells are copied in (`draw.Src`, a plain row copy).
- Gio gets the grid as `paint.ImageOp`s with `FilterNearest`. Gio caches GPU textures by
  the op's handle and has no partial-update API, so a changed image needs a new
  `ImageOp`, which means a new texture and a **full upload** of that image.
  - `-strips=1`: one op for the whole grid; every change uploads 5.4MB.
  - `-strips=8`: 8 strips of 8 rows (the last has 4), one op each; only changed strips
    get a new op. A one-cell change uploads one strip (about 0.7MB).
- Checked visually that the grid renders correctly in both variants (screenshot during
  `-stream`; not committed).

## Results (2026-10-28, same machine and setup as Task 0, single runs)

All modes were re-measured in this session so the numbers are comparable. Go heap is
read after a forced GC. "fps" is frames per second achieved over the run; one-cell
changes arrive every 50ms, so 20fps is the full rate there and 60fps under stream.

| Measure | Goal | text | atlas (clip) | atlas (sub) | **image (1 op)** | image (8 strips) | bare window |
|---|---|---|---|---|---|---|---|
| One-cell frame p50 | | 4.50ms | 24.69ms | 21.90ms | 1.22ms | 0.49ms | |
| One-cell frame p99 | < 10ms | 6.70ms PASS | 41.85ms FAIL | 27.43ms FAIL | **2.19ms PASS** | 1.02ms PASS | |
| One-cell late frames (>16.7ms) | | 0 / 387 | 386 / 390 | 369 / 388 | 0 / 386 | 0 / 389 | |
| One-cell layout-only p99 | | 1.10ms | 0.08ms | 0.05ms | 0.03ms | 0.04ms | |
| Stream frame p50 | | 45.44ms | 26.42ms | 19.62ms | 2.66ms | 3.29ms | 0.10ms |
| Stream frame p99 | < 16.7ms | 80.81ms FAIL | 46.93ms FAIL | 32.22ms FAIL | **3.51ms PASS** | 4.68ms PASS | 0.26ms |
| Stream late frames (>16.7ms) | | 381 / 381 | 696 / 696 | 729 / 890 | 0 / 1167 | 0 / 1168 | 0 / 1158 |
| Stream frames over 33.4ms | | 365 | 40 | 6 | 0 | 0 | 0 |
| Stream fps achieved | 60 | 19.9 | 36.3 | 45.8 | 60.0 | 60.0 | 59.9 |
| Stream layout-only p99 (incl. CPU drawing) | | 52.76ms | 1.90ms | 0.72ms | 1.10ms | 1.13ms | 0.03ms |
| Stream submit-only p50 (Frame, incl. upload) | | 18.28ms | 25.60ms | 19.27ms | 1.88ms | 2.51ms | 0.09ms |
| Uploaded under stream (20s) | | | | | 6290MB (1170 x 5.4MB) | 6261MB (9291 strips) | |
| Private memory, 10s idle | reported | 347.2MB | 161.1MB | 230.6MB | 179.1MB | 180.3MB | 83.6MB |
| Go HeapAlloc after GC | | 56.1MB | 16.1MB | 12.5MB | 15.9MB | 15.9MB | 0.9MB |
| Idle CPU over 30s | ~0 | see note | 0.05s | 0.00s | 0.00s PASS | 0.00s PASS | 0.00s |
| Visual tearing under `-stream` | none | to be checked by Patrick | same | same | same | same | |

Notes:

- The Present floor (bare window, nothing drawn, `-stream`) is 0.10ms p50: Gio waits for
  vsync before delivering a frame, not inside `Frame`, so these frame times are work, not
  vsync waiting (see the Task 0 review fixes above).
- Under stream, nearly every frame changes rows in all 8 strips, so the strip variant
  uploads about the same bytes as the single image but in 8 textures, which costs a little
  more per frame. Strips only pay off when changes are local (one-cell).
- Idle CPU: text mode received 198 frames during one idle run (4.1s CPU) and 2 frames
  (0.89s CPU) on a re-run; image (1 op) had one idle run with 1.1s CPU over 30s and two
  re-runs at 0.00s. These look like mouse or focus activity over the window, as in
  Task 0, not steady-state cost. The image and bare numbers in the table are the 0.00s
  re-runs.
- Uploading a new 5.4MB texture per changed frame is the image mode's main cost (submit
  p50 1.9ms). Larger windows or high-DPI screens scale it with pixel count; the strip
  variant is the fallback if that becomes a problem.

## Decision for Task 10

| Goal | Met by image (1 op)? |
|---|---|
| Stream frame p99 < 16.7ms | Yes, 3.51ms |
| One-cell frame p99 < 10ms | Yes, 2.19ms |
| Idle CPU ~0 | Yes, 0.00s over 30s |
| Memory | No cap; 179.1MB private (bare Gio window alone is 83.6MB) |
| Visual tearing | To be checked by Patrick |

Winner: **image, one `ImageOp` for the whole grid.** Order by stream p99: image (1 op)
3.51ms, image (8 strips) 4.68ms, atlas (sub) 32.22ms, atlas (clip) 46.93ms, text
80.81ms. Text, atlas (clip) and atlas (sub) miss the stream goal; atlas (clip) and atlas
(sub) also miss the one-cell goal.
