package ui

import (
	"image"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Tab strip metrics in dp (see the spaces and tabs design).
const (
	stripHeightDp = 36
	stripTabMinDp = 120
	stripTabMaxDp = 230
	stripCloseDp  = 18
	stripPlusDp   = 28
	stripClosePad = 6 // gap between the close x and the tab's right edge
)

// stripMetrics holds the strip sizes in pixels.
type stripMetrics struct{ Height, TabMin, TabMax, Close, Plus, ClosePad int }

// newStripMetrics converts the dp sizes at pxPerDp pixels per dp.
func newStripMetrics(pxPerDp float32) stripMetrics {
	c := func(dp int) int { return int(float32(dp)*pxPerDp + 0.5) }
	return stripMetrics{c(stripHeightDp), c(stripTabMinDp), c(stripTabMaxDp), c(stripCloseDp), c(stripPlusDp), c(stripClosePad)}
}

// stripTab is one tab's rects, relative to the strip's top-left, scroll applied.
type stripTab struct {
	ID    tabs.TabID
	Rect  image.Rectangle
	Close image.Rectangle
}

// tabStripLayout is the result of layoutTabStrip.
type tabStripLayout struct {
	Tabs      []stripTab
	Plus      image.Rectangle
	Height    int
	ViewW     int // width of the scrolling tab area (strip width less the + when pinned)
	Scroll    int // clamped scroll offset
	MaxScroll int
}

// Scrolls reports whether the tabs overflow the strip.
func (l tabStripLayout) Scrolls() bool { return l.MaxScroll > 0 }

type stripHitKind int

const (
	hitNone stripHitKind = iota
	hitTab
	hitClose
	hitPlus
)

// stripHit is what lies under a point; Index is into Tabs for hitTab and hitClose.
type stripHit struct {
	Kind  stripHitKind
	Index int
}

// layoutTabStrip places ids in a strip of width pixels. Tabs share the room, from
// TabMax down to TabMin; past that they scroll and the + is pinned at the right
// edge. scroll is clamped, then moved the least needed to show the active tab
// (when active is in ids).
func layoutTabStrip(m stripMetrics, width int, ids []tabs.TabID, active tabs.TabID, scroll int) tabStripLayout {
	if width < 0 {
		width = 0
	}
	l := tabStripLayout{Height: m.Height}
	n := len(ids)
	view := width - m.Plus
	if view < 0 {
		view = 0
	}
	tw := m.TabMax
	if n > 0 {
		tw = view / n
		if tw > m.TabMax {
			tw = m.TabMax
		}
		if tw < m.TabMin {
			tw = m.TabMin
		}
	}
	content := n * tw
	l.ViewW = view
	l.MaxScroll = content - view
	if l.MaxScroll < 0 {
		l.MaxScroll = 0
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > l.MaxScroll {
		scroll = l.MaxScroll
	}
	for i, id := range ids {
		if id != active {
			continue
		}
		left, right := i*tw, (i+1)*tw
		if left < scroll {
			scroll = left
		} else if right > scroll+view {
			scroll = right - view
		}
		if scroll > l.MaxScroll {
			scroll = l.MaxScroll
		}
		if scroll < 0 {
			scroll = 0
		}
		break
	}
	l.Scroll = scroll
	l.Tabs = make([]stripTab, n)
	cy := (m.Height - m.Close) / 2
	for i, id := range ids {
		x := i*tw - scroll
		r := image.Rect(x, 0, x+tw, m.Height)
		cx := r.Max.X - m.ClosePad - m.Close
		l.Tabs[i] = stripTab{ID: id, Rect: r, Close: image.Rect(cx, cy, cx+m.Close, cy+m.Close)}
	}
	if l.Scrolls() {
		l.Plus = image.Rect(width-m.Plus, 0, width, m.Height)
		if width < m.Plus {
			l.Plus = image.Rect(0, 0, width, m.Height)
		}
	} else {
		l.Plus = image.Rect(content, 0, content+m.Plus, m.Height)
	}
	return l
}

// Hit reports what is under p (relative to the strip's top-left). The + wins
// over tabs scrolled beneath it, and a tab is hit only inside the view.
func (l tabStripLayout) Hit(p image.Point) stripHit {
	if p.Y < 0 || p.Y >= l.Height {
		return stripHit{}
	}
	if p.In(l.Plus) {
		return stripHit{Kind: hitPlus}
	}
	if p.X < 0 || p.X >= l.ViewW {
		return stripHit{}
	}
	for i, t := range l.Tabs {
		if !p.In(t.Rect) {
			continue
		}
		if p.In(t.Close) {
			return stripHit{Kind: hitClose, Index: i}
		}
		return stripHit{Kind: hitTab, Index: i}
	}
	return stripHit{}
}
