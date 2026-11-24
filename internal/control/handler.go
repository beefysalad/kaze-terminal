package control

import (
	"context"
	"strings"
)

// AllowedKeys lists the case-insensitive control key names.
const AllowedKeys = "enter, esc, tab, up, down, left, right, backspace, ctrl+c, ctrl+d, ctrl+l"

// ValidKey reports whether name is in the control allowlist.
func ValidKey(name string) bool {
	switch strings.ToLower(name) {
	case "enter", "esc", "tab", "up", "down", "left", "right", "backspace", "ctrl+c", "ctrl+d", "ctrl+l":
		return true
	}
	return false
}

// Handle executes one authenticated request synchronously. It starts no workers
// except for wait requests. Host implementations own terminal access and submit sequencing.
func Handle(host Host, req Request) Reply {
	return HandleContext(context.Background(), host, req)
}

// HandleContext cancels pending waits when the transport disconnects.
func HandleContext(ctx context.Context, host Host, req Request) Reply {
	if req.Op == "wait" {
		return wait(ctx, host, req)
	}
	if req.Op == "keys" || req.Op == "interrupt" {
		if req.Op == "keys" {
			if len(req.Args) == 0 || len(req.Args) > MaxKeys {
				return Reply{Error: &Error{CodeInvalidRequest, "keys requires 1 to 64 key names"}}
			}
			for _, name := range req.Args {
				if !ValidKey(name) {
					return Reply{Error: &Error{CodeUnknownKey, "allowed keys: " + AllowedKeys}}
				}
			}
		}
		tab, err := Resolve(host.Tabs(), req.Tab)
		if err != nil {
			return Reply{Error: err}
		}
		var hostErr error
		if req.Op == "keys" {
			hostErr = host.Keys(tab.ID, req.Args)
		} else {
			hostErr = host.Interrupt(tab.ID, req.CtrlC)
		}
		if hostErr != nil {
			return Reply{Error: &Error{CodeHostError, "could not submit to tab"}}
		}
		return Reply{OK: true}
	}
	if req.Op == "ls" {
		return Reply{OK: true, Tabs: host.Tabs()}
	}
	if req.Op == "read" {
		if req.Lines < 0 {
			return Reply{Error: &Error{CodeInvalidRequest, "lines must not be negative"}}
		}
		tab, err := Resolve(host.Tabs(), req.Tab)
		if err != nil {
			return Reply{Error: err}
		}
		text, hostErr := host.Screen(tab.ID, min(req.Lines, MaxLines))
		if hostErr != nil {
			return Reply{Error: &Error{CodeHostError, "could not read tab"}}
		}
		return Reply{OK: true, Text: text}
	}
	if req.Op == "send" {
		if len(req.Text) > MaxTextBytes {
			return Reply{Error: &Error{CodeTextTooLarge, "text exceeds 32 KiB"}}
		}
		tab, err := Resolve(host.Tabs(), req.Tab)
		if err != nil {
			return Reply{Error: err}
		}
		if err := host.Submit(tab.ID, []byte(req.Text)); err != nil {
			return Reply{Error: &Error{CodeHostError, "could not submit to tab"}}
		}
		return Reply{OK: true}
	}
	return Reply{Error: &Error{CodeUnknownOp, "unknown control operation"}}
}

// Resolve finds an exact id first, otherwise an exact, unique nonempty name.
// It does not normalize names or interpret a name as a numeric tab index.
func Resolve(tabs []TabInfo, address string) (TabInfo, *Error) {
	if address == "" {
		return TabInfo{}, &Error{CodeInvalidRequest, "tab address is required"}
	}
	for _, tab := range tabs {
		if tab.ID == address {
			return tab, nil
		}
	}
	var found TabInfo
	matched := false
	for _, tab := range tabs {
		if tab.Name != address {
			continue
		}
		if matched {
			return TabInfo{}, &Error{CodeAmbiguousTab, "tab name is ambiguous"}
		}
		found, matched = tab, true
	}
	if !matched {
		return TabInfo{}, &Error{CodeUnknownTab, "unknown tab"}
	}
	return found, nil
}
