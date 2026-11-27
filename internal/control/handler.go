package control

import (
	"bytes"
	"context"
	"errors"
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
	if RequiresOwnerKey(req.Op) || (req.Op == "wait" && req.Children != "") {
		if e := VerifyOwnerKey(host, req.From, req.CallerKey); e != nil {
			return Reply{Error: e}
		}
	}
	if req.Op == "templates" {
		if h, ok := host.(interface{ Templates(context.Context) Reply }); ok {
			return h.Templates(ctx)
		}
		return Reply{Error: &Error{CodeUnknownOp, "templates unavailable"}}
	}
	if req.Op == "spawn" || req.Op == "kill" {
		return workers(ctx, host, req, opt)
	}
	if req.Op == "name" || req.Op == "label" {
		return metadata(ctx, host, req, opt)
	}
	if req.Op == "wait" {
		return wait(ctx, host, req, opt)
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
		tabs := host.Tabs()
		tab, _, err := resolveScoped(tabs, req.Tab, req.From, opt.CrossSpace)
		if err != nil {
			return Reply{Error: err}
		}
		if e := WorkerInputGuard(tab); e != nil {
			return Reply{Error: e}
		}
		var hostErr error
		if req.Op == "keys" {
			hostErr = host.Keys(tab.ID, req.Args)
		} else {
			hostErr = host.Interrupt(tab.ID, req.CtrlC)
		}
		if hostErr != nil {
			return inputError(hostErr)
		}
		return Reply{OK: true, Warning: crossSpace(tabs, req.From, tab, opt.CrossSpace)}
	}
	if req.Op == "report" {
		return report(host, req)
	}
	if req.Op == "ls" {
		visible, err := VisibleTo(host.Tabs(), req.From, opt.CrossSpace)
		if err != nil {
			return Reply{Error: err}
		}
		return Reply{OK: true, Tabs: visible}
	}
	if req.Op == "read" {
		if req.Lines < 0 {
			return Reply{Error: &Error{CodeInvalidRequest, "lines must not be negative"}}
		}
		tabs := host.Tabs()
		tab, _, err := resolveScoped(tabs, req.Tab, req.From, opt.CrossSpace)
		if err != nil {
			return Reply{Error: err}
		}
		text, hostErr := host.Screen(tab.ID, min(req.Lines, MaxLines))
		if hostErr != nil {
			return Reply{Error: &Error{CodeHostError, "could not read tab"}}
		}
		return Reply{OK: true, Text: text, Warning: crossSpace(tabs, req.From, tab, opt.CrossSpace)}
	}
	if req.Op == "send" {
		if len(req.Text) > MaxTextBytes && (!opt.AgentTag || req.From == "") {
			return Reply{Error: &Error{CodeTextTooLarge, "text exceeds 32 KiB"}}
		}
		tabs := host.Tabs()
		tab, _, err := resolveScoped(tabs, req.Tab, req.From, opt.CrossSpace)
		if err != nil {
			return Reply{Error: err}
		}
		if e := WorkerInputGuard(tab); e != nil {
			return Reply{Error: e}
		}
		if tab.Status == "working" && req.IfIdle {
			return Reply{Error: &Error{CodeBusy, "target is working"}}
		}
		text := req.Text
		if opt.AgentTag && req.From != "" && !req.Raw {
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
			return inputError(err)
		}
		warning := ""
		if tab.Status == "working" {
			warning = "busy"
		}
		if spaceWarning := crossSpace(tabs, req.From, tab, opt.CrossSpace); spaceWarning != "" {
			if warning != "" {
				warning += "; "
			}
			warning += spaceWarning
		}
		return Reply{OK: true, Status: tab.Status, Warning: warning, Tabs: []TabInfo{tab}}
	}
	return Reply{Error: &Error{CodeUnknownOp, "unknown control operation"}}
}

// spaceOf is the identity of a tab's space. Names repeat (two repositories can both
// be called "project", and a repository can be called "Shell"), so the host's key wins.
func spaceOf(t TabInfo) string {
	if t.SpaceKey != "" {
		return "key:" + t.SpaceKey
	}
	return "name:" + t.Space
}

// VisibleTo returns the tabs the caller may see: its own space, or every tab for the
// owner (no from) and in cross_space mode. A from that names no open tab is refused, so
// dropping or garbling KAZE_TAB_ID does not widen what a caller can reach.
func VisibleTo(tabs []TabInfo, from string, cross bool) ([]TabInfo, *Error) {
	if cross || from == "" {
		return tabs, nil
	}
	for _, sender := range tabs {
		if sender.ID != from {
			continue
		}
		visible := make([]TabInfo, 0, len(tabs))
		for _, tab := range tabs {
			if spaceOf(tab) == spaceOf(sender) {
				visible = append(visible, tab)
			}
		}
		return visible, nil
	}
	return nil, &Error{CodeUnknownTab, "your tab is not known to Kaze; run this inside a Kaze tab"}
}

// resolveScoped resolves only visible tabs and refuses targets in other spaces.
func resolveScoped(tabs []TabInfo, address, from string, cross bool) (TabInfo, []TabInfo, *Error) {
	visible, scopeErr := VisibleTo(tabs, from, cross)
	if scopeErr != nil {
		return TabInfo{}, nil, scopeErr
	}
	tab, err := ResolveFrom(visible, address, from)
	if err != nil && err.Code == CodeUnknownTab && matchesAnyTab(tabs, address) {
		return TabInfo{}, visible, &Error{CodeCrossSpace, "that tab is in another space; cross-space control is off"}
	}
	return tab, visible, err
}

// matchesAnyTab reports whether an id, name or agent name addresses some tab, even
// when several tabs in other spaces share it.
func matchesAnyTab(tabs []TabInfo, address string) bool {
	for _, tab := range tabs {
		if tab.ID == address || (tab.Name != "" && tab.Name == address) || (tab.Agent != "" && strings.EqualFold(tab.Agent, address)) {
			return true
		}
	}
	return false
}

// SameSpace returns the tabs sharing a space with tab id (every tab in cross_space
// mode). Tab names only have to be unique inside this set.
func SameSpace(tabs []TabInfo, id string, cross bool) []TabInfo {
	if cross {
		return tabs
	}
	for _, target := range tabs {
		if target.ID != id {
			continue
		}
		same := make([]TabInfo, 0, len(tabs))
		for _, tab := range tabs {
			if spaceOf(tab) == spaceOf(target) {
				same = append(same, tab)
			}
		}
		return same
	}
	return nil
}

type callerKey struct{}

// CallerFrom is the tab that made the request a host method is serving, or "" for
// the owner. Hosts that apply a change later use it to re-check scope.
func CallerFrom(ctx context.Context) string {
	from, _ := ctx.Value(callerKey{}).(string)
	return from
}

// crossSpace warns about a different project without refusing an explicit target.
func crossSpace(tabs []TabInfo, from string, target TabInfo, enabled bool) string {
	if !enabled || from == "" {
		return ""
	}
	for _, sender := range tabs {
		if sender.ID == from {
			if spaceOf(sender) == spaceOf(target) {
				return ""
			}
			return fmt.Sprintf("target %s is in space %s, not your space %s; use it only if the owner named that tab", target.ID, target.Space, sender.Space)
		}
	}
	return ""
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
	return ResolveFrom(tabs, address, "")
}

// ResolveFrom prefers a unique agent in the caller's space after exact ids and names.
func ResolveFrom(tabs []TabInfo, address, from string) (TabInfo, *Error) {
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
	for _, sender := range tabs {
		if sender.ID != from {
			continue
		}
		var local []TabInfo
		for _, tab := range byAgent {
			if spaceOf(tab) == spaceOf(sender) {
				local = append(local, tab)
			}
		}
		if len(local) == 1 {
			return local[0], nil
		}
		break
	}
	switch len(byAgent) {
	case 1:
		return byAgent[0], nil
	case 0:
		return TabInfo{}, &Error{CodeUnknownTab, fmt.Sprintf("unknown tab %q%s", address, describeTabs(tabs))}
	}
	return TabInfo{}, &Error{CodeAmbiguousTab, fmt.Sprintf("%d tabs run %q, use an id%s", len(byAgent), address, describeTabs(byAgent))}
}

// describeTabs lists ids, agents, names and spaces so a caller
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
		if tab.Space != "" {
			detail = append(detail, "space "+tab.Space)
		}
		if len(detail) == 0 {
			parts = append(parts, tab.ID)
			continue
		}
		parts = append(parts, tab.ID+" ("+strings.Join(detail, ", ")+")")
	}
	return "; tabs: " + strings.Join(parts, ", ")
}

// WorkerInputGuard reserves startup trust and approval prompts for native owner input.
func WorkerInputGuard(tab TabInfo) *Error {
	if tab.Role == "worker" && (tab.FirstMessage != "delivered" || tab.Status == "needs") {
		return &Error{CodeNotAllowed, "the owner has to answer worker trust or approval prompts in that tab"}
	}
	return nil
}

func inputError(err error) Reply {
	var refusal *Error
	if errors.As(err, &refusal) && refusal.Code == CodeNotAllowed {
		return Reply{Error: refusal}
	}
	return Reply{Error: &Error{CodeHostError, "could not submit to tab"}}
}
