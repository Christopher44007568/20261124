package main

import "testing"

func totals(stats []FrameStat) (miss, over, cand int) {
	for _, s := range stats {
		miss += s.Misses
		over += s.Overdraw
		cand += s.Candidates
	}
	return
}

// The simulation must produce a non-trivial ground truth: some frames
// require several objects, and quick head turns must make the stale
// shared-union strategy miss objects.
func TestScenarioIsInteresting(t *testing.T) {
	stats, unionMiss := RunSim(nil, t.TempDir())
	if len(unionMiss) == 0 {
		t.Fatal("union strategy never misses; scenario does not exercise reveals")
	}
	maxReq := 0
	for _, s := range stats["none"] {
		if s.Rendered > maxReq {
			maxReq = s.Rendered
		}
	}
	if maxReq < 5 {
		t.Fatalf("ground truth too sparse: max required per frame = %d", maxReq)
	}
}

// Path prediction must miss strictly less than no-prediction, which in
// turn must miss less than the shared-union reuse, while the union
// strategy stays the cheapest in candidate count.
func TestStrategyOrdering(t *testing.T) {
	stats, _ := RunSim(nil, t.TempDir())
	missNone, _, candNone := totals(stats["none"])
	missUnion, _, candUnion := totals(stats["union"])
	missPred, _, candPred := totals(stats["predict"])

	if missUnion <= missNone {
		t.Fatalf("expected union misses > none misses, got %d vs %d", missUnion, missNone)
	}
	if missPred >= missNone {
		t.Fatalf("expected predict misses < none misses, got %d vs %d", missPred, missNone)
	}
	if candUnion >= candNone {
		t.Fatalf("expected union candidates < none candidates, got %d vs %d", candUnion, candNone)
	}
	if candPred <= candUnion {
		t.Fatalf("expected predict candidates > union candidates, got %d vs %d", candPred, candUnion)
	}
}

// After a prediction failure the query margin must grow, and it must
// relax back once frames are clean again.
func TestAdaptiveExpansion(t *testing.T) {
	p := NewPredictStrategy()
	base := p.margin
	p.Observe(nil, 3)
	grown := p.margin
	if grown <= base {
		t.Fatalf("margin did not expand after a miss: %v -> %v", base, grown)
	}
	for i := 0; i < 40; i++ {
		p.Observe(nil, 0)
	}
	if p.margin != p.minMarg {
		t.Fatalf("margin did not relax to minimum: %v (min %v)", p.margin, p.minMarg)
	}
}
