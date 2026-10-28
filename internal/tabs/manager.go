// Package tabs owns the open terminal tabs: their shells, order, the active
// tab and which tab may ask the window for a frame.
package tabs

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/patrick-gogo/kaze/internal/ptyx"
	"github.com/patrick-gogo/kaze/internal/term"
)

// TabID identifies a tab for its lifetime; IDs are never reused.
type TabID int

// Tab is one shell. Title and Exited are refreshed by Tabs and Active, so read
// them only on the goroutine that calls those (the UI goroutine).
type Tab struct {
	ID      TabID
	Session *term.Session
	Title   string
	Exited  bool
}

// entry holds what the session's notifier goroutine needs without taking the manager lock.
type entry struct {
	tab     *Tab
	sess    atomic.Pointer[term.Session]
	removed atomic.Bool
	// Touched only by this session's single notifier goroutine.
	seenTitle  string
	seenExited bool
}

// Manager is driven from the UI goroutine; onChange may be called from any goroutine.
type Manager struct {
	start    func(env []string) (ptyx.Session, error)
	onChange func()
	baseEnv  []string

	mu      sync.Mutex
	entries []*entry
	nextID  TabID
	cols    int
	rows    int

	active atomic.Int64 // TabID of the active tab, 0 when there is none
}

// NewManager starts no shell yet. start launches one shell with the given env;
// onChange asks the window for a frame.
func NewManager(start func(env []string) (ptyx.Session, error), cols, rows int, onChange func()) *Manager {
	if onChange == nil {
		onChange = func() {}
	}
	return &Manager{start: start, onChange: onChange, baseEnv: baseEnv(os.Environ()), cols: cols, rows: rows}
}

// baseEnv drops an inherited KAZE_TAB_ID so a kaze started inside a kaze tab gets clean IDs.
func baseEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if len(e) >= 12 && strings.EqualFold(e[:12], "KAZE_TAB_ID=") { // Windows env names ignore case
			continue
		}
		out = append(out, e)
	}
	return out
}

// New starts a shell in a new tab at the end and makes it active.
func (m *Manager) New() (*Tab, error) {
	m.mu.Lock()
	m.nextID++
	id := m.nextID
	cols, rows := m.cols, m.rows
	m.mu.Unlock()

	env := append(slices.Clip(m.baseEnv), "KAZE_TAB_ID="+strconv.Itoa(int(id)))
	p, err := m.start(env)
	if err != nil {
		return nil, err
	}
	e := &entry{tab: &Tab{ID: id}}
	s := term.NewSession(p, cols, rows, func() { m.notify(e) })
	e.sess.Store(s)
	e.tab.Session = s

	m.mu.Lock()
	m.entries = append(m.entries, e)
	m.mu.Unlock()
	m.active.Store(int64(id))
	return e.tab, nil
}

// notify runs on the tab's notifier goroutine. A background tab keeps applying
// output but only asks for a frame when the tab bar would change.
func (m *Manager) notify(e *entry) {
	if e.removed.Load() {
		return
	}
	if TabID(m.active.Load()) == e.tab.ID {
		m.onChange()
		return
	}
	s := e.sess.Load()
	if s == nil {
		return
	}
	exited, _ := s.Exited()
	s.Lock()
	title := s.Applier().Title()
	s.Unlock()
	if title != e.seenTitle || exited != e.seenExited {
		e.seenTitle, e.seenExited = title, exited
		m.onChange()
	}
}

func refresh(t *Tab) {
	t.Exited, _ = t.Session.Exited()
	t.Session.Lock()
	t.Title = t.Session.Applier().Title()
	t.Session.Unlock()
}

// Tabs returns the tabs in display order with Title and Exited refreshed.
func (m *Manager) Tabs() []*Tab {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Tab, len(m.entries))
	for i, e := range m.entries {
		refresh(e.tab)
		out[i] = e.tab
	}
	return out
}

// Active is the active tab, or nil when none are open.
func (m *Manager) Active() *Tab {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.indexLocked(TabID(m.active.Load()))
	if i < 0 {
		return nil
	}
	t := m.entries[i].tab
	refresh(t)
	return t
}

func (m *Manager) indexLocked(id TabID) int {
	for i, e := range m.entries {
		if e.tab.ID == id {
			return i
		}
	}
	return -1
}

// SetActive switches to id; an unknown id is ignored.
func (m *Manager) SetActive(id TabID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.indexLocked(id) >= 0 {
		m.active.Store(int64(id))
	}
}

func (m *Manager) step(d int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.entries)
	if n == 0 {
		return
	}
	i := m.indexLocked(TabID(m.active.Load()))
	i = ((i+d)%n + n) % n
	m.active.Store(int64(m.entries[i].tab.ID))
}

// Next and Prev move the active tab, wrapping at the ends.
func (m *Manager) Next() { m.step(1) }
func (m *Manager) Prev() { m.step(-1) }

// Close removes the tab and kills its shell in the background (ConPTY can take up
// to a second to release). Closing the active tab activates its right neighbour,
// or its left one when it was last.
func (m *Manager) Close(id TabID) {
	m.mu.Lock()
	i := m.indexLocked(id)
	if i < 0 {
		m.mu.Unlock()
		return
	}
	e := m.entries[i]
	e.removed.Store(true)
	m.entries = slices.Delete(m.entries, i, i+1)
	if TabID(m.active.Load()) == id {
		switch {
		case len(m.entries) == 0:
			m.active.Store(0)
		case i < len(m.entries):
			m.active.Store(int64(m.entries[i].tab.ID))
		default:
			m.active.Store(int64(m.entries[i-1].tab.ID))
		}
	}
	m.mu.Unlock()
	go e.tab.Session.Close()
}

// CloseAll closes every tab and waits for the shells to be released.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	es := m.entries
	m.entries = nil
	m.active.Store(0)
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, e := range es {
		e.removed.Store(true)
		wg.Add(1)
		go func() { defer wg.Done(); e.tab.Session.Close() }()
	}
	wg.Wait()
}

// Empty reports that the last tab was closed; the window quits then.
func (m *Manager) Empty() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries) == 0
}

// Size is the grid size every tab has, and new tabs start with.
func (m *Manager) Size() (cols, rows int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cols, m.rows
}

// ResizeAll resizes every tab's screen and PTY; an unchanged size does nothing.
func (m *Manager) ResizeAll(cols, rows int) {
	m.mu.Lock()
	if cols == m.cols && rows == m.rows {
		m.mu.Unlock()
		return
	}
	m.cols, m.rows = cols, rows
	es := slices.Clone(m.entries)
	m.mu.Unlock()
	for _, e := range es {
		e.tab.Session.Resize(cols, rows)
	}
}
