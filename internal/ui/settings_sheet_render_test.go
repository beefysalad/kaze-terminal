package ui

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/gpu/headless"
	"github.com/beefysalad/kaze-terminal/internal/vt"
)

// Actual Gio frames only. No native windows, input injection, or network.
func TestSettingsSheetRenders(t *testing.T) {
	dir := os.Getenv("KAZE_SHEET_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_SHEET_RENDER_DIR for headless evidence")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"midnight", "washi"} {
		for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
			sections := append(defaultSections(), workspacesSection())
			for i, section := range sections {
				win := sheetWindow(t, key)
				if err := win.frame(pageContext(size.X, size.Y)); err != nil {
					t.Fatal(err)
				}
				tab := win.mgr.Active()
				tab.Session.Lock()
				parser := vt.NewParser(tab.Session.Applier())
				parser.Feed([]byte("\r$ go test ./internal/ui/\r\nok  kaze/internal/ui\r\n$ git status\r\nOn branch feat/settings-sheet\r\n"))
				for n := 0; n < 36; n++ {
					parser.Feed([]byte(fmt.Sprintf("ok  kaze/internal/ui/check-%02d\r\n", n)))
				}
				tab.Session.Unlock()
				if i == len(sections)-1 {
					win.selectSection(wsIndex(t))
					win.activate("ws.new", 0)
				} else {
					win.selectSection(i)
					if section.title == "Workspaces" {
						f := newWSPage(t)
						win.wsp.doc = f.win.wsp.doc
					}
				}
				g := pageContext(size.X, size.Y)
				if err := win.frame(g); err != nil {
					t.Fatal(err)
				}
				gpu, err := headless.NewWindow(size.X, size.Y)
				if err != nil {
					t.Fatal(err)
				}
				if err = gpu.Frame(g.Ops); err != nil {
					gpu.Release()
					t.Fatal(err)
				}
				img := image.NewRGBA(image.Rectangle{Max: size})
				err = gpu.Screenshot(img)
				gpu.Release()
				if err != nil {
					t.Fatal(err)
				}
				if size.X >= 900 {
					alpha := uint8(140)
					if key == "washi" {
						alpha = 64
					}
					// Gio blends paint in linear light, then encodes the screenshot as sRGB.
					dim := func(c uint8) uint8 {
						v := float64(c) / 255
						if v <= 0.04045 {
							v /= 12.92
						} else {
							v = math.Pow((v+0.055)/1.055, 2.4)
						}
						v *= 1 - float64(alpha)/255
						if v <= 0.0031308 {
							v *= 12.92
						} else {
							v = 1.055*math.Pow(v, 1/2.4) - 0.055
						}
						return uint8(math.Round(v * 255))
					}
					want := win.ui.th.Chrome
					want.R, want.G, want.B = dim(want.R), dim(want.G), dim(want.B)
					got := img.RGBAAt(20, 500)
					distance := func(a, b uint8) int {
						if a > b {
							return int(a - b)
						}
						return int(b - a)
					}
					if distance(got.R, want.R) > 2 || distance(got.G, want.G) > 2 || distance(got.B, want.B) > 2 {
						t.Fatalf("%s backdrop pixel %v, want %v", key, got, want)
					}
					corner := img.RGBAAt(win.set.sheet.Min.X, win.set.sheet.Min.Y)
					bg := win.ui.pageColors().bg
					if distance(corner.R, bg.R)+distance(corner.G, bg.G)+distance(corner.B, bg.B) < 5 {
						t.Fatal("sheet corner is opaque and square")
					}
				}
				suffix := "wide"
				if size.X < 900 {
					suffix = "narrow"
				}
				title := strings.ToLower(section.title)
				if i == len(sections)-1 {
					title += "-editor"
				}
				name := key + "-" + suffix + "-" + title + ".png"
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, img)
				closeErr := f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		}
	}
}
