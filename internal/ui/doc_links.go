package ui

import (
	"github.com/beefysalad/kaze-terminal/internal/gitinfo"
	"github.com/beefysalad/kaze-terminal/internal/mdoc"
	"path/filepath"
)

func (win *window) followDocLink(d *docView, dest string) {
	t := mdoc.Classify(dest)
	switch t.Kind {
	case mdoc.TargetWeb:
		win.open(t.URL)
	case mdoc.TargetAnchor:
		win.docAnchor(d, t.Anchor)
	case mdoc.TargetDoc:
		top := gitinfo.Top
		if win.topFn != nil {
			top = win.topFn
		}
		dir := filepath.Dir(d.path)
		if d.path == "" { // an agent report: relative links start at the session's folder
			dir = win.termFolder(win.mgr.Lookup(d.session))
		}
		path, e := mdoc.Resolve(t.Path, mdoc.Roots{Dir: dir, Home: win.home, Top: top})
		if e != nil {
			// Then the docs the agent touched: this doc's session first.
			if _, isAbs := win.absRef(t.Path); !isAbs {
				if path = win.seen.find(d.session, t.Path); path == "" {
					path = win.seen.findOthers(d.session, win.tabOrder(), t.Path)
				}
			}
			if path == "" {
				win.note("Can't find " + t.Path)
				return
			}
		}
		nd := win.openDoc(d.session, path)
		if nd != nil && t.Anchor != "" {
			win.docAnchor(nd, t.Anchor)
		}
	}
}
func (win *window) docAnchor(d *docView, slug string) { d.anchor = slug; win.w.Invalidate() }
