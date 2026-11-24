package ui

import (
	"context"
	"encoding/json"
	"fmt"
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

type controlSession struct {
	session *term.Session
	send    sync.Mutex // keep control sequences and paste/Enter pairs together
}
type controlSnapshot struct {
	tabs     []ctl.TabInfo
	sessions map[string]*controlSession
}

// controlHost keeps socket access on immutable snapshots and synchronized sessions.
// Renames cross a bounded inbox; its wake callback only invalidates the window.
type controlHost struct {
	snapshot  atomic.Pointer[controlSnapshot]
	changesMu sync.Mutex
	changes   chan struct{}
	renameMu  sync.Mutex
	renames   []*controlRename
	wake      func() // immutable after initialization; only invalidates like onNotify
	labelsMu  sync.Mutex
	labels    map[string]map[string]string
}

var _ ctl.Host = (*controlHost)(nil)

func (h *controlHost) Tabs() []ctl.TabInfo {
	if s := h.snapshot.Load(); s != nil {
		return s.tabs
	}
	return nil
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
	s := target.session
	s.Lock()
	modes := s.Applier().Modes()
	s.Unlock()
	mark := s.EchoMark()
	if result := s.Submit(input.EncodePaste(string(text), modes)); result != term.Accepted {
		return fmt.Errorf("paste to %s: %s", id, result)
	}
	s.WaitEcho(mark, controlPasteEnterDelay)
	if result := s.Submit([]byte{'\r'}); result != term.Accepted {
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
	next := &controlSnapshot{tabs: make([]ctl.TabInfo, 0, len(list)), sessions: make(map[string]*controlSession, len(list))}
	for _, t := range list {
		id := "t" + strconv.Itoa(int(t.ID))
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
		}
		if win.alerts.markOf(t.ID) == markNeeds {
			status = "needs"
		}
		name := win.names[t.ID]
		if name == "" {
			name = t.FixedTitle
		}
		folder := loc.cwd
		if folder == "" {
			folder = t.Dir
		}
		next.tabs = append(next.tabs, ctl.TabInfo{ID: id, Name: name, Space: controlSpaceName(loc), Folder: folder,
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
