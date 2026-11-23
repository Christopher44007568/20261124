package main

import (
	"fmt"
	"math"

	"stereoval/internal/harness"
	"stereoval/internal/metrics"
)

// n renders NaN/Inf as JSON null instead of aborting the whole report.
func n(v float64) interface{} {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return v
}

func encodeAgg(a metrics.Aggregate) map[string]interface{} {
	return map[string]interface{}{
		"frames":               a.Frames,
		"left":                 encodeEye(a.Left),
		"right":                encodeEye(a.Right),
		"pooled_reproj_rms":    n(a.PooledReprojRMS),
		"pooled_reproj_mean":   n(a.PooledReprojMean),
		"disparity_err_rms":    n(a.DispErrRMS),
		"disparity_err_max":    n(a.DispErrMax),
		"disparity_change_rms": n(a.DispChangeRMS),
		"disparity_change_max": n(a.DispChangeMax),
	}
}

func encodeEye(e metrics.EyeAgg) map[string]interface{} {
	return map[string]interface{}{
		"reproj_mean":       n(e.ReprojMean),
		"reproj_rms":        n(e.ReprojRMS),
		"reproj_max":        n(e.ReprojMax),
		"hole_ratio":        n(e.HoleMean),
		"fill_ratio":        n(e.FillMean),
		"edge_misalign":     n(e.EdgeMean),
		"ghost_ratio":       n(e.GhostMean),
		"fill_ghost_ratio":  n(e.FillGhostMean),
		"marker_found_rate": n(e.FoundRate),
	}
}

func encodeFrames(rs []metrics.FrameReport) []map[string]interface{} {
	out := make([]map[string]interface{}, len(rs))
	for i, r := range rs {
		out[i] = map[string]interface{}{
			"frame":  r.Frame,
			"left":   encodeEyeReport(r.Left),
			"right":  encodeEyeReport(r.Right),
			"stereo": encodeStereo(r.Stereo),
		}
	}
	return out
}

func encodeEyeReport(e metrics.EyeReport) map[string]interface{} {
	return map[string]interface{}{
		"eye":              e.Eye,
		"pose_tag":         e.PoseTag,
		"reproj_mean":      n(e.ReprojMean),
		"reproj_rms":       n(e.ReprojRMS),
		"reproj_max":       n(e.ReprojMax),
		"markers_found":    e.MarkersFound,
		"markers_total":    e.MarkersTotal,
		"hole_ratio":       n(e.HoleRatio),
		"fill_ratio":       n(e.FillRatio),
		"edge_directed":    n(e.EdgeDirected),
		"edge_symmetric":   n(e.EdgeSymmetric),
		"ghost_ratio":      n(e.GhostRatio),
		"fill_ghost_ratio": n(e.FillGhostRatio),
	}
}

func encodeStereo(s metrics.StereoReport) map[string]interface{} {
	return map[string]interface{}{
		"disp_err_rms":      n(s.DispErrRMS),
		"disp_err_max":      n(s.DispErrMax),
		"disp_vert_rms":     n(s.DispVertRMS),
		"disp_change_rms":   n(s.DispChangeRMS),
		"disp_change_max":   n(s.DispChangeMax),
		"fill_contaminated": s.FillContaminated,
		"matched_markers":   s.MatchedMarkers,
	}
}

func printSummary(v harness.Verdict) {
	status := "PASS"
	if !v.Pass {
		status = "FAIL"
	}
	fmt.Printf("== %-18s %s\n", v.Mode, status)
	fmt.Printf("   reproj  L/R rms=%.3f/%.3f  pooled rms=%.3f mean=%.3f\n",
		v.Agg.Left.ReprojRMS, v.Agg.Right.ReprojRMS,
		v.Agg.PooledReprojRMS, v.Agg.PooledReprojMean)
	fmt.Printf("   dispar  err rms=%.3f max=%.3f  change rms=%.3f max=%.3f\n",
		v.Agg.DispErrRMS, v.Agg.DispErrMax, v.Agg.DispChangeRMS, v.Agg.DispChangeMax)
	fmt.Printf("   ghost   L=%.4f R=%.4f  edge L=%.3f R=%.3f  fill L=%.4f R=%.4f\n",
		v.Agg.Left.GhostMean, v.Agg.Right.GhostMean,
		v.Agg.Left.EdgeMean, v.Agg.Right.EdgeMean,
		v.Agg.Left.FillMean, v.Agg.Right.FillMean)
	for _, f := range v.Failures {
		fmt.Printf("   - [%s] %s got=%.3f limit=%.3f\n", f.Channel, f.Metric, f.Got, f.Limit)
	}
}
