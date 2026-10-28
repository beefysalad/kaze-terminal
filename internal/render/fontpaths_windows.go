package render

import (
	"os"
	"path/filepath"
)

// systemFontPaths lists the Windows fonts tried, in order, for runes the bundled fonts lack.
func systemFontPaths() []string {
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	dir := filepath.Join(win, "Fonts")
	return []string{
		filepath.Join(dir, "seguisym.ttf"), // Segoe UI Symbol: dingbats, technical, arrows, Braille
		filepath.Join(dir, "seguiemj.ttf"), // Segoe UI Emoji: drawn from its monochrome outlines
		filepath.Join(dir, "cambria.ttc"),  // Cambria Math
		// Japanese last: kanji outside the bundled cp932 subset (JIS X 0213, e.g. 𠮟).
		filepath.Join(dir, "YuGothM.ttc"),  // Yu Gothic Medium
		filepath.Join(dir, "msgothic.ttc"), // MS Gothic, on installs without Yu Gothic
	}
}
