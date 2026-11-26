package control

import "strings"

const MaxReportBytes = 4 << 10

// report resolves only the caller's exact snapshot id, never a supplied parent.
// Inbox hosts publish first and defer typing until the UI permits delivery.
func report(host Host, req Request) Reply {
	if len(req.Text) > MaxReportBytes {
		return Reply{Error: &Error{CodeTextTooLarge, "report exceeds 4 KiB"}}
	}
	if strings.TrimSpace(req.Text) == "" {
		return Reply{Error: &Error{CodeInvalidRequest, "report text is required"}}
	}
	tabs := host.Tabs()
	for _, sender := range tabs {
		if sender.ID != req.From || sender.Parent == "" || sender.Role != "worker" {
			continue
		}
		var parent TabInfo
		for _, tab := range tabs {
			if tab.ID == sender.Parent {
				parent = tab
				break
			}
		}
		if parent.ID == "" {
			return Reply{Error: &Error{CodeUnknownTab, "report parent is closed"}}
		}
		from := sender.ID
		if sender.Name != "" {
			from += " " + sender.Name
		}
		text := `<kaze from="` + escapeAttribute(from) + `" kind="report">` + stripPeerClosingTags(req.Text) + `</kaze>`
		var deliveryError error
		if h, ok := host.(ReportInboxHost); ok {
			deliveryError = h.EnqueueInbox(parent.ID, "report", text)
		} else {
			deliveryError = host.Submit(parent.ID, []byte(text))
		}
		if deliveryError != nil {
			return Reply{Error: &Error{CodeHostError, "could not submit report to parent"}}
		}
		return Reply{OK: true, Tabs: []TabInfo{parent}}
	}
	return Reply{Error: &Error{CodeNotAllowed, "only a worker with an open parent can report"}}
}

// ReportInboxHost stores a report without typing into a working parent.
type ReportInboxHost interface {
	EnqueueInbox(id, kind, text string) error
}
