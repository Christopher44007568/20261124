package posetimeline

import "testing"

// TestAllFaultsAtOnce feeds one batch where duplicate timestamps, a
// backwards tick, an expired pose, a future pose and an unknown clock appear
// together, and checks which single pose re-projection must choose.
func TestAllFaultsAtOnce(t *testing.T) {
	clocks := NewClockRegistry()
	clocks.Register("ext", 500)
	cfg := Config{MaxPoseAge: 400, EyeTolerance: 1000}
	v := NewValidator(cfg, clocks)

	// Anchor a valid timeline first.
	v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 10, 500, 5),
		sampleAt(EyeRight, "", 10, 500, 5),
	})

	batch1 := []Sample{
		sampleAt(EyeLeft, "", 11, 550, 5),  // expired (age 450 > 400), after anchor
		sampleAt(EyeLeft, "", 12, 700, 7),  // valid pose
		sampleAt(EyeLeft, "", 13, 700, 77), // duplicate timestamp
		sampleAt(EyeRight, "", 11, 700, 7),
	}
	decisions1 := v.Submit(1000, batch1)
	assertReject(t, decisionByIndex(decisions1, 0), RejExpired)
	assertAccept(t, decisionByIndex(decisions1, 1))
	assertReject(t, decisionByIndex(decisions1, 2), RejDuplicateTimestamp)

	batch2 := []Sample{
		sampleAt(EyeLeft, "", 14, 650, 65),   // backwards vs 700 -> break
		sampleAt(EyeLeft, "", 15, 1200, 12),  // future, also broken
		sampleAt(EyeLeft, "ghost", 16, 1, 1), // unknown clock, never ordered
	}
	decisions2 := v.Submit(1000, batch2)
	assertReject(t, decisionByIndex(decisions2, 0), RejClockBackwards)
	future := decisionByIndex(decisions2, 1)
	assertReject(t, future, RejFuture)
	assertReject(t, future, RejTimelineBroken)
	assertReject(t, decisionByIndex(decisions2, 2), RejUnknownClock)

	// Right eye stays healthy and re-projection selects 700, never any of
	// the rejected poses and never an array tail.
	sel, ok := v.SelectForReprojection(EyeRight, 1000, 0)
	if !ok || sel.GlobalTime != 700 {
		t.Fatalf("right-eye selection wrong: %+v ok=%v", sel, ok)
	}

	// Left timeline is broken: no selection and no motion accumulation.
	if _, ok := v.SelectForReprojection(EyeLeft, 1000, 0); ok {
		t.Fatal("broken left timeline must not yield a re-projection pose")
	}
	if motion := v.IntegratedMotion(EyeLeft); motion.Valid {
		t.Fatalf("motion accumulation must stop: %+v", motion)
	}

	// The decision log preserves batch order and reference time.
	log := v.Decisions()
	if len(log) != len(batch1)+len(batch2)+2 {
		t.Fatalf("decision log length=%d want %d", len(log), len(batch1)+len(batch2)+2)
	}
	for _, d := range log {
		if d.ReferenceTime != 1000 {
			t.Fatalf("reference time not recorded: %+v", d)
		}
	}
}

func TestIntegrationOnlyAcrossAcceptedSamples(t *testing.T) {
	v := NewValidator(Config{}, nil)
	v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 1, 100, 0),
		sampleAt(EyeLeft, "", 2, 200, 10),
		sampleAt(EyeLeft, "", 3, 200, 99), // duplicate: no integration step
		sampleAt(EyeLeft, "", 4, 300, 25),
	})
	motion := v.IntegratedMotion(EyeLeft)
	if !motion.Valid || motion.Steps != 2 {
		t.Fatalf("motion=%+v, want 2 integration steps", motion)
	}
	if motion.Translation.X != 25 {
		t.Fatalf("translation=%v want 25", motion.Translation.X)
	}

	pose, ts, ok := v.LastAcceptedPose(EyeLeft)
	if !ok || ts != 300 || pose.Position.X != 25 {
		t.Fatalf("last accepted pose wrong: %+v ts=%d ok=%v", pose, ts, ok)
	}
}

func TestEmptyAndResetTimeline(t *testing.T) {
	v := NewValidator(Config{}, nil)
	if _, ok := v.SelectForReprojection(EyeLeft, 100, 0); ok {
		t.Fatal("empty timeline must not select a pose")
	}
	if motion := v.IntegratedMotion(EyeLeft); motion.Valid {
		t.Fatal("empty timeline has no valid motion")
	}
}
