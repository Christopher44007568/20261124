package harness

import (
	"math"

	"stereoval/internal/metrics"
)

// Thresholds are the acceptance limits. They apply per eye and to the
// cross-eye disparity channel independently.
type Thresholds struct {
	PerEyeReprojRMS float64
	PooledReprojRMS float64
	PerEyeReprojP95 float64
	MarkerFoundRate float64
	DispErrRMS      float64
	DispErrP95      float64
	DispChangeRMS   float64
	DispChangeP95   float64
	EdgeSymmetric   float64
	GhostRatio      float64
	FillGhostRatio  float64
}

// DefaultThresholds returns the production limits.
func DefaultThresholds() Thresholds {
	return Thresholds{
		PerEyeReprojRMS: 1.0,
		PooledReprojRMS: 0.9,
		PerEyeReprojP95: 2.0,
		MarkerFoundRate: 0.85,
		DispErrRMS:      0.8,
		DispErrP95:      3.0,
		DispChangeRMS:   0.8,
		DispChangeP95:   2.5,
		EdgeSymmetric:   1.6,
		GhostRatio:      0.06,
		FillGhostRatio:  0.12,
	}
}

// Failure records one violated rule.
type Failure struct {
	Channel string
	Metric  string
	Got     float64
	Limit   float64
}

// Verdict is the final run result.
type Verdict struct {
	Mode     Mode
	Pass     bool
	Failures []Failure
	Agg      metrics.Aggregate
}

// Aggregate reduces frame reports to run-level statistics.
func AggregateReports(rs []metrics.FrameReport) metrics.Aggregate {
	a := metrics.Aggregate{Frames: len(rs)}
	var lMean, lRMS, lMax, lHole, lFill, lEdge, lGhost, lFG, lFound []float64
	var rMean, rRMS, rMax, rHole, rFill, rEdge, rGhost, rFG, rFound []float64
	var pooled, dErr, dErrMax, dChg, dChgMax []float64

	for _, r := range rs {
		lMean = append(lMean, r.Left.ReprojMean)
		lRMS = append(lRMS, r.Left.ReprojRMS)
		lMax = append(lMax, r.Left.ReprojMax)
		lHole = append(lHole, r.Left.HoleRatio)
		lFill = append(lFill, r.Left.FillRatio)
		lEdge = append(lEdge, r.Left.EdgeSymmetric)
		lGhost = append(lGhost, r.Left.GhostRatio)
		if r.Left.FillGhostRatio > 0 {
			lFG = append(lFG, r.Left.FillGhostRatio)
		}
		lFound = append(lFound, foundRate(r.Left))

		rMean = append(rMean, r.Right.ReprojMean)
		rRMS = append(rRMS, r.Right.ReprojRMS)
		rMax = append(rMax, r.Right.ReprojMax)
		rHole = append(rHole, r.Right.HoleRatio)
		rFill = append(rFill, r.Right.FillRatio)
		rEdge = append(rEdge, r.Right.EdgeSymmetric)
		rGhost = append(rGhost, r.Right.GhostRatio)
		if r.Right.FillGhostRatio > 0 {
			rFG = append(rFG, r.Right.FillGhostRatio)
		}
		rFound = append(rFound, foundRate(r.Right))

		if !math.IsNaN(r.Left.ReprojRMS) {
			pooled = append(pooled, r.Left.ReprojRMS)
		}
		if !math.IsNaN(r.Right.ReprojRMS) {
			pooled = append(pooled, r.Right.ReprojRMS)
		}
		dErr = append(dErr, r.Stereo.DispErrRMS)
		dErrMax = append(dErrMax, r.Stereo.DispErrMax)
		dChg = append(dChg, r.Stereo.DispChangeRMS)
		dChgMax = append(dChgMax, r.Stereo.DispChangeMax)
	}

	a.Left = metrics.EyeAgg{
		ReprojMean: metrics.Mean(lMean), ReprojRMS: metrics.RMS(lRMS),
		ReprojMax: metrics.MaxAbs(lMax), ReprojP95: metrics.Percentile(lMax, 0.95),
		HoleMean: metrics.Mean(lHole),
		FillMean: metrics.Mean(lFill), EdgeMean: metrics.Mean(lEdge),
		GhostMean: metrics.Mean(lGhost), FillGhostMean: metrics.Mean(lFG),
		FoundRate: metrics.Mean(lFound),
	}
	a.Right = metrics.EyeAgg{
		ReprojMean: metrics.Mean(rMean), ReprojRMS: metrics.RMS(rRMS),
		ReprojMax: metrics.MaxAbs(rMax), ReprojP95: metrics.Percentile(rMax, 0.95),
		HoleMean: metrics.Mean(rHole),
		FillMean: metrics.Mean(rFill), EdgeMean: metrics.Mean(rEdge),
		GhostMean: metrics.Mean(rGhost), FillGhostMean: metrics.Mean(rFG),
		FoundRate: metrics.Mean(rFound),
	}
	a.PooledReprojRMS = metrics.RMS(pooled)
	a.PooledReprojMean = metrics.Mean(append(append([]float64{}, lMean...), rMean...))
	a.DispErrRMS = metrics.RMS(dErr)
	a.DispErrMax = metrics.MaxAbs(dErrMax)
	a.DispErrP95 = metrics.Percentile(dErrMax, 0.95)
	a.DispChangeRMS = metrics.RMS(dChg)
	a.DispChangeMax = metrics.MaxAbs(dChgMax)
	a.DispChangeP95 = metrics.Percentile(dChgMax, 0.95)
	return a
}

func foundRate(e metrics.EyeReport) float64 {
	if e.MarkersTotal == 0 {
		return math.NaN()
	}
	return float64(e.MarkersFound) / float64(e.MarkersTotal)
}

// Judge applies the acceptance rules. The disparity rules are independent of
// the average-error rule: a run whose pooled error is within limit but whose
// disparity is distorted must still fail.
func Judge(mode Mode, agg metrics.Aggregate, th Thresholds) Verdict {
	v := Verdict{Mode: mode, Pass: true, Agg: agg}
	add := func(ch, metric string, got, limit float64) {
		if math.IsNaN(got) || math.IsInf(got, 0) {
			return // metric not applicable for this policy
		}
		if got > limit {
			v.Pass = false
			v.Failures = append(v.Failures, Failure{Channel: ch, Metric: metric, Got: got, Limit: limit})
		}
	}

	add("left", "reproj_rms", agg.Left.ReprojRMS, th.PerEyeReprojRMS)
	add("right", "reproj_rms", agg.Right.ReprojRMS, th.PerEyeReprojRMS)
	add("pooled", "reproj_rms", agg.PooledReprojRMS, th.PooledReprojRMS)
	add("left", "reproj_p95", agg.Left.ReprojP95, th.PerEyeReprojP95)
	add("right", "reproj_p95", agg.Right.ReprojP95, th.PerEyeReprojP95)
	add("left", "marker_found_rate", 1-agg.Left.FoundRate, 1-th.MarkerFoundRate)
	add("right", "marker_found_rate", 1-agg.Right.FoundRate, 1-th.MarkerFoundRate)

	add("left", "edge_misalign", agg.Left.EdgeMean, th.EdgeSymmetric)
	add("right", "edge_misalign", agg.Right.EdgeMean, th.EdgeSymmetric)
	add("left", "ghost_ratio", agg.Left.GhostMean, th.GhostRatio)
	add("right", "ghost_ratio", agg.Right.GhostMean, th.GhostRatio)
	// Fill ghosting only applies when cross-eye fill actually ran.
	if !math.IsNaN(agg.Left.FillGhostMean) {
		add("left", "fill_ghost_ratio", agg.Left.FillGhostMean, th.FillGhostRatio)
	}
	if !math.IsNaN(agg.Right.FillGhostMean) {
		add("right", "fill_ghost_ratio", agg.Right.FillGhostMean, th.FillGhostRatio)
	}

	// Independent stereo gate: cannot be rescued by a good monocular average.
	add("stereo", "disparity_rms", agg.DispErrRMS, th.DispErrRMS)
	add("stereo", "disparity_p95", agg.DispErrP95, th.DispErrP95)
	add("stereo", "disparity_change_rms", agg.DispChangeRMS, th.DispChangeRMS)
	add("stereo", "disparity_change_p95", agg.DispChangeP95, th.DispChangeP95)

	return v
}
