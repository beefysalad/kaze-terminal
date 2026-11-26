package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const controlNoteQuiet = 5 * time.Second
const controlInboxLimit = 64
const controlInboxBytes = 32 << 10

type inboxEntry struct {
	message   ctl.InboxMessage
	delivered bool
}
type controlInbox struct {
	sequence uint64
	entries  []inboxEntry
}

// inboxMessages runs with labelsMu held. Every publication owns its slice.
func (h *controlHost) inboxMessages(id string) []ctl.InboxMessage {
	box := h.inbox[id]
	if box == nil || len(box.entries) == 0 {
		return nil
	}
	out := make([]ctl.InboxMessage, len(box.entries))
	for i, e := range box.entries {
		out[i] = e.message
	}
	return out
}
func (h *controlHost) publishInboxLocked() {
	old := h.snapshot.Load()
	if old == nil {
		return
	}
	next := *old
	next.tabs = slices.Clone(old.tabs)
	for i := range next.tabs {
		tab := &next.tabs[i]
		tab.Inbox = h.inboxMessages(tab.ID)
		tab.Unread = len(tab.Inbox)
	}
	h.publish(&next)
}

// EnqueueInbox runs on socket or UI goroutines and owns no window state.
// Bounded storage refuses excess reports instead of growing without a limit.
func (h *controlHost) EnqueueInbox(id, kind, text string) error {
	h.labelsMu.Lock()
	s := h.snapshot.Load()
	if s == nil || s.sessions[id] == nil {
		h.labelsMu.Unlock()
		return fmt.Errorf("parent is closed")
	}
	if h.inbox == nil {
		h.inbox = make(map[string]*controlInbox)
	}
	box := h.inbox[id]
	if box == nil {
		box = &controlInbox{}
		h.inbox[id] = box
	}
	size := len(text)
	for _, entry := range box.entries {
		size += len(entry.message.Text)
	}
	if len(box.entries) >= controlInboxLimit || size > controlInboxBytes {
		h.labelsMu.Unlock()
		return fmt.Errorf("parent inbox is full")
	}
	box.sequence++
	box.entries = append(box.entries, inboxEntry{message: ctl.InboxMessage{Sequence: box.sequence, Kind: kind, Text: text}})
	h.publishInboxLocked()
	h.labelsMu.Unlock()
	if h.wake != nil {
		h.wake()
	}
	return nil
}
func (h *controlHost) ConsumeInbox(id string, through uint64) []ctl.InboxMessage {
	h.labelsMu.Lock()
	var out []ctl.InboxMessage
	if box := h.inbox[id]; box != nil {
		n := 0
		for _, entry := range box.entries {
			if entry.message.Sequence > through {
				break
			}
			out = append(out, entry.message)
			n++
		}
		box.entries = slices.Delete(box.entries, 0, n)
		if n > 0 {
			h.publishInboxLocked()
		}
	}
	h.labelsMu.Unlock()
	if len(out) > 0 && h.wake != nil {
		h.wake()
	}
	return out
}
func (h *controlHost) recordOwnerKey(id tabs.TabID, now time.Time) {
	if s := h.snapshot.Load(); s != nil {
		if target := s.sessions[workerID(id)]; target != nil {
			target.lastKey.Store(now.UnixNano())
		}
	}
}
func (win *window) workerNote(id tabs.TabID, w *worker, event string) {
	if event == "exited" && w.killedByParent {
		w.killedByParent = false
		return
	}
	if w.noted[event] {
		return
	}
	name := win.names[id]
	if name == "" && win.mgr != nil {
		if tab := win.mgr.Lookup(id); tab != nil {
			name = tab.FixedTitle
		}
	}
	child := workerID(id)
	label := child
	if name != "" {
		label += " (" + name + ")"
	}
	var body string
	switch event {
	case "blocked":
		body = label + " is waiting for an approval prompt. The owner has to answer it in that tab."
	case "done":
		body = label + " finished working and is idle."
	case "exited":
		body = label + " exited. Its screen remains available while the tab is open."
	case "not_started":
		body = label + " first message was not confirmed. Check the worker tab for a trust dialog or missing hooks."
	}
	// Names are validated single-line text. Escape XML metacharacters in note text.
	body = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(body)
	text := `<kaze from="kaze" kind="note" child="` + child + `">` + body + `</kaze>`
	if win.control.EnqueueInbox(workerID(w.parent), "note", text) == nil {
		if w.noted == nil {
			w.noted = map[string]bool{}
		}
		w.noted[event] = true
	}
}

// noteTick runs on the UI goroutine. Only an idle parent inside its quiet
// window requests a future frame. A working parent waits for a normal hook.
// submit is injected for tests; nil checks readiness without sending input.
func (win *window) noteTick(now time.Time, submit func(string, []byte) error) (wake time.Time, ok bool) {
	h := &win.control
	h.labelsMu.Lock()
	defer h.labelsMu.Unlock()
	if len(h.inbox) == 0 {
		return
	}
	s := h.snapshot.Load()
	if s == nil {
		return
	}
	for _, tab := range s.tabs {
		box := h.inbox[tab.ID]
		if box == nil {
			continue
		}
		pending := false
		for _, entry := range box.entries {
			if !entry.delivered {
				pending = true
				break
			}
		}
		if !pending {
			continue
		}
		target := s.sessions[tab.ID]
		if target == nil || target.noteSending.Load() || tab.Status != "idle" {
			continue
		}
		at := target.pending.Load()
		if at != 0 && now.UnixNano()-at < int64(controlPendingGrace) {
			due := time.Unix(0, at).Add(controlPendingGrace)
			if !ok || due.Before(wake) {
				wake, ok = due, true
			}
			continue
		}
		due := now
		if keyAt := target.lastKey.Load(); keyAt != 0 {
			quiet := win.noteQuiet
			if quiet == 0 {
				quiet = controlNoteQuiet
			}
			due = time.Unix(0, keyAt).Add(quiet)
		}
		if due.After(now) || submit == nil {
			if due.Before(now) {
				due = now
			}
			if !ok || due.Before(wake) {
				wake, ok = due, true
			}
			continue
		}
		var parts []string
		for i := range box.entries {
			if !box.entries[i].delivered {
				parts = append(parts, box.entries[i].message.Text)
				box.entries[i].delivered = true
			}
		}
		text := strings.Join(parts, "\n")
		through := box.entries[len(box.entries)-1].message.Sequence
		id := tab.ID
		target.noteSending.Store(true)
		go func() {
			err := submit(id, []byte(text))
			if err == errNoteDeferred {
				h.labelsMu.Lock()
				if current := h.inbox[id]; current != nil {
					for i := range current.entries {
						if current.entries[i].message.Sequence <= through {
							current.entries[i].delivered = false
						}
					}
				}
				h.labelsMu.Unlock()
			}
			target.noteSending.Store(false)
			// A new message may have arrived while this submit was in flight.
			if h.wake != nil {
				h.wake()
			}
		}()
	}
	return
}

var errNoteDeferred = errors.New("parent is not idle and quiet")

// submitNote checks again after taking the send lock, because a hook, owner
// key, or another submit can arrive after the UI decided to deliver a note.
// It reads only immutable snapshots and synchronized session data.
func (h *controlHost) submitNote(id string, text []byte) error {
	target, err := h.session(id)
	if err != nil {
		return err
	}
	target.send.Lock()
	defer target.send.Unlock()
	now := time.Now()
	last := target.lastKey.Load()
	quiet := h.noteQuiet
	if quiet == 0 {
		quiet = controlNoteQuiet
	}
	if last != 0 && now.Sub(time.Unix(0, last)) < quiet {
		return errNoteDeferred
	}
	idle := false
	for _, tab := range h.Tabs() {
		if tab.ID == id {
			idle = tab.Status == "idle"
			break
		}
	}
	if !idle {
		return errNoteDeferred
	}
	return h.submitLocked(id, text, target)
}
