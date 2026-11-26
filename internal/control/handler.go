package control

import (
	"bytes"
	"context"
	"fmt"
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
func HandleContext(ctx context.Context, host Host, req Request, options ...Options) Reply {
	opt := Options{Enabled: true, AgentTag: true}
	if len(options) > 0 {
		opt = options[0]
	}
	if !opt.Enabled {
		return Reply{Error: &Error{CodeDisabled, "control is disabled"}}
	}
	if req.Op == "name" || req.Op == "label" {
		return metadata(ctx, host, req)
	}
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
		if len(req.Text) > MaxTextBytes && (!opt.AgentTag || req.From == "") {
			return Reply{Error: &Error{CodeTextTooLarge, "text exceeds 32 KiB"}}
		}
		tabs := host.Tabs()
		tab, err := Resolve(tabs, req.Tab)
		if err != nil {
			return Reply{Error: err}
		}
		if tab.Status == "working" && req.IfIdle {
			return Reply{Error: &Error{CodeBusy, "target is working"}}
		}
		text := req.Text
		if opt.AgentTag && req.From != "" {
			for _, sender := range tabs {
				if sender.ID == req.From {
					from := sender.ID
					if sender.Name != "" {
						from += " " + sender.Name
					}
					text = "<kaze from=\"" + escapeAttribute(from) + "\">" + stripPeerClosingTags(text) + "</kaze>"
					break
				}
			}
		}
		if len(text) > MaxTextBytes {
			return Reply{Error: &Error{CodeTextTooLarge, "text exceeds 32 KiB"}}
		}
		if err := host.Submit(tab.ID, []byte(text)); err != nil {
			return Reply{Error: &Error{CodeHostError, "could not submit to tab"}}
		}
		warning := ""
		if tab.Status == "working" {
			warning = "busy"
		}
		return Reply{OK: true, Status: tab.Status, Warning: warning, Tabs: []TabInfo{tab}}
	}
	return Reply{Error: &Error{CodeUnknownOp, "unknown control operation"}}
}

// Strip while copying so removing one marker also catches a newly joined one.
// Each byte is appended once and removed at most once, even for nested markers.
func stripPeerClosingTags(text string) string {
	const marker = "</kaze>"
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		out = append(out, text[i])
		if text[i] == '>' && bytes.HasSuffix(out, []byte(marker)) {
			out = out[:len(out)-len(marker)]
		}
	}
	return string(out)
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
	if matched {
		return found, nil
	}
	// No tab has that id or name: an agent name (claude, codex) addresses the
	// one tab running it, so an agent that only knows "codex" can still reach it.
	var byAgent []TabInfo
	for _, tab := range tabs {
		if tab.Agent != "" && strings.EqualFold(tab.Agent, address) {
			byAgent = append(byAgent, tab)
		}
	}
	switch len(byAgent) {
	case 1:
		return byAgent[0], nil
	case 0:
		return TabInfo{}, &Error{CodeUnknownTab, fmt.Sprintf("unknown tab %q%s", address, describeTabs(tabs))}
	}
	return TabInfo{}, &Error{CodeAmbiguousTab, fmt.Sprintf("%d tabs run %q, use an id%s", len(byAgent), address, describeTabs(byAgent))}
}

// describeTabs lists tabs as "; tabs: t1 (claude, main), t2 (codex)" so a caller
// that guessed wrong can correct itself without another command.
func describeTabs(tabs []TabInfo) string {
	if len(tabs) == 0 {
		return "; no tabs are open"
	}
	const maxListed = 12
	parts := make([]string, 0, len(tabs))
	for i, tab := range tabs {
		if i == maxListed {
			parts = append(parts, fmt.Sprintf("and %d more", len(tabs)-maxListed))
			break
		}
		var detail []string
		if tab.Agent != "" {
			detail = append(detail, tab.Agent)
		}
		if tab.Name != "" {
			detail = append(detail, tab.Name)
		}
		if len(detail) == 0 {
			parts = append(parts, tab.ID)
			continue
		}
		parts = append(parts, tab.ID+" ("+strings.Join(detail, ", ")+")")
	}
	return "; tabs: " + strings.Join(parts, ", ")
}
