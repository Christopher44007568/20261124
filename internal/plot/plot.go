// Package plot renders simple line charts to PNG using only the Go
// standard library (image, image/png). No graphics SDK is involved.
package plot

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"

	"strings"
)

// Series is one named polyline.
type Series struct {
	Name  string
	Color color.RGBA
	Y     []float64
}

// Canvas is a minimal RGBA raster with line and text primitives.
type Canvas struct {
	Img *image.RGBA
}

// NewCanvas returns a white canvas of the given size.
func NewCanvas(w, h int) *Canvas {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	return &Canvas{Img: img}
}

// Set draws a pixel if inside bounds.
func (c *Canvas) Set(x, y int, col color.RGBA) {
	if image.Pt(x, y).In(c.Img.Rect) {
		c.Img.SetRGBA(x, y, col)
	}
}

// Line draws a Bresenham line.
func (c *Canvas) Line(x0, y0, x1, y1 int, col color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	err := dx + dy
	for {
		c.Set(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// Rect fills a rectangle.
func (c *Canvas) Rect(x, y, w, h int, col color.RGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.Set(i, j, col)
		}
	}
}

// Text draws uppercase 5x7 bitmap text, scaled by an integer factor.
func (c *Canvas) Text(x, y int, s string, scale int, col color.RGBA) {
	if scale < 1 {
		scale = 1
	}
	cx := x
	for _, r := range strings.ToUpper(s) {
		glyph, ok := font[r]
		if !ok {
			glyph = font[' ']
		}
		for row := 0; row < 7; row++ {
			for bit := 0; bit < 5; bit++ {
				if glyph[row]&(0x10>>bit) != 0 {
					c.Rect(cx+bit*scale, y+row*scale, scale, scale, col)
				}
			}
		}
		cx += 6 * scale
	}
}

// SavePNG writes the canvas as PNG.
func (c *Canvas) SavePNG(w io.Writer) error {
	return png.Encode(w, c.Img)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// Chart renders the series into a PNG with axes, gridlines, value
// labels, a title, and a legend. yMax <= 0 auto-scales to the data.
func Chart(w io.Writer, title, yLabel string, series []Series, yMax float64) error {
	const (
		W, H   = 900, 420
		left   = 90
		right  = 20
		top    = 40
		bottom = 50
	)
	c := NewCanvas(W, H)
	black := color.RGBA{0x20, 0x20, 0x20, 0xFF}
	grey := color.RGBA{0xDD, 0xDD, 0xDD, 0xFF}

	max := yMax
	n := 0
	for _, s := range series {
		if len(s.Y) > n {
			n = len(s.Y)
		}
		for _, v := range s.Y {
			if v > max {
				max = v
			}
		}
	}
	if max <= 0 || n == 0 {
		max = 1
	}
	max *= 1.05

	plotW := W - left - right
	plotH := H - top - bottom
	// Gridlines and y tick labels.
	for i := 0; i <= 4; i++ {
		y := top + plotH*i/4
		c.Line(left, y, W-right, y, grey)
		v := max * float64(4-i) / 4
		c.Text(4, y-3, fmt.Sprintf("%.1f", v), 1, black)
	}
	// Axes.
	c.Line(left, top, left, H-bottom, black)
	c.Line(left, H-bottom, W-right, H-bottom, black)
	c.Text(left, 12, title, 2, black)
	c.Text(W-right-120, 12, yLabel, 1, black)

	// Series polylines.
	for _, s := range series {
		m := len(s.Y)
		if m == 0 {
			continue
		}
		xOf := func(i int) int { return left + plotW*i/(n-1) }
		yOf := func(v float64) int {
			y := H - bottom - int(float64(plotH)*v/max)
			if y < top {
				y = top
			}
			return y
		}
		for i := 1; i < m; i++ {
			c.Line(xOf(i-1), yOf(s.Y[i-1]), xOf(i), yOf(s.Y[i]), s.Color)
		}
	}
	// Legend.
	lx := left + 10
	for _, s := range series {
		c.Rect(lx, H-bottom+18, 14, 8, s.Color)
		c.Text(lx+20, H-bottom+16, s.Name, 1, black)
		lx += 20 + 6*len(s.Name) + 24
	}
	return c.SavePNG(w)
}

// LossBands converts per-frame tracking-loss flags into a 0/yMax series
// that can be overlaid on a chart to mark loss intervals.
func LossBands(lost []bool, yMax float64) []float64 {
	out := make([]float64, len(lost))
	for i, l := range lost {
		if l {
			out[i] = yMax
		}
	}
	return out
}
