package ui

import (
	"image"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/beefysalad/kaze-terminal/assets/branding"
)

// lockSeg is one path command in the lockup SVG's coordinates: M, L, Q (one
// control point, then the end) or C (two, then the end); Z closes.
type lockSeg struct {
	op  byte
	pts [3]f32.Point
}

type lockPath struct{ segs []lockSeg }

// lockBox is a box in the SVG's coordinates.
type lockBox struct{ Min, Max f32.Point }

func (b lockBox) Dx() float32 { return b.Max.X - b.Min.X }
func (b lockBox) Dy() float32 { return b.Max.Y - b.Min.Y }

func (b lockBox) Union(o lockBox) lockBox {
	return lockBox{
		Min: f32.Pt(min(b.Min.X, o.Min.X), min(b.Min.Y, o.Min.Y)),
		Max: f32.Pt(max(b.Max.X, o.Max.X), max(b.Max.Y, o.Max.Y)),
	}
}

// lockup is the approved logo's mark and wordmark as vector paths. The SVG's
// outlined byline is left out: the About page sets "by Patrick" as text.
// Bounds cover the on-curve points, which is where the shapes visibly end.
type lockup struct {
	mark, word             []lockPath
	markBounds, wordBounds lockBox
	bounds                 lockBox // mark and word together
}

// Fill colours the SVG gives each group.
const (
	lockMarkFill = "#c2b4eb"
	lockWordFill = "#efedf3"
)

var (
	lockOnce sync.Once
	lockData lockup

	lockGroup = regexp.MustCompile(`(?s)<g fill="(#[0-9a-f]{6})" transform="translate\(([-\d.]+) ([-\d.]+)\)">(.*?)</g>`)
	lockPathR = regexp.MustCompile(`<path d="([^"]*)"(?: transform="translate\(([-\d.]+) ([-\d.]+)\) scale\(([-\d.]+) ([-\d.]+)\)")?/>`)
	lockToken = regexp.MustCompile(`[MLHVCQZ]|-?(?:\d+\.?\d*|\.\d+)`)
)

// lockupShapes parses the embedded lockup once.
func lockupShapes() *lockup {
	lockOnce.Do(func() { lockData = parseLockup(string(branding.Lockup)) })
	return &lockData
}

func parseLockup(svg string) lockup {
	var lk lockup
	for _, g := range lockGroup.FindAllStringSubmatch(svg, -1) {
		var dst *[]lockPath
		switch g[1] {
		case lockMarkFill:
			dst = &lk.mark
		case lockWordFill:
			dst = &lk.word
		default:
			continue
		}
		gx, gy := atof(g[2]), atof(g[3])
		for _, m := range lockPathR.FindAllStringSubmatch(g[4], -1) {
			tx, ty, sx, sy := float32(0), float32(0), float32(1), float32(1)
			if m[2] != "" {
				tx, ty, sx, sy = atof(m[2]), atof(m[3]), atof(m[4]), atof(m[5])
			}
			at := func(x, y float32) f32.Point { return f32.Pt(gx+tx+sx*x, gy+ty+sy*y) }
			if p := parsePathD(m[1], at); len(p.segs) > 0 {
				*dst = append(*dst, p)
			}
		}
	}
	lk.markBounds, lk.wordBounds = pathBounds(lk.mark), pathBounds(lk.word)
	lk.bounds = lk.markBounds.Union(lk.wordBounds)
	return lk
}

func atof(s string) float32 {
	v, _ := strconv.ParseFloat(s, 32)
	return float32(v)
}

// parsePathD reads the absolute commands the logo uses (M L H V C Q Z, with
// implicit repeats), mapping each point through at.
func parsePathD(d string, at func(x, y float32) f32.Point) lockPath {
	var p lockPath
	toks := lockToken.FindAllString(d, -1)
	var cmd byte
	var cx, cy float32 // current point, untransformed
	num := func(i *int) float32 {
		v := atof(toks[*i])
		*i++
		return v
	}
	isNum := func(i int) bool { return i < len(toks) && !strings.ContainsAny(toks[i][:1], "MLHVCQZ") }
	for i := 0; i < len(toks); {
		if !isNum(i) {
			cmd = toks[i][0]
			i++
			if cmd == 'Z' {
				p.segs = append(p.segs, lockSeg{op: 'Z'})
				continue
			}
		}
		need := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'Q': 4, 'C': 6}[cmd]
		if need == 0 || i+need > len(toks) {
			break // unknown command or a truncated path: keep what is whole
		}
		switch cmd {
		case 'M', 'L':
			cx, cy = num(&i), num(&i)
			p.segs = append(p.segs, lockSeg{op: cmd, pts: [3]f32.Point{at(cx, cy)}})
			if cmd == 'M' {
				cmd = 'L' // further pairs after M are lines
			}
		case 'H':
			cx = num(&i)
			p.segs = append(p.segs, lockSeg{op: 'L', pts: [3]f32.Point{at(cx, cy)}})
		case 'V':
			cy = num(&i)
			p.segs = append(p.segs, lockSeg{op: 'L', pts: [3]f32.Point{at(cx, cy)}})
		case 'Q':
			x1, y1 := num(&i), num(&i)
			cx, cy = num(&i), num(&i)
			p.segs = append(p.segs, lockSeg{op: 'Q', pts: [3]f32.Point{at(x1, y1), at(cx, cy)}})
		case 'C':
			x1, y1, x2, y2 := num(&i), num(&i), num(&i), num(&i)
			cx, cy = num(&i), num(&i)
			p.segs = append(p.segs, lockSeg{op: 'C', pts: [3]f32.Point{at(x1, y1), at(x2, y2), at(cx, cy)}})
		}
	}
	return p
}

// pathBounds is the box around the on-curve points of ps.
func pathBounds(ps []lockPath) lockBox {
	b := lockBox{Min: f32.Pt(math.MaxFloat32, math.MaxFloat32), Max: f32.Pt(-math.MaxFloat32, -math.MaxFloat32)}
	for _, p := range ps {
		for _, s := range p.segs {
			var e f32.Point
			switch s.op {
			case 'M', 'L':
				e = s.pts[0]
			case 'Q':
				e = s.pts[1]
			case 'C':
				e = s.pts[2]
			default:
				continue
			}
			b.Min.X, b.Min.Y = min(b.Min.X, e.X), min(b.Min.Y, e.Y)
			b.Max.X, b.Max.Y = max(b.Max.X, e.X), max(b.Max.Y, e.Y)
		}
	}
	return b
}

// lockupGeom is where a lockup drawn at a width lands, in pixels.
type lockupGeom struct {
	rect              image.Rectangle
	wordX, markBottom int
	scale             float32
}

// placeLockup sizes the lockup to w pixels wide with its top-left at at.
func (lk *lockup) place(at image.Point, w int) lockupGeom {
	s := float32(w) / lk.bounds.Dx()
	h := int(math.Round(float64(lk.bounds.Dy() * s)))
	return lockupGeom{
		rect:       image.Rectangle{Min: at, Max: at.Add(image.Pt(w, h))},
		wordX:      at.X + int(math.Round(float64((lk.wordBounds.Min.X-lk.bounds.Min.X)*s))),
		markBottom: at.Y + int(math.Round(float64((lk.markBounds.Max.Y-lk.bounds.Min.Y)*s))),
		scale:      s,
	}
}

// draw fills the mark and wordmark at g in their colours.
func (lk *lockup) draw(ops *op.Ops, g lockupGeom, markCol, wordCol color.NRGBA) {
	o := f32.Pt(float32(g.rect.Min.X), float32(g.rect.Min.Y))
	at := func(p f32.Point) f32.Point { return p.Sub(lk.bounds.Min).Mul(g.scale).Add(o) }
	fillLock(ops, lk.mark, at, markCol)
	fillLock(ops, lk.word, at, wordCol)
}

// drawMark fills the mark alone, h pixels tall and centred in box.
func (lk *lockup) drawMark(ops *op.Ops, box image.Rectangle, h int, col color.NRGBA) {
	if len(lk.mark) == 0 || lk.markBounds.Dy() <= 0 {
		return
	}
	s := float32(h) / lk.markBounds.Dy()
	o := f32.Pt(float32(box.Min.X)+(float32(box.Dx())-lk.markBounds.Dx()*s)/2, float32(box.Min.Y+(box.Dy()-h)/2))
	fillLock(ops, lk.mark, func(p f32.Point) f32.Point { return p.Sub(lk.markBounds.Min).Mul(s).Add(o) }, col)
}

// fillLock fills each path, its points mapped through at.
func fillLock(ops *op.Ops, ps []lockPath, at func(f32.Point) f32.Point, col color.NRGBA) {
	for _, lp := range ps {
		var p clip.Path
		p.Begin(ops)
		for _, sg := range lp.segs {
			switch sg.op {
			case 'M':
				p.MoveTo(at(sg.pts[0]))
			case 'L':
				p.LineTo(at(sg.pts[0]))
			case 'Q':
				p.QuadTo(at(sg.pts[0]), at(sg.pts[1]))
			case 'C':
				p.CubeTo(at(sg.pts[0]), at(sg.pts[1]), at(sg.pts[2]))
			case 'Z':
				p.Close()
			}
		}
		paint.FillShape(ops, col, clip.Outline{Path: p.End()}.Op())
	}
}
