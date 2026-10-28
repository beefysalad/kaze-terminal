//go:build !windows && !darwin

package render

// systemFontPaths is empty where no fallback list is defined; missing runes draw .notdef.
func systemFontPaths() []string { return nil }
