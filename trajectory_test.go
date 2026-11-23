package main

import (
	"math"
	"testing"
	"time"
)

func TestEStopTrajectory(t *testing.T) {
	tr := EStopTrajectory()
	before := tr.Yaw(StopAt - time.Millisecond)
	at := tr.Yaw(StopAt)
	after := tr.Yaw(StopAt + 50*time.Millisecond)
	if math.Abs(at-before-EStopSpeed*0.001) > 1e-12 {
		t.Fatalf("pre-stop yaw step = %v", at-before)
	}
	if after != at {
		t.Fatalf("post-stop yaw drifted: %v vs %v", after, at)
	}
	if tr.Omega(StopAt+time.Millisecond) != 0 {
		t.Fatal("omega must be zero after the stop")
	}
}

func TestSineTrajectoryDerivative(t *testing.T) {
	tr := SineTrajectory()
	at := 500 * time.Millisecond
	got := (tr.Yaw(at+SamplePeriod) - tr.Yaw(at-SamplePeriod)) / (2 * SamplePeriod.Seconds())
	want := tr.Omega(at)
	if math.Abs(got-want) > 1e-3 {
		t.Fatalf("finite-difference omega = %v, want %v", got, want)
	}
	// Sine starts at zero yaw with positive peak velocity.
	if tr.Yaw(0) != 0 || tr.Omega(0) <= 0 {
		t.Fatalf("unexpected sine initial conditions: yaw=%v omega=%v", tr.Yaw(0), tr.Omega(0))
	}
}

func TestConstantTrajectory(t *testing.T) {
	tr := ConstantTrajectory()
	if tr.Omega(0) != ConstantSpeed {
		t.Fatalf("omega = %v, want %v", tr.Omega(0), ConstantSpeed)
	}
	if got := tr.Yaw(100 * time.Millisecond); math.Abs(got-ConstantSpeed*0.1) > 1e-12 {
		t.Fatalf("yaw = %v", got)
	}
}
