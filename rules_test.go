package posetimeline

import "testing"

func TestExpiredAndFuturePoses(t *testing.T) {
	cfg := Config{MaxPoseAge: 500}
	v := NewValidator(cfg, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 1, 200, 2),   // age 800 -> expired
		sampleAt(EyeLeft, "", 2, 900, 9),   // fresh
		sampleAt(EyeLeft, "", 3, 1100, 11), // future
	})
	assertReject(t, decisionByIndex(decisions, 0), RejExpired)
	assertAccept(t, decisionByIndex(decisions, 1))
	assertReject(t, decisionByIndex(decisions, 2), RejFuture)

	// The future sample sits last in storage but must never be chosen; the
	// only eligible pose is the accepted one at 900.
	if _, ok := v.SelectForReprojection(EyeLeft, 1050, 100); ok {
		t.Fatal("pose at 900 must age out when maxAge=100 at target 1050")
	}
	sel, ok := v.SelectForReprojection(EyeLeft, 1050, 0)
	if !ok || sel.GlobalTime != 900 {
		t.Fatalf("want pose at 900, got %+v ok=%v", sel, ok)
	}
}

func TestEyeTimeMismatch(t *testing.T) {
	v := NewValidator(Config{EyeTolerance: 10}, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 1, 100, 1),
		sampleAt(EyeRight, "", 1, 130, 1), // 30ns apart > tolerance
		sampleAt(EyeLeft, "", 2, 200, 2),
		sampleAt(EyeRight, "", 2, 205, 2), // within tolerance
	})
	assertReject(t, decisionByIndex(decisions, 0), RejEyeMismatch)
	assertReject(t, decisionByIndex(decisions, 1), RejEyeMismatch)
	assertAccept(t, decisionByIndex(decisions, 2))
	assertAccept(t, decisionByIndex(decisions, 3))
}

func TestOneFrameMultiplePoses(t *testing.T) {
	v := NewValidator(Config{}, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 7, 100, 1),
		sampleAt(EyeLeft, "", 7, 200, 2), // same frame id, later timestamp
		sampleAt(EyeLeft, "", 8, 300, 3),
	})
	assertAccept(t, decisionByIndex(decisions, 0))
	assertReject(t, decisionByIndex(decisions, 1), RejDuplicateFrame)
	assertAccept(t, decisionByIndex(decisions, 2))
}

func TestCrossClockDomain(t *testing.T) {
	clocks := NewClockRegistry()
	clocks.Register("imu", 1000) // imu tick + 1000 = primary tick
	v := NewValidator(Config{EyeTolerance: 5}, clocks)

	decisions := v.Submit(2000, []Sample{
		sampleAt(EyeLeft, "imu", 1, 500, 1),   // -> global 1500
		sampleAt(EyeRight, "", 1, 1502, 1),    // primary, within tolerance
		sampleAt(EyeLeft, "mystery", 2, 9, 9), // unmapped domain
	})
	if got := decisionByIndex(decisions, 0); !got.Accepted || got.GlobalTime != 1500 {
		t.Fatalf("translated sample: %+v", got)
	}
	assertAccept(t, decisionByIndex(decisions, 1))
	assertReject(t, decisionByIndex(decisions, 2), RejUnknownClock)
	if v.TimelineBroken(EyeLeft) {
		t.Fatal("an untranslatable sample must not break the orderable timeline")
	}

	// Cross-clock ordering still works: translated 1500 precedes primary 1800.
	more := v.Submit(2000, []Sample{
		sampleAt(EyeLeft, "imu", 3, 800, 8), // -> 1800
	})
	assertAccept(t, more[0])
}
