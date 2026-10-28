package term

import (
	"bytes"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxCwd bounds what a hostile program can make us keep.
const maxCwd = 4096

// Cwd is the folder the shell last reported with OSC 7, "" until then.
// RIS (ESC c) deliberately keeps it: the shell is still in that folder.
func (a *Applier) Cwd() string { return a.cwd }

// parseOSC7 turns "7;file://host/path" into an OS-native path. The host is
// deliberately ignored, so UNC paths (\server\share) are not supported.
func parseOSC7(data []byte, goos string) (string, bool) {
	rest, ok := bytes.CutPrefix(data, []byte("7;file://"))
	if !ok || len(rest) > maxCwd*3 {
		return "", false
	}
	i := bytes.IndexByte(rest, '/')
	if i < 0 {
		return "", false
	}
	p, err := url.PathUnescape(string(rest[i:]))
	if err != nil || len(p) > maxCwd || !utf8.ValidString(p) {
		return "", false
	}
	// Escapes like %1b would otherwise reach the status bar and window title.
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	if goos == "windows" {
		// Windows shells send /C:/dir; drop the slash before the drive.
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' && unicode.IsLetter(rune(p[1])) && (len(p) == 3 || p[3] == '/') {
			p = p[1:]
			if len(p) == 2 {
				p += "/" // a bare drive means its root
			}
		}
		p = strings.ReplaceAll(p, "/", `\`)
	}
	return p, true
}
