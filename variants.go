package reprojlab

import (
	"fmt"
	"math"
)

type ChannelResult struct {
	Name        string
	Channels    [3]*Raster
	Stats       [3]WarpStats
	Holes       [3]int
	Reproject   WarpStats
	Reprojected *Raster
}

// RunPerChannelDistortion applies reprojection once, then distorts red, green
// and blue independently. The green channel retains the nominal lens model;
// red and blue use slightly different coefficients to model channel-dependent
// lateral color.
func RunPerChannelDistortion(scene *Scene, cfg Config) ChannelResult {
	projected, reprojectStats := WarpNearest(scene.Raster, cfg.Width, cfg.Height, 1, 1, reprojectMap(cfg))
	out := ChannelResult{
		Name:        "per-channel-distortion",
		Reproject:   reprojectStats,
		Reprojected: projected,
	}
	factors := [3]float64{0.96, 1.00, 1.04}
	for c := 0; c < 3; c++ {
		gray := NewRaster(projected.W, projected.H, projected.ScaleX, projected.ScaleY)
		for n := range projected.Filled {
			if !projected.Filled[n] {
				continue
			}
			v := projected.Color[n].R
			if c == 1 {
				v = projected.Color[n].G
			} else if c == 2 {
				v = projected.Color[n].B
			}
			gray.Color[n] = RGBA{R: v, G: v, B: v, A: 255}
			gray.Depth[n] = projected.Depth[n]
			gray.Filled[n] = true
		}
		d := cfg.Distortion.Scale(factors[c])
		warped, stats := WarpNearest(gray, cfg.Width, cfg.Height, 1, 1, func(in Sample) (Point, float64, bool) {
			return DistortPixel(Point{X: in.X, Y: in.Y}, cfg.Intrinsics, d), in.Depth, true
		})
		out.Channels[c] = warped
		out.Stats[c] = stats
		out.Holes[c] = warped.HoleCount()
	}
	return out
}

func (r ChannelResult) UnionHoles() int {
	holes := 0
	for n := range r.Channels[0].Filled {
		if !r.Channels[0].Filled[n] || !r.Channels[1].Filled[n] || !r.Channels[2].Filled[n] {
			holes++
		}
	}
	return holes
}

// ChannelDisplacement reports how far a channel-specific distortion moves a
// source pixel relative to the nominal model.
func ChannelDisplacement(p Point, cfg Config, factor float64) float64 {
	q := DistortPixel(p, cfg.Intrinsics, cfg.Distortion.Scale(factor))
	base := DistortPixel(p, cfg.Intrinsics, cfg.Distortion)
	return math.Hypot(q.X-base.X, q.Y-base.Y)
}

func (r ChannelResult) Summary() string {
	return fmt.Sprintf("%s holes r/g/b=%d/%d/%d union=%d", r.Name, r.Holes[0], r.Holes[1], r.Holes[2], r.UnionHoles())
}
