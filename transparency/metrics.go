package transparency

import "math"

// ComputeMetrics compares a resolved frame to the supersampled reference.
// StaleCoverage counts near-layer alpha that exists outside the current near
// layer footprints, which is precisely the old-coverage-as-near-surface bug.
func ComputeMetrics(actual, reference FrameResult, config Config) FrameMetrics {
	config = config.withDefaults()
	n := config.Width * config.Height
	if n == 0 || len(actual.RGB) < n || len(reference.RGB) < n {
		return FrameMetrics{}
	}
	var colorSq, alphaAbs, stale float64
	var intersection, union, orderMatches int
	staleThreshold := math.Max(config.NearAlphaCutoff*2, 0.10)
	for i := 0; i < n; i++ {
		dr := actual.RGB[i].R - reference.RGB[i].R
		dg := actual.RGB[i].G - reference.RGB[i].G
		db := actual.RGB[i].B - reference.RGB[i].B
		colorSq += (dr*dr + dg*dg + db*db) / 3
		alphaAbs += math.Abs(actual.Alpha[i] - reference.Alpha[i])

		actualCovered := actual.NearAlpha[i] >= config.CoverageCutoff
		referenceCovered := reference.NearAlpha[i] >= config.CoverageCutoff
		if actualCovered && referenceCovered {
			intersection++
		}
		if actualCovered || referenceCovered {
			union++
		}
		if actual.NearTop[i] == reference.NearTop[i] {
			orderMatches++
		}
		if actual.NearSupport[i] == 0 && actual.NearAlpha[i] >= staleThreshold {
			stale++
		}
	}
	iou := 1.0
	if union > 0 {
		iou = float64(intersection) / float64(union)
	}
	return FrameMetrics{
		ColorRMSE:      math.Sqrt(colorSq / float64(n)),
		AlphaMAE:       alphaAbs / float64(n),
		CoverageIoU:    iou,
		OrderAgreement: float64(orderMatches) / float64(n),
		StaleCoverage:  stale / float64(n),
	}
}

func averageMetrics(frames []FrameMetrics) FrameMetrics {
	if len(frames) == 0 {
		return FrameMetrics{}
	}
	var total FrameMetrics
	for _, m := range frames {
		total.ColorRMSE += m.ColorRMSE
		total.AlphaMAE += m.AlphaMAE
		total.CoverageIoU += m.CoverageIoU
		total.OrderAgreement += m.OrderAgreement
		total.StaleCoverage += m.StaleCoverage
	}
	n := float64(len(frames))
	return FrameMetrics{
		ColorRMSE:      total.ColorRMSE / n,
		AlphaMAE:       total.AlphaMAE / n,
		CoverageIoU:    total.CoverageIoU / n,
		OrderAgreement: total.OrderAgreement / n,
		StaleCoverage:  total.StaleCoverage / n,
	}
}
