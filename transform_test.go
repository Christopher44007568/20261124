package headpose

import (
	"math"
	"testing"
)

func TestTransformApplyPointAndInverse(t *testing.T) {
	transform := mustSimilarity(t, mustQuat(t, Vec3{Y: 1}, 90), Vec3{X: 1, Y: 2, Z: 3}, 2.5)
	source := Vec3{X: 1, Y: -2, Z: 0.5}

	got := transform.ApplyPoint(source)
	want := Vec3{X: 2.25, Y: -3, Z: 0.5}
	requireVec(t, got, want, 1e-12)
	requireVec(t, transform.Inverse().ApplyPoint(got), source, 1e-12)
	if math.Abs(transform.Determinant()-math.Pow(2.5, 3)) > 1e-12 {
		t.Fatalf("determinant = %g", transform.Determinant())
	}
}

func TestTransformComposeMatchesSequentialApplication(t *testing.T) {
	first := mustSimilarity(t, mustQuat(t, Vec3{X: 1}, 35), Vec3{X: 1, Y: 2, Z: 3}, 2)
	second := mustSimilarity(t, mustQuat(t, Vec3{Z: 1}, -70), Vec3{X: -4, Y: 0.5, Z: 2}, 0.25)
	point := Vec3{X: 0.3, Y: -1.2, Z: 2.4}

	want := second.ApplyPoint(first.ApplyPoint(point))
	got := first.Compose(second).ApplyPoint(point)
	requireVec(t, got, want, 1e-12)
	if math.Abs(first.Compose(second).Scale()-0.5) > 1e-12 {
		t.Fatalf("composed scale = %g, want 0.5", first.Compose(second).Scale())
	}
}

func TestTransformRejectsInvalidScale(t *testing.T) {
	q := IdentityQuat()
	for _, scale := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := NewSimilarity(q, Vec3{}, scale); err == nil {
			t.Errorf("expected scale %g to fail", scale)
		}
	}
	if _, err := NewSimilarity(Quat{}, Vec3{}, 1); err == nil {
		t.Error("expected zero quaternion to fail")
	}
	if _, err := NewSimilarity(q, Vec3{X: math.NaN()}, 1); err == nil {
		t.Error("expected non-finite translation to fail")
	}
}

func TestTransformNegativeQuaternionIsEquivalent(t *testing.T) {
	q := mustQuat(t, Vec3{Y: 1, Z: 1}, 73)
	positive := mustSimilarity(t, q, Vec3{X: 0.3, Y: 1.4, Z: -2}, 1)
	negative := mustSimilarity(t, q.Negate(), Vec3{X: 0.3, Y: 1.4, Z: -2}, 1)
	point := Vec3{X: -0.2, Y: 0.7, Z: 0.9}

	requireVec(t, positive.ApplyPoint(point), negative.ApplyPoint(point), 1e-12)
	requireVec(t, negative.ApplyPoint(point), positive.ApplyPoint(point), 1e-12)
	if !positive.AlmostEqual(negative, 1e-12) {
		t.Fatal("transforms with q and -q must compare equal")
	}
}
