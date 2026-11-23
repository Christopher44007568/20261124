// Package checker implements a 2:2 checkerboard shading-rate pipeline with an
// aperiodic, edge-aware reconstruction of the skipped peripheral pixels.
//
// The package has no third-party dependencies: every type and routine below is
// built on the Go standard library.
package checker

import "math"

// RGB is a linear-ish float color channel triple in [0,1].
type RGB struct {
	R, G, B float64
}

// Image is a densely addressable RGB raster.
type Image struct {
	W, H int
	Px   []RGB
}

// NewImage allocates an RGB raster of the given size.
func NewImage(w, h int) *Image {
	return &Image{W: w, H: h, Px: make([]RGB, w*h)}
}

// At returns the color at (x,y); out-of-range access returns black.
func (m *Image) At(x, y int) RGB {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return RGB{}
	}
	return m.Px[y*m.W+x]
}

// Set writes the color at (x,y), ignoring out-of-range coordinates.
func (m *Image) Set(x, y int, c RGB) {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return
	}
	m.Px[y*m.W+x] = c
}

// Clone returns a deep copy of the image.
func (m *Image) Clone() *Image {
	out := NewImage(m.W, m.H)
	copy(out.Px, m.Px)
	return out
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

// Luma is the Rec.709 luminance of a color.
func Luma(c RGB) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// Hue returns the RGB hue in radians on [0, 2*pi), together with the chroma.
// Near-achromatic colors report a zero chroma and an undefined hue of zero.
func Hue(c RGB) (hue, chroma float64) {
	mx := math.Max(c.R, math.Max(c.G, c.B))
	mn := math.Min(c.R, math.Min(c.G, c.B))
	chroma = mx - mn
	if chroma < 1e-12 {
		return 0, 0
	}
	var h float64
	switch mx {
	case c.R:
		h = math.Mod((c.G-c.B)/chroma, 6)
	case c.G:
		h = (c.B-c.R)/chroma + 2
	default:
		h = (c.R-c.G)/chroma + 4
	}
	h *= math.Pi / 3
	if h < 0 {
		h += 2 * math.Pi
	}
	return h, chroma
}

// HueDistance is the shortest angular distance between two hues in radians.
func HueDistance(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	return d
}
