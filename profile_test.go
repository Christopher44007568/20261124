package foveq

import "testing"

func TestDefaultLadderJointMonotonic(t *testing.T) {
	ladder := DefaultLadder()
	if ladder.Len() != 5 {
		t.Fatalf("expected 5 tiers, got %d", ladder.Len())
	}
	for i := 1; i < ladder.Len(); i++ {
		prev, _ := ladder.At(i - 1)
		cur, _ := ladder.At(i)
		if cur.EyeWidth <= prev.EyeWidth || cur.EyeHeight <= prev.EyeHeight {
			t.Fatalf("buffer not increasing at tier %d", i)
		}
		if cur.FoveaScale <= prev.FoveaScale {
			t.Fatalf("fovea quality not increasing at tier %d", i)
		}
		if cur.FrameBudgetMicros <= prev.FrameBudgetMicros {
			t.Fatalf("frame budget not increasing at tier %d", i)
		}
		if cur.Tier != i {
			t.Fatalf("tier index not assigned: got %d want %d", cur.Tier, i)
		}
	}
}

func TestNewLadderRejectsNonMonotonic(t *testing.T) {
	base := []Profile{
		{Name: "a", EyeWidth: 100, EyeHeight: 100, FoveaScale: 0.5, FrameBudgetMicros: 10},
		{Name: "b", EyeWidth: 200, EyeHeight: 200, FoveaScale: 0.7, FrameBudgetMicros: 20},
	}
	// Break only the fovea dimension: independent knob tweaks are forbidden.
	bad := append([]Profile(nil), base...)
	bad[1].FoveaScale = 0.4
	if _, err := NewLadder(bad); err == nil {
		t.Fatal("expected non-monotonic ladder to be rejected")
	}
}

func TestClampTierStaysInBounds(t *testing.T) {
	ladder := DefaultLadder()
	if got := ladder.clampTier(-9); got != 0 {
		t.Fatalf("lower clamp violated: %d", got)
	}
	if got := ladder.clampTier(99); got != ladder.Max() {
		t.Fatalf("upper clamp violated: %d", got)
	}
}

func TestLadderCopiesInput(t *testing.T) {
	in := []Profile{
		{Name: "a", EyeWidth: 100, EyeHeight: 100, FoveaScale: 0.5, FrameBudgetMicros: 10},
		{Name: "b", EyeWidth: 200, EyeHeight: 200, FoveaScale: 0.7, FrameBudgetMicros: 20},
	}
	ladder, err := NewLadder(in)
	if err != nil {
		t.Fatal(err)
	}
	in[0].EyeWidth = 1
	got, _ := ladder.At(0)
	if got.EyeWidth != 100 {
		t.Fatalf("ladder must copy profiles, got width %d", got.EyeWidth)
	}
}
