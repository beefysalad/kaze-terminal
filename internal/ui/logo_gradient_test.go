package ui

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/op"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// The real Gio draw must keep the violet gradient and the separate terminal
// underscore, rather than filling the new paths with the old flat lilac.
func TestMarkDrawsGradientAndTerminalUnderscore(t *testing.T) {
	for _, col := range []color.NRGBA{aboutMarkDark, aboutMarkLight} {
		ops := new(op.Ops)
		lockupShapes().drawMark(ops, image.Rect(10, 10, 298, 234), 224, col)
		gpu, err := headless.NewWindow(310, 250)
		if err != nil {
			t.Fatal(err)
		}
		if err = gpu.Frame(ops); err != nil {
			gpu.Release()
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 310, 250))
		err = gpu.Screenshot(img)
		gpu.Release()
		if err != nil {
			t.Fatal(err)
		}
		top, bottom := img.RGBAAt(45, 45), img.RGBAAt(45, 200)
		if top.A < 250 || bottom.A < 250 || int(top.R)-int(bottom.R) < 8 {
			t.Errorf("mark lost its gradient: top %v, bottom %v", top, bottom)
		}
		if img.RGBAAt(270, 222).A < 250 {
			t.Error("terminal underscore is missing")
		}
		if img.RGBAAt(228, 222).A != 0 {
			t.Error("underscore merged into the K")
		}
	}
}

// Both ends of the gradient remain readable on every page, navigation and
// title-bar background where the mark appears.
func TestMarkGradientContrastAcrossThemes(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		page := c.pageColors()
		for _, bg := range []color.NRGBA{page.bg, page.nav, th.UI.Chrome, mix(th.UI.Chrome, th.Term.BG, 0.45)} {
			start, end := aboutMarkDark, aboutMarkDarkEnd
			if isLight(bg) {
				start, end = aboutMarkLight, aboutMarkLightEnd
			}
			for _, fg := range []color.NRGBA{start, end} {
				if ratio := contrast(fg, bg); ratio < 3 {
					t.Errorf("%s logo contrast %.2f for %v on %v", th.Key, ratio, fg, bg)
				}
			}
		}
	}
}
