package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/screen"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

// controlPasteEnterDelay bounds the echo wait between a paste and its Enter.
const controlPasteEnterDelay = 100 * time.Millisecond

// controlPendingGrace is how long a tab reads as working after a send while
// its prompt hook has not arrived; without it a wait right after a send
// returns on the previous turn's idle state.
var controlPendingGrace = 10 * time.Second

type controlSession struct {
	session     *term.Session
	send        sync.Mutex   // keep control sequences and paste/Enter pairs together
	lastKey     atomic.Int64 // last owner key press, retained across publications
	noteSending atomic.Bool
	pending     atomic.Int64 // unix nanos of the last send still awaiting its prompt hook; 0 when none
}
type controlSnapshot struct {
	ownerKeys map[string]string // private immutable authentication data; never serialized
	tabs      []ctl.TabInfo
	sessions  map[string]*controlSession
}

// controlHost keeps socket access on immutable snapshots and synchronized sessions.
// Renames cross a bounded inbox; its wake callback only invalidates the window.
type controlHost struct {
	noteQuiet      time.Duration // immutable after initialization; zero uses the default
	snapshot       atomic.Pointer[controlSnapshot]
	changesMu      sync.Mutex
	changes        chan struct{}
	spawnMu        sync.Mutex
	workerRequests []*controlWorkerRequest
	renameMu       sync.Mutex
	renames        []*controlRename
	wake           func() // immutable after initialization; only invalidates like onNotify
	labelsMu       sync.Mutex
	labels         map[string]map[string]string
	inbox          map[string]*controlInbox
}

var _ ctl.Host = (*controlHost)(nil)

// Tabs returns the published tabs. A hooked agent that was just sent input
// reads as working until its prompt hook lands or the grace passes.
func (h *controlHost) Tabs() []ctl.TabInfo {
	for {
		s := h.snapshot.Load()
		if s == nil {
			return nil
		}
		tabs := overlayPending(s, time.Now().UnixNano())
		// A prompt hook publishes before it clears pending, so a cleared hold
		// seen above means a newer snapshot exists: retry rather than report
		// the old idle state.
		if h.snapshot.Load() == s {
			return tabs
		}
	}
}

func overlayPending(s *controlSnapshot, now int64) []ctl.TabInfo {
	out, cloned := s.tabs, false
	for i, tab := range s.tabs {
		if tab.Status != "idle" && tab.Status != "needs" || (tab.Status == "needs" && tab.Role == "worker") {
			continue
		}
		at := s.sessions[tab.ID].pending.Load()
		if at == 0 || now-at >= int64(controlPendingGrace) {
			continue
		}
		if !cloned {
			out, cloned = slices.Clone(s.tabs), true
		}
		out[i].Status = "working"
	}
	return out
}

// holdPending marks a send awaiting its prompt hook and wakes waiters when the
// grace ends, so a hook that never comes cannot leave a wait stuck.
func (h *controlHost) holdPending(target *controlSession) (release func()) {
	at := time.Now().UnixNano()
	target.pending.Store(at)
	time.AfterFunc(controlPendingGrace, h.wakeWaiters)
	return func() { target.pending.CompareAndSwap(at, 0) }
}

func (h *controlHost) clearPending(id string) {
	if s := h.snapshot.Load(); s != nil && s.sessions[id] != nil {
		s.sessions[id].pending.Store(0)
	}
}

// wakeWaiters closes the change channel without a new snapshot.
func (h *controlHost) wakeWaiters() {
	h.changesMu.Lock()
	if h.changes != nil {
		close(h.changes)
		h.changes = nil
	}
	h.changesMu.Unlock()
}
func (h *controlHost) session(id string) (*controlSession, error) {
	if s := h.snapshot.Load(); s != nil {
		if target := s.sessions[id]; target != nil {
			return target, nil
		}
	}
	return nil, fmt.Errorf("tab %s is closed or unknown", id)
}
func (h *controlHost) Screen(id string, lines int) (string, error) {
	target, err := h.session(id)
	if err != nil {
		return "", err
	}
	s := target.session
	s.Lock()
	defer s.Unlock()
	return screen.Text(s.Screen(), min(max(lines, 0), ctl.MaxLines)), nil
}
func (h *controlHost) Submit(id string, text []byte) error {
	target, err := h.session(id)
	if err != nil {
		return err
	}
	target.send.Lock()
	defer target.send.Unlock()
	return h.submitLocked(id, text, target)
}

// submitLocked keeps the paste and Enter under the caller's send lock.
func (h *controlHost) submitLocked(id string, text []byte, target *controlSession) error {
	s := target.session
	s.Lock()
	modes := s.Applier().Modes()
	s.Unlock()
	// Hold before Enter is observable, so the prompt hook cannot clear first.
	release := h.holdPending(target)
	mark := s.EchoMark()
	if result := s.Submit(input.EncodePaste(string(text), modes)); result != term.Accepted {
		release()
		return fmt.Errorf("paste to %s: %s", id, result)
	}
	s.WaitEcho(mark, controlPasteEnterDelay)
	if result := s.Submit([]byte{'\r'}); result != term.Accepted {
		release()
		return fmt.Errorf("enter to %s: %s", id, result)
	}
	return nil
}

// publishControl runs only on the UI goroutine at existing state changes. It
// never invalidates a window or schedules work. Old snapshots stay immutable.
func (win *window) publishControl() {
	if win.mgr == nil {
		return
	}
	win.control.labelsMu.Lock()
	defer win.control.labelsMu.Unlock()
	old := win.control.snapshot.Load()
	list := win.mgr.Tabs()
	next := &controlSnapshot{ownerKeys: make(map[string]string, len(list)), tabs: make([]ctl.TabInfo, 0, len(list)), sessions: make(map[string]*controlSession, len(list))}
	for _, t := range list {
		id := "t" + strconv.Itoa(int(t.ID))
		if key := win.mgr.OwnerKey(t.ID); key != "" {
			next.ownerKeys[id] = key
		}
		// Discovery belongs to the existing UI model refresh. Publishing itself
		// must not start a worker whose completion could request a frame.
		loc := win.locs[t.ID]
		t.Session.Lock()
		cwd := t.Session.Applier().Cwd()
		t.Session.Unlock()
		if cwd != "" && cwd != loc.cwd {
			loc = locEntry{cwd: cwd}
		}
		a := win.rowAgent(t.ID)
		status := "shell"
		if a.agentName() != "" {
			status = "idle"
			if a.running {
				status = "working"
			}
			if win.agentOf(t.ID).agentName() == "" {
				status = "unknown" // found by process only: no hook says idle or working
			}
		}
		if win.alerts.markOf(t.ID) == markNeeds && t.Role != "worker" {
			status = "needs"
		}
		parent, firstMessage := "", ""
		if t.Parent != 0 {
			parent = workerID(t.Parent)
		}
		if w := win.workers[t.ID]; w != nil {
			firstMessage = w.firstMessage
			if !w.hooked {
				status = "starting"
			}
			if w.needs {
				status = "needs"
			}
			if a.agentName() == "" {
				a.agent = w.agent
			}
		}
		if t.Exited {
			status = "exited"
		}
		name := win.names[t.ID]
		if name == "" {
			name = t.FixedTitle
		}
		folder := loc.cwd
		if folder == "" {
			folder = t.Dir
		}
		messages := win.control.inboxMessages(id)
		next.tabs = append(next.tabs, ctl.TabInfo{Unread: len(messages), Inbox: messages, Parent: parent, Role: t.Role, FirstMessage: firstMessage, ID: id, Name: name, Space: controlSpaceName(loc), Folder: folder,
			Labels: cloneLabels(win.control.labels[id]), Agent: a.agentName(), Status: status, Since: a.since, Workspace: t.Group != nil})
		var target *controlSession
		if old != nil {
			target = old.sessions[id]
		}
		if target == nil || target.session != t.Session {
			target = &controlSession{session: t.Session}
		}
		next.sessions[id] = target
	}
	for id := range win.control.labels {
		if next.sessions[id] == nil {
			delete(win.control.labels, id)
		}
	}
	for id := range win.control.inbox {
		if next.sessions[id] == nil {
			delete(win.control.inbox, id)
		}
	}
	win.control.publish(next)
}

func controlAdapter(host ctl.Host) func(json.RawMessage) any {
	return func(raw json.RawMessage) any { return controlContextAdapter(host)(context.Background(), raw) }
}

func controlContextAdapter(host ctl.Host, options ...ctl.Options) func(context.Context, json.RawMessage) any {
	return func(ctx context.Context, raw json.RawMessage) any {
		var req ctl.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return ctl.Reply{Error: &ctl.Error{Code: ctl.CodeInvalidRequest, Message: "invalid control request"}}
		}
		return ctl.HandleContext(ctx, host, req, options...)
	}
}

// publish stores the snapshot and broadcasts under the subscription lock, so a
// waiter cannot pair a stale snapshot with a newer notification channel.
func (h *controlHost) publish(next *controlSnapshot) {
	h.changesMu.Lock()
	h.snapshot.Store(next)
	if h.changes != nil {
		close(h.changes)
		h.changes = nil
	}
	h.changesMu.Unlock()
}
func (h *controlHost) StatusChanges() ([]ctl.TabInfo, <-chan struct{}) {
	h.changesMu.Lock()
	defer h.changesMu.Unlock()
	if h.changes == nil {
		h.changes = make(chan struct{})
	}
	return h.Tabs(), h.changes
}
func (h *controlHost) OutputChanges(id string) (<-chan struct{}, func(), time.Time, error) {
	target, err := h.session(id)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	wake := make(chan struct{}, 1)
	remove := target.session.AddOutputObserver(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	return wake, remove, target.session.LastOutputTime(), nil
}

// OwnerKey reads only the immutable published snapshot on socket goroutines.
func (h *controlHost) OwnerKey(id string) string {
	if s := h.snapshot.Load(); s != nil {
		return s.ownerKeys[id]
	}
	return ""
}
