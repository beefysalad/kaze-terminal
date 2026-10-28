package render

// systemFontPaths lists the macOS fonts tried, in order, for runes the bundled fonts lack.
func systemFontPaths() []string {
	return []string{
		"/System/Library/Fonts/Apple Symbols.ttf",
		"/System/Library/Fonts/Menlo.ttc",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		// Japanese last: kanji outside the bundled cp932 subset. Missing files are skipped.
		"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc", // Hiragino Sans W3
		"/System/Library/Fonts/Hiragino Sans GB.ttc",
	}
}
