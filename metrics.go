package reprojection

// FrameMetrics contains the checks used by the three-way comparison. Ratios
// are in [0,1]; distances are display pixels; times are milliseconds.
type FrameMetrics struct {
	UIRequiredPixels  float64
	UIVisiblePixels   float64
	DepthRejected     float64
	DepthRejectedRate float64
	UIPresenceError   float64
	ReferenceMismatch float64
	FixedUIDriftPx    float64
	CursorErrorPx     float64
	FollowLatencyMs   float64
	ScanoutSkewMs     float64
	WorldCoverage     float64
	WorldHoleRate     float64
	WorldNearCoverage float64
	NearMaskAgreement float64
}

func maxMetrics(a, b FrameMetrics) FrameMetrics {
	maxf := func(x, y float64) float64 {
		if x > y {
			return x
		}
		return y
	}
	return FrameMetrics{
		UIRequiredPixels:  maxf(a.UIRequiredPixels, b.UIRequiredPixels),
		UIVisiblePixels:   maxf(a.UIVisiblePixels, b.UIVisiblePixels),
		DepthRejected:     maxf(a.DepthRejected, b.DepthRejected),
		DepthRejectedRate: maxf(a.DepthRejectedRate, b.DepthRejectedRate),
		UIPresenceError:   maxf(a.UIPresenceError, b.UIPresenceError),
		ReferenceMismatch: maxf(a.ReferenceMismatch, b.ReferenceMismatch),
		FixedUIDriftPx:    maxf(a.FixedUIDriftPx, b.FixedUIDriftPx),
		CursorErrorPx:     maxf(a.CursorErrorPx, b.CursorErrorPx),
		FollowLatencyMs:   maxf(a.FollowLatencyMs, b.FollowLatencyMs),
		ScanoutSkewMs:     maxf(a.ScanoutSkewMs, b.ScanoutSkewMs),
		WorldCoverage:     maxf(a.WorldCoverage, b.WorldCoverage),
		WorldHoleRate:     maxf(a.WorldHoleRate, b.WorldHoleRate),
		WorldNearCoverage: maxf(a.WorldNearCoverage, b.WorldNearCoverage),
		NearMaskAgreement: maxf(a.NearMaskAgreement, b.NearMaskAgreement),
	}
}

func addMetrics(a, b FrameMetrics) FrameMetrics {
	return FrameMetrics{
		UIRequiredPixels:  a.UIRequiredPixels + b.UIRequiredPixels,
		UIVisiblePixels:   a.UIVisiblePixels + b.UIVisiblePixels,
		DepthRejected:     a.DepthRejected + b.DepthRejected,
		DepthRejectedRate: a.DepthRejectedRate + b.DepthRejectedRate,
		UIPresenceError:   a.UIPresenceError + b.UIPresenceError,
		ReferenceMismatch: a.ReferenceMismatch + b.ReferenceMismatch,
		FixedUIDriftPx:    a.FixedUIDriftPx + b.FixedUIDriftPx,
		CursorErrorPx:     a.CursorErrorPx + b.CursorErrorPx,
		FollowLatencyMs:   a.FollowLatencyMs + b.FollowLatencyMs,
		ScanoutSkewMs:     a.ScanoutSkewMs + b.ScanoutSkewMs,
		WorldCoverage:     a.WorldCoverage + b.WorldCoverage,
		WorldHoleRate:     a.WorldHoleRate + b.WorldHoleRate,
		WorldNearCoverage: a.WorldNearCoverage + b.WorldNearCoverage,
		NearMaskAgreement: a.NearMaskAgreement + b.NearMaskAgreement,
	}
}

func divMetrics(a FrameMetrics, n float64) FrameMetrics {
	if n == 0 {
		return a
	}
	return FrameMetrics{
		UIRequiredPixels:  a.UIRequiredPixels / n,
		UIVisiblePixels:   a.UIVisiblePixels / n,
		DepthRejected:     a.DepthRejected / n,
		DepthRejectedRate: a.DepthRejectedRate / n,
		UIPresenceError:   a.UIPresenceError / n,
		ReferenceMismatch: a.ReferenceMismatch / n,
		FixedUIDriftPx:    a.FixedUIDriftPx / n,
		CursorErrorPx:     a.CursorErrorPx / n,
		FollowLatencyMs:   a.FollowLatencyMs / n,
		ScanoutSkewMs:     a.ScanoutSkewMs / n,
		WorldCoverage:     a.WorldCoverage / n,
		WorldHoleRate:     a.WorldHoleRate / n,
		WorldNearCoverage: a.WorldNearCoverage / n,
		NearMaskAgreement: a.NearMaskAgreement / n,
	}
}
