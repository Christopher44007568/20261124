package main

import (
	"image"
	"image/png"
	"math"
	"os"
)

// RGB is a plain 8-bit color.
type RGB struct {
	R, G, B uint8
}

var (
	colBg       = RGB{18, 20, 26}
	colWall     = RGB{210, 214, 220}
	colObjIdle  = RGB{70, 76, 88}
	colRendered = RGB{64, 200, 110}
	colOverdraw = RGB{235, 150, 40}
	colMiss     = RGB{235, 40, 220}
	colLight    = RGB{250, 220, 90}
	colEye      = RGB{90, 200, 250}
	colViewer   = RGB{245, 245, 245}
	colText     = RGB{220, 224, 232}
)

// Canvas is a minimal software rasterizer over image.RGBA.
type Canvas struct {
	Img *image.RGBA
	W   int
	H   int
}

func NewCanvas(w, h int, bg RGB) *Canvas {
	c := &Canvas{Img: image.NewRGBA(image.Rect(0, 0, w, h)), W: w, H: h}
	for i := 0; i < w*h; i++ {
		o := i * 4
		c.Img.Pix[o] = bg.R
		c.Img.Pix[o+1] = bg.G
		c.Img.Pix[o+2] = bg.B
		c.Img.Pix[o+3] = 255
	}
	return c
}

func (c *Canvas) Set(x, y int, col RGB) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	o := c.Img.PixOffset(x, y)
	c.Img.Pix[o] = col.R
	c.Img.Pix[o+1] = col.G
	c.Img.Pix[o+2] = col.B
	c.Img.Pix[o+3] = 255
}

// Blend alpha-composites col over the existing pixel (a in [0,1]).
func (c *Canvas) Blend(x, y int, col RGB, a float64) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	o := c.Img.PixOffset(x, y)
	ia := 1 - a
	c.Img.Pix[o] = uint8(float64(c.Img.Pix[o])*ia + float64(col.R)*a)
	c.Img.Pix[o+1] = uint8(float64(c.Img.Pix[o+1])*ia + float64(col.G)*a)
	c.Img.Pix[o+2] = uint8(float64(c.Img.Pix[o+2])*ia + float64(col.B)*a)
}

func (c *Canvas) Line(x0, y0, x1, y1 int, col RGB) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
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

func (c *Canvas) FillCircle(cx, cy, r int, col RGB) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				c.Set(cx+x, cy+y, col)
			}
		}
	}
}

func (c *Canvas) Ring(cx, cy, r int, col RGB) {
	steps := 6*r + 24
	for i := 0; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		c.Set(cx+int(math.Round(math.Cos(a)*float64(r))),
			cy+int(math.Round(math.Sin(a)*float64(r))), col)
	}
}

// FillTri fills a triangle with alpha blending (used for frustum cones).
func (c *Canvas) FillTri(p0, p1, p2 [2]int, col RGB, a float64) {
	minX := min3(p0[0], p1[0], p2[0])
	maxX := max3(p0[0], p1[0], p2[0])
	minY := min3(p0[1], p1[1], p2[1])
	maxY := max3(p0[1], p1[1], p2[1])
	edge := func(ax, ay, bx, by, px, py int) float64 {
		return float64((bx-ax)*(py-ay) - (by-ay)*(px-ax))
	}
	area := edge(p0[0], p0[1], p1[0], p1[1], p2[0], p2[1])
	if math.Abs(area) < 1e-9 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			w0 := edge(p1[0], p1[1], p2[0], p2[1], x, y)
			w1 := edge(p2[0], p2[1], p0[0], p0[1], x, y)
			w2 := edge(p0[0], p0[1], p1[0], p1[1], x, y)
			if (w0 >= 0 && w1 >= 0 && w2 >= 0) || (w0 <= 0 && w1 <= 0 && w2 <= 0) {
				c.Blend(x, y, col, a)
			}
		}
	}
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func max3(a, b, c int) int {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

// SavePNG writes the canvas to disk.
func (c *Canvas) SavePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, c.Img)
}

// Cam maps world coordinates to screen pixels.
type Cam struct {
	Scale float64
	Cx    float64
	Cy    float64
}

func (cam Cam) P(v Vec2) (int, int) {
	return int(math.Round(cam.Cx + v.X*cam.Scale)),
		int(math.Round(cam.Cy + v.Y*cam.Scale))
}
