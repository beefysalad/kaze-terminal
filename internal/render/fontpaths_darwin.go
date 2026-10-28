package render

// systemFontPaths lists the macOS fonts tried, in order, for runes the bundled fonts lack.
func systemFontPaths() []string {
	return []string{
		"/System/Library/Fonts/Apple Symbols.ttf",
		"/System/Library/Fonts/Menlo.ttc",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	}
}
