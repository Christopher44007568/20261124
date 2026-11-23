// Package warp implements two single-frame timewarp paths: pure rotation and
// depth-aware translation reprojection.
package warp

import (
	"image/color"
	"math"

	"timewarp/geom"
	"timewarp/scene"
)

// RejectReason is stored per target pixel for the diagnostic masks.
type RejectReason uint8

const (
	RejectNone RejectReason = iota
	RejectOutOfBounds
	RejectNoCoverage
	RejectBehindCamera
)

func (r RejectReason) String() string {
	switch r {
	case RejectOutOfBounds:
		return "out of bounds"
	case RejectNoCoverage:
		return "no coverage"
	case RejectBehindCamera:
		return "behind camera"
	default:
		return "valid"
	}
}

// Source is the previous frame handed to a timewarp. Missing depth is
// represented by DepthValid=false; color remains available for diagnostics.
type Source struct {
	W, H       int
	Pix        []color.RGBA
	Depth      []float64
	DepthValid []bool
}

func SourceFromFrame(f *scene.Frame) *Source {
	src := &Source{
		W: f.W, H: f.H,
		Pix:        f.Pix,
		Depth:      f.Depth,
		DepthValid: f.DepthValid,
	}
	return src
}

// Stats records both target rejections and source projections that fell
// outside the target view. No counter is incremented by clamped sampling:
// rejected coordinates never reach the sampler.
type Stats struct {
	Evaluated       int
	Valid           int
	Rejected        int
	OutOfBounds     int
	NoCoverage      int
	BehindCamera    int
	SourceNoDepth   int
	SourceOffscreen int
}

func (s Stats) Coverage() float64 {
	if s.Evaluated == 0 {
		return 0
	}
	return float64(s.Valid) / float64(s.Evaluated)
}

// Result is a warped frame. Rotation results deliberately leave Depth and
// DepthValid untouched; that path never invents depth.
type Result struct {
	W, H       int
	Pix        []color.RGBA
	Depth      []float64
	DepthValid []bool
	Reject     []RejectReason
	Stats      Stats
}

func newResult(w, h int) *Result {
	n := w * h
	return &Result{
		W: w, H: h,
		Pix:        make([]color.RGBA, n),
		Depth:      make([]float64, n),
		DepthValid: make([]bool, n),
		Reject:     make([]RejectReason, n),
	}
}

func (r *Result) At(x, y int) color.RGBA { return r.Pix[y*r.W+x] }

func sampleBilinear(pix []color.RGBA, w, h int, u, v float64) (color.RGBA, bool) {
	const eps = 1e-7
	if w <= 0 || h <= 0 || !(u >= -eps) || !(v >= -eps) || !(u <= float64(w-1)+eps) || !(v <= float64(h-1)+eps) {
		return color.RGBA{}, false
	}
	u = math.Min(math.Max(u, 0), float64(w-1))
	v = math.Min(math.Max(v, 0), float64(h-1))
	x0 := int(u)
	y0 := int(v)
	x1, y1 := x0+1, y0+1
	if x1 >= w {
		x1 = w - 1
	}
	if y1 >= h {
		y1 = h - 1
	}
	fx := u - float64(x0)
	fy := v - float64(y0)
	c00 := pix[y0*w+x0]
	c10 := pix[y0*w+x1]
	c01 := pix[y1*w+x0]
	c11 := pix[y1*w+x1]
	lerp := func(a, b uint8, t float64) float64 {
		return float64(a) + (float64(b)-float64(a))*t
	}
	mix := func(a, b, c, d uint8) uint8 {
		top := lerp(a, b, fx)
		bot := lerp(c, d, fx)
		return uint8(top + (bot-top)*fy + 0.5)
	}
	return color.RGBA{
		R: mix(c00.R, c10.R, c01.R, c11.R),
		G: mix(c00.G, c10.G, c01.G, c11.G),
		B: mix(c00.B, c10.B, c01.B, c11.B),
		A: 255,
	}, true
}

// PixelCoord converts an integer pixel index to the image-space sample
// coordinate used by geom.Intrinsics. Pixel centers are at integer indices.
func PixelCoord(x, y int) (float64, float64) {
	return float64(x), float64(y)
}

func rotationDelta(from, to geom.Pose) geom.Mat3 {
	// A target camera ray d_t becomes a world ray R_t^T d_t, then a source
	// camera ray R_s R_t^T d_t.
	return from.R.Mul(to.R.Transpose())
}
