package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/input"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func TestWorkerConsentBannerRenders(t *testing.T) {
	dir := os.Getenv("KAZE_CONSENT_RENDER_DIR")
	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"midnight", "washi"} {
		for _, size := range []image.Point{image.Pt(1200, 800), image.Pt(560, 640)} {
			t.Run(fmt.Sprintf("%s-%dx%d", key, size.X, size.Y), func(t *testing.T) {
				win, _ := workerWindow(t)
				win.opt.Config.Orchestrator.Confirm = "first"
				th, _ := theme.ByKey(key)
				win.applyTheme(th)
				win.agents = map[tabs.TabID]agentState{1: {claude: true, running: true, shellAt: shellSeq(win.mgr.Active())}}
				r := consentRequest("t1")
				r.req.CWD = filepath.Join(win.opt.Dir, "kaze-terminal")
				if err := os.Mkdir(r.req.CWD, 0700); err != nil {
					t.Fatal(err)
				}
				win.holdWorkerConsent(r)
				var router input.Router
				for range 3 {
					g := pageContext(size.X, size.Y)
					g.Now = t0
					g.Source = router.Source()
					if err := win.frame(g); err != nil {
						t.Fatal(err)
					}
					router.Frame(g.Ops)
					if dir != "" {
						if _, err := writeShot(t, g.Ops, size, filepath.Join(dir, fmt.Sprintf("consent-%s-%dx%d.png", key, size.X, size.Y))); err != nil {
							t.Fatal(err)
						}
					}
				}
				if len(win.consent.hits) != 3 {
					t.Fatalf("missing banner %v", win.consent.hits)
				}
				for _, h := range win.consent.hits {
					if h.rect.Intersect(image.Rectangle{Max: size}) != h.rect {
						t.Fatalf("banner outside window %v", h.rect)
					}
				}
				g := pageContext(size.X, size.Y)
				draw := func() {
					g.Ops.Reset()
					if !win.drawWorkerConsent(g, win.mgr.Active(), win.consent.cache.rect) {
						t.Fatal("missing consent")
					}
				}
				for range 3 {
					draw()
				}
				if n := testing.AllocsPerRun(30, draw); n != 0 {
					t.Fatalf("unchanged banner allocates %v", n)
				}
				// Route through headless Gio only. No native keyboard or mouse injection.
				clickAt(t, win, &router, centre(win.consent.hits[2].rect))
				consentCode(t, r, "consent_denied")
			})
		}
	}
}
