package xr

import "testing"

func feed(t *testing.T, sm *Machine, tk *Tracker, clock *Clock, untilMs float64) {
	t.Helper()
	for clock.Now() < untilMs {
		clock.Advance(stepMs)
		s, ev := tk.Poll()
		sm.Observe(clock.Now(), s, ev, nil)
	}
}

func TestNormalToLostAndRecovery(t *testing.T) {
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(DefaultConfig(), PolicyFreeze)
	feed(t, sm, tk, clock, 1000)
	if sm.State() != StateNormal {
		t.Fatalf("expected NORMAL, got %s", sm.State())
	}
	tk.ScheduleOutage(1000, 1300)
	feed(t, sm, tk, clock, 1290)
	if sm.State() != StateLost {
		t.Fatalf("expected LOST during outage, got %s", sm.State())
	}
	feed(t, sm, tk, clock, 1320)
	if sm.State() != StateRecovery {
		t.Fatalf("expected RECOVERY after sample returns, got %s", sm.State())
	}
	feed(t, sm, tk, clock, 1600)
	if sm.State() != StateNormal {
		t.Fatalf("expected NORMAL after recovery window, got %s", sm.State())
	}
}

func TestInertialBudgetThenFreeze(t *testing.T) {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyInertial)
	feed(t, sm, tk, clock, 1000)
	tk.ScheduleOutage(1000, 1400)

	var sawExtrapolation, sawFrozen bool
	for clock.Now() < 1400 {
		clock.Advance(stepMs)
		s, ev := tk.Poll()
		r := sm.Observe(clock.Now(), s, ev, nil)
		if r.Extrapolated {
			sawExtrapolation = true
		}
		if r.Frozen {
			sawFrozen = true
		}
	}
	if !sawExtrapolation {
		t.Fatal("inertial policy never extrapolated")
	}
	if !sawFrozen {
		t.Fatal("inertial policy never degraded to freeze after budget")
	}
}

func TestFadePolicyGoesBlack(t *testing.T) {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyFade)
	feed(t, sm, tk, clock, 1000)
	tk.ScheduleOutage(1000, 1300)

	var blackFrames int
	for clock.Now() < 1300 {
		clock.Advance(stepMs)
		s, ev := tk.Poll()
		r := sm.Observe(clock.Now(), s, ev, nil)
		if r.Black {
			blackFrames++
		}
	}
	if blackFrames == 0 {
		t.Fatal("fade policy produced no black frames during a 300 ms outage")
	}
}

func TestRelocationVisitsAllFourStates(t *testing.T) {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyInertial)
	feed(t, sm, tk, clock, 1000)
	tk.ScheduleOutage(1000, 1250)

	roomB := RigidTransform{Q: QuatFromAxisAngle(Vec3{0, 1, 0}, 1.2), T: Vec3{X: 5}}
	seen := map[State]bool{StateNormal: true}
	for clock.Now() <= 1500 {
		clock.Advance(stepMs)
		if clock.Now() == 1250 {
			tk.Relocate(1250, "room-B", roomB)
		}
		s, ev := tk.Poll()
		r := sm.Observe(clock.Now(), s, ev, nil)
		seen[r.State] = true
	}
	for _, st := range []State{StateLost, StateRelocalizing, StateRecovery, StateNormal} {
		if !seen[st] {
			t.Fatalf("state %s was never visited; seen=%v", st, seen)
		}
	}
}

func TestClockJumpDoesNotTriggerLoss(t *testing.T) {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyFade)
	feed(t, sm, tk, clock, 1000)
	clock.Jump(500)
	feed(t, sm, tk, clock, clock.Now()+300)
	if sm.State() != StateNormal {
		t.Fatalf("clock jump caused spurious state %s", sm.State())
	}
}
