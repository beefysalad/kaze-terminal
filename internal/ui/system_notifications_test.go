package ui

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/bench"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/sysnotify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

type fakeSystemBackend struct {
	posts                  []sysnotify.Notification
	removed                []string
	raised, checks, closed int
	status                 string
	err                    error
}

func (f *fakeSystemBackend) Post(n sysnotify.Notification) error {
	f.posts = append(f.posts, n)
	return f.err
}
func (f *fakeSystemBackend) Remove(id string)  { f.removed = append(f.removed, id) }
func (f *fakeSystemBackend) CheckPermission()  { f.checks++ }
func (f *fakeSystemBackend) Retry()            { f.checks++ }
func (f *fakeSystemBackend) Status() string    { return f.status }
func (f *fakeSystemBackend) SetWindow(uintptr) {}
func (f *fakeSystemBackend) Raise()            { f.raised++ }
func (f *fakeSystemBackend) Close()            { f.closed++ }

func notificationWindow(t *testing.T) (*window, *fakeSystemBackend) {
	t.Helper()
	w, _, _ := liveWindow(t, 2)
	w.opt.Config.SystemNotifications = true
	f := new(fakeSystemBackend)
	w.system = newSystemNotifications(f, "abc", true)
	w.publish(w.mgr.Active(), false)
	t.Cleanup(w.system.close)
	return w, f
}

func TestSystemNotificationWithoutAFrame(t *testing.T) {
	w, f := notificationWindow(t)
	id := w.mgr.Tabs()[1].ID
	w.names = map[tabs.TabID]string{id: "Audit <API>"}
	w.publish(w.mgr.Active(), false)
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds, Reason: "permission\x1b\n"})
	w.system.flush()
	if len(f.posts) != 1 {
		t.Fatalf("posts=%v", f.posts)
	}
	n := f.posts[0]
	if n.Title != "Audit <API>" || !strings.Contains(n.Body, "permission") || strings.ContainsAny(n.Body, "\x1b") || n.ID == "" {
		t.Fatalf("notification=%+v", n)
	}
	if w.inbox.size() != 1 {
		t.Fatal("existing cards were not queued")
	}
}

func TestSystemNotificationPromptAndFirstAlertWithoutFrames(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(fmt.Sprint(fresh), func(t *testing.T) {
			w, f := notificationWindow(t)
			id := w.mgr.Tabs()[1].ID
			if fresh {
				tab, err := w.mgr.New()
				if err != nil {
					t.Fatal(err)
				}
				id = tab.ID
			}
			w.onNotify(notify.Msg{Tab: int(id), Agent: "codex", Kind: notify.KindPrompt, Command: "/review", Args: "checkout"})
			w.onNotify(notify.Msg{Tab: int(id), Agent: "codex", Kind: notify.KindNeeds, Reason: "permission"})
			w.system.flush()
			if len(f.posts) != 1 || f.posts[0].Title != "checkout" || !strings.Contains(f.posts[0].Context, "/review") {
				t.Fatalf("no-frame metadata=%+v", f.posts)
			}
		})
	}
}

func TestSystemNotificationFirstAlertKeepsFixedTitle(t *testing.T) {
	w, f := notificationWindow(t)
	tab, err := w.mgr.NewOpts(tabs.StartOpts{Title: "API server"})
	if err != nil {
		t.Fatal(err)
	}
	w.onNotify(notify.Msg{Tab: int(tab.ID), Agent: "codex", Kind: notify.KindPrompt, Command: "/review", Args: "checkout"})
	w.onNotify(notify.Msg{Tab: int(tab.ID), Agent: "codex", Kind: notify.KindNeeds})
	w.system.flush()
	if len(f.posts) != 1 || f.posts[0].Title != "API server" {
		t.Fatalf("fixed title lost: %+v", f.posts)
	}
}

func TestSystemNotificationPolicy(t *testing.T) {
	for _, tc := range []struct {
		name             string
		focused, enabled bool
		tab              int
		kind             notify.Kind
		want             int
	}{
		{"needs", false, true, 2, notify.KindNeeds, 1}, {"done", false, true, 2, notify.KindDone, 1},
		{"focused other session", true, true, 2, notify.KindNeeds, 0}, {"focused active", true, true, 1, notify.KindDone, 0},
		{"disabled", false, false, 2, notify.KindNeeds, 0}, {"closed", false, true, 99, notify.KindNeeds, 0},
		{"prompt", false, true, 2, notify.KindPrompt, 0}, {"session", false, true, 2, notify.KindSession, 0},
		{"interrupt", false, true, 2, notify.KindInterrupt, 0}, {"doc", false, true, 2, notify.KindDoc, 0},
		{"usage", false, true, 2, notify.KindUsage, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, f := notificationWindow(t)
			w.system.setEnabled(tc.enabled)
			w.publish(w.mgr.Active(), tc.focused)
			w.onNotify(notify.Msg{Tab: tc.tab, Kind: tc.kind, Reason: "ready"})
			w.system.flush()
			if len(f.posts) != tc.want {
				t.Fatalf("posts=%v want %d", f.posts, tc.want)
			}
		})
	}
}

func TestSystemNotificationReplaceVisitAndStaleClick(t *testing.T) {
	w, f := notificationWindow(t)
	id := w.mgr.Tabs()[1].ID
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds})
	w.system.flush()
	first := f.posts[0].ID
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindDone})
	w.system.flush()
	second := f.posts[1].ID
	if first == second || strings.Join(strings.Split(first, "-")[:2], "-") != strings.Join(strings.Split(second, "-")[:2], "-") {
		t.Fatalf("unstable identity: %s %s", first, second)
	}
	w.system.activate(first)
	w.drainSystemActivations()
	if f.raised != 0 {
		t.Fatal("superseded click raised window")
	}
	w.system.activate(second)
	w.drainSystemActivations()
	if f.raised != 1 || w.mgr.Active().ID != id {
		t.Fatal("click did not raise and select without a frame")
	}
	w.focused = true
	w.visit(w.mgr.Active())
	w.system.flush()
	if len(f.removed) == 0 {
		t.Fatal("visit did not withdraw")
	}
	w.system.activate(second)
	w.drainSystemActivations()
	if f.raised != 1 {
		t.Fatal("visited notification still active")
	}
}

func TestSystemNotificationQueuedCancellation(t *testing.T) {
	for _, how := range []string{"focus", "disable", "visit", "close", "shutdown"} {
		t.Run(how, func(t *testing.T) {
			w, f := notificationWindow(t)
			id := w.mgr.Tabs()[1].ID
			w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds})
			switch how {
			case "focus":
				w.publish(w.mgr.Active(), true)
			case "disable":
				w.system.setEnabled(false)
			case "visit":
				w.system.remove(id)
			case "close":
				w.mgr.Close(id)
				w.publish(w.mgr.Active(), false)
			case "shutdown":
				w.system.close()
			}
			w.system.flush()
			if len(f.posts) != 0 {
				t.Fatalf("stale queued post: %v", f.posts)
			}
		})
	}
}

func TestSystemNotificationBoundAndGeneration(t *testing.T) {
	f := new(fakeSystemBackend)
	s := newSystemNotifications(f, "def", true)
	defer s.close()
	for i := 1; i <= systemNotificationLimit+20; i++ {
		id := tabs.TabID(i)
		s.metadata(id, notificationMeta{title: fmt.Sprintf("session %d", i)})
		s.notify(notify.Msg{Tab: i, Kind: notify.KindNeeds})
	}
	if len(s.entries) > systemNotificationLimit || len(s.pending) > systemNotificationLimit {
		t.Fatalf("unbounded: %d entries %d pending", len(s.entries), len(s.pending))
	}
	s.flush()
	if len(f.posts) > systemNotificationLimit {
		t.Fatalf("unbounded posts %d", len(f.posts))
	}
	s.close()
	s.activate(f.posts[len(f.posts)-1].ID)
	if f.raised != 0 {
		t.Fatal("closed controller raised")
	}
}

func TestSystemNotificationCurrentMetadataAndFailure(t *testing.T) {
	w, f := notificationWindow(t)
	id := w.mgr.Tabs()[1].ID
	w.names = map[tabs.TabID]string{id: "Old"}
	w.publish(w.mgr.Active(), false)
	w.names[id] = "Renamed"
	w.publish(w.mgr.Active(), false)
	f.err = errors.New("native unavailable")
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindDone})
	w.system.flush()
	if f.posts[0].Title != "Renamed" || !strings.Contains(w.system.status(), "native unavailable") {
		t.Fatalf("post %v status %s", f.posts, w.system.status())
	}
	w.system.activate(f.posts[0].ID)
	w.mgr.Close(id)
	w.drainSystemActivations()
	if f.raised != 0 {
		t.Fatal("closed tab activation raised window")
	}
}

func TestSystemNotificationSettings(t *testing.T) {
	w, f := notificationWindow(t)
	f.status = "Blocked in macOS notification settings"
	rows := sharedAgentRows(w)
	found := false
	for _, r := range rows {
		for _, c := range r.ctrls {
			if c.id == "system_notifications" {
				found = true
				if !c.on || !strings.Contains(r.hint, "Blocked") {
					t.Fatalf("row=%+v", r)
				}
			}
		}
	}
	if !found {
		t.Fatal("missing System notifications row")
	}
	agentsAct(w, "system_notifications", 0)
	if w.opt.Config.SystemNotifications {
		t.Fatal("toggle did not update config")
	}
	agentsAct(w, "system_notifications", 1)
	if !w.opt.Config.SystemNotifications || f.checks != 1 {
		t.Fatalf("enable=%v checks=%d", w.opt.Config.SystemNotifications, f.checks)
	}
}

func TestSystemNotificationBenchNeverInitializesNativeBackend(t *testing.T) {
	w := window{bench: new(bench.Recorder)}
	w.initSystemNotifications(func(sysnotify.Callbacks) sysnotify.Backend {
		t.Fatal("bench initialized native notifications")
		return nil
	})
	if w.system != nil {
		t.Fatal("bench started notification controller")
	}
}

type systemChannelBackend struct {
	fakeSystemBackend
	posted chan sysnotify.Notification
}

type systemCurrentBackend struct {
	fakeSystemBackend
	current map[string]string
}

func notificationStable(id string) string {
	parts := strings.Split(id, "-")
	return strings.Join(parts[:2], "-")
}
func (f *systemCurrentBackend) Post(n sysnotify.Notification) error {
	f.current[notificationStable(n.ID)] = n.ID
	return nil
}
func (f *systemCurrentBackend) Remove(id string) {
	key := notificationStable(id)
	if f.current[key] == id {
		delete(f.current, key)
	}
}

func TestSystemNotificationRemovePendingReplacementWithdrawsSubmittedAlert(t *testing.T) {
	for _, how := range []string{"visit", "disable", "focus"} {
		t.Run(how, func(t *testing.T) {
			f := &systemCurrentBackend{current: map[string]string{}}
			s := newSystemNotifications(f, "aaa", true)
			defer s.close()
			s.metadata(1, notificationMeta{title: "One"})
			s.notify(notify.Msg{Tab: 1, Kind: notify.KindNeeds})
			s.flush()
			s.notify(notify.Msg{Tab: 1, Kind: notify.KindDone})
			switch how {
			case "visit":
				s.remove(1)
			case "disable":
				s.setEnabled(false)
			case "focus":
				s.focus(true)
			}
			s.flush()
			if len(f.current) != 0 {
				t.Fatalf("old submitted notification survived %s: %v", how, f.current)
			}
		})
	}
}

func TestSystemNotificationFailedPostAfterQueuedVisitWithdrawsOldAlert(t *testing.T) {
	f := &systemCurrentBackend{current: map[string]string{}}
	s := newSystemNotifications(f, "aba", true)
	defer s.close()
	s.metadata(1, notificationMeta{title: "One"})
	s.notify(notify.Msg{Tab: 1, Kind: notify.KindNeeds})
	s.flush()
	s.remove(1)
	// Another alert before the removal worker runs must not lose that removal.
	s.notify(notify.Msg{Tab: 1, Kind: notify.KindDone})
	s.remove(1)
	s.flush()
	if len(f.current) != 0 {
		t.Fatalf("queued withdrawal lost: %v", f.current)
	}
}

func (f *systemChannelBackend) Post(n sysnotify.Notification) error { f.posted <- n; return nil }

func TestSystemNotificationWorkerRunsWithoutFrames(t *testing.T) {
	f := &systemChannelBackend{posted: make(chan sysnotify.Notification, 1)}
	s := newSystemNotifications(f, "fed", true)
	defer s.close()
	s.metadata(4, notificationMeta{title: "Worker"})
	s.start()
	s.notify(notify.Msg{Tab: 4, Kind: notify.KindNeeds})
	select {
	case n := <-f.posted:
		if n.Title != "Worker" {
			t.Fatal(n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker waited for a frame")
	}
}

func TestSystemNotificationConcurrentPublicationAndHooks(t *testing.T) {
	f := new(fakeSystemBackend)
	s := newSystemNotifications(f, "eef", true)
	defer s.close()
	var wg sync.WaitGroup
	for lane := 0; lane < 3; lane++ {
		wg.Go(func() {
			for i := 0; i < 64; i++ {
				s.metadata(1, notificationMeta{title: "Concurrent"})
				s.notify(notify.Msg{Tab: 1, Kind: notify.KindNeeds})
				s.focus(i%2 == 0)
				s.setEnabled(i%3 != 0)
			}
		})
	}
	wg.Wait()
	s.flush()
}

func TestSystemNotificationStillMetadataDoesNotAllocate(t *testing.T) {
	w, _ := notificationWindow(t)
	w.publishSystemMetadata()
	if allocs := testing.AllocsPerRun(20, w.publishSystemMetadata); allocs != 0 {
		t.Fatalf("unchanged metadata allocated %g times", allocs)
	}
}

func TestSystemNotificationErrorClearsAfterSuccessfulPost(t *testing.T) {
	w, f := notificationWindow(t)
	id := w.mgr.Tabs()[1].ID
	f.err = errors.New("Blocked in Windows notification settings")
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds, Reason: "first"})
	w.system.flush()
	if !strings.Contains(w.system.status(), "Blocked") {
		t.Fatalf("status=%q", w.system.status())
	}
	f.err = nil
	w.onNotify(notify.Msg{Tab: int(id), Kind: notify.KindNeeds, Reason: "second"})
	w.system.flush()
	if s := w.system.status(); s != "" {
		t.Fatalf("stale status after a successful post: %q", s)
	}
}
