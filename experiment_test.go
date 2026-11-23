package main

import (
	"math"
	"testing"
	"time"
)

func runByName(t *testing.T, name string, mode Mode, cfg Config) RunResult {
	for _, tr := range AllTrajectories() {
		if tr.Name == name {
			return Run(tr, mode, cfg)
		}
	}
	t.Fatalf("scenario %q not found", name)
	return RunResult{}
}

func TestZOHRightEyeErrorDoubleLeft(t *testing.T) {
	res := runByName(t, "constant", ModeZOH, DefaultConfig())
	left := res.Stats["left"].MeanAbsRad
	right := res.Stats["right"].MeanAbsRad
	if math.Abs(right-2*left) > 1e-6 {
		t.Fatalf("right mean %v is not twice left mean %v", right, left)
	}
	// Expected ZOH lag: omega * offset = 10 rad/s * 11 ms.
	want := ConstantSpeed * LeftEyeOffset.Seconds()
	if math.Abs(left-want) > 1e-9 {
		t.Fatalf("ZOH left lag = %v, want %v", left, want)
	}
}

func TestEStopVelocityOvershootsAndClampReducesIt(t *testing.T) {
	cfg := DefaultConfig()
	vel := runByName(t, "estop", ModeVelocity, cfg)
	clamp := runByName(t, "estop", ModeClamped, cfg)
	if vel.Stats["both"].MaxOvershoot <= 0 {
		t.Fatal("velocity mode must overshoot after the emergency stop")
	}
	if clamp.Stats["both"].MaxOvershoot >= vel.Stats["both"].MaxOvershoot {
		t.Fatal("clamped mode must reduce peak overshoot")
	}
}

func TestDropScenarioRejectsLongGapQueries(t *testing.T) {
	res := runByName(t, "drop", ModeVelocity, DefaultConfig())
	reasons := map[RejectReason]int{}
	for _, rj := range res.Rejects {
		reasons[rj.Reason]++
	}
	if reasons[RejectBeyondHorizon] == 0 {
		t.Fatal("long frame gap must produce beyond-horizon rejects")
	}
	if reasons[RejectTimestampRolled] == 0 || reasons[RejectNaNYaw] == 0 {
		t.Fatalf("fault injections missing: %v", reasons)
	}
	// Some accepted rows must carry a positive gap age.
	sawGap := false
	for _, r := range res.Records {
		if r.GapAge > 0 {
			sawGap = true
			break
		}
	}
	if !sawGap {
		t.Fatal("no record captured frame-gap age")
	}
}

func TestAllEyesHaveRecordsAndDeterministic(t *testing.T) {
	cfg := DefaultConfig()
	first := map[string]int{}
	for _, tr := range AllTrajectories() {
		res := Run(tr, ModeVelocity, cfg)
		if res.Stats["left"].Count == 0 || res.Stats["right"].Count == 0 {
			t.Fatalf("%s produced no eye records", tr.Name)
		}
		first[tr.Name] = res.Stats["both"].Count
	}
	// Re-run and confirm identical counts (no randomness, fixed timestamps).
	for _, tr := range AllTrajectories() {
		res := Run(tr, ModeVelocity, cfg)
		if res.Stats["both"].Count != first[tr.Name] {
			t.Fatalf("%s non-deterministic counts: %d vs %d",
				tr.Name, first[tr.Name], res.Stats["both"].Count)
		}
	}
}

func TestEyeOffsetsAreElevenMs(t *testing.T) {
	if LeftEyeOffset != 11*time.Millisecond {
		t.Fatalf("left offset = %v", LeftEyeOffset)
	}
	if RightEyeOffset != 22*time.Millisecond {
		t.Fatalf("right offset = %v", RightEyeOffset)
	}
}
