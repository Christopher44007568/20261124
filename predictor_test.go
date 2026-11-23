package main

import (
	"math"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		HistoryWindow: 20 * time.Millisecond,
		Horizon:       50 * time.Millisecond,
		VelocityTau:   8 * time.Millisecond,
		MaxAngularVel: 6.0,
	}
}

func feedConstant(p *Predictor, n int, dt time.Duration, speed float64) {
	for i := 0; i < n; i++ {
		t := time.Duration(i+1) * dt
		p.AddSample(t, float64(i+1)*speed*dt.Seconds())
	}
}

func TestZOHReusesLatestYaw(t *testing.T) {
	p := NewPredictor(ModeZOH, testConfig())
	feedConstant(p, 5, time.Millisecond, 3.0)
	got, ok := p.PredictAt(5*time.Millisecond, 16*time.Millisecond)
	if !ok {
		t.Fatal("query rejected")
	}
	if math.Abs(got-0.015) > 1e-12 {
		t.Fatalf("ZOH yaw = %v, want latest 0.015", got)
	}
}

func TestVelocityExtrapolation(t *testing.T) {
	p := NewPredictor(ModeVelocity, testConfig())
	feedConstant(p, 500, time.Millisecond, 3.0)
	got, ok := p.PredictAt(500*time.Millisecond, 511*time.Millisecond)
	if !ok {
		t.Fatal("query rejected")
	}
	want := 500*0.003 + 3.0*0.011
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("velocity yaw = %v, want %v", got, want)
	}
}

func TestClampedLimitsVelocity(t *testing.T) {
	cfg := testConfig()
	p := NewPredictor(ModeClamped, cfg)
	feedConstant(p, 500, time.Millisecond, 20.0) // 20 rad/s > 6 clamp
	got, ok := p.PredictAt(500*time.Millisecond, 511*time.Millisecond)
	if !ok {
		t.Fatal("query rejected")
	}
	want := 500*0.02 + cfg.MaxAngularVel*0.011
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("clamped yaw = %v, want %v", got, want)
	}
}

func TestRejectRolledBackAndStaleSamples(t *testing.T) {
	p := NewPredictor(ModeZOH, testConfig())
	feedConstant(p, 25, time.Millisecond, 1.0)
	// Duplicate timestamp: rolled back.
	if p.AddSample(25*time.Millisecond, 1) {
		t.Fatal("duplicate timestamp accepted")
	}
	// Older than the retained window: before window.
	if p.AddSample(time.Millisecond, 1) {
		t.Fatal("stale sample accepted")
	}
	if math.IsNaN(0) {
		t.Fatal("sanity")
	}
	if p.AddSample(26*time.Millisecond, math.NaN()) {
		t.Fatal("NaN yaw accepted")
	}
	reasons := map[RejectReason]bool{}
	for _, r := range p.Rejects() {
		if r.Kind != "sample" {
			t.Fatalf("unexpected reject kind %q", r.Kind)
		}
		reasons[r.Reason] = true
	}
	for _, want := range []RejectReason{RejectTimestampRolled, RejectBeforeWindow, RejectNaNYaw} {
		if !reasons[want] {
			t.Fatalf("missing reject reason %q, got %v", want, reasons)
		}
	}
}

func TestRejectQueries(t *testing.T) {
	cfg := testConfig()
	p := NewPredictor(ModeZOH, cfg)
	if _, ok := p.PredictAt(0, time.Millisecond); ok {
		t.Fatal("empty predictor answered a query")
	}
	feedConstant(p, 25, time.Millisecond, 1.0)
	// Target older than the retained window.
	if _, ok := p.PredictAt(25*time.Millisecond, time.Millisecond); ok {
		t.Fatal("stale query accepted")
	}
	// Target further than the prediction horizon.
	if _, ok := p.PredictAt(25*time.Millisecond, 25*time.Millisecond+cfg.Horizon+time.Millisecond); ok {
		t.Fatal("over-horizon query accepted")
	}
	reasons := map[RejectReason]bool{}
	for _, r := range p.Rejects() {
		reasons[r.Reason] = true
	}
	for _, want := range []RejectReason{RejectNoSamples, RejectBeforeWindow, RejectBeyondHorizon} {
		if !reasons[want] {
			t.Fatalf("missing query reject %q, got %v", want, reasons)
		}
	}
}

func TestHistoryWindowTrim(t *testing.T) {
	cfg := testConfig()
	p := NewPredictor(ModeZOH, cfg)
	feedConstant(p, 40, time.Millisecond, 1.0)
	// Window is 20 ms; newest is 40 ms so samples before 20 ms are dropped.
	if p.Len() > 21 {
		t.Fatalf("history length = %d, expected <= 21", p.Len())
	}
	last, ok := p.LastT()
	if !ok || last != 40*time.Millisecond {
		t.Fatalf("LastT = %v (%v), want 40ms", last, ok)
	}
}
