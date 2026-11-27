package control

import (
	"context"
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Options is the per-handler configuration. Omitted options use enabled defaults.
type Options struct{ Enabled, AgentTag, CrossSpace bool }

type MetadataHost interface {
	Name(context.Context, string, string) *Error
	Label(string, string, string) *Error
}

var tabID = regexp.MustCompile(`^t[0-9]+$`)
var labelKey = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,32}$`)

func hasControl(s string) bool        { return strings.ContainsFunc(s, unicode.IsControl) }
func escapeAttribute(s string) string { return html.EscapeString(s) }

// ValidateName normalizes a rename and rejects collisions with other tabs.
func ValidateName(tabs []TabInfo, id, name string) (string, *Error) {
	if hasControl(name) {
		return "", &Error{CodeInvalidRequest, "name must not contain control characters"}
	}
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "-") {
		return "", &Error{CodeInvalidRequest, "name must not start with a hyphen"}
	}
	if name == "" || utf8.RuneCountInString(name) > 64 || tabID.MatchString(name) {
		return "", &Error{CodeInvalidRequest, "name must be 1 to 64 runes and must not look like a tab id"}
	}
	for _, tab := range tabs {
		if tab.ID != id && tab.Name == name {
			return "", &Error{CodeAmbiguousTab, "tab name already exists"}
		}
	}
	return name, nil
}
func ValidateLabel(key, value string) *Error {
	if !labelKey.MatchString(key) || utf8.RuneCountInString(value) > 128 || hasControl(value) {
		return &Error{CodeInvalidRequest, "label key must be 1 to 32 letters, digits, _, - or .; value must be at most 128 runes without control characters"}
	}
	return nil
}
func metadata(ctx context.Context, host Host, req Request) Reply {
	tabs := host.Tabs()
	tab, e := Resolve(tabs, req.Tab)
	if e != nil {
		return Reply{Error: e}
	}
	mh, ok := host.(MetadataHost)
	if !ok {
		return Reply{Error: &Error{CodeHostError, "metadata is unavailable"}}
	}
	if req.Op == "name" {
		name, err := ValidateName(tabs, tab.ID, req.Name)
		if err != nil {
			return Reply{Error: err}
		}
		e = mh.Name(ctx, tab.ID, name)
	} else {
		if e = ValidateLabel(req.Key, req.Value); e == nil {
			e = mh.Label(tab.ID, req.Key, req.Value)
		}
	}
	return Reply{OK: e == nil, Error: e}
}
