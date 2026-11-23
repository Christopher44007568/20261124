package ae

import "testing"

func TestFirstFrameAfterSceneCutUsesOnlyCurrentZones(t *testing.T) {
	scenario := Scenarios()[0]
	trace, err := RunScenario(
		scenario,
		ModeZonal,
		DefaultMeterConfig(),
		DefaultControlConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
	before := trace.Samples[scenario.LastTransition-1]
	first := trace.Samples[scenario.LastTransition]
	if !first.SceneCut {
		t.Fatal("first post-jump sample is not marked as a scene cut")
	}
	if before.Generation != 0 || first.Generation != 1 {
		t.Fatalf("generation before/after cut = %d/%d, want 0/1",
			before.Generation, first.Generation)
	}
	// The exposed black field is about 0.009 at the pre-cut exposure. A stale
	// bright zone would report roughly 0.18 and fail this bound.
	if first.Control > 0.02 {
		t.Fatalf("first post-cut control = %.6f, old zones appear to be used", first.Control)
	}
	if first.StaleZoneReads != 0 {
		t.Fatalf("stale zone reads = %d, want 0", first.StaleZoneReads)
	}
}

func TestCompareRecordsAllScenariosAndMeters(t *testing.T) {
	modes := []MeterMode{ModeWholeFrame, ModeZonal, ModeTemporalWindow}
	traces, err := Compare(Scenarios(), modes, DefaultMeterConfig(), DefaultControlConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != len(Scenarios())*len(modes) {
		t.Fatalf("trace count = %d, want %d", len(traces), len(Scenarios())*len(modes))
	}
	for _, trace := range traces {
		if len(trace.Samples) != scenarioFrames {
			t.Fatalf("%s/%s has %d samples", trace.Scenario.Name, trace.Mode, len(trace.Samples))
		}
		if !trace.Performance.Settled {
			t.Fatalf("%s/%s did not settle: %+v",
				trace.Scenario.Name, trace.Mode, trace.Performance)
		}
		for _, sample := range trace.Samples {
			if sample.StaleZoneReads != 0 {
				t.Fatalf("%s/%s read stale zones", trace.Scenario.Name, trace.Mode)
			}
		}
	}
}

func TestMeteringStrategiesShowExpectedTradeoffs(t *testing.T) {
	modes := []MeterMode{ModeWholeFrame, ModeZonal, ModeTemporalWindow}
	traces, err := Compare(Scenarios(), modes, DefaultMeterConfig(), DefaultControlConfig())
	if err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]Trace, len(traces))
	for _, trace := range traces {
		byKey[trace.Scenario.Name+"/"+string(trace.Mode)] = trace
	}

	wholeLocal := byKey["local-highlight/whole-frame"].Performance
	zonalLocal := byKey["local-highlight/zonal"].Performance
	if wholeLocal.FinalEV >= zonalLocal.FinalEV-0.1 {
		t.Fatalf("whole-frame local highlight final EV %.4f should be at least 0.1 below zonal %.4f",
			wholeLocal.FinalEV, zonalLocal.FinalEV)
	}

	wholeBlack := byKey["black-field/whole-frame"].Performance
	windowBlack := byKey["black-field/temporal-window"].Performance
	if windowBlack.ConvergenceFrames <= wholeBlack.ConvergenceFrames {
		t.Fatalf("window convergence %d should be slower than whole-frame %d",
			windowBlack.ConvergenceFrames, wholeBlack.ConvergenceFrames)
	}
	if windowBlack.OvershootEV <= 0 {
		t.Fatalf("window black-field overshoot = %g, want a positive overshoot",
			windowBlack.OvershootEV)
	}

	wholeFlash := byKey["flash/whole-frame"].Performance
	windowFlash := byKey["flash/temporal-window"].Performance
	if windowFlash.OvershootEV <= wholeFlash.OvershootEV {
		t.Fatalf("window flash overshoot %.5f should exceed whole-frame %.5f",
			windowFlash.OvershootEV, wholeFlash.OvershootEV)
	}
}
