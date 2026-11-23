package stereoholes

import "testing"

func demoFixture(t *testing.T) (*Frame, *Frame, *HoleSet) {
	t.Helper()
	rig := NewStereoRig(480, 320, 0.36, 46)
	scene := NewDemoScene()
	target := scene.Render(rig.Left)
	source := scene.Render(rig.Right)
	return target, source, DemoHoles(target)
}

func groupMeasure(ref *Frame, res FillResult, h *HoleSet, name string) ModeMetrics {
	for i := range h.Groups {
		if h.Groups[i].Name == name {
			return measure(ref, res, h.Groups[i].Mask, 0.08)
		}
	}
	return ModeMetrics{}
}

func TestGuardedFillHasNoAcceptedGhosts(t *testing.T) {
	target, source, holes := demoFixture(t)
	res := CrossEyeFill(target, source, holes.Mask, DefaultFillOptions())
	if res.Stats.Filled == 0 {
		t.Fatal("guarded fill accepted nothing")
	}
	for i, isHole := range holes.Mask {
		if !isHole || res.Status[i] != StatusFilled {
			continue
		}
		if err := colorError(res.RGB[i], target.RGB[i]); err > 0.08 {
			t.Fatalf("accepted ghost at (%d,%d), error %.4f", i%target.Width, i/target.Width, err)
		}
	}
}

func TestGuardRejectsEachSingleEyeFailure(t *testing.T) {
	target, source, holes := demoFixture(t)
	res := CrossEyeFill(target, source, holes.Mask, DefaultFillOptions())
	tests := []struct {
		group  string
		reason RejectReason
	}{
		{"wall-corner", ReasonMaskSurface},
		{"specular-only", ReasonMaskSurface},
		{"transparent-only", ReasonSourceTracking},
		{"asymmetric-mask", ReasonNoOverlap},
	}
	for _, tc := range tests {
		m := groupMeasure(target, res, holes, tc.group)
		if m.RejectedByReason[string(tc.reason)] == 0 {
			t.Errorf("%s did not report %q: %v", tc.group, tc.reason, m.RejectedByReason)
		}
	}
}

func TestSharedSurfaceIsFilledNearlyExactly(t *testing.T) {
	target, source, holes := demoFixture(t)
	res := CrossEyeFill(target, source, holes.Mask, DefaultFillOptions())
	m := groupMeasure(target, res, holes, "shared-surface")
	if m.Holes == 0 || m.FillRate < 0.95 {
		t.Fatalf("shared surface fill rate = %.3f (%d/%d)", m.FillRate, m.Filled, m.Holes)
	}
	if m.MeanError > 0.01 {
		t.Fatalf("shared surface mean error = %.5f", m.MeanError)
	}
}

func TestUnguardedWarpGhostsMoreThanGuarded(t *testing.T) {
	target, source, holes := demoFixture(t)
	guarded := CrossEyeFill(target, source, holes.Mask, DefaultFillOptions())
	naive := UnguardedCrossEyeFill(target, source, holes.Mask)
	gm := measure(target, guarded, holes.Mask, 0.08)
	nm := measure(target, naive, holes.Mask, 0.08)
	if gm.BadFilled != 0 {
		t.Fatalf("guarded fill has %d bad pixels", gm.BadFilled)
	}
	if nm.BadFilled <= 0 || nm.GhostRate <= gm.GhostRate {
		t.Fatalf("unguarded ghost rate %.3f is not above guarded %.3f",
			nm.GhostRate, gm.GhostRate)
	}
}

func TestNoCrossEyeFillsFromTargetOnly(t *testing.T) {
	target, _, holes := demoFixture(t)
	res := NoCrossEyeFill(target, holes.Mask)
	if res.Stats.HolePixels != res.Stats.Filled {
		t.Fatalf("same-eye fill covered %d of %d holes",
			res.Stats.Filled, res.Stats.HolePixels)
	}
}
