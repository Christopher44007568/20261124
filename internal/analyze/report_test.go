package analyze

import (
	"encoding/json"
	"math"
	"testing"

	"posebench/internal/geom"
	"posebench/internal/motion"
	"posebench/internal/scan"
	"posebench/internal/strategy"
)

func TestPerRowHasZeroErrorAndStartIsWorseThanMid(t *testing.T) {
	profile := motion.Catalog()[2]
	sh := scan.DefaultShutter()
	cfg := DefaultConfig()
	cfg.Frames = 60
	scenario, err := RunScenario(profile, sh, cfg)
	if err != nil {
		t.Fatal(err)
	}
	start := findStrategy(t, scenario, string(strategy.UniformStart))
	mid := findStrategy(t, scenario, string(strategy.UniformMid))
	perRow := findStrategy(t, scenario, string(strategy.PerRow))
	if start.Deployable || !mid.Deployable {
		t.Fatal("start must be forbidden and mid must be deployable")
	}
	if start.ErrorDeg.Mean <= mid.ErrorDeg.Mean {
		t.Fatalf("start mean error %.6f <= mid %.6f", start.ErrorDeg.Mean, mid.ErrorDeg.Mean)
	}
	if perRow.ErrorDeg.Max > 1e-4 {
		t.Fatalf("per-row max error = %g", perRow.ErrorDeg.Max)
	}
	if perRow.ErrorMM.Max > 1e-9 {
		t.Fatalf("per-row translation error = %g", perRow.ErrorMM.Max)
	}
}

func TestSmoothSegmentationMeetsBoundaryBudget(t *testing.T) {
	sh := scan.DefaultShutter()
	cfg := DefaultConfig()
	cfg.Frames = 60
	for _, profile := range motion.Catalog() {
		scenario, err := RunScenario(profile, sh, cfg)
		if err != nil {
			t.Fatal(err)
		}
		smooth := findStrategy(t, scenario, string(strategy.Segmented))
		hard := findStrategy(t, scenario, string(strategy.HardSegments))
		if !smooth.Deployable || !smooth.BoundaryOK {
			t.Fatalf("%s smooth boundary failed: residual %.4f deg / %.4f px",
				profile.Name, smooth.BoundaryResidualDeg, smooth.BoundaryResidualPx)
		}
		if hard.Deployable || hard.BoundaryOK {
			t.Fatalf("%s hard segmentation should be rejected", profile.Name)
		}
		if hard.BoundaryResidualDeg <= smooth.BoundaryResidualDeg {
			t.Fatalf("%s hard residual %.4f <= smooth %.4f",
				profile.Name, hard.BoundaryResidualDeg, smooth.BoundaryResidualDeg)
		}
	}
}

func TestImageShearAppearsWithMotion(t *testing.T) {
	sh := scan.DefaultShutter()
	static := motion.Profile{Name: "static", PeriodSec: 1, BaseT: geom.Vec3{Z: 550}}
	still := SummarizeShear([]ShearFrame{
		MeasureShear(static, sh, 0),
		MeasureShear(static, sh, 0.25),
	})
	if still.MaxShiftPx > 1e-9 {
		t.Fatalf("static shear = %g px", still.MaxShiftPx)
	}

	fast := motion.Catalog()[0]
	var frames []ShearFrame
	for i := 0; i < 120; i++ {
		frames = append(frames, MeasureShear(fast, sh, fast.PeriodSec*float64(i)/120))
	}
	moving := SummarizeShear(frames)
	if moving.MaxShiftPx < 2 {
		t.Fatalf("fast-turn max shear = %.3f px, want at least 2 px", moving.MaxShiftPx)
	}
	if moving.MeanLandmarkRMS <= 0 {
		t.Fatalf("fast-turn landmark shift = %g", moving.MeanLandmarkRMS)
	}
}

func TestDistribution(t *testing.T) {
	d := Summarize([]float64{0, 1, 2, 3, 4})
	if math.Abs(d.Mean-2) > 1e-12 || math.Abs(d.RMS-math.Sqrt(6)) > 1e-12 {
		t.Fatalf("distribution = %+v", d)
	}
	if math.Abs(d.P95-3.8) > 1e-12 || d.Max != 4 {
		t.Fatalf("distribution = %+v", d)
	}
}

func TestReportMarshalsToJSON(t *testing.T) {
	sh := scan.DefaultShutter()
	cfg := DefaultConfig()
	cfg.Frames = 8
	report, err := Run(motion.Catalog()[:1], sh, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("report contains a non-JSON value: %v", err)
	}
}

func findStrategy(t *testing.T, scenario ScenarioReport, kind string) StrategySummary {
	t.Helper()
	for _, s := range scenario.Strategies {
		if s.Kind == kind {
			return s
		}
	}
	t.Fatalf("strategy %q not found", kind)
	return StrategySummary{}
}
