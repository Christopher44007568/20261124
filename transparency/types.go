package transparency

import "math"

// Vec2 is a point or displacement in pixel space.
type Vec2 struct {
	X float64
	Y float64
}

func (v Vec2) Add(o Vec2) Vec2 { return Vec2{v.X + o.X, v.Y + o.Y} }
func (v Vec2) Sub(o Vec2) Vec2 { return Vec2{v.X - o.X, v.Y - o.Y} }
func (v Vec2) Mul(s float64) Vec2 {
	return Vec2{v.X * s, v.Y * s}
}

func (v Vec2) Length() float64 { return math.Hypot(v.X, v.Y) }

// RGB stores linear color components. Values are not gamma encoded.
type RGB struct {
	R float64
	G float64
	B float64
}

func (c RGB) Add(o RGB) RGB {
	return RGB{c.R + o.R, c.G + o.G, c.B + o.B}
}

func (c RGB) Mul(s float64) RGB {
	return RGB{c.R * s, c.G * s, c.B * s}
}

func (c RGB) Lerp(o RGB, t float64) RGB {
	return RGB{
		R: c.R + (o.R-c.R)*t,
		G: c.G + (o.G-c.G)*t,
		B: c.B + (o.B-c.B)*t,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// LayerKind identifies the transparent phenomenon being rendered.
type LayerKind string

const (
	KindFog      LayerKind = "fog"
	KindGlass    LayerKind = "glass"
	KindFilament LayerKind = "filament"
)

// Shape answers two distinct questions:
//   - Inside reports whether a subpixel sample hits the object.
//   - Support reports whether the pixel footprint can contain the object.
//
// Support is deliberately larger than Inside. Temporal history is only allowed
// inside Support, so coverage left behind by a moving object cannot survive as
// a fake near surface.
type Shape interface {
	Inside(p Vec2) bool
	Support(p Vec2) bool
}

// Layer is one transparent object for one frame.
type Layer struct {
	ID             string
	Kind           LayerKind
	Depth          float64
	DepthTolerance float64
	Opacity        float64
	Color          RGB
	Motion         Vec2
	HistoryWeight  float64
	Shape          Shape
	DepthAt        func(Vec2) float64
	Shade          func(Vec2) float64
}

// Surface is the sampled state of a layer at one pixel.
type Surface struct {
	Depth   float64
	Support float64
	Alpha   float64
	Color   RGB
}

// LayerImage stores a rasterized transparent object. Layers remain separate
// until temporal resolution and depth-ordered compositing.
type LayerImage struct {
	ID             string
	Kind           LayerKind
	Depth          float64
	DepthTolerance float64
	Motion         Vec2
	HistoryWeight  float64
	Pixels         []Surface
}

func (img *LayerImage) At(width, x, y int) *Surface {
	return &img.Pixels[y*width+x]
}

// FrameResult is a resolved frame plus the metadata needed to audit coverage
// and draw order. Alpha is the cumulative alpha of all transparent surfaces.
// NearAlpha and NearTop only concern layers closer than NearDepthCutoff.
type FrameResult struct {
	RGB         []RGB
	Alpha       []float64
	NearAlpha   []float64
	NearSupport []float64
	NearTop     []string
}

// FrameMetrics compares one rendered frame to the supersampled reference.
type FrameMetrics struct {
	ColorRMSE      float64 `json:"color_rmse"`
	AlphaMAE       float64 `json:"alpha_mae"`
	CoverageIoU    float64 `json:"coverage_iou"`
	OrderAgreement float64 `json:"order_agreement"`
	StaleCoverage  float64 `json:"stale_coverage"`
}

// Config controls the deterministic comparison.
type Config struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Frames           int     `json:"frames"`
	ReferenceSamples int     `json:"reference_samples"`
	NearDepthCutoff  float64 `json:"near_depth_cutoff"`
	CoverageCutoff   float64 `json:"coverage_cutoff"`
	NearAlphaCutoff  float64 `json:"near_alpha_cutoff"`
}

func (c Config) withDefaults() Config {
	if c.Width <= 0 {
		c.Width = 96
	}
	if c.Height <= 0 {
		c.Height = 64
	}
	if c.Frames <= 0 {
		c.Frames = 24
	}
	if c.ReferenceSamples <= 0 {
		c.ReferenceSamples = 6
	}
	if c.NearDepthCutoff <= 0 {
		c.NearDepthCutoff = 10
	}
	if c.CoverageCutoff <= 0 {
		c.CoverageCutoff = 0.25
	}
	if c.NearAlphaCutoff <= 0 {
		c.NearAlphaCutoff = 0.05
	}
	return c
}

// Strategy names are stable so reports and tests can refer to them.
const (
	StrategyPerLayer   = "per-layer"
	StrategyWholeBlock = "whole-block"
	StrategyNoHistory  = "no-history"
)
