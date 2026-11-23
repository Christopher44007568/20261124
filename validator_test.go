package posetimeline

import (
	"testing"
)

func sampleAt(eye Eye, clock string, frameID uint64, ts int64, x float64) Sample {
	return Sample{
		Eye:       eye,
		Clock:     clock,
		FrameID:   frameID,
		Timestamp: ts,
		Pose: Pose{
			Position:    Vec3{X: x},
			Orientation: Quat{W: 1},
		},
	}
}

func decisionByIndex(decisions []Decision, idx int) Decision {
	for _, d := range decisions {
		if d.Index == idx {
			return d
		}
	}
	return Decision{}
}

func assertReject(t *testing.T, d Decision, want RejectReason) {
	t.Helper()
	if d.Accepted {
		t.Fatalf("sample %d accepted, want rejection %s", d.Index, want)
	}
	if d.Reasons&want == 0 {
		t.Fatalf("sample %d reasons=%s, want bit %s", d.Index, d.Reasons, want)
	}
}

func assertAccept(t *testing.T, d Decision) {
	t.Helper()
	if !d.Accepted {
		t.Fatalf("sample %d rejected: %s", d.Index, d.Reasons)
	}
}

func TestAcceptCleanTimeline(t *testing.T) {
	v := NewValidator(Config{}, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 1, 100, 1),
		sampleAt(EyeRight, "", 1, 100, 1),
		sampleAt(EyeLeft, "", 2, 200, 2),
		sampleAt(EyeRight, "", 2, 200, 2),
	})
	for _, d := range decisions {
		assertAccept(t, d)
		if d.ReferenceTime != 1000 || d.GlobalTime != d.Sample.Timestamp {
			t.Fatalf("unexpected decision metadata: %+v", d)
		}
	}
	if v.AcceptedCount(EyeLeft) != 2 || v.AcceptedCount(EyeRight) != 2 {
		t.Fatalf("accepted counts wrong: %d/%d", v.AcceptedCount(EyeLeft), v.AcceptedCount(EyeRight))
	}
}

func TestDuplicateTimestampNotLastWins(t *testing.T) {
	// The batch is deliberately shuffled: the duplicate at the end of the
	// array carries an older timestamp and must not become "the latest".
	v := NewValidator(Config{}, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(EyeLeft, "", 1, 300, 3),
		sampleAt(EyeLeft, "", 2, 200, 2),
		sampleAt(EyeLeft, "", 3, 200, 99),
	})
	assertAccept(t, decisionByIndex(decisions, 0))
	assertAccept(t, decisionByIndex(decisions, 1))
	assertReject(t, decisionByIndex(decisions, 2), RejDuplicateTimestamp)

	sel, ok := v.SelectForReprojection(EyeLeft, 500, 0)
	if !ok || sel.GlobalTime != 300 || sel.Sample.Pose.Position.X != 3 {
		t.Fatalf("selector picked %+v ok=%v", sel, ok)
	}
}

func TestBackwardsClockBreaksTimeline(t *testing.T) {
	v := NewValidator(Config{}, nil)
	first := v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 1, 300, 3)})
	assertAccept(t, first[0])

	second := v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 2, 250, 2.5)})
	assertReject(t, second[0], RejClockBackwards)
	if !v.TimelineBroken(EyeLeft) {
		t.Fatal("timeline should be broken after clock regression")
	}

	// Even a fresh, well-ordered sample is refused while broken...
	third := v.Submit(2000, []Sample{sampleAt(EyeLeft, "", 3, 400, 4)})
	assertReject(t, third[0], RejTimelineBroken)
	// ...and no motion accumulates along the invalid timeline.
	if motion := v.IntegratedMotion(EyeLeft); motion.Valid {
		t.Fatalf("motion must be invalid on broken timeline: %+v", motion)
	}

	v.Reset()
	fourth := v.Submit(2000, []Sample{sampleAt(EyeLeft, "", 4, 400, 4)})
	assertAccept(t, fourth[0])
	if v.TimelineBroken(EyeLeft) {
		t.Fatal("reset must clear the broken flag")
	}
}
