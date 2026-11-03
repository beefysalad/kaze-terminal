package render

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emojiSample is an emoji-heavy terminal sample: a Claude Code style status
// table, text-default marks and Claude's ✳ spinner beside Japanese text.
var emojiSample = strings.Join([]string{
	"\x1b[38;2;215;119;87m✳\x1b[0m Thinking… \x1b[38;2;153;153;153m(12s · esc to interrupt)\x1b[0m",
	"⏺ Updated 3 files. ✔ tests pass  ⚠ 1 warning  ❤ thanks!",
	"",
	"| #    | Task                     | Status     |",
	"|------|--------------------------|------------|",
	"| 0041 | Shared 3D Secure helper  | ✅ Done    |",
	"| 0042 | Retry webhook delivery   | 🔄 Running |",
	"| 0043 | Tenant scoping audit     | ⏳ Waiting |",
	"| 0044 | Refund flow tests        | ❌ Failed  |",
	"| 0045 | Cache warmup on deploy   | 🚀 Shipped |",
	"| 0046 | 日本語の説明を追加       | 📝 Draft   |",
	"",
	"🟢 api  🟡 web  🔴 cli   🐛 2 bugs  🔥 hot  💡 idea  📦 0.25  🧪 92%  🔧 ✨ 🎉",
}, "\r\n")

// Opt-in development capture of the emoji sample as kaze's renderer draws it
// (CPU strips, no GPU or window). Not a product screenshot.
func TestEmojiRenders(t *testing.T) {
	dir := os.Getenv("KAZE_EMOJI_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_EMOJI_RENDER_DIR for a capture of the emoji sample")
	}
	if e := os.MkdirAll(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	h := newHarness(t, 80, 14)
	h.r = NewRenderer(16.125, DefaultPalette(), Options{}) // a size no other test uses
	h.out(emojiSample)
	h.frame()
	// out returns on the first change; draw again once everything has landed.
	h.out("\x1b[?25l")
	h.frame()
	w := 80 * h.r.cellW
	img := image.NewRGBA(image.Rect(0, 0, w, 14*h.r.cellH))
	for _, s := range h.r.strips {
		if s.img == nil {
			continue
		}
		at := image.Pt(0, s.row0*h.r.cellH)
		draw.Draw(img, s.img.Bounds().Add(at), s.img, image.Point{}, draw.Src)
	}
	f, e := os.Create(filepath.Join(dir, "emoji-sample.png"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
}
