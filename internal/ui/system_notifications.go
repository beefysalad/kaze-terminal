package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/sysnotify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const systemNotificationLimit = 256

type notificationMeta struct {
	title, context  string
	tab             *tabs.Tab // identity, so an activation cannot target a replacement tab
	branch, command string
	named           bool
}

// notificationInput is comparable, so unchanged frames do not clean/allocate
// labels or context strings again.
type notificationInput struct {
	tab                           *tabs.Tab
	title, fixed, name, home, def string
	exited                        bool
	loc                           locEntry
	agent                         agentState
}
type systemNotice struct {
	n                   sysnotify.Notification
	meta                notificationMeta
	seq                 uint64
	sent                bool
	submitted, previous string // actual native IDs, including one Post in flight
}
type systemWork struct {
	n                   sysnotify.Notification
	remove              bool
	seq                 uint64
	submitted, previous string
}

// systemNotifications owns immutable metadata and bounded latest-per-tab work.
// Nothing here reads UI maps from hook or native callback goroutines.
type systemNotifications struct {
	mu                       sync.Mutex
	backend                  sysnotify.Backend
	prefix                   string
	enabled, focused, closed bool
	seq                      uint64
	meta                     map[tabs.TabID]notificationMeta
	entries                  map[tabs.TabID]systemNotice
	pending                  map[tabs.TabID]systemWork
	retired                  []string // bounded cleanup of evicted, already delivered notifications
	err                      string
	wake                     chan struct{}
	stop                     chan struct{}
	activations              chan string
	inputs                   map[tabs.TabID]notificationInput // UI goroutine only
	changed                  func()
}

func newSystemNotifications(b sysnotify.Backend, prefix string, enabled bool) *systemNotifications {
	return &systemNotifications{backend: b, prefix: prefix, enabled: enabled, meta: make(map[tabs.TabID]notificationMeta), entries: make(map[tabs.TabID]systemNotice), pending: make(map[tabs.TabID]systemWork), inputs: make(map[tabs.TabID]notificationInput), wake: make(chan struct{}, 1), stop: make(chan struct{}), activations: make(chan string, 1)}
}

func notificationPrefix() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("notification identity: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func (s *systemNotifications) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *systemNotifications) start() {
	go func() {
		for {
			select {
			case <-s.stop:
				return
			case <-s.wake:
				s.flush()
			}
		}
	}()
}

func (s *systemNotifications) metadata(id tabs.TabID, m notificationMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.meta[id] = m
	}
}

// note carries prompt identity forward even when a minimized window cannot
// drain its UI inbox. All inputs here are immutable tab fields or parsed hooks.
func (s *systemNotifications) note(m notify.Msg, t *tabs.Tab) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	id := t.ID
	meta, ok := s.meta[id]
	if !ok {
		title := t.FixedTitle
		if title == "" {
			title = folderName(t.Dir)
		}
		if title == "" {
			title = m.Agent
		}
		if title == "" {
			title = "session"
		}
		meta = notificationMeta{title: title, tab: t, named: t.FixedTitle != ""}
	}
	switch m.Kind {
	case notify.KindPrompt:
		s.removeLocked(id)
		if m.Command != "" {
			meta.command = cleanReason(m.Command)
			if arg := firstWord(m.Args, maxAgentArg); arg != "" && !meta.named {
				meta.title = arg
			}
		}
	case notify.KindSession:
		meta.command = ""
		if !meta.named {
			meta.title = m.Agent
			if meta.title == "" {
				meta.title = "claude"
			}
		}
	}
	meta.context = notificationContext(meta.branch, meta.command)
	s.meta[id] = meta
}

func notificationContext(branch, command string) string {
	if branch != "" && command != "" {
		return branch + " · " + command
	}
	return branch + command
}

func (s *systemNotifications) focus(f bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.focused = f
	if f {
		for id, q := range s.pending {
			if !q.remove {
				s.removeLocked(id)
			}
		}
	}
}

func (s *systemNotifications) setEnabled(on bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.enabled = on
	s.err = ""
	if !on {
		for id := range s.entries {
			s.removeLocked(id)
		}
	}
}

func (s *systemNotifications) notify(m notify.Msg) {
	if s == nil || (m.Kind != notify.KindNeeds && m.Kind != notify.KindDone) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.enabled || s.focused {
		return
	}
	id := tabs.TabID(m.Tab)
	meta, ok := s.meta[id]
	if !ok {
		return
	}
	if _, exists := s.entries[id]; !exists && len(s.entries) == systemNotificationLimit {
		var oldest tabs.TabID
		var seq uint64
		for key, e := range s.entries {
			if oldest == 0 || e.seq < seq {
				oldest, seq = key, e.seq
			}
		}
		e := s.entries[oldest]
		s.retire(e.submitted)
		s.retire(e.previous)
		delete(s.entries, oldest)
		delete(s.pending, oldest)
	}
	s.seq++
	reason := cleanReason(m.Reason)
	status := cardStatus(m.Kind, reason, 0, false)
	body := status
	if reason != "" {
		body = status + "\n" + reason
	}
	n := sysnotify.Notification{ID: fmt.Sprintf("%s-%x-%x", s.prefix, id, s.seq), Title: cleanReason(meta.title), Context: cleanReason(meta.context), Body: body}
	old := s.entries[id]
	s.entries[id] = systemNotice{n: n, meta: meta, seq: s.seq, sent: old.sent, submitted: old.submitted, previous: old.previous}
	s.enqueue(id, systemWork{n: n, seq: s.seq})
}

// retire is only for already-visible notifications evicted by the fixed bound.
// At most three identities can belong to one submitted/in-flight notification.
func (s *systemNotifications) retire(id string) {
	for _, old := range s.retired {
		if old == id {
			return
		}
	}
	if id != "" && len(s.retired) < 3*systemNotificationLimit {
		s.retired = append(s.retired, id)
	}
}
func (s *systemNotifications) enqueue(id tabs.TabID, w systemWork) {
	if old, ok := s.pending[id]; ok && old.remove && !w.remove {
		// A fresh alert may overtake a queued visit/disable withdrawal. Retain
		// that cleanup even if the fresh post is canceled or fails later.
		s.retire(old.n.ID)
		s.retire(old.submitted)
		s.retire(old.previous)
	}
	if _, ok := s.pending[id]; !ok && len(s.pending) == systemNotificationLimit {
		var oldest tabs.TabID
		var seq uint64
		for key, q := range s.pending {
			if oldest == 0 || q.seq < seq {
				oldest, seq = key, q.seq
			}
		}
		q := s.pending[oldest]
		e := s.entries[oldest]
		if q.remove {
			s.retire(q.n.ID)
			s.retire(q.submitted)
			s.retire(q.previous)
		} else {
			s.retire(e.submitted)
			s.retire(e.previous)
		}
		delete(s.pending, oldest)
		delete(s.entries, oldest)
	}
	s.pending[id] = w
	s.signal()
}
func (s *systemNotifications) removeLocked(id tabs.TabID) {
	e, ok := s.entries[id]
	if !ok {
		return
	}
	delete(s.entries, id)
	// Remove also cancels a backend's permission-pending desired notification.
	s.seq++
	s.enqueue(id, systemWork{n: e.n, remove: true, seq: s.seq, submitted: e.submitted, previous: e.previous})
}
func (s *systemNotifications) remove(id tabs.TabID) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.removeLocked(id)
	}
}

// flush is the serial worker turn; tests call it directly, never a native API.
func (s *systemNotifications) flush() {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		if len(s.retired) > 0 {
			id := s.retired[0]
			s.retired = s.retired[1:]
			s.mu.Unlock()
			s.backend.Remove(id)
			continue
		}
		var id tabs.TabID
		var q systemWork
		for key, w := range s.pending {
			if id == 0 || w.seq < q.seq {
				id, q = key, w
			}
		}
		if id == 0 {
			s.mu.Unlock()
			return
		}
		delete(s.pending, id)
		if !q.remove && (!s.enabled || s.focused || s.entries[id].n.ID != q.n.ID) {
			s.mu.Unlock()
			continue
		}
		var previous string
		if !q.remove {
			e := s.entries[id]
			e.sent = true
			previous = e.submitted
			e.previous, e.submitted = previous, q.n.ID
			s.entries[id] = e
		}
		s.mu.Unlock()
		if q.remove {
			if q.previous != "" {
				s.backend.Remove(q.previous)
			}
			if q.submitted != "" {
				s.backend.Remove(q.submitted)
			}
			s.backend.Remove(q.n.ID)
			continue
		}
		err := s.backend.Post(q.n)
		s.mu.Lock()
		if e, ok := s.entries[id]; ok && e.submitted == q.n.ID {
			if err != nil {
				e.submitted = previous
			}
			e.previous = ""
			s.entries[id] = e
		}
		if err != nil {
			s.err = "System notifications: " + cleanReason(err.Error())
		} else {
			s.err = ""
		}
		s.mu.Unlock()
		if err != nil && s.changed != nil {
			s.changed()
		}
	}
}

func (s *systemNotifications) activate(id string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.enabled {
		return
	}
	for _, e := range s.entries {
		if e.n.ID == id {
			select {
			case s.activations <- id:
			default:
			}
			return
		}
	}
}
func (s *systemNotifications) activation(id string) (tabs.TabID, notificationMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.enabled {
		for tab, e := range s.entries {
			if e.n.ID == id {
				return tab, e.meta, true
			}
		}
	}
	return 0, notificationMeta{}, false
}
func (s *systemNotifications) status() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	err := s.err
	s.mu.Unlock()
	if err != "" {
		return err
	}
	return s.backend.Status()
}
func (s *systemNotifications) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	clear(s.entries)
	clear(s.pending)
	clear(s.meta)
	s.retired = nil
	close(s.stop)
	s.mu.Unlock()
	s.backend.Close()
}

func (win *window) publishSystemMetadata() {
	s := win.system
	if s == nil || win.mgr == nil {
		return
	}
	win.systemTabs = win.mgr.TabsInto(win.systemTabs[:0])
	for _, t := range win.systemTabs {
		loc := win.locOf(t)
		a := win.rowAgent(t.ID)
		key := notificationInput{tab: t, title: t.Title, fixed: t.FixedTitle, name: win.names[t.ID], home: win.home, def: win.defTitle, exited: t.Exited, loc: loc, agent: a}
		if old, ok := s.inputs[t.ID]; ok && old == key {
			continue
		}
		s.inputs[t.ID] = key
		in := sideInput{Home: win.home, DefTitle: win.defTitle, Name: func(id tabs.TabID) string { return win.names[id] }}
		if win.ui != nil {
			in.Clean = win.ui.fit
		}
		title := in.label(t, loc, a)
		if title == "" {
			title = "session"
		}
		cmd := win.agentOf(t.ID).command
		s.metadata(t.ID, notificationMeta{title: title, context: notificationContext(loc.branch, cmd), tab: t, branch: loc.branch, command: cmd, named: key.name != "" || t.FixedTitle != ""})
	}
	s.mu.Lock()
	for id := range s.meta {
		if !win.mgr.Has(id) {
			delete(s.meta, id)
			delete(s.inputs, id)
			s.removeLocked(id)
		}
	}
	s.mu.Unlock()
}

func (win *window) drainSystemActivations() {
	s := win.system
	if s == nil {
		return
	}
	for {
		select {
		case id := <-s.activations:
			win.systemActivation(id)
		default:
			return
		}
	}
}

func (win *window) systemActivation(id string) {
	s := win.system
	tab, meta, ok := s.activation(id)
	if !ok || win.mgr == nil || win.mgr.Lookup(tab) != meta.tab {
		return
	}
	s.backend.Raise()
	win.toastGo(tab)
	s.remove(tab)
}

// Only Run calls this. Tests use an injected fake and bench runs never create
// a backend, register an application or ask the OS for permission.
func (win *window) initSystemNotifications(factory func(sysnotify.Callbacks) sysnotify.Backend) {
	if win.bench != nil {
		return
	}
	s := newSystemNotifications(nil, notificationPrefix(), win.opt.Config.SystemNotifications)
	s.focused = win.focused
	s.changed = win.invalidate
	callbacks := sysnotify.Callbacks{Activate: s.activate, Changed: win.invalidate}
	if win.w != nil {
		callbacks.Run = win.w.Run
	}
	s.backend = factory(callbacks)
	win.system = s
	s.start()
}

func (win *window) systemNotificationHint() string {
	hint := "When Kaze is in the background. Click a notification to open its session."
	if state := win.system.status(); state != "" {
		hint += " " + strings.TrimSpace(state)
	}
	return hint
}
