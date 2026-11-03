//go:build !windows && !darwin

package render

// systemFontPaths is empty where no fallback list is defined; missing runes draw .notdef.
func systemFontPaths() []string { return nil }

// emojiFontPath is empty: colour emoji (Noto's CBDT) are not drawn here.
func emojiFontPath() string { return "" }
