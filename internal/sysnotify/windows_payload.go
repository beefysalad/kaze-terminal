package sysnotify

import (
	"bytes"
	"encoding/xml"
	"errors"
	"path"
	"strings"
)

const windowsAUMID = "com.patrickgogo.kaze"

// Windows restricts both tag and group to sixteen characters on Windows 10.
// The revision stays in the opaque activation ID, not the replacement identity.
func windowsToastIdentity(id string) (group, tag string, err error) {
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		return "", "", errors.New("invalid notification identity")
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 16 {
			return "", "", errors.New("invalid notification identity")
		}
		for _, c := range p {
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
				return "", "", errors.New("invalid notification identity")
			}
		}
	}
	return parts[0], parts[1], nil
}

func windowsToastXML(n Notification) string {
	var b bytes.Buffer
	escape := func(s string) {
		// Invalid XML characters cannot be represented even as numeric escapes.
		s = strings.Map(func(r rune) rune {
			if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff {
				return r
			}
			return -1
		}, s)
		_ = xml.EscapeText(&b, []byte(s))
	}
	b.WriteString(`<toast launch="`)
	escape(n.ID)
	b.WriteString(`"><visual><binding template="ToastGeneric">`)
	for _, s := range []string{n.Title, n.Context, n.Body} {
		if s == "" {
			continue
		}
		b.WriteString("<text>")
		escape(s)
		b.WriteString("</text>")
	}
	b.WriteString(`</binding></visual><audio silent="true"/></toast>`)
	return b.String()
}

// Compare only canonical absolute executable paths. A shortcut at Kaze's name
// is not evidence of ownership; an unrelated target is never overwritten.
func windowsShortcutTarget(exe, existing string) error {
	clean := func(s string) string { return path.Clean(strings.ReplaceAll(s, `\`, "/")) }
	exe = clean(exe)
	abs := strings.HasPrefix(exe, "//") || len(exe) > 3 && exe[1] == ':' && exe[2] == '/'
	if !abs || !strings.EqualFold(path.Base(exe), "kaze.exe") {
		return errors.New("system notifications require the canonical kaze.exe")
	}
	if existing != "" && !strings.EqualFold(exe, clean(existing)) {
		return errors.New("Kaze Start menu shortcut belongs to another executable")
	}
	return nil
}
