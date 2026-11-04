package ui

import (
	"image"
	"image/color"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/version"
)

const aboutRepository = "https://github.com/beefysalad/kaze-terminal"

// The lines under the logo: kaze (風) is Japanese for "wind".
const (
	aboutTagline = "Light, fast, and out of the way."
	aboutKanji   = "風"
	aboutMeaning = "Kaze means “wind”."
)

// The mark's lilac on dark pages, and a deeper one that holds contrast on light pages.
var (
	aboutMarkDark  = color.NRGBA{R: 0xc2, G: 0xb4, B: 0xeb, A: 0xff}
	aboutMarkLight = color.NRGBA{R: 0x75, G: 0x63, B: 0xbf, A: 0xff}
)

// aboutGeom is where the About page put things, for tests.
type aboutGeom struct {
	lockup, by, card  image.Rectangle
	meaning           image.Rectangle // the kanji and the meaning, together
	wordX, markBottom int
	byBase            int // the byline's baseline
}

func aboutSection() settingsSection {
	return settingsSection{title: "About", draw: drawAbout, act: aboutAct}
}

func aboutAct(win *window, id string, _ int) {
	switch id {
	case "about_copy":
		win.set.aboutCopy = "kaze " + version.Number + "\n" + win.set.build + "\n" + runtime.GOOS + "/" + runtime.GOARCH
	case "about_github":
		if err := openURL(aboutRepository); err != nil {
			win.note("Could not open GitHub: " + err.Error())
		}
	case "about_issue":
		if err := openURL(aboutRepository + "/issues/new"); err != nil {
			win.note("Could not open GitHub: " + err.Error())
		}
	}
}

// openURL starts the platform URL handler without blocking the UI while the browser runs.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// platformName is the OS as people say it.
func platformName(goos string) string {
	switch goos {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	}
	return goos
}

// isLight reports a light page colour.
func isLight(c color.NRGBA) bool {
	return 299*int(c.R)+587*int(c.G)+114*int(c.B) > 128*1000
}

// drawAbout is a compact, centred colophon: the lockup with "by Patrick",
// the tagline and the name's meaning, then one card holding the facts with
// the page's actions in its footer.
func drawAbout(p *page, r image.Rectangle, win *window) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	if text := win.set.aboutCopy; text != "" {
		win.set.aboutCopy = ""
		if win.toClipboard != nil {
			win.toClipboard(text)
		} else {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		}
		win.note("Build details copied")
	}
	geom := &win.set.about
	*geom = aboutGeom{}
	narrow := r.Dx() < gtx.Dp(420)
	colW := min(r.Dx(), gtx.Dp(440))
	cx0 := r.Min.X + (r.Dx()-colW)/2
	centerX := func(w int) int { return r.Min.X + (r.Dx()-w)/2 }
	y := r.Min.Y + gtx.Dp(14)

	// The lockup, with "by Patrick" under the wordmark on the mark's baseline.
	lw, bySize := gtx.Dp(208), unit.Sp(11)
	if narrow {
		lw, bySize = gtx.Dp(168), 10
	}
	lk := lockupShapes()
	lg := lk.place(image.Pt(centerX(lw), y), lw)
	markCol := aboutMarkDark
	if isLight(col.bg) {
		markCol = aboutMarkLight
	}
	lk.draw(ops, lg, markCol, col.title)
	bc, bs, bb := p.monoBase("by Patrick", bySize, col.muted)
	by := image.Rectangle{Min: image.Pt(lg.wordX, lg.markBottom-bb)}
	by.Max = by.Min.Add(bs)
	place(ops, bc, by.Min)
	geom.lockup, geom.by, geom.wordX, geom.markBottom, geom.byBase = lg.rect, by, lg.wordX, lg.markBottom, by.Min.Y+bb
	y = max(lg.rect.Max.Y, by.Max.Y) + gtx.Dp(20)

	// The tagline, then the meaning.
	tc, ts := p.textW(aboutTagline, 15, col.title, font.Medium)
	place(ops, tc, image.Pt(centerX(ts.X), y))
	y += ts.Y + gtx.Dp(2)
	// The kanji and the sentence are set separately, a small gap apart: in one
	// run the Japanese font's wide space opens a hole after the kanji.
	kc, ks := p.text(aboutKanji, 12.5, col.muted)
	mc, ms := p.text(aboutMeaning, 12.5, col.muted)
	kgap := gtx.Dp(5)
	mx := centerX(ks.X + kgap + ms.X)
	mh := max(ks.Y, ms.Y)
	place(ops, kc, image.Pt(mx, y+(mh-ks.Y)/2))
	place(ops, mc, image.Pt(mx+ks.X+kgap, y+(mh-ms.Y)/2))
	geom.meaning = image.Rect(mx, y, mx+ks.X+kgap+ms.X, y+mh)
	y += mh + gtx.Dp(24)

	// The card: fact rows, then the footer with the actions.
	cardTop := y
	rec := op.Record(ops)
	padX, labelW := gtx.Dp(18), gtx.Dp(78)
	state, dot := "", col.done
	switch {
	case win.upd.newer:
		state, dot = win.upd.rel.Version+" available", col.accent
	case win.upd.checked && win.upd.err == "":
		state = "Up to date"
	}
	type fact struct{ label, value, extra string }
	facts := []fact{
		{"Version", version.Number, ""},
		{"Platform", platformName(runtime.GOOS), runtime.GOARCH},
		{"License", "MIT", ""},
	}
	for i, f := range facts {
		if i > 0 {
			paint.FillShape(ops, col.line, clip.Rect(image.Rect(cx0, y, cx0+colW, y+1)).Op())
		}
		top := gtx.Dp(11)
		if i == 0 {
			top = gtx.Dp(13)
		}
		lc, ls := p.text(f.label, 12.5, col.dim)
		vc, vs := p.mono(f.value, 12.5, col.title)
		h := max(ls.Y, vs.Y)
		ry := y + top
		place(ops, lc, image.Pt(cx0+padX, ry+(h-ls.Y)/2))
		vx := cx0 + padX + labelW
		place(ops, vc, image.Pt(vx, ry+(h-vs.Y)/2))
		if f.extra != "" {
			ec, es := p.mono(" · "+f.extra, 12.5, col.muted)
			place(ops, ec, image.Pt(vx+vs.X, ry+(h-es.Y)/2))
		}
		if i == 0 && state != "" {
			sc, ss := p.textW(state, 12, dot, font.Medium)
			sx := cx0 + colW - padX - ss.X
			place(ops, sc, image.Pt(sx, ry+(h-ss.Y)/2))
			d := gtx.Dp(6)
			dx, dy := sx-gtx.Dp(7)-d, ry+(h-d)/2
			paint.FillShape(ops, dot, clip.Ellipse(image.Rect(dx, dy, dx+d, dy+d)).Op(ops))
		}
		y = ry + h + gtx.Dp(11)
	}

	// The footer: Copy at the left, Report and GitHub at the right; stacked when narrow.
	footTop := y
	fy := y + gtx.Dp(12)
	fx0, fx1 := cx0+gtx.Dp(14), cx0+colW-gtx.Dp(14)
	copyB := ctrl{kind: ctlButton, id: "about_copy", text: "Copy build info", ghost: true}
	issueB := ctrl{kind: ctlButton, id: "about_issue", text: "Report an issue", ghost: true}
	ghB := ctrl{kind: ctlButton, id: "about_github", text: "View on GitHub", solid: true}
	put := func(b ctrl, w int, at func(size image.Point) image.Point) image.Point {
		call, sz, from := p.ctrl(b, w)
		pt := at(sz)
		p.st.shift(from, len(p.st.controls), pt)
		place(ops, call, pt)
		return sz
	}
	if narrow {
		for _, b := range []ctrl{ghB, copyB, issueB} {
			sz := put(b, fx1-fx0, func(image.Point) image.Point { return image.Pt(fx0, fy) })
			fy += sz.Y + gtx.Dp(8)
		}
		fy -= gtx.Dp(8)
	} else {
		gap := gtx.Dp(8)
		sz := put(copyB, 0, func(image.Point) image.Point { return image.Pt(fx0, fy) })
		right := fx1
		var gs image.Point
		gs = put(ghB, 0, func(s image.Point) image.Point { return image.Pt(right-s.X, fy) })
		right -= gs.X + gap
		put(issueB, 0, func(s image.Point) image.Point { return image.Pt(right-s.X, fy) })
		fy += max(sz.Y, gs.Y)
	}
	y = fy + gtx.Dp(12)
	call := rec.Stop()

	card := image.Rect(cx0, cardTop, cx0+colW, y)
	rad := gtx.Dp(12)
	paint.FillShape(ops, col.card, clip.UniformRRect(card, rad).Op(ops))
	foot := image.Rect(cx0, footTop, cx0+colW, y)
	paint.FillShape(ops, col.raised, clip.RRect{Rect: foot, SE: rad, SW: rad}.Op(ops))
	paint.FillShape(ops, col.line, clip.Rect(image.Rect(cx0, footTop, cx0+colW, footTop+1)).Op())
	call.Add(ops)
	stroke(ops, card, rad, col.line)
	p.st.cards = append(p.st.cards, card)
	geom.card = card

	y += gtx.Dp(18)
	nc, ns := p.text("Made with Go and Gio, for the long session.", 11.5, col.dim)
	place(ops, nc, image.Pt(centerX(ns.X), y))
	return y + ns.Y + gtx.Dp(16)
}
