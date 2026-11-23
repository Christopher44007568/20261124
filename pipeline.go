package reprojlab

import (
	"fmt"
	"math"
)

type Order string

const (
	ReprojectThenDistort Order = "reproject-then-distort"
	DistortThenReproject Order = "distort-then-reproject"
)

type StageResult struct {
	Name  string
	Stats WarpStats
}

type ProbeResult struct {
	Probe         string
	Note          string
	SourceDepth   float64
	ExpectedPoint Point
	ExpectedDepth float64
	ActualDepth   float64
	Filled        bool
	DepthError    float64
}

type PipelineResult struct {
	Name    string
	Order   Order
	Final   *Raster
	Stages  []StageResult
	Stats   WarpStats
	Probes  []ProbeResult
	Variant string
}

func RunOrder(scene *Scene, cfg Config, order Order) PipelineResult {
	return RunOrderNamed(scene, cfg, order, string(order))
}

func RunOrderNamed(scene *Scene, cfg Config, order Order, name string) PipelineResult {
	var first, second *Raster
	var s1, s2 WarpStats
	switch order {
	case ReprojectThenDistort:
		first, s1 = WarpNearest(scene.Raster, cfg.Width, cfg.Height, 1, 1, reprojectMap(cfg))
		second, s2 = WarpNearest(first, cfg.Width, cfg.Height, 1, 1, distortMap(cfg))
	case DistortThenReproject:
		first, s1 = WarpNearest(scene.Raster, cfg.Width, cfg.Height, 1, 1, distortMap(cfg))
		second, s2 = WarpNearest(first, cfg.Width, cfg.Height, 1, 1, reprojectMap(cfg))
	default:
		panic(fmt.Sprintf("unknown order %q", order))
	}
	res := PipelineResult{
		Name:   name,
		Order:  order,
		Final:  second,
		Stages: []StageResult{{Name: stageName(order, 1), Stats: s1}, {Name: stageName(order, 2), Stats: s2}},
		Stats:  MergeStats(s1, s2),
	}
	res.Probes = traceProbes(scene, cfg, order, first, second)
	return res
}

func stageName(order Order, stage int) string {
	if order == ReprojectThenDistort {
		if stage == 1 {
			return "reproject"
		}
		return "distort"
	}
	if stage == 1 {
		return "distort"
	}
	return "reproject"
}

func reprojectMap(cfg Config) PixelMap {
	return func(in Sample) (Point, float64, bool) {
		return Reproject(Point{X: in.X, Y: in.Y}, in.Depth, cfg.Intrinsics, cfg.Translation)
	}
}

func distortMap(cfg Config) PixelMap {
	return func(in Sample) (Point, float64, bool) {
		p := DistortPixel(Point{X: in.X, Y: in.Y}, cfg.Intrinsics, cfg.Distortion)
		return p, in.Depth, true
	}
}

// RunLowResDistortThenReproject deliberately distorts a coarse intermediate
// grid. Its samples still carry full-resolution coordinates, so the following
// reprojection targets the full output grid.
func RunLowResDistortThenReproject(scene *Scene, cfg Config) PipelineResult {
	scale := cfg.LowResScale
	if scale < 2 {
		scale = 2
	}
	lw := (cfg.Width + scale - 1) / scale
	lh := (cfg.Height + scale - 1) / scale
	first, s1 := WarpNearest(scene.Raster, lw, lh, float64(scale), float64(scale), distortMap(cfg))
	second, s2 := WarpNearest(first, cfg.Width, cfg.Height, 1, 1, reprojectMap(cfg))
	res := PipelineResult{
		Name:    "distort-lowres-then-reproject",
		Order:   DistortThenReproject,
		Final:   second,
		Stages:  []StageResult{{Name: "distort-lowres", Stats: s1}, {Name: "reproject", Stats: s2}},
		Stats:   MergeStats(s1, s2),
		Variant: fmt.Sprintf("distort at 1/%d resolution", scale),
	}
	res.Probes = traceProbes(scene, cfg, DistortThenReproject, first, second)
	return res
}

// RunWrongDepthReuse distorts color but leaves the old depth image in its
// original lattice locations. Reprojection then pairs distorted color with
// stale depth, which is especially visible at depth discontinuities.
func RunWrongDepthReuse(scene *Scene, cfg Config) PipelineResult {
	color, s1 := WarpNearest(scene.Raster, cfg.Width, cfg.Height, 1, 1, distortMap(cfg))
	stale := NewRaster(cfg.Width, cfg.Height, 1, 1)
	for n := range color.Filled {
		if !color.Filled[n] {
			continue
		}
		stale.Color[n] = color.Color[n]
		stale.Depth[n] = scene.Raster.Depth[n]
		stale.Filled[n] = true
	}
	final, s2 := WarpNearest(stale, cfg.Width, cfg.Height, 1, 1, reprojectMap(cfg))
	res := PipelineResult{
		Name:    "distort-then-reproject-stale-depth",
		Order:   DistortThenReproject,
		Final:   final,
		Stages:  []StageResult{{Name: "distort-color-stale-depth", Stats: s1}, {Name: "reproject", Stats: s2}},
		Stats:   MergeStats(s1, s2),
		Variant: "distorted color paired with undistorted source depth",
	}
	correct := RunOrder(scene, cfg, DistortThenReproject)
	res.Probes = ProbeErrorsAgainst(final, correct.Probes)
	return res
}

// ProbeErrorsAgainst evaluates the correct pipeline's probe destinations
// against another final raster. It is used to expose stale-depth reuse without
// letting the faulty depth drive its own expected value.
func ProbeErrorsAgainst(final *Raster, expected []ProbeResult) []ProbeResult {
	out := make([]ProbeResult, 0, len(expected))
	for _, e := range expected {
		actual, filled := math.NaN(), false
		_, actual, filled = final.PixelAt(e.ExpectedPoint)
		err := math.Inf(1)
		if filled {
			err = DepthDelta(actual, e.ExpectedDepth)
		}
		e.ActualDepth = actual
		e.Filled = filled
		e.DepthError = err
		out = append(out, e)
	}
	return out
}

func traceProbes(scene *Scene, cfg Config, order Order, first, final *Raster) []ProbeResult {
	out := make([]ProbeResult, 0, len(scene.Probes))
	for _, probe := range scene.Probes {
		i, j := scene.Raster.Grid(probe.Point)
		_, z, ok := scene.Raster.At(i, j)
		if !ok {
			continue
		}
		var firstMap, secondMap PixelMap
		switch order {
		case ReprojectThenDistort:
			firstMap, secondMap = reprojectMap(cfg), distortMap(cfg)
		case DistortThenReproject:
			firstMap, secondMap = distortMap(cfg), reprojectMap(cfg)
		default:
			panic(fmt.Sprintf("unknown order %q", order))
		}
		p1, _, valid := firstMap(Sample{X: probe.Point.X, Y: probe.Point.Y, Depth: z})
		if !valid {
			continue
		}
		i1, j1 := first.Grid(p1)
		if !first.Inside(i1, j1) {
			out = append(out, ProbeResult{
				Probe: probe.Name, Note: probe.Note, SourceDepth: z,
				ExpectedPoint: p1, Filled: false, DepthError: math.Inf(1),
			})
			continue
		}
		_, z1, filled1 := first.At(i1, j1)
		if !filled1 {
			out = append(out, ProbeResult{
				Probe: probe.Name, Note: probe.Note, SourceDepth: z,
				ExpectedPoint: p1, Filled: false, DepthError: math.Inf(1),
			})
			continue
		}
		p1 = first.Pos(i1, j1)
		p2, expected, valid := secondMap(Sample{X: p1.X, Y: p1.Y, Depth: z1})
		if !valid {
			continue
		}
		i2, j2 := final.Grid(p2)
		if !final.Inside(i2, j2) {
			out = append(out, ProbeResult{
				Probe: probe.Name, Note: probe.Note, SourceDepth: z,
				ExpectedPoint: p2, ExpectedDepth: expected,
				ActualDepth: math.NaN(), Filled: false, DepthError: math.Inf(1),
			})
			continue
		}
		p2 = final.Pos(i2, j2)
		_, actual, filled := final.PixelAt(p2)
		err := math.Inf(1)
		if filled {
			err = DepthDelta(actual, expected)
		}
		out = append(out, ProbeResult{
			Probe: probe.Name, Note: probe.Note, SourceDepth: z,
			ExpectedPoint: p2, ExpectedDepth: expected,
			ActualDepth: actual, Filled: filled, DepthError: err,
		})
	}
	return out
}

func ProbeFailures(probes []ProbeResult, maxDepthError float64) int {
	failures := 0
	for _, p := range probes {
		if !p.Filled || !(p.DepthError <= maxDepthError) {
			failures++
		}
	}
	return failures
}
