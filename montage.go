package stereoholes

import (
	"image"
	"image/color"
	"image/draw"
)

// Panel is one labeled image in a comparison sheet.
type Panel struct {
	Title  string
	Image  image.Image
	Accent color.RGBA
}

var tinyGlyphs = map[rune][7]uint8{
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0F, 0x10, 0x10, 0x10, 0x10, 0x10, 0x0F},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0F, 0x10, 0x10, 0x17, 0x11, 0x11, 0x0E},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x1F},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'-': {0, 0, 0, 0x0E, 0, 0, 0},
	'(': {0x02, 0x04, 0x08, 0x08, 0x08, 0x04, 0x02},
	')': {0x08, 0x04, 0x02, 0x02, 0x02, 0x04, 0x08},
}

func drawTinyText(dst *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	cursor := x
	for _, r := range text {
		rows, ok := tinyGlyphs[r]
		if !ok {
			cursor += 6 * scale
			continue
		}
		for gy := 0; gy < 7; gy++ {
			for gx := 0; gx < 5; gx++ {
				if rows[gy]&(1<<uint(4-gx)) == 0 {
					continue
				}
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						dst.SetRGBA(cursor+gx*scale+sx, y+gy*scale+sy, c)
					}
				}
			}
		}
		cursor += 6 * scale
	}
}

// ComparisonSheet lays panels out in a grid with accent bars and labels.
func ComparisonSheet(panels []Panel, cols int) *image.RGBA {
	if cols < 1 {
		cols = 1
	}
	rows := (len(panels) + cols - 1) / cols
	cellW, cellH := 0, 0
	for _, p := range panels {
		cellW = max(cellW, p.Image.Bounds().Dx())
		cellH = max(cellH, p.Image.Bounds().Dy())
	}
	const margin, gap, header, border = 18, 14, 28, 2
	w := margin*2 + cols*cellW + (cols-1)*gap
	h := margin*2 + rows*(cellH+header) + (rows-1)*gap
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{8, 11, 17, 255}},
		image.Point{}, draw.Src)

	for i, p := range panels {
		row, col := i/cols, i%cols
		x := margin + col*(cellW+gap)
		y := margin + row*(cellH+header+gap)
		draw.Draw(dst, image.Rect(x, y, x+cellW, y+header),
			&image.Uniform{p.Accent}, image.Point{}, draw.Src)
		drawTinyText(dst, x+9, y+6, p.Title, 2, color.RGBA{245, 246, 250, 255})
		iy := y + header
		draw.Draw(dst, image.Rect(x-border, iy-border,
			x+cellW+border, iy+cellH+border), &image.Uniform{p.Accent},
			image.Point{}, draw.Src)
		draw.Draw(dst, image.Rect(x, iy, x+cellW, iy+cellH), p.Image,
			p.Image.Bounds().Min, draw.Src)
	}
	return dst
}
