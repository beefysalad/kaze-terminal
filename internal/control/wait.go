package control

import (
	"context"
	"strings"
	"time"
)

const DefaultWaitTimeout = 120
const MaxWaitTimeout = 600
const DefaultQuiet = 1500

// WaitHost provides a consistent snapshot and its next change signal. The
// channel is closed on publication; callers subscribe before checking state.
// OutputChanges installs a nonblocking observer and returns the last output time.
type WaitHost interface {
	StatusChanges() ([]TabInfo, <-chan struct{})
	OutputChanges(id string) (<-chan struct{}, func(), time.Time, error)
}

func wait(ctx context.Context, host Host, req Request, opt Options) Reply {
	if req.Timeout < 0 || req.Timeout > MaxWaitTimeout || req.Quiet < 0 || req.Quiet > int((1<<63-1)/int64(time.Millisecond)) || len(req.For) > MaxTextBytes {
		return Reply{Error: &Error{CodeInvalidRequest, "invalid wait limits"}}
	}
	if req.Children != "" {
		return waitChildren(ctx, host, req)
	}
	h, ok := host.(WaitHost)
	if !ok {
		return Reply{Error: &Error{CodeHostError, "wait unavailable"}}
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = DefaultWaitTimeout
	}
	quiet := req.Quiet
	if quiet == 0 {
		quiet = DefaultQuiet
	}
	quietDuration := time.Duration(quiet) * time.Millisecond
	deadline := time.NewTimer(time.Duration(timeout) * time.Second)
	defer deadline.Stop()
	var output <-chan struct{}
	var remove func()
	defer func() {
		if remove != nil {
			remove()
		}
	}()
	var quietTimer *time.Timer
	var quietC <-chan time.Time
	quietReady := false
	defer func() {
		if quietTimer != nil {
			quietTimer.Stop()
		}
	}()
	var id string
	for {
		tabs, changed := h.StatusChanges()
		address := id
		if address == "" {
			address = req.Tab
		}
		tab, _, err := resolveScoped(tabs, address, req.From, opt.CrossSpace)
		if err != nil {
			return Reply{Error: err}
		}
		id = tab.ID
		if req.For == "" && (tab.Status == "idle" || tab.Status == "needs") {
			return Reply{OK: true}
		}
		if output == nil && (req.For != "" || quietStatus(tab.Status)) {
			var last time.Time
			var hostErr error
			output, remove, last, hostErr = h.OutputChanges(id)
			if hostErr != nil {
				return Reply{Error: &Error{CodeHostError, "could not observe tab"}}
			}
			if req.For == "" {
				delay := quietDuration
				if !last.IsZero() {
					delay = max(0, quietDuration-time.Since(last))
				}
				quietTimer = time.NewTimer(delay)
				quietC = quietTimer.C
			}
		}
		if quietReady && quietStatus(tab.Status) {
			select {
			case <-output:
				quietTimer.Reset(quietDuration)
				quietReady = false
			default:
				return Reply{OK: true}
			}
		}
		if req.For != "" {
			text, hostErr := host.Screen(id, 200)
			if hostErr != nil {
				return Reply{Error: &Error{CodeHostError, "could not read tab"}}
			}
			if strings.Contains(text, req.For) {
				return Reply{OK: true}
			}
		}
		select {
		case <-ctx.Done():
			return Reply{Error: &Error{"cancelled", "wait cancelled"}}
		case <-deadline.C:
			return Reply{Error: &Error{"timeout", "wait timeout"}}
		case <-changed:
		case <-output:
			if quietTimer != nil {
				quietTimer.Reset(quietDuration)
				quietReady = false
			}
		case <-quietC:
			// Re-read the snapshot before deciding: a status publication may have
			// raced with timer expiry. Pending output also takes precedence above.
			quietReady = true
		}
	}
}

// quietStatus marks tabs with no hook state, a plain shell or an agent found
// only by process detection, which are waited on by output quiescence.
func quietStatus(status string) bool { return status == "shell" || status == "unknown" }

// InboxHost consumes only messages visible in the snapshot that satisfied a wait.
// Later arrivals remain unread. Implementations synchronize without touching UI.
type InboxHost interface {
	ConsumeInbox(id string, through uint64) []InboxMessage
}

func waitChildren(ctx context.Context, host Host, req Request) Reply {
	if (req.Children != "all" && req.Children != "any") || req.Tab != "" || req.For != "" || req.Quiet != 0 {
		return Reply{Error: &Error{CodeInvalidRequest, "invalid children wait"}}
	}
	h, ok := host.(WaitHost)
	if !ok {
		return Reply{Error: &Error{CodeHostError, "wait unavailable"}}
	}
	inbox, ok := host.(InboxHost)
	if !ok {
		return Reply{Error: &Error{CodeHostError, "inbox unavailable"}}
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = DefaultWaitTimeout
	}
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()
	expired := false
	for {
		tabs, changed := h.StatusChanges()
		var children []TabInfo
		var parent TabInfo
		stopped := 0
		for _, tab := range tabs {
			if tab.ID == req.From {
				parent = tab
			}
			if tab.Parent != req.From || tab.Role != "worker" {
				continue
			}
			children = append(children, tab)
			if tab.Status == "idle" || tab.Status == "needs" || tab.Status == "exited" {
				stopped++
			}
		}
		if parent.ID == "" {
			return Reply{Error: &Error{CodeUnknownTab, "parent is closed"}}
		}
		reason := ""
		if len(parent.Inbox) > 0 {
			reason = parent.Inbox[0].Kind
		} else if req.Children == "all" && stopped == len(children) {
			reason = "all_stopped"
		} else if req.Children == "any" && stopped > 0 {
			reason = "any_stopped"
		}
		if reason != "" {
			r := Reply{OK: true, Tabs: children, Reason: reason}
			if len(parent.Inbox) > 0 {
				messages := inbox.ConsumeInbox(req.From, parent.Inbox[len(parent.Inbox)-1].Sequence)
				// Another concurrent wait may already have consumed this snapshot.
				if len(messages) == 0 {
					continue
				}
				r.Reason = messages[0].Kind
				parts := make([]string, len(messages))
				for i, m := range messages {
					parts[i] = m.Text
				}
				r.Text = strings.Join(parts, "\n")
			}
			return r
		}
		if expired {
			return Reply{Reason: "timeout", Tabs: children, Error: &Error{"timeout", "wait timeout"}}
		}
		select {
		case <-ctx.Done():
			return Reply{Error: &Error{"cancelled", "wait cancelled"}}
		case <-timer.C:
			expired = true
		case <-changed:
		}
	}
}
