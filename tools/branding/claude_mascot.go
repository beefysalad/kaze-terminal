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

// 12 columns by 8 rows; # is body, . is empty. Eyes are the two holes.
var grid = []string{
	"..########..",
	"..##.##.##..",
	"..##.##.##..",
	"############",
	"############",
	"..########..",
	"..#.#..#.#..",
	"..#.#..#.#..",
}

func main() {
	const size, cell = 96, 8
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	offY := (size - len(grid)*cell) / 2
	offX := (size - len(grid[0])*cell) / 2
	for r, row := range grid {
		for c, ch := range row {
			if ch != '#' {
				continue
			}
			for y := 0; y < cell; y++ {
				for x := 0; x < cell; x++ {
					img.SetNRGBA(offX+c*cell+x, offY+r*cell+y, color.NRGBA{255, 255, 255, 255})
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
