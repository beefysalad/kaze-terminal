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
	g, ops, col := p.gtx, p.gtx.Ops, p.col
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
		top := y
		h := g.Dp(52)
		narrow := r.Dx() < g.Dp(480)
		if narrow {
			h = g.Dp(86)
		}
		bg := image.Rect(r.Min.X, top, r.Max.X, top+h)
		paint.FillShape(ops, col.card, clip.UniformRRect(bg, g.Dp(7)).Op(ops))
		stroke(ops, bg, g.Dp(7), col.line)
		// The whole header toggles; action hit regions are registered over it.
		x := r.Min.X + g.Dp(12)
		// A vector chevron carries expansion state.
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
		button := func(k ctrl) {
			call, sz, from := p.ctrl(k, 0)
			at := image.Pt(right-sz.X, top+(g.Dp(52)-sz.Y)/2)
			if narrow {
				at.Y = top + g.Dp(48)
			}
			p.st.shift(from, len(p.st.controls), at)
			place(ops, call, at)
			right = at.X - g.Dp(8)
		}
		button(ctrl{kind: ctlButton, id: "ws.menu", arg: idx, text: "...", ghost: true})
		button(ctrl{kind: ctlButton, id: "ws.run", arg: idx, text: row.RunLabel, strong: row.State == "Closed"})
		pc, ps := p.pill(row.State, row.Tone)
		py := top + (g.Dp(52)-ps.Y)/2
		if narrow {
			py = top + g.Dp(50)
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
		y = bg.Max.Y
		if row.Open {
			y = p.rows(r, y, []settingRow{{label: row.Name}})
			for _, t := range row.Tabs {
				top := y
				y += g.Dp(10)
				x := r.Min.X + g.Dp(20)
				room := max(r.Dx()-g.Dp(40), 1)
				put := func(text string, mono bool) {
					var call op.CallOp
					var sz image.Point
					if mono {
						call, sz = p.mono(p.fitStart(text, 11.5, room), 11.5, col.muted)
					} else {
						call, sz = p.text(p.fitSans(text, 12, room, font.Normal), 12, col.fg)
					}
					place(ops, call, image.Pt(x, y))
					y += sz.Y + g.Dp(5)
				}
				put(t.Title, false)
				put(t.Folder, true)
				put(t.Cmd, true)
				if t.More != "" {
					put(t.More, false)
				}
				put(t.Wait, false)
				y += g.Dp(8)
				paint.FillShape(ops, p.c.memberColor(t.Color), clip.Rect(image.Rect(r.Min.X+g.Dp(10), top+g.Dp(10), r.Min.X+g.Dp(13), y-g.Dp(8))).Op())
			}
		}
		if row.Open || w.wsp.menu == row.ID {
			y = p.rows(r, y, []settingRow{{ctrls: []ctrl{{kind: ctlButton, id: "ws.edit", text: "Edit", arg: idx, ghost: true}, {kind: ctlButton, id: "ws.delete", text: "Delete", arg: idx, ghost: true}, {kind: ctlButton, id: "ws.row.startup", text: "Run at startup", arg: idx, ghost: true}}}})
		}
		y += g.Dp(8)
	}
	return y
}
