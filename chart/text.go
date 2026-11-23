package chart

import (
	"image"
	"image/color"
)

var glyphs = map[rune][]string{
	'R': {
		"11110",
		"10001",
		"10001",
		"11110",
		"10100",
		"10010",
		"10001",
	},
	'G': {
		"01110",
		"10001",
		"10000",
		"10111",
		"10001",
		"10001",
		"01110",
	},
	'B': {
		"11110",
		"10001",
		"10001",
		"11110",
		"10001",
		"10001",
		"11110",
	},
	'C': {
		"01111",
		"10000",
		"10000",
		"10000",
		"10000",
		"10000",
		"01111",
	},
	'A': {
		"01110",
		"10001",
		"10001",
		"11111",
		"10001",
		"10001",
		"10001",
	},
}

func drawColoredString(img *image.RGBA, text string, x, y, scale int, colors []color.RGBA) {
	if scale < 1 || len(colors) == 0 {
		return
	}
	cursor := x
	for index, r := range text {
		rows, ok := glyphs[r]
		if !ok {
			cursor += 6 * scale
			continue
		}
		c := colors[index%len(colors)]
		for row, bits := range rows {
			for col, bit := range bits {
				if bit != '1' {
					continue
				}
				fillRect(img,
					cursor+col*scale,
					y+row*scale,
					cursor+(col+1)*scale,
					y+(row+1)*scale,
					c,
				)
			}
		}
		cursor += 6 * scale
	}
}
