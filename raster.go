package reprojlab

import "math"

type RGBA struct {
	R, G, B, A uint8
}

// Raster stores samples on an evenly spaced grid. ScaleX and ScaleY express
// one output sample's pitch in full-resolution pixel coordinates.
type Raster struct {
	W, H           int
	ScaleX, ScaleY float64
	Color          []RGBA
	Depth          []float64
	Filled         []bool
}

func NewRaster(w, h int, scaleX, scaleY float64) *Raster {
	n := w * h
	depth := make([]float64, n)
	for i := range depth {
		depth[i] = math.NaN()
	}
	return &Raster{
		W:      w,
		H:      h,
		ScaleX: scaleX,
		ScaleY: scaleY,
		Color:  make([]RGBA, n),
		Depth:  depth,
		Filled: make([]bool, n),
	}
}

func (r *Raster) Pos(i, j int) Point {
	return Point{
		X: (float64(i)+0.5)*r.ScaleX - 0.5,
		Y: (float64(j)+0.5)*r.ScaleY - 0.5,
	}
}

func (r *Raster) Grid(p Point) (int, int) {
	i := int(math.Round((p.X+0.5)/r.ScaleX - 0.5))
	j := int(math.Round((p.Y+0.5)/r.ScaleY - 0.5))
	return i, j
}

func (r *Raster) Inside(i, j int) bool {
	return i >= 0 && j >= 0 && i < r.W && j < r.H
}

func (r *Raster) At(i, j int) (RGBA, float64, bool) {
	if !r.Inside(i, j) {
		return RGBA{}, 0, false
	}
	n := j*r.W + i
	if !r.Filled[n] {
		return RGBA{}, 0, false
	}
	return r.Color[n], r.Depth[n], true
}

func (r *Raster) PixelAt(p Point) (RGBA, float64, bool) {
	i, j := r.Grid(p)
	return r.At(i, j)
}

func (r *Raster) HoleCount() int {
	holes := 0
	for _, ok := range r.Filled {
		if !ok {
			holes++
		}
	}
	return holes
}

func (r *Raster) Clone() *Raster {
	out := NewRaster(r.W, r.H, r.ScaleX, r.ScaleY)
	copy(out.Color, r.Color)
	copy(out.Depth, r.Depth)
	copy(out.Filled, r.Filled)
	return out
}

type Sample struct {
	X, Y  float64
	Color RGBA
	Depth float64
}

type PixelMap func(Sample) (Point, float64, bool)

type WarpStats struct {
	Passes        int
	SourceSamples int64
	Writes        int64
	Offscreen     int64
	Occluded      int64
	Invalid       int64
	InputHoles    int64
	OutputHoles   int
	OutputPixels  int
}

// WarpNearest forward-scatters source samples using depth testing. It is
// deliberately simple: every source sample contributes at most one destination
// sample, which makes resampling and hole counts deterministic.
func WarpNearest(in *Raster, outW, outH int, scaleX, scaleY float64, fn PixelMap) (*Raster, WarpStats) {
	out := NewRaster(outW, outH, scaleX, scaleY)
	stats := WarpStats{Passes: 1, OutputPixels: outW * outH}
	for j := 0; j < in.H; j++ {
		for i := 0; i < in.W; i++ {
			n := j*in.W + i
			if !in.Filled[n] {
				stats.InputHoles++
				continue
			}
			stats.SourceSamples++
			s := Sample{X: in.Pos(i, j).X, Y: in.Pos(i, j).Y, Color: in.Color[n], Depth: in.Depth[n]}
			p, z, ok := fn(s)
			if !ok {
				stats.Invalid++
				continue
			}
			oi, oj := out.Grid(p)
			if !out.Inside(oi, oj) {
				stats.Offscreen++
				continue
			}
			on := oj*out.W + oi
			if !out.Filled[on] {
				stats.Writes++
			} else {
				if z > out.Depth[on]+1e-9 {
					stats.Occluded++
					continue
				}
				stats.Occluded++
			}
			out.Color[on] = s.Color
			out.Depth[on] = z
			out.Filled[on] = true
		}
	}
	stats.OutputHoles = out.HoleCount()
	return out, stats
}

func MergeStats(stages ...WarpStats) WarpStats {
	var total WarpStats
	for _, s := range stages {
		total.Passes += s.Passes
		total.SourceSamples += s.SourceSamples
		total.Writes += s.Writes
		total.Offscreen += s.Offscreen
		total.Occluded += s.Occluded
		total.Invalid += s.Invalid
		total.InputHoles += s.InputHoles
	}
	if len(stages) > 0 {
		last := stages[len(stages)-1]
		total.OutputHoles = last.OutputHoles
		total.OutputPixels = last.OutputPixels
	}
	return total
}
