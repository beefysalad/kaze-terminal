package control

// Handle executes one authenticated request synchronously. It starts no workers
// or timers. Host implementations own terminal access and submit sequencing.
func Handle(host Host, req Request) Reply {
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
