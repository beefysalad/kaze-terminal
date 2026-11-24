package control

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
