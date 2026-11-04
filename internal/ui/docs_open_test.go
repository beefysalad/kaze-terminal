package ui

import (
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestShortcutClickOpensADocPath(t *testing.T) {
	st := &recStarter{}
	win := testWindow(t, st)
	win.ui = newChrome()
	var opened []string
	win.openLink = func(u string) error { opened = append(opened, u); return nil }
	dir := t.TempDir()
	want := writeDoc(t, dir, filepath.Join("docs", "plan.md"), "# Plan\n")
	tab, _ := shellTab(t, win, st, "wrote docs/plan.md ok", func() bool {
		a := win.mgr.Active()
		return a != nil && textShown(a, 20, 'k')()
	})
	report(t, tab, dir)
	r := &pointerRun{t: t, win: win, tab: tab}
	r.send(pointer.Press, pointer.ButtonPrimary, 0, 8, 0)
	r.send(pointer.Release, 0, 0, 8, 0)
	if win.shownDoc() != nil {
		t.Fatal("a plain click opened the doc")
	}
	r.at += time.Second
	r.send(pointer.Press, pointer.ButtonPrimary, key.ModShortcut, 8, 0)
	r.send(pointer.Release, 0, key.ModShortcut, 8, 0)
	d := win.shownDoc()
	if d == nil || d.path != want || d.session != tab.ID {
		t.Fatalf("doc %+v, want %s under %d", d, want, tab.ID)
	}
	if len(opened) != 0 {
		t.Fatalf("a doc path went to the OS: %v", opened)
	}
}
func TestOpenNeverHandsANonWebURLToTheOS(t *testing.T) {
	win := testWindow(t, &recStarter{})
	var got []string
	win.openLink = func(s string) error { got = append(got, s); return nil }
	for _, s := range []string{"file:///etc/passwd", "docs/a.md", "javascript:alert(1)", `C:\x.md`} {
		win.open(s)
	}
	if len(got) != 0 {
		t.Fatal(got)
	}
	win.open("https://a.io")
	if len(got) != 1 {
		t.Fatal(got)
	}
}
func TestDocPathFallsBackToTheRepoRoot(t *testing.T) {
	win := testWindow(t, &recStarter{})
	win.ui = newChrome()
	a, _ := win.mgr.New()
	root := t.TempDir()
	p := writeDoc(t, root, "docs/a.md", "hi")
	a.Dir = filepath.Join(root, "sub")
	win.topFn = func(string) string { return root }
	win.openDocRef(a, false, "docs/a.md")
	if d := win.shownDoc(); d == nil || d.path != p {
		t.Fatal(d)
	}
	win.selectTab(a.ID)
	win.openDocRef(a, false, "nope.md")
	if want := "Can't find nope.md in " + tildeHome(a.Dir, win.home); win.notice != want {
		t.Fatal(win.notice)
	}
}

// ctrlClickLine prints line in a wide session whose folder is cwd, then
// Ctrl+clicks the first character of the first `at` on it.
func ctrlClickLine(t *testing.T, line, at, cwd string) *window {
	t.Helper()
	st := &recStarter{}
	win := testWindow(t, st)
	win.ui = newChrome()
	win.topFn = func(string) string { return "" } // never run real git
	win.mgr.ResizeAll(240, 24)
	n := utf8.RuneCountInString(line)
	last, _ := utf8.DecodeLastRuneInString(line)
	tab, _ := shellTab(t, win, st, line, func() bool {
		a := win.mgr.Active()
		return a != nil && textShown(a, n-1, last)()
	})
	report(t, tab, cwd)
	i := strings.Index(line, at)
	if i < 0 {
		t.Fatalf("setup: %q not on %q", at, line)
	}
	col := utf8.RuneCountInString(line[:i])
	r := &pointerRun{t: t, win: win, tab: tab}
	r.send(pointer.Press, pointer.ButtonPrimary, key.ModShortcut, col, 0)
	r.send(pointer.Release, 0, key.ModShortcut, col, 0)
	return win
}

func TestClickOpensDocInFolderPrintedEarlier(t *testing.T) {
	dir := t.TempDir()
	want := writeDoc(t, dir, filepath.Join("T-1", "plan.md"), "# Plan")
	line := "Full plan: " + filepath.Join(dir, "T-1") + string(filepath.Separator) + " (plan.md, design.md)"
	win := ctrlClickLine(t, line, "plan.md", t.TempDir())
	if d := win.shownDoc(); d == nil || d.path != want {
		t.Fatalf("doc %+v, want %s (note %q)", d, want, win.notice)
	}
}

func TestClickOpensDocWithSpaceInPath(t *testing.T) {
	dir := t.TempDir()
	want := writeDoc(t, dir, filepath.Join("Patrick Obsidian", "Patrick", "x.md"), "# X")
	line := "Write(" + filepath.ToSlash(dir) + "/Patrick Obsidian/Patrick/x.md)"
	win := ctrlClickLine(t, line, "x.md", t.TempDir())
	if d := win.shownDoc(); d == nil || d.path != want {
		t.Fatalf("doc %+v, want %s (note %q)", d, want, win.notice)
	}
}

// "cd <tmp> then edit docs/notes.md" underlines from <tmp> (Ruling 21); the
// click still opens docs/notes.md in the session folder through a shorter tail.
func TestClickFallsBackToShorterTail(t *testing.T) {
	cwd := t.TempDir()
	want := writeDoc(t, cwd, filepath.Join("docs", "notes.md"), "# Notes")
	win := ctrlClickLine(t, "cd "+t.TempDir()+" then edit docs/notes.md", "notes.md", cwd)
	if d := win.shownDoc(); d == nil || d.path != want {
		t.Fatalf("doc %+v, want %s (note %q)", d, want, win.notice)
	}
}

func TestDocMissNamesTheLastFolderTried(t *testing.T) {
	win := testWindow(t, &recStarter{})
	win.ui = newChrome()
	win.topFn = func(string) string { return "" }
	a, _ := win.mgr.New()
	a.Dir = t.TempDir()
	far := filepath.Join(t.TempDir(), "T-1")
	win.openDocRefs(a, false, []string{"plan.md", filepath.Join(far, "plan.md")})
	if want := "Can't find plan.md in " + tildeHome(far, win.home); win.notice != want {
		t.Fatalf("note %q, want %q", win.notice, want)
	}
}

// A missed "cd <tmp> then edit docs/notes.md" names the session folder, not
// "<tmp> then edit docs": an absolute candidate with spaces is no folder.
func TestDocMissSkipsSpacedFolder(t *testing.T) {
	cwd := t.TempDir()
	win := ctrlClickLine(t, "cd "+t.TempDir()+" then edit docs/notes.md", "notes.md", cwd)
	if want := "Can't find notes.md in " + tildeHome(cwd, win.home); win.notice != want {
		t.Fatalf("note %q, want %q", win.notice, want)
	}
}

// A missed path with spaces in an existing folder still names that folder.
func TestDocMissNamesExistingSpacedFolder(t *testing.T) {
	win := testWindow(t, &recStarter{})
	win.ui = newChrome()
	win.topFn = func(string) string { return "" }
	a, _ := win.mgr.New()
	a.Dir = t.TempDir()
	far := filepath.Join(t.TempDir(), "My Notes")
	if err := os.MkdirAll(far, 0o755); err != nil {
		t.Fatal(err)
	}
	win.openDocRefs(a, false, []string{filepath.Join(far, "plan.md"), "Notes/plan.md"})
	if want := "Can't find plan.md in " + tildeHome(far, win.home); win.notice != want {
		t.Fatalf("note %q, want %q", win.notice, want)
	}
}
