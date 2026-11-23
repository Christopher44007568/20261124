// Package imaging contains the dependency-free image primitives used by the
// synthetic evaluation suite.
package imaging

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// RGB is a linear, normalized color sample in the range [0, 1].
type RGB struct {
	R float64
	G float64
	B float64
}

func (c RGB) Clamp() RGB {
	return RGB{clamp01(c.R), clamp01(c.G), clamp01(c.B)}
}

func (c RGB) Add(o RGB) RGB {
	return RGB{c.R + o.R, c.G + o.G, c.B + o.B}
}

func (c RGB) Sub(o RGB) RGB {
	return RGB{c.R - o.R, c.G - o.G, c.B - o.B}
}

func (c RGB) Scale(s float64) RGB {
	return RGB{c.R * s, c.G * s, c.B * s}
}

func (c RGB) Abs() RGB {
	return RGB{math.Abs(c.R), math.Abs(c.G), math.Abs(c.B)}
}

func (c RGB) Mean() float64 {
	return (c.R + c.G + c.B) / 3
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Frame is a tightly packed RGB image with no external image-library
// representation in the hot path.
type Frame struct {
	W   int
	H   int
	Pix []RGB
}

func NewFrame(w, h int) *Frame {
	if w <= 0 || h <= 0 {
		panic("imaging: frame dimensions must be positive")
	}
	return &Frame{W: w, H: h, Pix: make([]RGB, w*h)}
}

func (f *Frame) Bounds() image.Rectangle {
	return image.Rect(0, 0, f.W, f.H)
}

func (f *Frame) Index(x, y int) int {
	return y*f.W + x
}

func (f *Frame) At(x, y int) RGB {
	return f.Pix[f.Index(x, y)]
}

func (f *Frame) Set(x, y int, c RGB) {
	f.Pix[f.Index(x, y)] = c
}

func (f *Frame) Clone() *Frame {
	g := &Frame{W: f.W, H: f.H, Pix: make([]RGB, len(f.Pix))}
	copy(g.Pix, f.Pix)
	return g
}

func (f *Frame) Fill(c RGB) {
	for i := range f.Pix {
		f.Pix[i] = c
	}
}

// NRGBA converts the normalized frame to the standard library image type.
func (f *Frame) NRGBA() *image.NRGBA {
	out := image.NewNRGBA(f.Bounds())
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			c := f.At(x, y).Clamp()
			i := out.PixOffset(x, y)
			out.Pix[i+0] = uint8(math.Round(c.R * 255))
			out.Pix[i+1] = uint8(math.Round(c.G * 255))
			out.Pix[i+2] = uint8(math.Round(c.B * 255))
			out.Pix[i+3] = 255
		}
	}
	return out
}

// FromImage converts an sRGB image to normalized samples. This is useful for
// retaining a small import/export seam without adding a third-party codec.
func FromImage(src image.Image) *Frame {
	b := src.Bounds()
	f := NewFrame(b.Dx(), b.Dy())
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			r, g, bl, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			f.Set(x, y, RGB{
				R: float64(r) / 65535,
				G: float64(g) / 65535,
				B: float64(bl) / 65535,
			})
		}
	}
	return f
}

func (c RGB) NRGBA() color.NRGBA {
	v := c.Clamp()
	return color.NRGBA{
		R: uint8(math.Round(v.R * 255)),
		G: uint8(math.Round(v.G * 255)),
		B: uint8(math.Round(v.B * 255)),
		A: 255,
	}
}

func (f *Frame) String() string {
	return fmt.Sprintf("frame(%dx%d)", f.W, f.H)
}
