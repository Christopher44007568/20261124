package ae

import (
	"math"
	"testing"
)

func approx(t *testing.T, got, want, tolerance float64, name string) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.9f, want %.9f (+/- %.9f)", name, got, want, tolerance)
	}
}

func TestZonalMeterDoesNotReadOldZonesAfterCut(t *testing.T) {
	cfg := DefaultMeterConfig()
	meter, err := NewMeter(ModeZonal, cfg)
	if err != nil {
		t.Fatal(err)
	}

	bright := NewUniformFrame(96, 72, 0.95, false)
	first, err := meter.Measure(bright)
	if err != nil {
		t.Fatal(err)
	}
	if first.Generation != 0 || first.Control < 0.94 {
		t.Fatalf("unexpected bright measurement: %+v", first)
	}

	dark := NewUniformFrame(96, 72, 0.02, true)
	afterCut, err := meter.Measure(dark)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, afterCut.Control, 0.02, 1e-9, "first-frame control")
	approx(t, afterCut.Mean, 0.02, 1e-9, "first-frame mean")
	if afterCut.HighFraction != 0 {
		t.Fatalf("old highlights leaked into new scene: high fraction = %g", afterCut.HighFraction)
	}
	if afterCut.Generation != 1 {
		t.Fatalf("generation = %d, want 1 after scene cut", afterCut.Generation)
	}
	stats := meter.Stats()
	if stats.Resets != 1 || stats.StaleZoneReads != 0 {
		t.Fatalf("unexpected meter stats: %+v", stats)
	}
}

func TestTemporalWindowStartsEmptyAfterCut(t *testing.T) {
	cfg := DefaultMeterConfig()
	cfg.WindowFrames = 3
	meter, err := NewMeter(ModeTemporalWindow, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := meter.Measure(NewUniformFrame(96, 72, 0.9, false)); err != nil {
			t.Fatal(err)
		}
	}
	dark := NewUniformFrame(96, 72, 0.03, true)
	measurement, err := meter.Measure(dark)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, measurement.Control, 0.03, 1e-9, "window control")
	if measurement.WindowFrames != 1 {
		t.Fatalf("window has %d frames after cut, want 1", measurement.WindowFrames)
	}
	if measurement.Generation != 1 || meter.Stats().Resets != 1 {
		t.Fatalf("window did not reset: measurement=%+v stats=%+v",
			measurement, meter.Stats())
	}
}

func TestZonalMedianRejectsLocalHighlight(t *testing.T) {
	var scenario Scenario
	for _, candidate := range Scenarios() {
		if candidate.Name == "local-highlight" {
			scenario = candidate
			break
		}
	}
	frame := scenario.Frames[stepFrame]
	frame = ApplyExposure(frame, InitialEV(scenario, DefaultControlConfig()))

	cfg := DefaultMeterConfig()
	whole, err := NewMeter(ModeWholeFrame, cfg)
	if err != nil {
		t.Fatal(err)
	}
	zonal, err := NewMeter(ModeZonal, cfg)
	if err != nil {
		t.Fatal(err)
	}
	wholeResult, err := whole.Measure(frame)
	if err != nil {
		t.Fatal(err)
	}
	zonalResult, err := zonal.Measure(frame)
	if err != nil {
		t.Fatal(err)
	}

	if zonalResult.Control+0.01 >= wholeResult.Control {
		t.Fatalf("zone median %.6f was not more robust than whole mean %.6f",
			zonalResult.Control, wholeResult.Control)
	}
	approx(t, zonalResult.Control, 0.18, 1e-9, "zone median")
}
