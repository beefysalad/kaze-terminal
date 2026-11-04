package ui

import (
	"github.com/patrick-gogo/kaze-terminal/internal/gitinfo"
	"github.com/patrick-gogo/kaze-terminal/internal/mdoc"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"os"
	"path/filepath"
	"strings"
)

func (win *window) termFolder(t *tabs.Tab) string {
	if t != nil {
		t.Session.Lock()
		cwd := t.Session.Applier().Cwd()
		t.Session.Unlock()
		if cwd != "" {
			return cwd
		}
		if t.Dir != "" {
			return t.Dir
		}
	}
	if win.opt.Dir != "" {
		return win.opt.Dir
	}
	return win.home
}
func (win *window) docParent(t *tabs.Tab, docked bool) tabs.TabID {
	if t == nil {
		return 0
	}
	if _, member := win.isMember(t); member {
		if docked {
			return win.dockOwner() // a docked member's docs belong to the session it is docked under
		}
		if ids := win.sessionIDs(); len(ids) > 0 {
			return ids[0]
		}
		return 0
	}
	return t.ID
}
func (win *window) openDocRef(t *tabs.Tab, docked bool, ref string) {
	win.openDocRefs(t, docked, []string{ref})
}

// openDocRefs opens the first of refs (best first, see render.Links.RefsAt) that
// is a file. Absolute refs are tried first, in order, then relative ones against
// the session folder and its git top, so a folder printed earlier on the line
// wins over a same-named file in the session folder. A miss names the folder of
// the last absolute ref tried that reads as one (see noteFolder), else the
// session folder.
func (win *window) openDocRefs(t *tabs.Tab, docked bool, refs []string) {
	if len(refs) == 0 {
		return
	}
	name := filepath.Base(filepath.FromSlash(refs[0]))
	id := win.docParent(t, docked)
	if id == 0 {
		win.note(cleanText("Open a session to read " + name))
		return
	}
	top := gitinfo.Top
	if win.topFn != nil {
		top = win.topFn
	}
	dir := win.termFolder(t)
	roots := mdoc.Roots{Dir: dir, Home: win.home, Top: top}
	where := ""
	for _, abs := range []bool{true, false} {
		for _, ref := range refs {
			full, isAbs := win.absRef(ref)
			if isAbs != abs {
				continue
			}
			if abs && noteFolder(full) {
				where = filepath.Dir(full)
			}
			if p, e := mdoc.Resolve(ref, roots); e == nil {
				win.openDoc(id, p)
				return
			}
		}
	}
	if where == "" {
		where = dir
	}
	win.note(cleanText("Can't find " + name + " in " + tildeHome(where, win.home)))
}

// noteFolder reports whether full's folder can name a miss: a path without
// spaces, or one whose folder exists. "/tmp then edit docs/notes.md" is a
// command line, not a folder.
func noteFolder(full string) bool {
	if !strings.Contains(full, " ") {
		return true
	}
	fi, err := os.Stat(filepath.Dir(full))
	return err == nil && fi.IsDir()
}

// absRef is ref as a full path, with ~ expanded, and whether it is absolute.
func (win *window) absRef(ref string) (string, bool) {
	ref = filepath.FromSlash(ref)
	if win.home != "" && (strings.HasPrefix(ref, "~/") || strings.HasPrefix(ref, `~\`)) {
		return filepath.Join(win.home, ref[2:]), true
	}
	return ref, filepath.IsAbs(ref)
}
