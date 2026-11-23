package eyebuffer

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Filter selects the sampling kernel.
type Filter int

const (
	FilterNearest Filter = iota
	FilterBilinear
)

func (f Filter) String() string {
	if f == FilterBilinear {
		return "bilinear"
	}
	return "nearest"
}

// AddressMode selects what a tap outside the selected mip level clamps to.
// ClampToAtlas intentionally models the common "clamp to the whole atlas"
// mistake and is useful while diagnosing bleed.
type AddressMode int

const (
	ClampToLevel AddressMode = iota
	ClampToAtlas
)

func (m AddressMode) String() string {
	if m == ClampToAtlas {
		return "atlas"
	}
	return "level"
}

// Tap records one texture fetch. Raw is the un-clamped local coordinate,
// Local is the coordinate after address handling, and Atlas is the final
// fetched atlas texel.
type Tap struct {
	Raw    image.Point
	Local  image.Point
	Atlas  image.Point
	Weight float64
}

// Sample is a sampled colour plus the exact fetch trace.
type Sample struct {
	Color   color.RGBA
	Filter  Filter
	Address AddressMode
	Taps    []Tap
}

// Sample fetches at normalized UV in [0,1] with level-local clamping.
func (a *Atlas) Sample(eyeIndex, level int, u, v float64, filter Filter) (Sample, error) {
	return a.SampleWithAddress(eyeIndex, level, u, v, filter, ClampToLevel)
}

// SampleWithAddress fetches at normalized UV and records every tap.
func (a *Atlas) SampleWithAddress(eyeIndex, level int, u, v float64, filter Filter, address AddressMode) (Sample, error) {
	lv, ok := a.Layout.Level(eyeIndex, level)
	if !ok {
		return Sample{}, fmt.Errorf("eye %d level %d does not exist", eyeIndex, level)
	}
	if math.IsNaN(u) || math.IsNaN(v) || math.IsInf(u, 0) || math.IsInf(v, 0) ||
		u < 0 || u > 1 || v < 0 || v > 1 {
		return Sample{}, fmt.Errorf("eye %q level %d UV (%v,%v) is outside [0,1]", lv.Eye, level, u, v)
	}
	if filter != FilterNearest && filter != FilterBilinear {
		return Sample{}, fmt.Errorf("unknown filter %d", filter)
	}
	if address != ClampToLevel && address != ClampToAtlas {
		return Sample{}, fmt.Errorf("unknown address mode %d", address)
	}

	plan := makeSamplePlan(lv.Width, lv.Height, u, v, filter)
	sample := Sample{Filter: filter, Address: address, Taps: make([]Tap, 0, len(plan))}
	var r, g, b float64
	for _, point := range plan {
		tap := Tap{Raw: point.raw, Weight: point.weight}
		if address == ClampToLevel {
			tap.Local = clampPoint(point.raw, lv.Width, lv.Height)
			tap.Atlas = image.Pt(lv.Rect.Min.X+tap.Local.X, lv.Rect.Min.Y+tap.Local.Y)
		} else {
			rawAtlas := image.Pt(lv.Rect.Min.X+point.raw.X, lv.Rect.Min.Y+point.raw.Y)
			tap.Atlas = clampPoint(rawAtlas, a.Layout.Width, a.Layout.Height)
			tap.Local = image.Pt(tap.Atlas.X-lv.Rect.Min.X, tap.Atlas.Y-lv.Rect.Min.Y)
		}
		c, ok := a.Pixel(tap.Atlas.X, tap.Atlas.Y)
		if !ok {
			return Sample{}, fmt.Errorf("eye %q level %d tap (%d,%d) is outside the atlas",
				lv.Eye, level, tap.Atlas.X, tap.Atlas.Y)
		}
		r += float64(c.R) * tap.Weight
		g += float64(c.G) * tap.Weight
		b += float64(c.B) * tap.Weight
		sample.Taps = append(sample.Taps, tap)
	}
	sample.Color = color.RGBA{R: round8(r), G: round8(g), B: round8(b), A: 255}
	return sample, nil
}

// ExpectedSample computes the colour a correct level-local sample should have.
func ExpectedSample(eyeIndex, level int, width, height int, u, v float64, filter Filter) color.RGBA {
	return expectedSample(TexelColor, eyeIndex, level, width, height, u, v, filter)
}

// TexelFunc supplies expected RGBA values for validator-backed source data.
type TexelFunc func(eyeIndex, level, x, y, width, height int) color.RGBA

func expectedSample(texel TexelFunc, eyeIndex, level, width, height int, u, v float64, filter Filter) color.RGBA {
	plan := makeSamplePlan(width, height, u, v, filter)
	var r, g, b float64
	for _, point := range plan {
		local := clampPoint(point.raw, width, height)
		c := texel(eyeIndex, level, local.X, local.Y, width, height)
		r += float64(c.R) * point.weight
		g += float64(c.G) * point.weight
		b += float64(c.B) * point.weight
	}
	return color.RGBA{R: round8(r), G: round8(g), B: round8(b), A: 255}
}

type plannedPoint struct {
	raw    image.Point
	weight float64
}

func makeSamplePlan(width, height int, u, v float64, filter Filter) []plannedPoint {
	if filter == FilterNearest {
		return []plannedPoint{{
			raw: image.Pt(
				int(math.Floor(u*float64(width))),
				int(math.Floor(v*float64(height))),
			),
			weight: 1,
		}}
	}

	x := u*float64(width) - 0.5
	y := v*float64(height) - 0.5
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	points := make([]plannedPoint, 0, 4)
	for _, dy := range []int{0, 1} {
		for _, dx := range []int{0, 1} {
			wx := 1 - fx
			if dx == 1 {
				wx = fx
			}
			wy := 1 - fy
			if dy == 1 {
				wy = fy
			}
			points = append(points, plannedPoint{
				raw:    image.Pt(x0+dx, y0+dy),
				weight: wx * wy,
			})
		}
	}
	return points
}

func clampPoint(p image.Point, width, height int) image.Point {
	p.X = min(max(p.X, 0), width-1)
	p.Y = min(max(p.Y, 0), height-1)
	return p
}

func round8(value float64) uint8 {
	return clamp8(int(math.Round(value)))
}
