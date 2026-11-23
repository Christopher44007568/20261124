package eval

import (
	"testing"

	"fidelitycheck/internal/inpaint"
	"fidelitycheck/internal/metrics"
	"fidelitycheck/internal/scenes"
)

func TestCenterRegressionCannotBeHiddenByGlobalGain(t *testing.T) {
	cfg := DefaultConfig()
	baseline := AlgoResult{
		Algorithm: "mean",
		Metrics: metrics.Result{
			GlobalMAE: 0.100,
			CenterMAE: 0.050,
			CenterP95: 0.055,
		},
	}
	candidate := metrics.Result{
		GlobalMAE: 0.080,
		CenterMAE: 0.070,
		CenterP95: 0.075,
	}

	guard := guardAgainst(baseline, candidate, cfg, scenes.FrequencyGradient)
	if guard.Passed {
		t.Fatal("global MAE gain incorrectly excused center regression")
	}
	if !hasViolation(guard, "center_regression") {
		t.Fatalf("missing center_regression violation: %+v", guard.Violations)
	}
}

func TestCenterP95RegressionAloneFails(t *testing.T) {
	cfg := DefaultConfig()
	baseline := AlgoResult{
		Algorithm: "mean",
		Metrics: metrics.Result{
			GlobalMAE: 0.100,
			CenterMAE: 0.050,
			CenterP95: 0.055,
		},
	}
	candidate := metrics.Result{
		GlobalMAE: 0.080,
		CenterMAE: 0.045,
		CenterP95: 0.070,
	}

	guard := guardAgainst(baseline, candidate, cfg, scenes.FrequencyGradient)
	if guard.Passed || !hasViolation(guard, "center_regression") {
		t.Fatalf("unstable center tail was accepted: %+v", guard)
	}
}

func TestFlickerRegressionFailsMotionScene(t *testing.T) {
	cfg := DefaultConfig()
	baseline := AlgoResult{
		Algorithm: "mean",
		Metrics: metrics.Result{
			GlobalMAE:  0.100,
			CenterMAE:  0.050,
			CenterP95:  0.050,
			FlickerP95: 0.010,
		},
	}
	candidate := metrics.Result{
		GlobalMAE:  0.080,
		CenterMAE:  0.045,
		CenterP95:  0.045,
		FlickerP95: 0.025,
	}

	guard := guardAgainst(baseline, candidate, cfg, scenes.SlowMotion)
	if guard.Passed || !hasViolation(guard, "temporal_flicker_regression") {
		t.Fatalf("temporal instability was accepted: %+v", guard)
	}
}

func TestEdgeRetentionRegressionFailsEdgeScene(t *testing.T) {
	cfg := DefaultConfig()
	baseline := AlgoResult{
		Algorithm: "mean",
		Metrics: metrics.Result{
			GlobalMAE:     0.100,
			CenterMAE:     0.050,
			CenterP95:     0.050,
			EdgeRetention: 0.80,
		},
	}
	candidate := metrics.Result{
		GlobalMAE:     0.080,
		CenterMAE:     0.045,
		CenterP95:     0.045,
		EdgeRetention: 0.70,
	}

	guard := guardAgainst(baseline, candidate, cfg, scenes.ColoredEdges)
	if guard.Passed || !hasViolation(guard, "edge_retention_regression") {
		t.Fatalf("color-edge loss was accepted: %+v", guard)
	}
}

func TestRunSweepsScenesAndAlgorithms(t *testing.T) {
	algorithms := inpaint.Defaults()
	mean, _ := inpaint.Find(algorithms, "mean")
	nearest, _ := inpaint.Find(algorithms, "nearest")
	cfg := Config{
		Width:          40,
		Height:         32,
		Frames:         3,
		RadiusPercents: []float64{20},
		Algorithms:     []inpaint.Algorithm{mean, nearest},
		Baseline:       "mean",
		GlobalEpsilon:  0.0001,
		CenterEpsilon:  0.0001,
		FlickerEpsilon: 0.0001,
		EdgeEpsilon:    0.01,
	}
	report, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Cases != len(scenes.AllKinds) {
		t.Fatalf("got %d cases, want %d", report.Summary.Cases, len(scenes.AllKinds))
	}
	for _, c := range report.Cases {
		if len(c.Results) != 2 {
			t.Fatalf("%s: got %d results, want 2", c.Scene, len(c.Results))
		}
		for _, r := range c.Results {
			if len(r.Metrics.PerFrame) != 3 {
				t.Fatalf("%s/%s: got %d frames, want 3", c.Scene, r.Algorithm, len(r.Metrics.PerFrame))
			}
			if r.Algorithm == cfg.Baseline && r.Guard.Compared {
				t.Fatalf("%s: baseline was compared to itself", c.Scene)
			}
		}
	}
}

func TestValidateRequiresContinuousSequence(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Frames = 1
	if err := validate(cfg); err == nil {
		t.Fatal("accepted a single-frame evaluation")
	}
}

func hasViolation(guard Guard, code string) bool {
	for _, violation := range guard.Violations {
		if violation.Code == code {
			return true
		}
	}
	return false
}
