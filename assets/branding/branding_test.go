package branding

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// The folded terminal K uses the enhanced violet gradient.
var (
	oldLilac = color.NRGBA{R: 0xc2, G: 0xb4, B: 0xeb, A: 0xff}
	violet   = []color.NRGBA{{R: 0x8b, G: 0x7b, B: 0xff, A: 0xff}, {R: 0xb9, G: 0xa6, B: 0xff, A: 0xff}}
)

func near(a, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// Check transparency, the solid tile and the violet gradient in every export.
func checkIcon(t *testing.T, name string, img image.Image) {
	t.Helper()
	b := img.Bounds()
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) }
	if c := at(b.Min.X, b.Min.Y); c.A != 0 {
		t.Errorf("%s %v: corner is not transparent: %v", name, b.Size(), c)
	}
	if c := at(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()*15/100); c.A < 250 {
		t.Errorf("%s %v: tile is not solid near the top: %v", name, b.Size(), c)
	}
	lil, old := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := at(x, y)
			if c.A < 250 {
				continue
			}
			if near(c, oldLilac, 6) {
				lil++
			}
			for _, o := range violet {
				if near(c, o, 10) {
					old++
				}
			}
		}
	}
	if lil > 0 {
		t.Errorf("%s %v: %d obsolete lilac pixels", name, b.Size(), lil)
	}
	if old < b.Dx()*b.Dy()/200 {
		t.Errorf("%s %v: only %d violet pixels", name, b.Size(), old)
	}
}

func TestWindowsIconIsTheApprovedMark(t *testing.T) {
	b, err := os.ReadFile("kaze-icon-512.png")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 512 {
		t.Fatalf("size %v", img.Bounds())
	}
	checkIcon(t, "kaze-icon-512.png", img)
}

// Every PNG size in the macOS icon set is the new artwork.
func TestMacIconSetIsTheApprovedMark(t *testing.T) {
	b, err := os.ReadFile("kaze.icns")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 8 || string(b[:4]) != "icns" {
		t.Fatal("not an icns file")
	}
	want := map[string]int{"ic07": 128, "ic08": 256, "ic09": 512, "ic10": 1024}
	for off := 8; off+8 <= len(b); {
		typ, n := string(b[off:off+4]), int(binary.BigEndian.Uint32(b[off+4:off+8]))
		if n < 8 || off+n > len(b) {
			t.Fatalf("bad entry %q at %d", typ, off)
		}
		if px, ok := want[typ]; ok {
			img, err := png.Decode(bytes.NewReader(b[off+8 : off+n]))
			if err != nil {
				t.Fatalf("%s: %v", typ, err)
			}
			if img.Bounds().Dx() != px {
				t.Errorf("%s is %v, want %d", typ, img.Bounds().Size(), px)
			}
			checkIcon(t, "kaze.icns "+typ, img)
			delete(want, typ)
		}
		off += n
	}
	if len(want) > 0 {
		t.Errorf("icns is missing %v", want)
	}
}

// The icon SVGs (the app's and the website's copy) draw the approved mark.
func TestIconSVGsUseTheApprovedMark(t *testing.T) {
	for _, p := range []string{"kaze-icon.svg", "kaze-mark.svg", "../../docs/site-assets/kaze-icon.svg", "kaze-logo.svg", "kaze-lockup.svg", "../../docs/site-assets/kaze-lockup.svg"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ToLower(string(b))
		if strings.Contains(s, "#c2b4eb") || !strings.Contains(s, "#8b7bff") || !strings.Contains(s, "#b9a6ff") || !strings.Contains(s, "m114 100h139") {
			t.Errorf("%s is not the approved folded terminal K", p)
		}
	}
}
