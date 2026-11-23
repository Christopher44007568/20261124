package harness

import (
	"testing"

	"stereoval/internal/metrics"
	"stereoval/internal/scene"
)

func runMode(t *testing.T, mode Mode) Verdict {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Mode = mode
	frames, k := Run(cfg, scene.New())
	reports := Evaluate(frames, k, cfg.Rig, scene.New())
	return Judge(mode, AggregateReports(reports), DefaultThresholds())
}

func TestHealthyPasses(t *testing.T) {
	v := runMode(t, ModeHealthy)
	if !v.Pass {
		t.Fatalf("healthy policy must pass, failures=%v", v.Failures)
	}
}

func TestRejectHistoryFailsMonocular(t *testing.T) {
	v := runMode(t, ModeRejectHist)
	if v.Pass {
		t.Fatalf("rejected-history policy must fail")
	}
	found := map[string]bool{}
	for _, f := range v.Failures {
		found[f.Channel+"/"+f.Metric] = true
	}
	if !found["right/marker_found_rate"] {
		t.Fatalf("expected right-eye marker found-rate failure, got %v", v.Failures)
	}
	// The unaffected eye must stay healthy: the defect is monocular.
	if v.Agg.Left.ReprojRMS > DefaultThresholds().PerEyeReprojRMS {
		t.Fatalf("left eye should remain healthy, rms=%v", v.Agg.Left.ReprojRMS)
	}
}

func TestCrossEyeFillFailsOnDisparityOrGhost(t *testing.T) {
	v := runMode(t, ModeCrossFill)
	if v.Pass {
		t.Fatalf("cross-eye fill policy must fail")
	}
	sawFillGhost := false
	sawDisparity := false
	for _, f := range v.Failures {
		if f.Metric == "fill_ghost_ratio" {
			sawFillGhost = true
		}
		if f.Channel == "stereo" {
			sawDisparity = true
		}
	}
	if !sawFillGhost && !sawDisparity {
		t.Fatalf("expected fill ghosting or disparity failure, got %v", v.Failures)
	}
}

func TestPoseSkewFailsOnDisparityEvenIfMonocularIsFine(t *testing.T) {
	v := runMode(t, ModePoseSkew)
	if v.Pass {
		t.Fatalf("pose-version skew must fail")
	}
	sawDisp := false
	for _, f := range v.Failures {
		if f.Channel == "stereo" {
			sawDisp = true
		}
	}
	if !sawDisp {
		t.Fatalf("pose skew must trigger an independent stereo/disparity failure, got %v", v.Failures)
	}
}

// TestDisparityGateIndependentOfAverage encodes the headline rule: a run
// whose pooled average reprojection error is within limit must still fail
// when the disparity channel is distorted.
func TestDisparityGateIndependentOfAverage(t *testing.T) {
	th := DefaultThresholds()
	goodAgg := metrics.Aggregate{
		PooledReprojRMS: th.PooledReprojRMS - 0.05,
		Left:            goodEye(),
		Right:           goodEye(),
		DispErrRMS:      th.DispErrRMS + 2.0, // distorted disparity
		DispErrP95:      th.DispErrP95 + 2.0,
		DispChangeRMS:   th.DispChangeRMS,
		DispChangeP95:   th.DispChangeP95,
	}
	v := Judge(ModePoseSkew, goodAgg, th)
	if v.Pass {
		t.Fatalf("average error within limit but disparity distorted must FAIL")
	}
	stereo := false
	for _, f := range v.Failures {
		if f.Channel == "stereo" {
			stereo = true
		}
	}
	if !stereo {
		t.Fatalf("failure must be attributed to the stereo channel, got %v", v.Failures)
	}
}

func goodEye() metrics.EyeAgg {
	return metrics.EyeAgg{
		ReprojRMS: 0.5, ReprojP95: 1.0, ReprojMax: 1.5,
		FoundRate: 0.99, EdgeMean: 0.5, GhostMean: 0.02,
		FillGhostMean: 0.0,
	}
}
