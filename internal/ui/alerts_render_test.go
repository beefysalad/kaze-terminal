package ui

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// cardScene fills a sidebar window with the cards of one capture: "stack" (a
// needs card asking for permission, a done card with a reply and its run time,
// a run summary) or "dock" (two cards over an open agent panel).
func cardScene(t *testing.T, key, scene string) (*window, *input.Router) {
	t.Helper()
	win, _ := sideWindow(t, true)
	win.km, _ = buildKeymap(runtime.GOOS, nil)
	win.binds, _ = resolveBindings(runtime.GOOS, nil)
	th, _ := theme.ByKey(key)
	win.applyTheme(th)
	branches := map[string]string{}
	root := filepath.Join(t.TempDir(), "code")
	win.home = filepath.Dir(root) // paths read ~\code\..., not the test's temp folder
	open := func(name, branch string) *tabs.Tab {
		dir := filepath.Join(root, name)
		os.MkdirAll(dir, 0o755)
		branches[dir] = branch
		return openIn(t, win, dir)
	}
	win.branchFn = func(dir string) string { return branches[filepath.Clean(dir)] }
	home := open("web", "main")
	auth := open("auth", "feature/V3-3149")
	links := open("kaze", "fix/md-links")
	pay := open("pay", "feat/payments-retry")
	win.mgr.SetActive(home.ID)
	var r input.Router
	sideFrame(t, win, &r)
	at := time.Unix(1000, 0)
	send := func(tab *tabs.Tab, m notify.Msg, when time.Time) {
		m.Tab = int(tab.ID)
		deliverNow(win, m, when)
	}
	switch scene {
	case "stack":
		send(links, notify.Msg{Kind: notify.KindPrompt, Command: "/ship", Args: "md-links"}, at.Add(-400*time.Second))
		send(links, notify.Msg{Kind: notify.KindDone, Reason: "Fixed the link split: a path with spaces in it now opens when the folder exists, and quoted paths are taken whole. All 214 tests in ./internal/ui pass, and I added two for the Obsidian folder."}, at)
		send(auth, notify.Msg{Kind: notify.KindPrompt, Command: "/review-pr", Args: "auth-tokens"}, at)
		send(auth, notify.Msg{Kind: notify.KindNeeds, Reason: "Claude needs your permission to use Bash: npm run test -- --runInBand src/auth"}, at)
		win.alerts.addRun(toast{Tab: home.ID, Run: 7, Title: "web stack started", Text: "3 of 3 members started: api on :8080, web on :5173, worker.", Kind: notify.KindDone, Agent: "workspace"})
	case "dock":
		k := dockScene(t, win, int(home.ID), "running")
		sideFrame(t, win, &r)
		win.openDock(home.ID, k)
		send(pay, notify.Msg{Kind: notify.KindPrompt, Agent: "codex", Command: "$fix-retries", Args: "payments"}, at)
		send(pay, notify.Msg{Kind: notify.KindNeeds, Agent: "codex", Reason: "Codex wants to run go test ./... outside the sandbox. Approve?"}, at)
		send(auth, notify.Msg{Kind: notify.KindPrompt, Command: "/review-pr", Args: "auth-tokens"}, at)
		send(auth, notify.Msg{Kind: notify.KindNeeds, Reason: "Which token store should the refresh use, Redis or the session table?"}, at)
	}
	return win, &r
}

// Opt-in development evidence from the actual Gio frame: KAZE_CARD_RENDER_DIR.
// No native window, no global input, no owner configuration.
func TestCardRenders(t *testing.T) {
	dir := os.Getenv("KAZE_CARD_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_CARD_RENDER_DIR for headless captures of the alert cards")
	}
	os.MkdirAll(dir, 0o700)
	for _, key := range []string{"midnight", "sumi", "washi"} {
		for _, c := range []struct {
			name, scene string
			w, h        int
		}{{"stack", "stack", 1400, 860}, {"dock", "dock", 1400, 860}, {"narrow", "stack", 900, 860}} {
			win, r := cardScene(t, key, c.scene)
			for range 2 {
				sizedFrame(t, win, r, c.w, c.h)
			}
			if len(win.ui.toastHits) == 0 {
				t.Fatalf("%s %s: no cards drawn", key, c.name)
			}
			if c.scene == "dock" && win.dockRect().Empty() {
				t.Fatalf("%s %s: the panel is not open", key, c.name)
			}
			g := pageContext(c.w, c.h)
			g.Now = time.Unix(1000, 0)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			writeRender(t, g.Ops, image.Pt(c.w, c.h), filepath.Join(dir, "cards-"+key+"-"+c.name+".png"))
			t.Logf("%s %s: %d cards; texts %q", key, c.name, len(win.ui.toastHits), strings.Join(cardTexts(win), " | "))
		}
	}
}
