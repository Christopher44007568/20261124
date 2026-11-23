package calib

import (
	"math"
	"testing"
)

// An offset applied to one eye only must show up as a binocular
// disparity error while the other eye stays clean.
func TestSingleEyeBias(t *testing.T) {
	r, m := rigAndMarkers()
	bias := Vec2{6, -2.5}
	for _, e := range Eyes() {
		var pert Perturbation
		pert.EyeBias[e] = bias
		d := r.Diagnose(m.All(), pert, tolPx)
		if d.Kind != EyeSpecificBias {
			t.Fatalf("%s-eye bias: kind = %v, want eye-specific-bias", e, d.Kind)
		}
		if got := math.Abs(d.Disparity - bias.Len()); got > 1e-9 {
			t.Fatalf("%s-eye bias: disparity error %.3f px, want %.3f",
				e, d.Disparity, bias.Len())
		}
		if d.PerEyeLeftover[e] > tolPx {
			t.Fatalf("%s-eye bias: biased eye not a clean translation: %.3f px",
				e, d.PerEyeLeftover[e])
		}
		other := Eye(1 - int(e))
		if d.PerEyeLeftover[other] > tolPx {
			t.Fatalf("%s-eye bias leaked into %s eye: %.3f px",
				e, other, d.PerEyeLeftover[other])
		}
	}
}

// Opposite offsets on the two eyes must not cancel in the disparity:
// the disparity error doubles while each eye alone looks like a
// clean, opposite translation.
func TestOppositeEyeBias(t *testing.T) {
	r, m := rigAndMarkers()
	bias := Vec2{4, 1.5}
	pert := Perturbation{EyeBias: [2]Vec2{bias, bias.Scale(-1)}}
	d := r.Diagnose(m.All(), pert, tolPx)
	if d.Kind != EyeSpecificBias {
		t.Fatalf("kind = %v, want eye-specific-bias", d.Kind)
	}
	want := 2 * bias.Len() // the shifts point oppositely, so they add
	if got := math.Abs(d.Disparity - want); got > 1e-9 {
		t.Fatalf("disparity error %.3f px, want %.3f", d.Disparity, want)
	}
	// A single shared translation cannot fix both eyes at once.
	if d.SharedLeftover <= tolPx {
		t.Fatalf("shared translation hid opposite biases: %.3f px", d.SharedLeftover)
	}
	for _, e := range Eyes() {
		if d.PerEyeLeftover[e] > tolPx {
			t.Fatalf("%s eye per-eye fit leftover %.3f px", e, d.PerEyeLeftover[e])
		}
	}
}

// Treating any center as any other center must always be judged an
// error: the reprojection error for every wrong assignment has to
// exceed the pixel tolerance for both eyes and every marker family.
func TestCenterConfusionAlwaysDetected(t *testing.T) {
	r, m := rigAndMarkers()
	for _, actual := range CenterKinds() {
		for _, assumed := range CenterKinds() {
			if assumed == actual {
				continue // the correct assignment, not a confusion
			}
			for _, e := range Eyes() {
				for name, pts := range map[string][]Vec3{
					"crosshair": m.Crosshair,
					"boundary":  m.Boundary,
					"depth":     m.Depth,
				} {
					got := r.ConfusionError(pts, e, assumed, actual)
					if got <= tolPx {
						t.Errorf("%s center used as %s center (%s eye, %s): "+
							"error %.3f px <= %.1f, confusion not detected",
							assumed, actual, e, name, got, tolPx)
					}
				}
			}
		}
	}
}

// The correct assignment (lens center as lens center) must of course
// be accepted, proving the detector is not just always firing.
func TestCorrectCenterAssignmentPasses(t *testing.T) {
	r, m := rigAndMarkers()
	for _, k := range CenterKinds() {
		for _, e := range Eyes() {
			if got := r.ConfusionError(m.All(), e, k, k); got > 1e-9 {
				t.Fatalf("%s eye: %s center self-assignment flagged, %.6f px",
					e, k, got)
			}
		}
	}
}
