# Gio speed spike

Throwaway measurement for Task 0 and Task 0b (which Gio drawing method kaze uses). Not product code. Results are in
`docs/spike-results.md`.

A Gio window draws a 200x60 grid of mixed ASCII and `日本語テスト`, monospace font (Consolas)
with a CJK fallback (MS Gothic). Each line is recorded once into its own op macro and only
re-recorded when the line changes.

## Modes

- `-mode=text`: one `widget.Label` per line through Gio's `text.Shaper`.
- `-mode=atlas`: every glyph is rasterized once (golang.org/x/image) into one `image.RGBA`
  atlas; cells are drawn as `paint.ImageOp`s.
  - `-atlasdraw=clip` (default): one atlas texture, offset and clipped to each cell.
  - `-atlasdraw=sub`: one sub-image `ImageOp` per glyph (Gio uploads each as its own texture).
- `-mode=image`: the whole grid is one CPU-drawn `image.RGBA`. Glyphs are rasterized once
  (atlas code) into opaque cell-sized images; only changed cells are copied into the grid
  image, and Gio gets the grid as `paint.ImageOp`s.
  - `-strips=1` (default): one `ImageOp` for the whole grid, re-created on every change.
  - `-strips=8`: 8 horizontal strips, one `ImageOp` each; only changed strips get a new op.
  A new `ImageOp` means a new GPU texture and a full upload of that image (Gio caches
  textures by the op's handle).
- `-mode=empty`: same window, fonts parsed, background only.
- `-mode=bare`: same window, no fonts parsed, background only. Baseline for memory and,
  with `-stream`, for the Present (vsync) floor.

## Flags

| Flag | Meaning |
|---|---|
| `-autotype` | change one cell every 50ms (stands in for keystrokes; real keys also change one cell) |
| `-stream` | rewrite 5 random lines every 2ms |
| `-duration 20s` | exit after this long and print frame stats (p50/p99/max, frames over 16.7ms) |
| `-log file.csv` | write per-frame times |
| `-memprofile file` | write a Go heap profile on exit |
| `-mono`, `-cjk` | font files (defaults: `C:\Windows\Fonts\consola.ttf`, `C:\Windows\Fonts\msgothic.ttc`) |

Frame time is measured with QueryPerformanceCounter around layout plus `FrameEvent.Frame`
(Go's `time.Now` on Windows is too coarse for sub-millisecond times). The first 3 frames
are dropped as startup. The output also prints submit-only time (frame minus layout),
which includes `Present`; Gio presents with vsync, so back-to-back frames block on vblank.
Go heap numbers are read after a forced `runtime.GC()`.

## Run

```bash
go build -o spike-gio.exe ./spike/gio   # do not commit the exe
./spike-gio.exe -mode=text -autotype -duration=20s
./spike-gio.exe -mode=atlas -stream -duration=20s
```

On Windows with Smart App Control on, a fresh unsigned build is sometimes blocked
("An Application Control policy has blocked this file"). Rebuilding (for example with
`-ldflags="-s -w"`) produces a binary that is usually allowed.
