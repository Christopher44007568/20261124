package gazedenoise

import (
	"fmt"
	"math"
	"sort"
)

// Mode selects the denoising policy under test.
type Mode int

const (
	// ModeRaw copies the input and leaves missing samples missing.
	ModeRaw Mode = iota
	// ModePartitioned filters only valid periphery pixels. Every source tap
	// and hole donor must have the same gaze membership as its target.
	ModePartitioned
	// ModeFull filters all pixels without respecting the gaze partition. It
	// exists only as a leakage/halo baseline.
	ModeFull
)

// String returns the stable report name of m.
func (m Mode) String() string {
	switch m {
	case ModeRaw:
		return "raw"
	case ModePartitioned:
		return "partitioned"
	case ModeFull:
		return "full"
	default:
		return fmt.Sprintf("mode(%d)", int(m))
	}
}

// ParseMode validates a command-line mode name.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "raw":
		return ModeRaw, nil
	case "partitioned":
		return ModePartitioned, nil
	case "full":
		return ModeFull, nil
	default:
		return ModeRaw, fmt.Errorf("unknown mode %q", s)
	}
}

// Params controls the edge-aware filter and hole filler.
type Params struct {
	Radius       int     `json:"radius"`
	HoleRadius   int     `json:"hole_radius"`
	SigmaSpatial float64 `json:"sigma_spatial"`
	SigmaNormal  float64 `json:"sigma_normal"`
	SigmaDepth   float64 `json:"sigma_depth"`
	SigmaLuma    float64 `json:"sigma_luma"`
	MinWeight    float64 `json:"min_weight"`
}

// DefaultParams is a compact bilateral support suitable for a 192x144
// preview while remaining wide enough to expose boundary leakage.
func DefaultParams() Params {
	return Params{
		Radius:       3,
		HoleRadius:   5,
		SigmaSpatial: 1.55,
		SigmaNormal:  0.20,
		SigmaDepth:   0.22,
		SigmaLuma:    0.045,
		MinWeight:    1e-4,
	}
}

// Validate checks filter parameters.
func (p Params) Validate() error {
	if p.Radius < 1 || p.Radius > 12 {
		return fmt.Errorf("radius %d outside [1,12]", p.Radius)
	}
	if p.HoleRadius < 1 || p.HoleRadius > 32 {
		return fmt.Errorf("hole radius %d outside [1,32]", p.HoleRadius)
	}
	for name, value := range map[string]float64{
		"spatial sigma": p.SigmaSpatial,
		"normal sigma":  p.SigmaNormal,
		"depth sigma":   p.SigmaDepth,
		"luma sigma":    p.SigmaLuma,
	} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be positive, got %v", name, value)
		}
	}
	if p.MinWeight < 0 || p.MinWeight >= 1 || math.IsNaN(p.MinWeight) {
		return fmt.Errorf("minimum weight %v outside [0,1)", p.MinWeight)
	}
	return nil
}

// SourceWeight records one donor used to fill a hole.
type SourceWeight struct {
	Pixel  int     `json:"pixel"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	Weight float64 `json:"weight"`
}

// HoleProvenance records every input pixel that contributed to a filled hole.
type HoleProvenance struct {
	Pixel          int            `json:"pixel"`
	X              int            `json:"x"`
	Y              int            `json:"y"`
	Sources        []SourceWeight `json:"sources"`
	CrossPartition bool           `json:"cross_partition"`
}

// Result is a filtered image plus its audit trail.
type Result struct {
	Mode   Mode
	Output *Frame

	Filled        []bool
	Provenance    []HoleProvenance
	UnfilledHoles int

	// CrossPartitionTaps counts edge-aware taps whose gaze membership differs
	// from the target. CrossPartitionWeight sums their normalized weights.
	CrossPartitionTaps   int
	CrossPartitionWeight float64
}

// Denoise filters src according to mode and params.
func Denoise(src *Frame, mode Mode, params Params) (*Result, error) {
	if err := src.Validate(); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	if mode < ModeRaw || mode > ModeFull {
		return nil, fmt.Errorf("invalid mode %d", mode)
	}

	out := src.Clone()
	result := &Result{
		Mode:   mode,
		Output: out,
		Filled: make([]bool, len(src.Color)),
	}
	if mode == ModeRaw {
		return result, nil
	}

	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			i := src.Index(x, y)
			if !src.Valid[i] {
				continue
			}
			if mode == ModePartitioned && src.Mask[i] {
				continue
			}
			color, crossTaps, crossWeight := filterValidPixel(src, i, params, mode)
			out.Color[i] = color
			result.CrossPartitionTaps += crossTaps
			result.CrossPartitionWeight += crossWeight
		}
	}

	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			i := src.Index(x, y)
			if src.Valid[i] {
				continue
			}
			if color, filled, prov, cross := fillHole(src, i, params, mode); filled {
				out.Color[i] = color
				out.Valid[i] = true
				result.Filled[i] = true
				result.Provenance = append(result.Provenance, prov)
				if cross {
					result.CrossPartitionTaps += len(prov.Sources)
					for _, source := range prov.Sources {
						result.CrossPartitionWeight += source.Weight
					}
				}
			} else {
				result.UnfilledHoles++
			}
		}
	}
	out.RecomputeLuma()
	return result, nil
}

func filterValidPixel(src *Frame, target int, p Params, mode Mode) (Vec3, int, float64) {
	tx := target % src.Width
	ty := target / src.Width
	targetMask := src.Mask[target]
	normalT := src.Normal[target]
	depthT := src.Depth[target]
	lumaT := src.Luma[target]

	var sumWeight, sumRed, sumGreen, sumBlue, crossWeight float64
	crossTaps := 0
	for dy := -p.Radius; dy <= p.Radius; dy++ {
		y := ty + dy
		if y < 0 || y >= src.Height {
			continue
		}
		for dx := -p.Radius; dx <= p.Radius; dx++ {
			x := tx + dx
			if x < 0 || x >= src.Width {
				continue
			}
			d2 := float64(dx*dx + dy*dy)
			if d2 > float64(p.Radius*p.Radius) {
				continue
			}
			q := src.Index(x, y)
			if !src.Valid[q] {
				continue
			}
			if mode == ModePartitioned && src.Mask[q] != targetMask {
				continue
			}

			w := spatialWeight(d2, p.SigmaSpatial)
			w *= normalWeight(normalT, src.Normal[q], p.SigmaNormal)
			w *= scalarWeight(depthT-src.Depth[q], p.SigmaDepth)
			w *= scalarWeight(lumaT-src.Luma[q], p.SigmaLuma)
			if w < p.MinWeight {
				continue
			}
			c := src.Color[q]
			sumWeight += w
			sumRed += w * c.X
			sumGreen += w * c.Y
			sumBlue += w * c.Z
			if src.Mask[q] != targetMask {
				crossTaps++
				crossWeight += w
			}
		}
	}
	if sumWeight == 0 {
		return src.Color[target], crossTaps, 0
	}
	return Vec3{
		X: sumRed / sumWeight,
		Y: sumGreen / sumWeight,
		Z: sumBlue / sumWeight,
	}, crossTaps, crossWeight / sumWeight
}

func spatialWeight(distanceSquared, sigma float64) float64 {
	if sigma <= 0 {
		if distanceSquared == 0 {
			return 1
		}
		return 0
	}
	return math.Exp(-distanceSquared / (2 * sigma * sigma))
}

func scalarWeight(delta, sigma float64) float64 {
	if sigma <= 0 {
		if delta == 0 {
			return 1
		}
		return 0
	}
	return math.Exp(-0.5 * delta * delta / (sigma * sigma))
}

func normalWeight(a, b Vec3, sigma float64) float64 {
	na, nb := a.Normalized(), b.Normalized()
	if na.Length() == 0 || nb.Length() == 0 {
		return 1
	}
	dot := Clamp(na.Dot(nb), -1, 1)
	if sigma <= 0 {
		if dot == 1 {
			return 1
		}
		return 0
	}
	return math.Exp(-(1 - dot) / (2 * sigma * sigma))
}

func fillHole(src *Frame, target int, p Params, mode Mode) (Vec3, bool, HoleProvenance, bool) {
	tx := target % src.Width
	ty := target / src.Width
	targetMask := src.Mask[target]
	holeSigma := math.Max(1, float64(p.HoleRadius)*0.5)
	prov := HoleProvenance{
		Pixel: target,
		X:     tx,
		Y:     ty,
	}

	for radius := 1; radius <= p.HoleRadius; radius++ {
		candidates := make([]SourceWeight, 0, radius*8)
		for dy := -radius; dy <= radius; dy++ {
			y := ty + dy
			if y < 0 || y >= src.Height {
				continue
			}
			for dx := -radius; dx <= radius; dx++ {
				x := tx + dx
				if x < 0 || x >= src.Width {
					continue
				}
				if maxAbs(dx, dy) != radius {
					continue
				}
				q := src.Index(x, y)
				if !src.Valid[q] {
					continue
				}
				if mode == ModePartitioned && src.Mask[q] != targetMask {
					continue
				}
				w := spatialWeight(float64(dx*dx+dy*dy), holeSigma)
				w *= normalWeight(src.Normal[target], src.Normal[q], p.SigmaNormal)
				w *= scalarWeight(src.Depth[target]-src.Depth[q], p.SigmaDepth)
				if w < p.MinWeight {
					continue
				}
				candidates = append(candidates, SourceWeight{
					Pixel:  q,
					X:      x,
					Y:      y,
					Weight: w,
				})
				if src.Mask[q] != targetMask {
					prov.CrossPartition = true
				}
			}
		}
		if len(candidates) == 0 {
			continue
		}

		var total float64
		for _, source := range candidates {
			total += source.Weight
		}
		if total <= 0 {
			total = float64(len(candidates))
			for i := range candidates {
				candidates[i].Weight = 1
			}
		}
		var color Vec3
		for i := range candidates {
			candidates[i].Weight /= total
			color = color.Add(src.Color[candidates[i].Pixel].Mul(candidates[i].Weight))
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Pixel < candidates[j].Pixel
		})
		prov.Sources = candidates
		return color, true, prov, prov.CrossPartition
	}
	return Vec3{}, false, prov, false
}

func maxAbs(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a > b {
		return a
	}
	return b
}
