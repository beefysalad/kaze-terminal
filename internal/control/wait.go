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

func wait(ctx context.Context, host Host, req Request) Reply {
	if req.Timeout < 0 || req.Timeout > MaxWaitTimeout || req.Quiet < 0 || req.Quiet > int((1<<63-1)/int64(time.Millisecond)) || len(req.For) > MaxTextBytes {
		return Reply{Error: &Error{CodeInvalidRequest, "invalid wait limits"}}
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
		tab, err := Resolve(tabs, address)
		if err != nil {
			return Reply{Error: err}
		}
		id = tab.ID
		if req.For == "" && (tab.Status == "idle" || tab.Status == "needs") {
			return Reply{OK: true}
		}
		if output == nil && (req.For != "" || tab.Status == "shell") {
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
		if quietReady && tab.Status == "shell" {
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
