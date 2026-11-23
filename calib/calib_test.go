package calib

import (
	"math"
	"testing"
)

const tolPx = 0.5 // half a pixel: anything above is a real error

func rigAndMarkers() (Rig, MarkerSet) {
	r := DefaultRig()
	return r, r.DefaultMarkers()
}

// A pure whole-image translation is fully explained by one fitted
// translation: the leftover must vanish for every marker family.
func TestPureTranslationIsRecoverable(t *testing.T) {
	r, m := rigAndMarkers()
	pert := Perturbation{Translation: Vec2{7.5, -3.25}}
	d := r.Diagnose(m.All(), pert, tolPx)
	if d.Kind != PureTranslation {
		t.Fatalf("kind = %v, want pure-translation", d.Kind)
	}
	if d.SharedLeftover > tolPx {
		t.Fatalf("leftover after translation fit = %.3f px, want <= %.1f",
			d.SharedLeftover, tolPx)
	}
	if got := d.SharedShift.Sub(pert.Translation).Len(); got > 1e-9 {
		t.Fatalf("recovered shift off by %.6f px", got)
	}
}

// Rotating the image around the panel center (not the lens center)
// must survive any single translation fit: the crosshair near the
// pivot looks fine, but the boundary markers carry a large residual.
func TestRotationCannotBeHiddenByTranslation(t *testing.T) {
	r, m := rigAndMarkers()
	pert := Perturbation{
		Rotation: 0.004, // ~0.23 deg
		Pivot:    r.Centers.Panel,
	}
	d := r.Diagnose(m.All(), pert, tolPx)
	if d.Kind != Rotation {
		t.Fatalf("kind = %v, want rotation-or-viewpoint", d.Kind)
	}
	if d.SharedLeftover <= tolPx {
		t.Fatalf("translation hid the rotation: leftover %.3f px", d.SharedLeftover)
	}
	// The crosshair sits close to the pivot, so judging by the
	// crosshair alone would wrongly accept the calibration.
	_, crossLeftover := FitTranslation(r.ObserveAll(m.Crosshair, Left, pert))
	if crossLeftover > tolPx {
		t.Fatalf("crosshair should look clean near pivot, got %.3f px", crossLeftover)
	}
	// Boundary markers must show the error even after the best fit.
	_, edgeLeftover := FitTranslation(r.ObserveAll(m.Boundary, Left, pert))
	if edgeLeftover <= tolPx {
		t.Fatalf("boundary markers failed to expose rotation: %.3f px", edgeLeftover)
	}
	t.Logf("crosshair leftover %.3f px vs boundary leftover %.3f px",
		crossLeftover, edgeLeftover)
}

// A viewpoint (eye position) error produces a 1/z-dependent pixel
// shift. Known-depth points must show a spread of residual
// magnitudes that no single translation can absorb.
func TestKnownDepthPointsExposeViewpointError(t *testing.T) {
	r, m := rigAndMarkers()
	pert := Perturbation{ViewpointShift: [2]Vec3{{X: 0.004}, {X: 0.004}}}
	res := r.ObserveAll(m.Depth, Left, pert)
	if got := DepthSensitivity(res); got <= tolPx {
		t.Fatalf("depth sensitivity %.3f px, want > %.1f", got, tolPx)
	}
	// The nearest point must move roughly 4x more than the farthest
	// (1 m vs 4 m), which is the parallax signature.
	near := res[0].Observed.Sub(res[0].Ideal).Len()
	far := res[2].Observed.Sub(res[2].Ideal).Len()
	if ratio := near / far; math.Abs(ratio-4) > 0.5 {
		t.Fatalf("near/far shift ratio = %.2f, want ~4", ratio)
	}
	d := r.Diagnose(m.All(), pert, tolPx)
	if d.Kind == PureTranslation {
		t.Fatalf("viewpoint error misclassified as pure translation")
	}
}

// With no perturbation the calibrated pipeline is exact: every
// residual and the binocular disparity error must vanish.
func TestNoPerturbationIsClean(t *testing.T) {
	r, m := rigAndMarkers()
	d := r.Diagnose(m.All(), Perturbation{}, tolPx)
	if d.Kind != None {
		t.Fatalf("kind = %v, want none (%+v)", d.Kind, d)
	}
}
