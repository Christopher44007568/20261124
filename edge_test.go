package posetimeline

import "testing"

func TestOutOfRangeEyeAndDuplicateAcrossBatches(t *testing.T) {
	v := NewValidator(Config{}, nil)
	decisions := v.Submit(1000, []Sample{
		sampleAt(Eye(9), "", 1, 100, 1),
		sampleAt(EyeLeft, "", 1, 100, 1),
	})
	assertReject(t, decisions[0], RejUnknownClock)
	assertAccept(t, decisions[1])

	// The same timestamp in a later batch remains a duplicate for that eye.
	again := v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 2, 100, 99)})
	assertReject(t, again[0], RejDuplicateTimestamp)

	// Selector and motion accessors tolerate the invalid eye value.
	if _, ok := v.SelectForReprojection(Eye(9), 1000, 0); ok {
		t.Fatal("invalid eye must not select")
	}
	if v.TimelineBroken(Eye(9)) || v.AcceptedCount(Eye(9)) != 0 {
		t.Fatal("invalid eye state must stay empty")
	}
}

func TestResetAllowsSameTimestampReanchor(t *testing.T) {
	v := NewValidator(Config{}, nil)
	v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 1, 300, 3)})
	v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 2, 200, 2)}) // backwards
	if !v.TimelineBroken(EyeLeft) {
		t.Fatal("expected broken timeline")
	}

	v.Reset()
	decisions := v.Submit(1000, []Sample{sampleAt(EyeLeft, "", 3, 300, 30)})
	assertAccept(t, decisions[0])
	sel, ok := v.SelectForReprojection(EyeLeft, 1000, 0)
	if !ok || sel.Sample.Pose.Position.X != 30 {
		t.Fatalf("re-anchored pose wrong: %+v ok=%v", sel, ok)
	}
	motion := v.IntegratedMotion(EyeLeft)
	if !motion.Valid || motion.Steps != 0 {
		t.Fatalf("fresh baseline must restart integration: %+v", motion)
	}
}
