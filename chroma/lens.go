package chroma

import (
	"fmt"
	"math"
)

// RadialScale returns the channel-specific radial scale at p. The normalized
// radius is zero at the selected eye's optical center and one at every
// unshifted image corner. Consequently the center has no chromatic
// displacement and the corners carry the largest radial displacement.
func RadialScale(cfg Config, p Point, ch Channel) float64 {
	center := cfg.OpticalCenter()
	dx := p.X - center.X
	dy := p.Y - center.Y
	r := math.Hypot(dx, dy) / math.Sqrt2
	return 1 + cfg.EffectiveAberration().For(ch)*r*r
}

// MapPoint maps a normalized field-of-view point through the radial model for
// one channel. p is expressed in the destination field of view and the result
// is the source position to sample for ch.
func MapPoint(cfg Config, p Point, ch Channel) Point {
	center := cfg.OpticalCenter()
	scale := RadialScale(cfg, p, ch)
	return Point{
		X: center.X + (p.X-center.X)*scale,
		Y: center.Y + (p.Y-center.Y)*scale,
	}
}

// Mapper converts output pixels to per-channel source coordinates. Output and
// source images share the same dimensions.
type Mapper struct {
	width  int
	height int
	config Config
}

// NewMapper builds a mapper for an image of width by height pixels.
func NewMapper(width, height int, cfg Config) (*Mapper, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("image dimensions must be positive, got %dx%d", width, height)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Mapper{width: width, height: height, config: cfg}, nil
}

// Bounds returns the mapper's output dimensions.
func (m *Mapper) Bounds() (width, height int) {
	return m.width, m.height
}

// Config returns the mapper's configuration.
func (m *Mapper) Config() Config {
	return m.config
}

// MapNormalized maps a normalized destination point for ch.
func (m *Mapper) MapNormalized(p Point, ch Channel) Point {
	return MapPoint(m.config, p, ch)
}

// MapPixel returns the source pixel coordinate used by ch for destination
// pixel (x, y). Integer pixel coordinates sit on pixel centers; for odd image
// dimensions the center pixel is exactly at normalized (0, 0).
func (m *Mapper) MapPixel(x, y int, ch Channel) Point {
	normalized := m.normalizedFromPixel(x, y)
	mapped := m.MapNormalized(normalized, ch)
	return Point{
		X: m.pixelFromNormalized(mapped.X, m.width),
		Y: m.pixelFromNormalized(mapped.Y, m.height),
	}
}

func (m *Mapper) normalizedFromPixel(x, y int) Point {
	return Point{
		X: normalizedFromPixel(x, m.width),
		Y: normalizedFromPixel(y, m.height),
	}
}

func normalizedFromPixel(v, size int) float64 {
	if size <= 1 {
		return 0
	}
	return 2*float64(v)/float64(size-1) - 1
}

func (m *Mapper) pixelFromNormalized(v float64, size int) float64 {
	if size <= 1 {
		return 0
	}
	return (v + 1) * 0.5 * float64(size-1)
}
