package reprojlab

import (
	"fmt"
	"math"
)

type RasterDiff struct {
	CommonPixels   int
	MissingPixels  int
	ExtraPixels    int
	ColorRMSE      float64
	DepthRMSE      float64
	MaxDepthError  float64
	HoleDifference int
}

func CompareRasters(got, ref *Raster) RasterDiff {
	var d RasterDiff
	if got.W != ref.W || got.H != ref.H {
		panic("raster dimensions differ")
	}
	var colorSq, depthSq float64
	for n := range got.Filled {
		okGot := got.Filled[n]
		okRef := ref.Filled[n]
		switch {
		case okGot && okRef:
			d.CommonPixels++
			dr := float64(int(got.Color[n].R) - int(ref.Color[n].R))
			dg := float64(int(got.Color[n].G) - int(ref.Color[n].G))
			db := float64(int(got.Color[n].B) - int(ref.Color[n].B))
			colorSq += dr*dr + dg*dg + db*db
			dz := DepthDelta(got.Depth[n], ref.Depth[n])
			if !math.IsInf(dz, 1) && !math.IsNaN(dz) {
				depthSq += dz * dz
				if dz > d.MaxDepthError {
					d.MaxDepthError = dz
				}
			}
		case okRef:
			d.MissingPixels++
		case okGot:
			d.ExtraPixels++
		}
	}
	if d.CommonPixels > 0 {
		d.ColorRMSE = math.Sqrt(colorSq / float64(d.CommonPixels*3))
		d.DepthRMSE = math.Sqrt(depthSq / float64(d.CommonPixels))
	}
	d.HoleDifference = got.HoleCount() - ref.HoleCount()
	return d
}

type OrderMetrics struct {
	Result        PipelineResult
	ReferenceDiff RasterDiff
	ProbeFailures int
	HolePercent   float64
}

type OrderDecision struct {
	Reference *Raster
	A, B      OrderMetrics
	Selected  Order
	Reason    string
}

// DecideOrder prefers measurable correctness over visual similarity. A single
// continuous composition of both coordinate maps is the reference. Color and
// depth error against that reference decide first; coverage, probe checks and
// resampling work are only tie-breakers. Fewer holes alone must never win.
func DecideOrder(scene *Scene, cfg Config, maxProbeDepthError float64) OrderDecision {
	ref, _ := scene.DirectReference(cfg)
	a := RunOrder(scene, cfg, ReprojectThenDistort)
	b := RunOrder(scene, cfg, DistortThenReproject)
	makeMetrics := func(r PipelineResult) OrderMetrics {
		return OrderMetrics{
			Result:        r,
			ReferenceDiff: CompareRasters(r.Final, ref),
			ProbeFailures: ProbeFailures(r.Probes, maxProbeDepthError),
			HolePercent:   100 * float64(r.Final.HoleCount()) / float64(r.Final.W*r.Final.H),
		}
	}
	decision := OrderDecision{Reference: ref, A: makeMetrics(a), B: makeMetrics(b)}
	winner, reason := chooseOrder(decision.A, decision.B)
	decision.Selected = winner
	decision.Reason = reason
	return decision
}

func chooseOrder(a, b OrderMetrics) (Order, string) {
	type criterion struct {
		name string
		a, b float64
	}
	criteria := []criterion{
		{"reference color RMSE", a.ReferenceDiff.ColorRMSE, b.ReferenceDiff.ColorRMSE},
		{"reference depth RMSE", a.ReferenceDiff.DepthRMSE, b.ReferenceDiff.DepthRMSE},
		{"reference max depth error", a.ReferenceDiff.MaxDepthError, b.ReferenceDiff.MaxDepthError},
		{"probe depth failures", float64(a.ProbeFailures), float64(b.ProbeFailures)},
		{"final holes", float64(a.Result.Final.HoleCount()), float64(b.Result.Final.HoleCount())},
		{"source resampling reads", float64(a.Result.Stats.SourceSamples), float64(b.Result.Stats.SourceSamples)},
	}
	for _, c := range criteria {
		if math.Abs(c.a-c.b) <= 1e-12 {
			continue
		}
		if c.a < c.b {
			return ReprojectThenDistort, fmt.Sprintf("%s: %.6g < %.6g", c.name, c.a, c.b)
		}
		return DistortThenReproject, fmt.Sprintf("%s: %.6g < %.6g", c.name, c.b, c.a)
	}
	return ReprojectThenDistort, "all measured criteria tied; deterministic tie-break"
}

func MeanProbeDepthError(probes []ProbeResult) float64 {
	var sum float64
	var count int
	for _, p := range probes {
		if !p.Filled || math.IsInf(p.DepthError, 1) || math.IsNaN(p.DepthError) {
			continue
		}
		sum += p.DepthError
		count++
	}
	if count == 0 {
		return math.Inf(1)
	}
	return sum / float64(count)
}
