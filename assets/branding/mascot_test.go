package branding

import (
	"bytes"
	"image/png"
	"testing"
)

// The Claude Code mascot as the terminal prints it (half-block art): 18 sub-pixels
// wide, each sub-pixel one unit wide and two tall, so the body is a wide block with
// arms, two single-pixel eyes in its top row and four short feet.
var mascot = []string{
	"...############...",
	"...##.######.##...",
	".################.",
	"...############...",
	"....#.#....#.#....",
}

func TestClaudeLogoIsTheClaudeCodeMascot(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(ClaudeLogo))
	if err != nil {
		t.Fatal(err)
	}
	const unitW, unitH = 5, 10
	b := img.Bounds()
	offX, offY := (b.Dx()-18*unitW)/2, (b.Dy()-len(mascot)*unitH)/2
	for y, row := range mascot {
		for x, ch := range row {
			_, _, _, a := img.At(offX+x*unitW+unitW/2, offY+y*unitH+unitH/2).RGBA()
			if (ch == '#') != (a > 0x8000) {
				t.Errorf("sub-pixel (%d,%d): want %q, alpha %#x", x, y, ch, a)
			}
		}
	}
	// Nothing is drawn outside the art, and the art is not blurred at its edges.
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			inside := x >= offX && x < offX+18*unitW && y >= offY && y < offY+len(mascot)*unitH
			if _, _, _, a := img.At(x, y).RGBA(); !inside && a != 0 {
				t.Fatalf("stray pixel at (%d,%d)", x, y)
			}
		}
	}
}
