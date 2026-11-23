package foveq

import "testing"

func testConfig(t *testing.T) (Config, *Telemetry) {
	t.Helper()
	tel := NewTelemetry(512)
	cfg := DefaultConfig(DefaultLadder())
	cfg.StartTier = 2
	cfg.Telemetry = tel
	return cfg, tel
}

func healthySample(frame int64) Sample {
	// Budget at tier 2 is 6700us; 4000us leaves ample headroom, low error.
	return Sample{
		Frame: frame, GPUMicros: 4000,
		CenterError: 0.02, PeripheralError: 0.05, GazeVelocity: 30,
	}
}

func eventKinds(events []Event) map[EventKind]int {
	out := make(map[EventKind]int)
	for _, e := range events {
		out[e.Kind]++
	}
	return out
}

func TestDownshiftOnSustainedOverrunAndClamps(t *testing.T) {
	cfg, tel := testConfig(t)
	c, err := NewController(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// 7000us exceeds the 6700us budget at tier 2; hold needs 3 frames.
	over := healthySample(0)
	over.GPUMicros = 7000
	for i := range int64(6) {
		c.Observe(over)
		over.Frame++
		_ = i
	}
	if c.CurrentTier() >= 2 {
		t.Fatalf("expected downshift below tier 2, got %d", c.CurrentTier())
	}

	// Keep overrunning forever: the tier must stop at 0, never go negative.
	for range 200 {
		over.Frame++
		c.Observe(over)
	}
	if c.CurrentTier() != 0 {
		t.Fatalf("downshift crossed lower boundary, tier=%d", c.CurrentTier())
	}
	kinds := eventKinds(tel.Events())
	if kinds[EventClamp] == 0 {
		t.Fatal("expected boundary clamp events at minimum tier")
	}
}

func TestLoadSpikeBlocksBothShifts(t *testing.T) {
	cfg, tel := testConfig(t)
	c, _ := NewController(cfg)

	over := healthySample(0)
	over.GPUMicros = 7000
	over.LoadSpike = true
	for range 10 {
		over.Frame++
		c.Observe(over)
	}
	if c.CurrentTier() != 2 {
		t.Fatalf("spike must freeze downshift, tier=%d", c.CurrentTier())
	}
	kinds := eventKinds(tel.Events())
	if kinds[EventSpikeFreeze] == 0 {
		t.Fatal("expected load-spike freeze events")
	}

	// After the spike cooldown, sustained overrun must still downshift.
	over.LoadSpike = false
	for range 20 {
		over.Frame++
		c.Observe(over)
	}
	if c.CurrentTier() >= 2 {
		t.Fatalf("downshift expected after spike clears, tier=%d", c.CurrentTier())
	}
}

func TestFastGazeBlocksUpshift(t *testing.T) {
	cfg, tel := testConfig(t)
	c, _ := NewController(cfg)

	s := healthySample(0)
	s.CenterError = 0.30 // strong upshift demand
	s.GazeVelocity = 300 // above FastGazeDps=120
	for range 30 {
		s.Frame++
		c.Observe(s)
	}
	if c.CurrentTier() != 2 {
		t.Fatalf("fast gaze must block upshift, tier=%d", c.CurrentTier())
	}
	if eventKinds(tel.Events())[EventGazeFreeze] == 0 {
		t.Fatal("expected fast-gaze freeze events")
	}

	// Gaze stabilizes: after the upshift hysteresis hold, tier rises.
	s.GazeVelocity = 20
	for range 30 {
		s.Frame++
		c.Observe(s)
	}
	if c.CurrentTier() <= 2 {
		t.Fatalf("expected upshift after gaze stabilized, tier=%d", c.CurrentTier())
	}
}

func TestUpshiftHysteresisAndTrialRollback(t *testing.T) {
	cfg, tel := testConfig(t)
	cfg.EMAAlpha = 0.9
	cfg.EMAWarmup = 1
	cfg.TrialFrames = 20
	c, _ := NewController(cfg)

	// Build up sustained quality demand with headroom to trigger an upshift.
	s := healthySample(0)
	s.CenterError = 0.25
	start := c.CurrentTier()
	for range int64(cfg.UpshiftHoldFrames) + cfg.ShiftCooldownFrames + 2 {
		s.Frame++
		c.Observe(s)
	}
	if c.CurrentTier() != start+1 {
		t.Fatalf("expected one upshift, tier=%d start=%d", c.CurrentTier(), start)
	}
	if eventKinds(tel.Events())[EventBlockedUp] == 0 {
		t.Fatal("expected upshift hysteresis hold events before the shift")
	}

	// During the trial window, sustained overrun must roll back to last good.
	rolledBack := false
	over := healthySample(s.Frame)
	over.GPUMicros = 8300 // modestly over tier-3 budget (8000us), reacts fast
	// Three consecutive smoothed overruns inside the trial window must roll
	// the probationary tier back before it could be promoted.
	for range cfg.TrialFrames {
		over.Frame++
		c.Observe(over)
		if c.CurrentTier() == start {
			rolledBack = true
			break
		}
	}
	if !rolledBack {
		t.Fatalf("trial never rolled back to last good tier %d (current %d)", start, c.CurrentTier())
	}
	if c.LastGoodTier() != start {
		t.Fatalf("last good tier = %d, want %d", c.LastGoodTier(), start)
	}
	if eventKinds(tel.Events())[EventTrialRollback] == 0 {
		t.Fatal("expected trial rollback event")
	}
}

func TestFrameFailureRollsBackAndBlocksTier(t *testing.T) {
	cfg, tel := testConfig(t)
	c, _ := NewController(cfg)

	s := healthySample(0)
	s.Failed = true
	c.Observe(s)
	if c.CurrentTier() != 1 {
		t.Fatalf("failure should downshift one tier, got %d", c.CurrentTier())
	}
	if eventKinds(tel.Events())[EventFrameFailure] == 0 {
		t.Fatal("expected frame failure event")
	}

	// Strong, healthy demand: tier 2 is in failure cooldown and must be skipped
	// for FailureCooldownFrames, so the controller stays at tier 1.
	demand := healthySample(1)
	demand.CenterError = 0.3
	// Stay well inside the failure cooldown: tier 2 must not be reselected.
	for range cfg.FailureCooldownFrames / 2 {
		demand.Frame++
		c.Observe(demand)
	}
	if c.CurrentTier() != 1 {
		t.Fatalf("failed tier must remain blocked, tier=%d", c.CurrentTier())
	}
}

func TestDecisionAndTrajectoryRecordedEveryFrame(t *testing.T) {
	cfg, tel := testConfig(t)
	c, _ := NewController(cfg)
	const frames = 25
	for i := range int64(frames) {
		s := healthySample(i + 1)
		c.Observe(s)
	}
	if len(tel.Decisions()) != frames {
		t.Fatalf("decisions=%d want %d", len(tel.Decisions()), frames)
	}
	if len(tel.Trajectory()) != frames {
		t.Fatalf("trajectory=%d want %d", len(tel.Trajectory()), frames)
	}
	tr := tel.Trajectory()
	if tr[0].EyeWidth == 0 || tr[0].BudgetMicros == 0 || tr[0].FoveaScale == 0 {
		t.Fatal("trajectory point must carry full joint profile fields")
	}
	// Every trajectory entry must match a real ladder profile.
	for _, p := range tr {
		prof, err := cfg.Ladder.At(p.Tier)
		if err != nil || prof.EyeWidth != p.EyeWidth ||
			prof.FrameBudgetMicros != p.BudgetMicros {
			t.Fatalf("trajectory point diverges from ladder: %+v", p)
		}
	}
}

func TestNoIndependentKnobsExposed(t *testing.T) {
	// Compile-time guarantee: Observe only accepts a Sample and returns a whole
	// Profile; the Controller exposes no per-knob setters.
	cfg, _ := testConfig(t)
	c, _ := NewController(cfg)
	prof, d := c.Observe(healthySample(1))
	if prof.Pixels() != prof.EyeWidth*prof.EyeHeight {
		t.Fatal("profile pixel helper mismatch")
	}
	if d.DecisionLog()["from_tier"] != d.FromTier {
		t.Fatal("decision log must expose tier inputs")
	}
}
