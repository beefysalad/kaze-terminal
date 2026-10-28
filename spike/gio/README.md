# Gio speed spike

Throwaway measurement for Task 0 (go/no-go on Gio). Not product code. Results are in
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
- `-mode=empty`: same window, background only. Baseline for memory.

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
are dropped as startup.

## Run

```bash
go build -o spike-gio.exe ./spike/gio   # do not commit the exe
./spike-gio.exe -mode=text -autotype -duration=20s
./spike-gio.exe -mode=atlas -stream -duration=20s
```

On Windows with Smart App Control on, a fresh unsigned build is sometimes blocked
("An Application Control policy has blocked this file"). Rebuilding (for example with
`-ldflags="-s -w"`) produces a binary that is usually allowed.
