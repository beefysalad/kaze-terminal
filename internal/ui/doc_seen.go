package ui

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/beefysalad/kaze-terminal/internal/mdoc"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// maxDocSeen bounds each tab's list of docs its agent touched.
const maxDocSeen = 64

// docSeen is, per tab, the markdown files the agent's hooks reported touching,
// newest first. It lives in memory only and is dropped with the tab. Safe from
// any goroutine: onNotify writes, the UI goroutine reads. A nil docSeen holds
// nothing.
type docSeen struct {
	mu   sync.Mutex
	by   map[tabs.TabID][]string
	goos string // "" is runtime.GOOS; tests set it
}

func (s *docSeen) sys() string {
	if s.goos == "" {
		return runtime.GOOS
	}
	return s.goos
}

// add puts paths at the front of tab id's list, in order (the last is newest).
// A path already listed moves to the front; the oldest fall off past the cap.
func (s *docSeen) add(id tabs.TabID, paths []string) {
	if s == nil || len(paths) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	goos := s.sys()
	list := s.by[id]
	for _, p := range paths {
		if p == "" {
			continue
		}
		key := mdoc.PathKey(p, goos)
		for i, q := range list {
			if mdoc.PathKey(q, goos) == key {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		list = append(list, "")
		copy(list[1:], list)
		list[0] = p
		if len(list) > maxDocSeen {
			list = list[:maxDocSeen]
		}
	}
	if s.by == nil {
		s.by = map[tabs.TabID][]string{}
	}
	s.by[id] = list
}

// list is a copy of tab id's paths, newest first.
func (s *docSeen) list(id tabs.TabID) []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.by[id]...)
}

// find is the newest path in tab id's list whose tail matches ref and that is
// still a regular file; "" if none.
func (s *docSeen) find(id tabs.TabID, ref string) string {
	if s == nil {
		return ""
	}
	goos := s.sys()
	for _, p := range s.list(id) { // stat outside the lock
		if docTailMatch(p, ref, goos) {
			if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
				return p
			}
		}
	}
	return ""
}

// findOthers is find over every tab in order except id.
func (s *docSeen) findOthers(id tabs.TabID, order []tabs.TabID, ref string) string {
	for _, o := range order {
		if o == id {
			continue
		}
		if p := s.find(o, ref); p != "" {
			return p
		}
	}
	return ""
}

// prune forgets tabs not in list.
func (s *docSeen) prune(list []*tabs.Tab) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.by {
		if !tabIn(list, id) {
			delete(s.by, id)
		}
	}
}

// docTailMatch reports whether ref's path components equal recorded's last
// ones: whole names only, / and \ alike, case-insensitive on Windows and
// macOS. A ref with .. or no components never matches.
func docTailMatch(recorded, ref, goos string) bool {
	want := docParts(ref)
	if len(want) == 0 {
		return false
	}
	for _, c := range want {
		if c == ".." {
			return false
		}
	}
	have := docParts(recorded)
	if len(want) > len(have) {
		return false
	}
	have = have[len(have)-len(want):]
	fold := goos == "windows" || goos == "darwin"
	for i, c := range want {
		if fold && !strings.EqualFold(c, have[i]) || !fold && c != have[i] {
			return false
		}
	}
	return true
}

// docParts splits p on / and \, dropping empty and . components.
func docParts(p string) []string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	out := parts[:0]
	for _, c := range parts {
		if c != "." {
			out = append(out, c)
		}
	}
	return out
}
