package gaze

import "testing"

func TestDeterministic(t *testing.T) {
	cfg := DefaultConfig()
	a := New(cfg, nil)
	b := New(cfg, nil)
	for i := 0; i < 500; i++ {
		sa, sb := a.Next(), b.Next()
		if sa != sb {
			t.Fatalf("frame %d diverged: %+v vs %+v", i, sa, sb)
		}
	}
}

func TestSaccadeReachesTarget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DriftAmpMM = 0
	cfg.MicroSaccMM = 0
	cfg.LossProb = 0
	target := Vec2{X: 3, Y: -2}
	g := New(cfg, []Event{{AtFrame: 10, Target: target, Duration: 5}})
	var last Sample
	for i := 0; i < 100; i++ {
		last = g.Next()
	}
	if d := hyp(last.Pupil.X-target.X, last.Pupil.Y-target.Y); d > 1e-9 {
		t.Fatalf("pupil did not settle on target: off by %v", d)
	}
}

func TestTrackingLossBursts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LossProb = 0.01
	g := New(cfg, nil)
	lost := 0
	for i := 0; i < 2000; i++ {
		if g.Next().Lost {
			lost++
		}
	}
	if lost == 0 {
		t.Fatal("expected some tracking-loss frames")
	}
	// With p=0.01 and bursts of 4-30 frames, expect roughly 17% loss.
	if lost > 800 {
		t.Fatalf("loss bursts too long: %d/2000 frames", lost)
	}
}

func hyp(x, y float64) float64 { return x*x + y*y }
