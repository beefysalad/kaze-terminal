package ui

import (
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"image"
	"strconv"
)

func (p *page) wsList(r image.Rectangle, y int, w *window) int {
	g := p.gtx
	if len(w.wsp.doc.Workspaces) <= 5 {
		w.wsp.query = ""
	}
	if len(w.wsp.doc.Workspaces) <= 1 {
		w.wsp.filter = wsAll
	}
	m := buildWSList(w, w.wsp.filter, w.wsp.query, w.wsp.open)
	if len(w.wsp.doc.Workspaces) > 1 {
		var chips []ctrl
		for i, name := range []string{"All", "Open", "Closed"} {
			chips = append(chips, ctrl{kind: ctlButton, id: "ws.filter", arg: i, text: name + " (" + strconv.Itoa(m.Counts[i]) + ")", strong: int(w.wsp.filter) == i})
		}
		y = p.rows(r, y, []settingRow{{ctrls: chips}})
	}
	if m.ShowSearch {
		y = p.rows(r, y, []settingRow{{ctrls: []ctrl{{kind: ctlText, id: "ws.search", val: w.wsp.query, placeholder: "Search workspaces and tabs", width: 300, set: func(s string) { w.wsp.query = s }}}}})
	}
	for _, row := range m.Rows {
		idx := 0
		for i, v := range w.wsp.doc.Workspaces {
			if v.ID == row.ID {
				idx = i
				break
			}
		}
		y = p.wsCard(r, y, w, row, idx) + g.Dp(8)
	}
	return y
}

// wsCard draws one workspace as a single bordered card: a header that toggles it,
// and when open a body with its tabs and the Edit and Delete actions. The content
// is recorded first so the card background can be sized to it.
func (p *page) wsCard(r image.Rectangle, top int, w *window, row wsListRow, idx int) int {
	g, ops, col := p.gtx, p.gtx.Ops, p.col
	narrow := r.Dx() < g.Dp(480)
	head := g.Dp(52)
	if narrow {
		head = g.Dp(96)
	}
	header := image.Rect(r.Min.X, top, r.Max.X, top+head)
	body := op.Record(ops)
	p.wsHeader(r, top, row, idx, narrow, header)
	y := header.Max.Y
	if row.Open {
		paint.FillShape(ops, col.line, clip.Rect(image.Rect(r.Min.X+g.Dp(1), y, r.Max.X-g.Dp(1), y+1)).Op())
		y += g.Dp(4)
		for _, t := range row.Tabs {
			y = p.wsTab(r, y, t)
		}
	}
	if row.Open || w.wsp.menu == row.ID {
		x := r.Min.X + g.Dp(14)
		y += g.Dp(4)
		ids := []ctrl{{kind: ctlButton, id: "ws.edit", text: "Edit", arg: idx, ghost: true}, {kind: ctlButton, id: "ws.delete", text: "Delete", arg: idx, ghost: true}}
		if w.wsp.menu == row.ID {
			ids = append(ids, ctrl{kind: ctlButton, id: "ws.row.startup", text: "Run at startup", arg: idx, ghost: true})
		}
		rowH := 0
		for _, k := range ids {
			call, sz, from := p.ctrl(k, 0)
			if x+sz.X > r.Max.X-g.Dp(10) {
				x = r.Min.X + g.Dp(14)
				y += rowH + g.Dp(6)
			}
			at := image.Pt(x, y)
			p.st.shift(from, len(p.st.controls), at)
			place(ops, call, at)
			x += sz.X + g.Dp(8)
			rowH = max(rowH, sz.Y)
		}
		y += rowH + g.Dp(12)
	}
	content := body.Stop()
	card := image.Rect(r.Min.X, top, r.Max.X, y)
	paint.FillShape(ops, col.card, clip.UniformRRect(card, g.Dp(7)).Op(ops))
	stroke(ops, card, g.Dp(7), col.line)
	content.Add(ops)
	return y
}

// wsHeader is the always-visible row: chevron, initial tile, name and tab count,
// member dots, the state pill and the Run and menu buttons.
func (p *page) wsHeader(r image.Rectangle, top int, row wsListRow, idx int, narrow bool, bg image.Rectangle) {
	g, ops, col := p.gtx, p.gtx.Ops, p.col
	x := r.Min.X + g.Dp(12)
	var path clip.Path
	path.Begin(ops)
	cx, cy := float32(x), float32(top+g.Dp(23))
	if row.Open {
		path.MoveTo(f32.Pt(cx, cy-2))
		path.LineTo(f32.Pt(cx+4, cy+2))
		path.LineTo(f32.Pt(cx+8, cy-2))
	} else {
		path.MoveTo(f32.Pt(cx, cy-4))
		path.LineTo(f32.Pt(cx+4, cy))
		path.LineTo(f32.Pt(cx, cy+4))
	}
	paint.FillShape(ops, col.muted, clip.Stroke{Path: path.End(), Width: 1.5}.Op())
	x += g.Dp(16)
	tile := image.Rect(x, top+g.Dp(12), x+g.Dp(26), top+g.Dp(38))
	paint.FillShape(ops, tint(col.accent, 0x30), clip.UniformRRect(tile, g.Dp(6)).Op(ops))
	ic, is := p.text(row.Initial, 12, col.title)
	place(ops, ic, image.Pt(tile.Min.X+(tile.Dx()-is.X)/2, tile.Min.Y+(tile.Dy()-is.Y)/2))
	x = tile.Max.X + g.Dp(10)
	right := r.Max.X - g.Dp(10)
	left := r.Min.X + g.Dp(10)
	button := func(k ctrl) {
		call, sz, from := p.ctrl(k, 0)
		at := image.Pt(right-sz.X, top+(g.Dp(52)-sz.Y)/2)
		if narrow {
			at = image.Pt(left, top+g.Dp(52)+(g.Dp(40)-sz.Y)/2)
			left += sz.X + g.Dp(8)
		}
		p.st.shift(from, len(p.st.controls), at)
		place(ops, call, at)
		right = at.X - g.Dp(8)
	}
	menu := ctrl{kind: ctlButton, id: "ws.menu", arg: idx, text: "...", ghost: true}
	run := ctrl{kind: ctlButton, id: "ws.run", arg: idx, text: row.RunLabel, strong: row.State == "Closed"}
	if narrow {
		button(run)
		button(menu)
	} else {
		button(menu)
		button(run)
	}
	pc, ps := p.pill(row.State, row.Tone)
	py := top + (g.Dp(52)-ps.Y)/2
	if narrow {
		py = top + g.Dp(52) + (g.Dp(40)-ps.Y)/2
		right = r.Max.X - g.Dp(10)
	}
	place(ops, pc, image.Pt(right-ps.X, py))
	right -= ps.X + g.Dp(10)
	if narrow {
		right = r.Max.X - g.Dp(12)
	} else {
		for _, dot := range row.Dots {
			right -= g.Dp(9)
			paint.FillShape(ops, p.c.memberColor(dot), clip.Ellipse(image.Rect(right, top+g.Dp(23), right+g.Dp(5), top+g.Dp(28))).Op(ops))
		}
	}
	nc, ns := p.textW(p.fitSans(row.Name, 13, max(1, right-x), font.SemiBold), 13, col.title, font.SemiBold)
	place(ops, nc, image.Pt(x, top+g.Dp(9)))
	sc, _ := p.text(p.fitSans(row.Sub, 11, max(1, right-x), font.Normal), 11, col.muted)
	place(ops, sc, image.Pt(x, top+g.Dp(10)+ns.Y))
	p.hit(ctlButton, bg, "ws.toggle", idx, false)
}

// wsTab draws one tab on two lines: a color bar, the title and its wait tag on
// the first, the folder, command chip and "+N more" on the second. When the
// card is too narrow for the chip beside the folder it drops to a third line.
func (p *page) wsTab(r image.Rectangle, y int, t wsListTab) int {
	g, ops, col := p.gtx, p.gtx.Ops, p.col
	top := y + g.Dp(6)
	barX := r.Min.X + g.Dp(14)
	x := barX + g.Dp(13)
	room := max(r.Max.X-g.Dp(14)-x, 1)
	y = top

	tagC, tagS := p.text(t.Wait, 11, col.muted)
	title, ts := p.textW(p.fitSans(t.Title, 12, max(room-tagS.X-g.Dp(12), 1), font.SemiBold), 12, col.title, font.SemiBold)
	place(ops, title, image.Pt(x, y))
	place(ops, tagC, image.Pt(x+room-tagS.X, y+(ts.Y-tagS.Y)/2))
	y += ts.Y + g.Dp(3)

	var moreC op.CallOp
	var moreS image.Point
	if t.More != "" {
		moreC, moreS = p.text(t.More, 11, col.muted)
	}
	gap := g.Dp(10)
	chipPad := g.Dp(6)
	cmd := p.fitEndMono(t.Cmd, 11, max(room-2*chipPad, 1))
	_, cs := p.mono(cmd, 11, col.accent)
	chipW := cs.X + 2*chipPad
	_, fs := p.mono("M", 11.5, col.muted)
	lineH := max(fs.Y, cs.Y+g.Dp(4))

	drawChip := func(cx, cy int) int {
		chip := image.Rect(cx, cy, cx+chipW, cy+lineH)
		stroke(ops, chip, g.Dp(4), col.line)
		call, sz := p.mono(cmd, 11, col.accent)
		place(ops, call, image.Pt(cx+chipPad, cy+(lineH-sz.Y)/2))
		cx += chipW + gap
		if t.More != "" {
			place(ops, moreC, image.Pt(cx, cy+(lineH-moreS.Y)/2))
			cx += moreS.X + gap
		}
		return cx
	}
	chipSpan := chipW + gap
	if t.More != "" {
		chipSpan += moreS.X + gap
	}
	folderRoom := room - chipSpan
	if folderRoom >= g.Dp(120) {
		fc, fsz := p.mono(p.fitStart(t.Folder, 11.5, folderRoom), 11.5, col.muted)
		place(ops, fc, image.Pt(x, y+(lineH-fsz.Y)/2))
		drawChip(x+min(g.Dp(210), folderRoom)+gap, y)
		y += lineH
	} else {
		fc, fsz := p.mono(p.fitStart(t.Folder, 11.5, room), 11.5, col.muted)
		place(ops, fc, image.Pt(x, y))
		y += fsz.Y + g.Dp(3)
		drawChip(x, y)
		y += lineH
	}
	y += g.Dp(6)
	paint.FillShape(ops, p.c.memberColor(t.Color), clip.UniformRRect(image.Rect(barX, top, barX+g.Dp(3), y-g.Dp(6)), g.Dp(1)).Op(ops))
	return y
}
