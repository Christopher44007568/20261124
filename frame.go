// Package gazedenoise builds deterministic foveated-denoising experiments.
//
// The package deliberately uses only the Go standard library. All image
// buffers are planar slices indexed by y*Width+x so that a pixel's colour,
// normal, depth, luminance, gaze membership and validity share one index.
package gazedenoise

import (
	"errors"
	"fmt"
	"math"
)

// Vec3 is a three-component vector used for linear RGB and normals.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

// Add returns a+b.
func (a Vec3) Add(b Vec3) Vec3 {
	return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z}
}

// Sub returns a-b.
func (a Vec3) Sub(b Vec3) Vec3 {
	return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z}
}

// Mul returns a scaled by s.
func (a Vec3) Mul(s float64) Vec3 {
	return Vec3{a.X * s, a.Y * s, a.Z * s}
}

// Dot returns the dot product of a and b.
func (a Vec3) Dot(b Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// Length returns the Euclidean length of a.
func (a Vec3) Length() float64 {
	return math.Sqrt(a.Dot(a))
}

// Normalized returns a unit vector, or the zero vector for zero input.
func (a Vec3) Normalized() Vec3 {
	n := a.Length()
	if n == 0 {
		return Vec3{}
	}
	return a.Mul(1 / n)
}

// Luma returns Rec. 709 linear luminance.
func (a Vec3) Luma() float64 {
	return 0.2126*a.X + 0.7152*a.Y + 0.0722*a.Z
}

// Clamp restricts x to [lo, hi].
func Clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// Clamp01 restricts x to [0, 1].
func Clamp01(x float64) float64 {
	return Clamp(x, 0, 1)
}

// Frame is a G-buffer used by the experiment. Mask is true inside the gaze
// region. Valid is false for missing samples that must be hole-filled.
type Frame struct {
	Width  int
	Height int

	Color  []Vec3
	Normal []Vec3
	Depth  []float64
	Luma   []float64
	Mask   []bool
	Valid  []bool
}

// NewFrame allocates a valid, fully outside-gaze frame.
func NewFrame(width, height int) (*Frame, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid frame size %dx%d", width, height)
	}
	n := width * height
	f := &Frame{
		Width:  width,
		Height: height,
		Color:  make([]Vec3, n),
		Normal: make([]Vec3, n),
		Depth:  make([]float64, n),
		Luma:   make([]float64, n),
		Mask:   make([]bool, n),
		Valid:  make([]bool, n),
	}
	for i := 0; i < n; i++ {
		f.Normal[i] = Vec3{Z: 1}
		f.Depth[i] = 1
		f.Valid[i] = true
	}
	return f, nil
}

// Index converts an in-bounds coordinate to the planar slice index.
func (f *Frame) Index(x, y int) int {
	return y*f.Width + x
}

// InBounds reports whether (x, y) is inside the frame.
func (f *Frame) InBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < f.Width && y < f.Height
}

// LuminanceAt returns the guide luminance at index i.
func (f *Frame) LuminanceAt(i int) float64 {
	if i < 0 || i >= len(f.Luma) {
		return 0
	}
	return f.Luma[i]
}

// RecomputeLuma refreshes the luminance guide from Color.
func (f *Frame) RecomputeLuma() {
	for i := range f.Color {
		f.Luma[i] = f.Color[i].Luma()
	}
}

// Clone returns a deep copy of f.
func (f *Frame) Clone() *Frame {
	if f == nil {
		return nil
	}
	out := &Frame{Width: f.Width, Height: f.Height}
	out.Color = append([]Vec3(nil), f.Color...)
	out.Normal = append([]Vec3(nil), f.Normal...)
	out.Depth = append([]float64(nil), f.Depth...)
	out.Luma = append([]float64(nil), f.Luma...)
	out.Mask = append([]bool(nil), f.Mask...)
	out.Valid = append([]bool(nil), f.Valid...)
	return out
}

// Validate checks that all buffers have matching dimensions.
func (f *Frame) Validate() error {
	if f == nil {
		return errors.New("nil frame")
	}
	if f.Width <= 0 || f.Height <= 0 {
		return fmt.Errorf("invalid frame size %dx%d", f.Width, f.Height)
	}
	n := f.Width * f.Height
	checks := []struct {
		name string
		size int
	}{
		{"color", len(f.Color)},
		{"normal", len(f.Normal)},
		{"depth", len(f.Depth)},
		{"luma", len(f.Luma)},
		{"mask", len(f.Mask)},
		{"valid", len(f.Valid)},
	}
	for _, c := range checks {
		if c.size != n {
			return fmt.Errorf("%s buffer has %d values, want %d", c.name, c.size, n)
		}
	}
	return nil
}

// CountMask returns the numbers of centre and periphery pixels.
func (f *Frame) CountMask() (centre, periphery int) {
	for _, inside := range f.Mask {
		if inside {
			centre++
		} else {
			periphery++
		}
	}
	return centre, periphery
}
