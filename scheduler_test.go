package foveq

import (
	"math"
	"testing"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestSchedulerWeightedShares(t *testing.T) {
	s, err := NewScheduler(SchedulerConfig{
		WeightLeft: 3, WeightRight: 1, QuantumMicros: 1000, StarvationFrames: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Both eyes always demand more than available: long-run shares must
	// converge to the 3:1 weights rather than an even 50/50 split.
	for range 200 {
		s.Allocate([2]float64{100000, 100000}, 2000)
	}
	shares := s.Shares()
	if !approx(shares[EyeLeft], 0.75, 0.03) {
		t.Fatalf("left share = %.3f, want ~0.75", shares[EyeLeft])
	}
	if !approx(shares[EyeRight], 0.25, 0.03) {
		t.Fatalf("right share = %.3f, want ~0.25", shares[EyeRight])
	}
}

func TestSchedulerAntiStarvation(t *testing.T) {
	s, err := NewScheduler(SchedulerConfig{
		WeightLeft: 5, WeightRight: 1, QuantumMicros: 500, StarvationFrames: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The left eye saturates the GPU while the right eye stays silent, so it
	// builds a large deficit. When the right eye finally requests budget, it
	// has already exceeded StarvationFrames and must jump the queue.
	for range 6 {
		s.Allocate([2]float64{10000, 0}, 1000)
	}
	allocs := s.Allocate([2]float64{10000, 300}, 1000)
	if allocs[EyeRight].Granted <= 0 || !allocs[EyeRight].Starving {
		t.Fatalf("starved right eye must be promoted first: %+v", allocs[EyeRight])
	}
	stats := s.Stats()
	if stats.Starvations[EyeRight] == 0 {
		t.Fatal("expected at least one explicit starvation promotion for right eye")
	}
	if stats.Served[EyeLeft] == 0 {
		t.Fatal("left eye must also keep being served")
	}
}

func TestSchedulerRespectsCapacityAndRetainsDeficit(t *testing.T) {
	s, _ := NewScheduler(SchedulerConfig{
		WeightLeft: 1, WeightRight: 1, QuantumMicros: 1000, StarvationFrames: 3,
	})
	var total float64
	for range 50 {
		allocs := s.Allocate([2]float64{800, 800}, 1000)
		round := allocs[0].Granted + allocs[1].Granted
		if round > 1000+1e-9 {
			t.Fatalf("allocation %.2f exceeded frame capacity", round)
		}
		total += round
	}
	if !approx(total, 50*1000, 1e-6) {
		t.Fatalf("unused capacity should be zero under full demand, total=%.2f", total)
	}
	stats := s.Stats()
	if !approx(stats.Allocated[EyeLeft], stats.Allocated[EyeRight], 1e-6) {
		t.Fatalf("equal weights must split evenly: %v", stats.Allocated)
	}
}

func TestSchedulerRejectsBadConfig(t *testing.T) {
	if _, err := NewScheduler(SchedulerConfig{WeightLeft: 0, WeightRight: 1, QuantumMicros: 10}); err == nil {
		t.Fatal("expected error for non-positive weight")
	}
	if _, err := NewScheduler(SchedulerConfig{WeightLeft: 1, WeightRight: 1, QuantumMicros: 0}); err == nil {
		t.Fatal("expected error for non-positive quantum")
	}
}
