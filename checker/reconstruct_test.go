package checker

import (
	"math"
	"testing"
)

// syntheticPeriodic builds a reconstruction whose peripheral error is a fixed
// 2:2 checkerboard term. Every periodicity probe must flag it; if a probe stays
// silent the test itself is broken.
func syntheticPeriodic(truth *Image, mask *Mask) *Image {
	rec := truth.Clone()
	for y := 0; y < truth.H; y++ {
		for x := 0; x < truth.W; x++ {
			idx := y*truth.W + x
			if mask.Shaded[idx] {
				continue
			}
			sign := 0.12
			if !phase0Shaded(x, y) {
				sign = -0.12
			}
			c := truth.Px[idx]
			rec.Px[idx] = RGB{
				R: clamp01(c.R + sign),
				G: clamp01(c.G + sign),
				B: clamp01(c.B + sign),
			}
		}
	}
	return rec
}

func TestProbesDetectSyntheticCheckerPeriod(t *testing.T) {
	truth := renderFBM(TestSize, TestSize)
	mask := BuildMask(TestSize, TestSize, TestCenter, 0)
	fg := func(c RGB) bool { return Luma(c) > 0.5 }
	rec := syntheticPeriodic(truth, mask)
	m := Evaluate(truth, rec, mask, fg)
	ac := math.Max(
		math.Abs(ErrorAutocorrelation(truth, rec, mask, 4, 0)),
		math.Abs(ErrorAutocorrelation(truth, rec, mask, 0, 4)))

	if m.CheckerEnergy < 0.9 {
		t.Fatalf("checker-energy probe failed to detect a fixed period: %.4f", m.CheckerEnergy)
	}
	if ac < 0.9 {
		t.Fatalf("period-autocorrelation probe failed to detect a fixed period: %.4f", ac)
	}
}

func TestReconstructionIsAperiodic(t *testing.T) {
	for _, sc := range Scenes() {
		m, _, _, _ := RunScene(sc)
		if !sc.Wellposed {
			continue
		}
		// The complementary shading phase must converge to the same image:
		// a fixed checkerboard term flips sign between phases and inflates this
		// difference, while content-driven reconstruction stays phase-stable.
		if m.PhaseDelta > 0.08 {
			t.Errorf("scene %s: phase delta %.4f reveals a checker period", sc.Name, m.PhaseDelta)
		}
		// On non-smooth content the error itself must not repeat at the block
		// period. Smooth analytic scenes are allowed a structural correlation
		// here because their error is a smooth field, which is why the phase
		// gate above is the primary periodicity criterion.
		if sc.Name != "analytic_blob" && sc.Name != "hue_sweep" && m.PeriodAC > 0.12 {
			t.Errorf("scene %s: period autocorrelation %.4f is too high", sc.Name, m.PeriodAC)
		}
	}
}

func TestNaiveBaselineIsPeriodic(t *testing.T) {
	// The deliberately-broken baseline must fail at least one periodicity
	// probe, proving the gates carry real discriminating power.
	failed := false
	for _, sc := range Scenes() {
		truth := sc.Render(TestSize, TestSize)
		mask := BuildMask(TestSize, TestSize, TestCenter, 0)
		rec := ReconstructNaive(Shade(truth, mask), mask)
		ac := math.Max(
			math.Abs(ErrorAutocorrelation(truth, rec, mask, 4, 0)),
			math.Abs(ErrorAutocorrelation(truth, rec, mask, 0, 4)))
		m := Evaluate(truth, rec, mask, sc.Foreground)
		if ac > 0.15 || m.CheckerEnergy > 0.3 {
			failed = true
		}
	}
	if !failed {
		t.Fatal("naive copy baseline was not flagged as periodic by any probe")
	}
}

func TestDeterminism(t *testing.T) {
	truth := renderFBM(TestSize, TestSize)
	mask := BuildMask(TestSize, TestSize, TestCenter, 0)
	first := Reconstruct(Shade(truth, mask), mask)
	second := Reconstruct(Shade(truth, mask), mask)
	for i := range first.Px {
		if first.Px[i] != second.Px[i] {
			t.Fatal("reconstruction must be deterministic across runs")
		}
	}
}
