package ui

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in development evidence for the pane reading work (KAZE_DOCK_RENDER_DIR):
// a run with real Go edits (line numbers, code colours), a quiet command, a
// failed edit and a failed command, in the docked panel at 1400x860.
func TestPaneReadingRenders(t *testing.T) {
	dir := os.Getenv("KAZE_DOCK_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_DOCK_RENDER_DIR for headless visual evidence")
	}
	os.MkdirAll(dir, 0o700)
	size := image.Pt(1400, 860)
	tr := rdLine("assistant", map[string]any{"type": "text", "text": "Taking the scratch key out of the sidebar first."}) +
		rdUse("r1", "Read", map[string]any{"file_path": "/w/kaze/internal/ui/agent.go"}) +
		rdUse("r2", "Read", map[string]any{"file_path": "/w/kaze/internal/ui/window.go"}) +
		rdPatchEdit("e1", "/w/kaze/internal/ui/agent.go", 41,
			" func (w *window) closeScratch(id int) {",
			"-	if s, ok := w.scratch[id]; ok { // old shell",
			"-		s.Close()",
			"+	if s := w.scratch[id]; s != nil {",
			"+		s.Close() // 0 is fine",
			"+		delete(w.scratch, id)",
			" 	}",
			" }") +
		rdUse("b1", "Bash", map[string]any{"command": "go vet ./internal/ui/", "description": "Run go vet on the ui package"}) +
		rdResult("b1", false, "ok") +
		rdPatchEdit("e2", "/w/kaze/internal/ui/window.go", 118,
			"-	scratchKey := \"ctrl+`\"",
			"+	const scratchKey = \"ctrl+`\" // removed in D6",
			" 	return nil") +
		rdUse("e3", "Edit", map[string]any{"file_path": "/w/kaze/internal/ui/sidebar.go", "old_string": "nope", "new_string": "x"}) +
		rdResult("e3", true, "String to replace not found in file.") +
		rdUse("b2", "Bash", map[string]any{"command": rdBashCmd, "description": "Run go vet and the ui tests"}) +
		rdResult("b2", true, "Exit code 1\n--- FAIL: TestOldScratchKeyIsTheShells (0.01s)\n    scratch_gone_test.go:26: boom\nFAIL\tgithub.com/x/ui\t4.8s")
	for _, key := range []string{"midnight", "washi"} {
		win, _, r, a, _, _, _ := memberWindow(t)
		win.binds, _ = resolveBindings(runtime.GOOS, nil)
		th, _ := theme.ByKey(key)
		win.applyTheme(th)
		win.dock.heightDp = 600
		k, _ := readScene(t, win, int(a.ID), tr)
		win.openDock(a.ID, k)
		s := win.dockOf(a.ID)
		s.follow = false
		s.scroll = 0
		for range 3 {
			sizedFrame(t, win, r, size.X, size.Y)
		}
		img := diffShot(t, win, r, size.X, size.Y)
		savePNG(t, img, filepath.Join(dir, "reading-"+key+".png"))
	}
}
