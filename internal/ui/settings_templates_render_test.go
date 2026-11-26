package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
)

// Render real Gio operations without opening a native window or injecting input.
func TestTemplateSettingsRenders(t *testing.T) {
	dir := os.Getenv("KAZE_TEMPLATE_SETTINGS_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_TEMPLATE_SETTINGS_RENDER_DIR for PNG evidence")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"midnight", "washi"} {
		for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
			for _, agent := range []string{"templates"} {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", key, size.X, size.Y, agent), func(t *testing.T) {
					win := sheetWindow(t, key)
					root := filepath.Dir(win.cfgPath)
					win.home = root
					win.claudePath = filepath.Join(root, ".claude", "settings.json")
					win.codexDir = filepath.Join(root, ".codex")
					for _, p := range []string{win.claudePath, filepath.Join(win.codexDir, "hooks.json")} {
						if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
							t.Fatal(e)
						}
						if e := os.WriteFile(p, []byte("{}"), 0600); e != nil {
							t.Fatal(e)
						}
					}
					installTestTemplate(t, win, "review", "---\nagent: codex\ndescription: Review changes and check the tests.\nmode: read-only\n---\nReview the diff.")
					installTestTemplate(t, win, "writer", "---\nagent: claude\ndescription: Update the project documentation.\n---\nWrite docs.")
					win.selectSection(claudeIndex(t))
					id := "templates_open"
					// Bring the relevant setup group into view in the shared Agents page.
					for i := 0; i < 4; i++ {
						g := pageContext(size.X, size.Y)
						if e := win.frame(g); e != nil {
							t.Fatal(e)
						}
						controls := controlsOf(win, id)
						if len(controls) != 1 {
							t.Fatalf("missing %s", id)
						}
						c := controls[0]
						if c.rect.In(win.set.pane) {
							break
						}
						offset := 75
						if size.X < 900 {
							offset = 180
						}
						win.set.scroll += c.rect.Min.Y - win.set.pane.Min.Y - offset
					}
					g := pageContext(size.X, size.Y)
					if e := win.frame(g); e != nil {
						t.Fatal(e)
					}
					c := controlsOf(win, id)
					if len(c) != 1 || !c[0].rect.In(win.set.pane) {
						t.Fatal("switch outside visible pane")
					}
					if !pageHas(win, "review") || !pageHas(win, "writer") {
						t.Fatal("missing help")
					}
					gpu, e := headless.NewWindow(size.X, size.Y)
					if e != nil {
						t.Fatal(e)
					}
					defer gpu.Release()
					if e = gpu.Frame(g.Ops); e != nil {
						t.Fatal(e)
					}
					img := image.NewRGBA(image.Rectangle{Max: size})
					if e = gpu.Screenshot(img); e != nil {
						t.Fatal(e)
					}
					name := fmt.Sprintf("%s-%s-%dx%d.png", key, agent, size.X, size.Y)
					f, e := os.Create(filepath.Join(dir, name))
					if e != nil {
						t.Fatal(e)
					}
					e = png.Encode(f, img)
					ce := f.Close()
					if e != nil {
						t.Fatal(e)
					}
					if ce != nil {
						t.Fatal(ce)
					}
				})
			}
		}
	}
}
