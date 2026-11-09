package ui

import (
	"context"
	"errors"
	"image"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/devbox"
)

// TestMain keeps every ui test away from the machine's real devbox key.
func TestMain(m *testing.M) {
	loadDevboxCreds = func() (devbox.Creds, bool) { return devbox.Creds{}, false }
	os.Exit(m.Run())
}

const dbxKey = "dbx-secret-for-tests"

var dbxCreds = devbox.Creds{URL: "https://box.example", Key: dbxKey, Name: "alex"}

var dbxJobs = devbox.Parse(strings.Join([]string{
	"j4d0ce4  running  lead  84m  [claude opus] Batch lap lap-a  40% · x",
	"j9b3fad  running  lead  4h  [claude opus] Batch lap lap-b  80% · x",
	"jd561e8  needs_you lead  138h  [claude opus] lap-c: Run  needs you: OK to push?",
	"j6ac836  failed  lead  88m  [claude opus] Batch lap lap-d",
	"j5dd154  stopped  lead  24h  [claude opus] lap-e: Run  10% · x",
	"ja15391  done  lead  78h  [claude opus] lap-f: Run  100% · x",
	"j000001  done  lead  79h  lap-g: Run  100%",
	"j000002  done  lead  80h  lap-h: Run  100%",
	"j000003  done  lead  81h  lap-i: Run  100%",
}, "\n"))

// devboxWindow is a window whose Settings opens on a paired machine, with list as the devbox.
func devboxWindow(t *testing.T, list func(context.Context, devbox.Creds) ([]devbox.Job, error)) *window {
	t.Helper()
	win, _ := alertWindow(t, 1)
	win.set.devbox.load = func() (devbox.Creds, bool) { return dbxCreds, true }
	win.set.devbox.list = list
	return win
}

func devboxIndex(win *window) int {
	return slices.IndexFunc(win.sections(), func(s settingsSection) bool { return s.title == "Devbox" })
}

func openDevbox(t *testing.T, win *window) {
	t.Helper()
	win.openSettings()
	i := devboxIndex(win)
	if i < 0 {
		t.Fatal("no Devbox section")
	}
	win.selectSection(i)
}

func waitDevbox(t *testing.T, win *window) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for win.set.devbox.fetching {
		if time.Now().After(deadline) {
			t.Fatal("the devbox check never finished")
		}
		win.drainUpdates()
		time.Sleep(time.Millisecond)
	}
}

func drawDevboxPage(win *window, w, h int) string {
	win.drawSettings(pageContext(w, h), image.Rect(0, 0, w, h))
	return strings.Join(win.set.texts, "\n")
}

func staticJobs(jobs []devbox.Job, err error) func(context.Context, devbox.Creds) ([]devbox.Job, error) {
	return func(context.Context, devbox.Creds) ([]devbox.Job, error) { return jobs, err }
}

func TestDevboxHiddenWithoutPairing(t *testing.T) {
	win, _ := alertWindow(t, 1)
	var calls atomic.Int32
	win.set.devbox.list = func(context.Context, devbox.Creds) ([]devbox.Job, error) {
		calls.Add(1)
		return nil, nil
	}
	win.openSettings()
	if devboxIndex(win) != -1 {
		t.Fatal("an unpaired machine must have no Devbox page")
	}
	if text := drawDevboxPage(win, 1000, 800); strings.Contains(text, "Devbox") {
		t.Fatal("Devbox appeared on the page")
	}
	if calls.Load() != 0 || win.set.devbox.fetching {
		t.Fatal("an unpaired machine must make no request")
	}
}

func TestDevboxSitsBeforeTheKazeGroup(t *testing.T) {
	win := devboxWindow(t, staticJobs(nil, nil))
	win.openSettings()
	secs := win.sections()
	i := devboxIndex(win)
	if i < 0 || secs[i+1].title != "Updates" || secs[i].group != "" {
		t.Fatalf("Devbox at %d of %d", i, len(secs))
	}
	if secs[i].icon == "" {
		t.Fatal("Devbox needs an icon")
	}
}

func TestDevboxChecksOnOpenAndRefreshOnly(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	win := devboxWindow(t, func(context.Context, devbox.Creds) ([]devbox.Job, error) {
		calls.Add(1)
		<-release
		return dbxJobs, nil
	})
	win.openSettings()
	if calls.Load() != 0 && win.set.devbox.fetching {
		t.Fatal("Settings opening on another page must not check")
	}
	if win.set.devbox.fetching {
		t.Fatal("checked before the Devbox page was shown")
	}
	win.selectSection(devboxIndex(win))
	win.activate("dbx_refresh", 0) // a second press while one is running
	close(release)
	waitDevbox(t, win)
	if got := calls.Load(); got != 1 {
		t.Fatalf("%d requests, want 1 while one was running", got)
	}
	for range 20 {
		drawDevboxPage(win, 1000, 900)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("drawing made requests: %d", got)
	}
	win.activate("dbx_refresh", 0)
	waitDevbox(t, win)
	if got := calls.Load(); got != 2 {
		t.Fatalf("%d requests after Refresh, want 2", got)
	}
}

func TestDevboxShowsJobsByGroup(t *testing.T) {
	win := devboxWindow(t, staticJobs(dbxJobs, nil))
	openDevbox(t, win)
	waitDevbox(t, win)
	text := drawDevboxPage(win, 1000, 1400)
	for _, want := range []string{"alex", "NEEDS YOU", "RUNNING", "RECENTLY FINISHED", "lap-a", "lap-c", "OK to push?", "Needs you", "Running", "Failed", "Stopped", "Done", "Show 1 older job"} {
		if !strings.Contains(text, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if strings.Contains(text, "lap-i") {
		t.Error("the oldest job should wait behind Show older")
	}
	if !strings.Contains(text, "lap-a") || strings.Index(text, "lap-a") > strings.Index(text, "lap-b") {
		t.Error("running jobs must be newest first")
	}
	if strings.Contains(text, dbxKey) {
		t.Fatal("the key was drawn")
	}
	win.activate("dbx_more", 0)
	if text := drawDevboxPage(win, 1000, 1400); !strings.Contains(text, "lap-i") {
		t.Error("Show older did not show the rest")
	}
}

func TestDevboxQueuedJobs(t *testing.T) {
	q := devbox.Parse("j7dcb67  queued  lead  5m  Batch lap lap-q  waiting for room, 2 in line")
	win := devboxWindow(t, staticJobs(q, nil))
	openDevbox(t, win)
	waitDevbox(t, win)
	text := drawDevboxPage(win, 1000, 900)
	for _, want := range []string{"QUEUED", "lap-q", "Queued"} {
		if !strings.Contains(text, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if strings.Contains(text, "Nothing is running") || strings.Contains(text, "RECENTLY FINISHED") {
		t.Errorf("a queued job is not finished: %s", text)
	}
	if win.set.devbox.wakes != 0 {
		t.Error("a queued job has no lane to animate")
	}
}

func TestDevboxUnreachableHidesOldStatuses(t *testing.T) {
	var fail atomic.Bool
	win := devboxWindow(t, func(context.Context, devbox.Creds) ([]devbox.Job, error) {
		if fail.Load() {
			return nil, errors.New("boom " + dbxKey)
		}
		return dbxJobs, nil
	})
	openDevbox(t, win)
	waitDevbox(t, win)
	fail.Store(true)
	win.activate("dbx_refresh", 0)
	waitDevbox(t, win)
	text := drawDevboxPage(win, 1000, 900)
	if !strings.Contains(text, "Couldn't reach the devbox") {
		t.Fatalf("no message: %s", text)
	}
	if strings.Contains(text, "lap-a") {
		t.Fatal("old statuses must not stay on screen")
	}
	if strings.Contains(text, dbxKey) || strings.Contains(text, "boom") {
		t.Fatal("raw error text was drawn")
	}
	if len(controlsOf(win, "dbx_refresh")) == 0 {
		t.Fatal("no way to try again")
	}
}

func TestDevboxRejectedKey(t *testing.T) {
	win := devboxWindow(t, staticJobs(nil, devbox.ErrRejected))
	openDevbox(t, win)
	waitDevbox(t, win)
	if text := drawDevboxPage(win, 1000, 900); !strings.Contains(text, "isn't paired any more") {
		t.Fatalf("no message: %s", text)
	}
}

func TestDevboxNothingRunning(t *testing.T) {
	done := devbox.Parse("ja15391  done  lead  78h  lap-f: Run  100%")
	win := devboxWindow(t, staticJobs(done, nil))
	openDevbox(t, win)
	waitDevbox(t, win)
	if text := drawDevboxPage(win, 1000, 900); !strings.Contains(text, "Nothing is running") || !strings.Contains(text, "lap-f") {
		t.Fatalf("page: %s", text)
	}
	if win.set.devbox.wakes != 0 {
		t.Fatal("no lanes means no animation")
	}
}

func TestDevboxAnimatesOnlyWhileVisibleAndFocused(t *testing.T) {
	win := devboxWindow(t, staticJobs(dbxJobs, nil))
	openDevbox(t, win)
	waitDevbox(t, win)
	drawDevboxPage(win, 1000, 1400)
	if win.set.devbox.wakes != 1 {
		t.Fatalf("wakes = %d, want 1 on a visible focused page", win.set.devbox.wakes)
	}
	win.focused = false
	drawDevboxPage(win, 1000, 1400)
	if win.set.devbox.wakes != 1 {
		t.Fatal("animated without focus")
	}
	win.focused = true
	win.set.scroll = 100000 // past every lane; clamped to the end of the page
	win.drawSettings(pageContext(1000, 300), image.Rect(0, 0, 1000, 300))
	win.set.scroll = win.set.scrollMax
	before := win.set.devbox.wakes
	win.drawSettings(pageContext(1000, 300), image.Rect(0, 0, 1000, 300))
	if win.set.devbox.wakes != before {
		t.Fatal("animated with every lane scrolled away")
	}
	win.set.scroll = 0
	win.selectSection(0)
	before = win.set.devbox.wakes
	drawDevboxPage(win, 1000, 1400)
	if win.set.devbox.wakes != before {
		t.Fatal("animated on another page")
	}
}

func TestDevboxFitsNarrowPanes(t *testing.T) {
	long := devbox.Parse("jabc123  running  lead  3m  " + strings.Repeat("a very long title ", 8) + "  50% · x")
	win := devboxWindow(t, staticJobs(append(long, dbxJobs...), nil))
	openDevbox(t, win)
	waitDevbox(t, win)
	for _, w := range []int{300, 420, 640, 1400} {
		win.set.devbox.more = false
		drawDevboxPage(win, w, 1600)
		for _, card := range win.set.cards {
			if card.Max.X > w {
				t.Fatalf("width %d: card %v runs off the page", w, card)
			}
		}
	}
}
