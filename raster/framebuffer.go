package raster

import (
	"image"
	"math"
)

// RGBA is a straight-alpha 8-bit color channel group.
type RGBA struct {
	R, G, B, A uint8
}

// Framebuffer stores an opaque color image and a positive linear depth buffer.
// Depth zero means "no sample".
type Framebuffer struct {
	Width, Height int
	Color         []RGBA
	Depth         []float64
	Background    RGBA
	glassOwner    []int // mesh index that last blended each pixel, -1 when none
}

// NewFramebuffer allocates a cleared buffer with the given background color.
func NewFramebuffer(w, h int, bg RGBA) *Framebuffer {
	fb := &Framebuffer{
		Width:      w,
		Height:     h,
		Color:      make([]RGBA, w*h),
		Depth:      make([]float64, w*h),
		glassOwner: make([]int, w*h),
		Background: bg,
	}
	fb.Clear()
	return fb
}

// Clear restores the background color and empties every depth cell.
func (fb *Framebuffer) Clear() {
	for i := range fb.Color {
		fb.Color[i] = fb.Background
		fb.Depth[i] = 0
		fb.glassOwner[i] = -1
	}
}

// At returns the cell index for a pixel.
func (fb *Framebuffer) At(x, y int) int { return y*fb.Width + x }

// Scissor limits rasterization to an axis-aligned tile.
type Scissor struct {
	X0, Y0, X1, Y1 int
}

// FullScissor covers every pixel.
func FullScissor(w, h int) Scissor { return Scissor{0, 0, w, h} }

func (s Scissor) clip(bx0, by0, bx1, by1, w, h int) (int, int, int, int) {
	x0 := max(bx0, s.X0)
	y0 := max(by0, s.Y0)
	x1 := min(bx1, s.X1)
	y1 := min(by1, s.Y1)
	return max(x0, 0), max(y0, 0), min(x1, w), min(y1, h)
}

// Image exports the color buffer as an NRGBA image.
func (fb *Framebuffer) Image() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, fb.Width, fb.Height))
	for i, c := range fb.Color {
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func blend(dst, src RGBA) RGBA {
	a := float64(src.A) / 255.0
	mix := func(d, s uint8) uint8 {
		v := float64(s)*a + float64(d)*(1-a)
		return uint8(math.Round(math.Min(255, math.Max(0, v))))
	}
	return RGBA{R: mix(dst.R, src.R), G: mix(dst.G, src.G), B: mix(dst.B, src.B), A: 255}
}
