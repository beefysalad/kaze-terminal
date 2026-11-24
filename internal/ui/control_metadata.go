package ui

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const controlRenameTimeout = 2 * time.Second

type controlRename struct {
	ctx      context.Context
	id, name string
	done     chan *ctl.Error
}

func (win *window) initControl() {
	w := win.w
	win.control.wake = func() {
		if w != nil {
			w.Invalidate()
		}
	}
}

// Name queues a rename, waiting only while this request is active. The socket
// worker never reads the window; its wake callback only invalidates, as onNotify does.
func (h *controlHost) Name(ctx context.Context, id, name string) *ctl.Error {
	ctx, cancel := context.WithTimeout(ctx, controlRenameTimeout)
	defer cancel()
	r := &controlRename{ctx: ctx, id: id, name: name, done: make(chan *ctl.Error, 1)}
	h.renameMu.Lock()
	if len(h.renames) >= 32 {
		h.renameMu.Unlock()
		return &ctl.Error{Code: ctl.CodeHostError, Message: "rename queue is full"}
	}
	h.renames = append(h.renames, r)
	h.renameMu.Unlock()
	defer func() {
		h.renameMu.Lock()
		if i := slices.Index(h.renames, r); i >= 0 {
			h.renames = slices.Delete(h.renames, i, i+1)
		}
		h.renameMu.Unlock()
	}()
	if h.wake != nil {
		h.wake()
	}
	select {
	case e := <-r.done:
		return e
	case <-ctx.Done():
		return &ctl.Error{Code: "timeout", Message: "rename was not applied within 2 seconds"}
	}
}

// drainControl runs in the frame on the UI goroutine, before models are built.
func (win *window) drainControl() {
	h := &win.control
	h.renameMu.Lock()
	pending := h.renames
	h.renames = nil
	h.renameMu.Unlock()
	for _, r := range pending {
		if r.ctx.Err() != nil {
			continue
		}
		name, e := ctl.ValidateName(h.Tabs(), r.id, r.name)
		if e == nil {
			n, err := strconv.Atoi(strings.TrimPrefix(r.id, "t"))
			var tab *tabs.Tab
			if err == nil && win.mgr != nil {
				tab = win.mgr.Lookup(tabs.TabID(n))
			}
			if tab == nil {
				e = &ctl.Error{Code: ctl.CodeUnknownTab, Message: "unknown tab"}
			} else {
				if tab.FixedTitle != "" {
					tab.FixedTitle = name
				} else {
					if win.names == nil {
						win.names = map[tabs.TabID]string{}
					}
					win.names[tab.ID] = name
				}
				if win.restore != nil {
					win.restore.Rename(int(tab.ID), name)
				}
				win.sideRedraw()
				win.publishControl()
			}
		}
		r.done <- e
	}
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[k] = v
	}
	return result
}

// Label changes only host-owned state. Copy-on-write publication lets ls see it
// immediately without touching the UI or requesting an otherwise unneeded frame.
func (h *controlHost) Label(id, key, value string) *ctl.Error {
	if e := ctl.ValidateLabel(key, value); e != nil {
		return e
	}
	h.labelsMu.Lock()
	defer h.labelsMu.Unlock()
	old := h.snapshot.Load()
	if old == nil || old.sessions[id] == nil {
		return &ctl.Error{Code: ctl.CodeUnknownTab, Message: "unknown tab"}
	}
	labels := h.labels[id]
	if _, exists := labels[key]; !exists && len(labels) >= 8 {
		return &ctl.Error{Code: ctl.CodeInvalidRequest, Message: "tab already has 8 labels"}
	}
	if h.labels == nil {
		h.labels = map[string]map[string]string{}
	}
	if labels == nil {
		labels = map[string]string{}
		h.labels[id] = labels
	}
	labels[key] = value
	next := &controlSnapshot{tabs: slices.Clone(old.tabs), sessions: old.sessions}
	for i := range next.tabs {
		if next.tabs[i].ID == id {
			next.tabs[i].Labels = cloneLabels(labels)
		}
	}
	h.publish(next)
	return nil
}
