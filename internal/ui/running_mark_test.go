package ui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Actual mark pixels: a solid core, a closed top and no paint beyond the slot.
// Opt-in with the render evidence suite, which needs a GPU-backed Gio context.
func TestRunningMarkPixels(t *testing.T) {
	if os.Getenv("KAZE_RUNNING_RENDER_DIR") == "" {
		t.Skip("set KAZE_RUNNING_RENDER_DIR for headless pixel verification")
	}
	for _, th := range theme.All() {
		for _, scale := range []float32{1, 2} {
			size := int(40 * scale)
			gpu, err := headless.NewWindow(size, size)
			if err != nil {
				t.Fatal(err)
			}
			g := pageContext(size, size)
			g.Metric = unit.Metric{PxPerDp: scale, PxPerSp: scale}
			c := newChrome()
			c.setTheme(th)
			paint.Fill(g.Ops, th.UI.Chrome)
			slot := image.Rect(g.Dp(12), g.Dp(12), g.Dp(26), g.Dp(26))
			c.stateDot(g, slot, markNone, true, false)
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rect(0, 0, size, size))
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			bg := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
			center := centre(slot)
			if color.NRGBAModel.Convert(img.At(center.X, center.Y)).(color.NRGBA) == bg {
				t.Errorf("%s %gx: running mark has a hollow center", th.Key, scale)
			}
			// The approved ring radius is .4 of the 14px slot; its top must be closed.
			if color.NRGBAModel.Convert(img.At(center.X, slot.Min.Y+int(scale))).(color.NRGBA) == bg {
				t.Errorf("%s %gx: ring top is open", th.Key, scale)
			}
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					if !(image.Pt(x, y).In(slot)) && color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) != bg {
						t.Errorf("%s %gx: running mark paints beyond slot", th.Key, scale)
						y = size
						break
					}
				}
			}
		}
	}
}

func TestActiveProjectHeaderPaint(t *testing.T) {
	if os.Getenv("KAZE_RUNNING_RENDER_DIR") == "" {
		t.Skip("set KAZE_RUNNING_RENDER_DIR for headless pixel verification")
	}
	c := newChrome()
	gpu, err := headless.NewWindow(260, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Release()
	capture := func(active string) *image.RGBA {
		g := pageContext(260, 40)
		paint.Fill(g.Ops, c.th.Chrome)
		c.sideGroupHeader(g, sideGroup{Key: "repo", Name: "Project", Count: 1}, sideState{activeRepo: active}, image.Rect(8, 6, 252, 34))
		if err := gpu.Frame(g.Ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 260, 40))
		if err := gpu.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	inactive := capture("")
	active := capture("repo")
	if active.RGBAAt(150, 12) == inactive.RGBAAt(150, 12) {
		t.Fatal("active repository header is not tinted")
	}
	unrelated := capture("different-repo")
	if !bytes.Equal(unrelated.Pix, inactive.Pix) {
		t.Fatal("another project highlighted this header")
	}
}
