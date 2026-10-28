package term

import (
	"bytes"
	"net/url"
	"strings"
)

// maxCwd bounds what a hostile program can make us keep.
const maxCwd = 4096

// Cwd is the folder the shell last reported with OSC 7, "" until then.
func (a *Applier) Cwd() string { return a.cwd }

// parseOSC7 turns "7;file://host/path" into an OS-native path. The host is ignored.
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
	if err != nil || strings.ContainsRune(p, 0) || len(p) > maxCwd {
		return "", false
	}
	if goos == "windows" {
		// Windows shells send /C:/dir; drop the slash before the drive.
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		p = strings.ReplaceAll(p, "/", `\`)
	}
	return p, true
}
