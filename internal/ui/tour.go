package ui

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"github.com/beefysalad/kaze-terminal/internal/tour"
	"image"
	"path/filepath"
)

type tourState struct {
	open                      bool
	step                      int
	replay, boardOpenedByTour bool
	tag                       int
	hits                      []tourHit
	hooks                     []tourHook
	pending                   bool
}
type tourHit struct {
	r      image.Rectangle
	action int
}

func (w *window) tourDir() string {
	if w.cfgPath == "" {
		return ""
	}
	return filepath.Dir(w.cfgPath)
}
func (w *window) openTour(replay bool) {
	w.tour = tourState{open: true, replay: replay}
	if !replay && w.tourDir() != "" {
		_ = tour.MarkSeen(w.tourDir())
	}
	w.wantFocus = true
}
func (w *window) closeTour() {
	if w.tour.boardOpenedByTour {
		w.closeBoard()
	}
	if !w.tour.replay && w.tourDir() != "" {
		_ = tour.MarkSeen(w.tourDir())
	}
	w.tour.open = false
	w.wantFocus = true
}
func (w *window) tourMove(n int) {
	if w.tour.boardOpenedByTour {
		w.closeBoard()
		w.tour.boardOpenedByTour = false
	}
	w.tour.step = n
	if tourSteps()[n].Board && !w.board.open && w.mgr != nil {
		w.openBoard(0)
		w.tour.boardOpenedByTour = true
	}
	if tourSteps()[n].Hooks {
		w.tour.hooks = w.tourHookStates()
	}
	w.wantFocus = true
}
func (w *window) tourNext() {
	if w.tour.step == len(tourSteps())-1 {
		w.closeTour()
	} else {
		w.tourMove(w.tour.step + 1)
	}
}
func (w *window) tourBack() {
	if w.tour.step > 0 {
		w.tourMove(w.tour.step - 1)
	}
}
func (w *window) tourEvents(g layout.Context) {
	for {
		e, ok := g.Event(key.Filter{Focus: &w.tour.tag, Name: key.NameEscape}, key.Filter{Focus: &w.tour.tag, Name: key.NameReturn}, key.Filter{Focus: &w.tour.tag, Name: key.NameLeftArrow}, key.Filter{Focus: &w.tour.tag, Name: key.NameRightArrow}, pointer.Filter{Target: &w.tour.tag, Kinds: pointer.Press | pointer.Release | pointer.Scroll})
		if !ok {
			return
		}
		switch e := e.(type) {
		case key.Event:
			if e.State != key.Press {
				continue
			}
			switch e.Name {
			case key.NameEscape:
				w.closeTour()
			case key.NameLeftArrow:
				w.tourBack()
			default:
				w.tourNext()
			}
		case pointer.Event:
			if e.Kind == pointer.Release {
				pt := image.Pt(int(e.Position.X), int(e.Position.Y))
				for _, h := range w.tour.hits {
					if pt.In(h.r) {
						switch h.action {
						case -1:
							w.closeTour()
						case -2:
							w.tourBack()
						case 0:
							w.tourNext()
						case 1, 2:
							w.tourFix(h.action)
						}
						break
					}
				}
			}
		}
		if !w.tour.open {
			return
		}
	}
}
func tourCardRect(w, target image.Rectangle, card image.Point) image.Rectangle {
	g := min(16, min(w.Dx(), w.Dy())/4)
	card.X = min(card.X, max(1, w.Dx()-2*g))
	card.Y = min(card.Y, max(1, w.Dy()-2*g))
	at := image.Pt(w.Min.X+(w.Dx()-card.X)/2, w.Min.Y+(w.Dy()-card.Y)/2)
	if !target.Empty() {
		at.X = target.Max.X + g
	}
	at.X = max(w.Min.X+g, min(at.X, w.Max.X-g-card.X))
	at.Y = max(w.Min.Y+g, min(at.Y, w.Max.Y-g-card.Y))
	return image.Rectangle{Min: at, Max: at.Add(card)}
}

var _ event.Tag
