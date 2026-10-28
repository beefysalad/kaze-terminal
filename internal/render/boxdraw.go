package render

import (
	"image"
	"math"
)

// Box drawing (U+2500-257F) and block elements (U+2580-259F) are drawn here instead
// of taken from the font, so lines fill the cell exactly and join their neighbours.

func isBoxRune(r rune) bool { return r >= 0x2500 && r <= 0x259F }

// boxArms gives each line character's arms as up, right, down, left:
// 0 none, 1 light, 2 heavy, 3 double. Dashed lines are drawn solid. Empty entries
// (arcs and diagonals) are drawn by their own code.
var boxArms = [0x80]string{
	"0101", "0202", "1010", "2020", "0101", "0202", "1010", "2020", // 2500-2507
	"0101", "0202", "1010", "2020", "0110", "0210", "0120", "0220", // 2508-250F
	"0011", "0012", "0021", "0022", "1100", "1200", "2100", "2200", // 2510-2517
	"1001", "1002", "2001", "2002", "1110", "1210", "2110", "1120", // 2518-251F
	"2120", "2210", "1220", "2220", "1011", "1012", "2011", "1021", // 2520-2527
	"2021", "2012", "1022", "2022", "0111", "0112", "0211", "0212", // 2528-252F
	"0121", "0122", "0221", "0222", "1101", "1102", "1201", "1202", // 2530-2537
	"2101", "2102", "2201", "2202", "1111", "1112", "1211", "1212", // 2538-253F
	"2111", "1121", "2121", "2112", "2211", "1122", "1221", "2212", // 2540-2547
	"1222", "2122", "2221", "2222", "0101", "0202", "1010", "2020", // 2548-254F
	"0303", "3030", "0310", "0130", "0330", "0013", "0031", "0033", // 2550-2557
	"1300", "3100", "3300", "1003", "3001", "3003", "1310", "3130", // 2558-255F
	"3330", "1013", "3031", "3033", "0313", "0131", "0333", "1303", // 2560-2567
	"3101", "3303", "1313", "3131", "3333", "", "", "", // 2568-256F
	"", "", "", "", "0001", "1000", "0100", "0010", // 2570-2577
	"0002", "2000", "0200", "0020", "0201", "1020", "0102", "2010", // 2578-257F
}

// quadrants for U+2596-259F: 1 upper left, 2 upper right, 4 lower left, 8 lower right.
var quadrants = [10]uint8{4, 8, 1, 13, 9, 7, 11, 2, 6, 14}

// drawBox returns an opaque-or-clear mask (shades excepted) of w x h for r.
func drawBox(r rune, w, h, cellW int) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, w, h))
	lt := max(1, int(math.Round(float64(cellW)/8)))
	switch {
	case r >= 0x2580:
		drawBlock(m, r, w, h)
	case r >= 0x256D && r <= 0x2570:
		drawArc(m, r, w, h, lt)
	case r >= 0x2571 && r <= 0x2573:
		if r != 0x2572 {
			drawDiagonal(m, w, h, lt, true)
		}
		if r != 0x2571 {
			drawDiagonal(m, w, h, lt, false)
		}
	default:
		drawArms(m, boxArms[r-0x2500], w, h, lt)
	}
	return m
}

func fillMask(m *image.Alpha, x0, y0, x1, y1 int, a uint8) {
	r := image.Rect(x0, y0, x1, y1).Intersect(m.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := m.Pix[y*m.Stride+r.Min.X : y*m.Stride+r.Max.X]
		for i := range row {
			row[i] = a
		}
	}
}

// width is the span an arm of weight k covers across its direction.
func armWidth(k byte, lt int) int {
	switch k {
	case '1':
		return lt
	case '2':
		return 2 * lt
	case '3':
		return 3 * lt
	}
	return 0
}

// stroke fills a band of weight k across [a0,a1) along the arm; a double weight is
// two light strokes with a light gap between them.
func stroke(m *image.Alpha, k byte, vertical bool, a0, a1, span, lt int) {
	bw := armWidth(k, lt)
	b0 := (span - bw) / 2
	bands := [][2]int{{b0, b0 + bw}}
	if k == '3' {
		bands = [][2]int{{b0, b0 + lt}, {b0 + 2*lt, b0 + 3*lt}}
	}
	for _, b := range bands {
		if vertical {
			fillMask(m, b[0], a0, b[1], a1, 0xff)
		} else {
			fillMask(m, a0, b[0], a1, b[1], 0xff)
		}
	}
}

// drawArms draws each arm from its edge through the junction, sized so arms of
// different weights still meet without gaps.
func drawArms(m *image.Alpha, arms string, w, h, lt int) {
	u, r, d, l := arms[0], arms[1], arms[2], arms[3]
	tv := max(armWidth(u, lt), armWidth(d, lt)) // vertical band the horizontal arms run into
	th := max(armWidth(l, lt), armWidth(r, lt))
	xs, xe := (w-tv)/2, (w-tv)/2+tv
	ys, ye := (h-th)/2, (h-th)/2+th
	if u != '0' {
		stroke(m, u, true, 0, ye, w, lt)
	}
	if d != '0' {
		stroke(m, d, true, ys, h, w, lt)
	}
	if l != '0' {
		stroke(m, l, false, 0, xe, h, lt)
	}
	if r != '0' {
		stroke(m, r, false, xs, w, h, lt)
	}
}

// drawArc draws a light rounded corner whose straight ends line up with │ and ─.
func drawArc(m *image.Alpha, r rune, w, h, lt int) {
	dx, dy := 1, 1 // direction of the horizontal and vertical arms
	switch r {
	case 0x256E:
		dx = -1
	case 0x256F:
		dx, dy = -1, -1
	case 0x2570:
		dy = -1
	}
	x0, y0 := (w-lt)/2, (h-lt)/2
	cx, cy := float64(x0)+float64(lt)/2, float64(y0)+float64(lt)/2 // stroke centre lines
	rad := math.Max(1, math.Min(math.Min(cx, float64(w)-cx), math.Min(cy, float64(h)-cy))-1)
	ox, oy := cx+float64(dx)*rad, cy+float64(dy)*rad // arc centre
	half := float64(lt) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if (px-ox)*float64(dx) > 0 || (py-oy)*float64(dy) > 0 {
				continue // only the quarter between the two arms
			}
			if math.Abs(math.Hypot(px-ox, py-oy)-rad) <= half {
				m.Pix[y*m.Stride+x] = 0xff
			}
		}
	}
	if dy > 0 {
		fillMask(m, x0, int(oy), x0+lt, h, 0xff)
	} else {
		fillMask(m, x0, 0, x0+lt, int(math.Ceil(oy)), 0xff)
	}
	if dx > 0 {
		fillMask(m, int(ox), y0, w, y0+lt, 0xff)
	} else {
		fillMask(m, 0, y0, int(math.Ceil(ox)), y0+lt, 0xff)
	}
}

// drawDiagonal steps one row at a time, which keeps the tall line connected.
func drawDiagonal(m *image.Alpha, w, h, lt int, rising bool) {
	for y := 0; y < h; y++ {
		t := (float64(y) + 0.5) / float64(h)
		if rising {
			t = 1 - t
		}
		x := int(t*float64(w)) - (lt-1)/2
		fillMask(m, x, y, x+lt, y+1, 0xff)
	}
}

func drawBlock(m *image.Alpha, r rune, w, h int) {
	eighthsH := func(n int) int { return int(math.Round(float64(h) * float64(n) / 8)) }
	eighthsW := func(n int) int { return int(math.Round(float64(w) * float64(n) / 8)) }
	switch {
	case r == 0x2580:
		fillMask(m, 0, 0, w, h/2, 0xff)
	case r <= 0x2588: // lower n eighths; 2588 is the full block
		fillMask(m, 0, h-eighthsH(int(r-0x2580)), w, h, 0xff)
	case r <= 0x258F: // left n eighths
		fillMask(m, 0, 0, eighthsW(int(0x2590-r)), h, 0xff)
	case r == 0x2590:
		fillMask(m, w/2, 0, w, h, 0xff)
	case r <= 0x2593: // shades as flat coverage
		fillMask(m, 0, 0, w, h, uint8(64*(r-0x2590)))
	case r == 0x2594:
		fillMask(m, 0, 0, w, eighthsH(1), 0xff)
	case r == 0x2595:
		fillMask(m, w-eighthsW(1), 0, w, h, 0xff)
	default:
		q := quadrants[r-0x2596]
		mx, my := w/2, h/2
		if q&1 != 0 {
			fillMask(m, 0, 0, mx, my, 0xff)
		}
		if q&2 != 0 {
			fillMask(m, mx, 0, w, my, 0xff)
		}
		if q&4 != 0 {
			fillMask(m, 0, my, mx, h, 0xff)
		}
		if q&8 != 0 {
			fillMask(m, mx, my, w, h, 0xff)
		}
	}
}
