//go:build ignore

// Command claude_mascot draws the Claude Code pixel mascot as a white-on-transparent
// 96x96 PNG (eyes are holes), the shape kaze tints for the Claude agent logo.
//
//	go run tools/branding/claude_mascot.go assets/branding/agents/claude-96.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// The mascot as Claude Code prints it with half blocks: 18 sub-pixels wide, each one
// unit wide and two tall. The two gaps in the second row are the eyes; the single
// pixels in the last row are the feet. # is body, . is empty.
var grid = []string{
	"...############...",
	"...##.######.##...",
	".################.",
	"...############...",
	"....#.#....#.#....",
}

func main() {
	const size, unitW, unitH = 96, 5, 10
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	offX := (size - len(grid[0])*unitW) / 2
	offY := (size - len(grid)*unitH) / 2
	for r, row := range grid {
		for c, ch := range row {
			if ch != '#' {
				continue
			}
			for y := 0; y < unitH; y++ {
				for x := 0; x < unitW; x++ {
					img.SetNRGBA(offX+c*unitW+x, offY+r*unitH+y, color.NRGBA{255, 255, 255, 255})
				}
			}
		}
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
