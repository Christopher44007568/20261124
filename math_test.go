package headpose

import (
	"math"
	"testing"
)

func TestFrameContracts(t *testing.T) {
	tests := []struct {
		frame Frame
		unit  Unit
	}{
		{World, Meter},
		{Device, Meter},
		{Camera, Meter},
		{Screen, Pixel},
	}
	for _, test := range tests {
		spec, ok := test.frame.Spec()
		if !ok {
			t.Fatalf("missing frame spec for %s", test.frame)
		}
		if spec.Unit != test.unit {
			t.Errorf("%s unit = %s, want %s", test.frame, spec.Unit, test.unit)
		}
		if spec.Handedness != RightHanded {
			t.Errorf("%s handedness = %s, want %s", test.frame, spec.Handedness, RightHanded)
		}
		if spec.QuatOrder != ScalarFirstQuaternionOrder {
			t.Errorf("%s quaternion order = %s", test.frame, spec.QuatOrder)
		}
		if spec.XAxis == "" || spec.YAxis == "" || spec.ZAxis == "" {
			t.Errorf("%s has incomplete axis declaration", test.frame)
		}
	}
	if len(FrameSpecs()) != 4 {
		t.Fatalf("got %d frame specs, want 4", len(FrameSpecs()))
	}
}

func TestQuatRightHandedQuarterTurns(t *testing.T) {
	tests := []struct {
		name string
		axis Vec3
		in   Vec3
		want Vec3
	}{
		{"X: +Y->+Z", Vec3{X: 1}, Vec3{Y: 1}, Vec3{Z: 1}},
		{"X: +Z->-Y", Vec3{X: 1}, Vec3{Z: 1}, Vec3{Y: -1}},
		{"Y: +Z->+X", Vec3{Y: 1}, Vec3{Z: 1}, Vec3{X: 1}},
		{"Y: +X->-Z", Vec3{Y: 1}, Vec3{X: 1}, Vec3{Z: -1}},
		{"Z: +X->+Y", Vec3{Z: 1}, Vec3{X: 1}, Vec3{Y: 1}},
		{"Z: +Y->-X", Vec3{Z: 1}, Vec3{Y: 1}, Vec3{X: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireVec(t, mustQuat(t, test.axis, 90).Rotate(test.in), test.want, 1e-12)
		})
	}
}

func TestQuatMultiplicationOrder(t *testing.T) {
	rotX := mustQuat(t, Vec3{X: 1}, 90)
	rotY := mustQuat(t, Vec3{Y: 1}, 90)
	point := Vec3{X: 1}

	sequential := rotY.Rotate(rotX.Rotate(point))
	composed := rotY.Mul(rotX).Rotate(point)
	reversed := rotX.Mul(rotY).Rotate(point)

	requireVec(t, composed, sequential, 1e-12)
	requireVec(t, composed, Vec3{Z: -1}, 1e-12)
	requireVec(t, reversed, Vec3{Y: 1}, 1e-12)
}

func TestQuatFromBasisAndNegativeSign(t *testing.T) {
	basis, err := QuatFromBasis(
		Vec3{Z: -1},
		Vec3{Y: 1},
		Vec3{X: 1},
	)
	if err != nil {
		t.Fatalf("QuatFromBasis: %v", err)
	}
	want := mustQuat(t, Vec3{Y: 1}, 90)
	if !basis.SameRotation(want, 1e-12) {
		t.Fatalf("basis quaternion %+v does not match Y-quarter-turn %+v", basis, want)
	}
	if !basis.SameRotation(basis.Negate(), 1e-12) {
		t.Fatal("q and -q must represent the same rotation")
	}
	requireVec(t, basis.Negate().Rotate(Vec3{X: 1}), Vec3{Z: -1}, 1e-12)
}

func TestQuatFromBasisRejectsLeftHandedBasis(t *testing.T) {
	_, err := QuatFromBasis(Vec3{X: 1}, Vec3{Y: 1}, Vec3{Z: -1})
	if err == nil {
		t.Fatal("expected a left-handed basis to be rejected")
	}
}

func TestQuatFromBasisCoversAllExtractionBranches(t *testing.T) {
	tests := []struct {
		name string
		axis Vec3
		deg  float64
	}{
		{"identity", Vec3{X: 1}, 0},
		{"X 180", Vec3{X: 1}, 180},
		{"Y 180", Vec3{Y: 1}, 180},
		{"Z 180", Vec3{Z: 1}, 180},
		{"mixed", Vec3{X: 1, Y: 2, Z: 3}, 73},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := mustQuat(t, test.axis, test.deg)
			got, err := QuatFromBasis(
				want.Rotate(Vec3{X: 1}),
				want.Rotate(Vec3{Y: 1}),
				want.Rotate(Vec3{Z: 1}),
			)
			if err != nil {
				t.Fatalf("QuatFromBasis: %v", err)
			}
			if !got.SameRotation(want, 1e-12) {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestQuatNormalizationAndFiniteChecks(t *testing.T) {
	q, err := QuatFromComponents(2, 0, 0, 0)
	if err != nil {
		t.Fatalf("QuatFromComponents: %v", err)
	}
	if math.Abs(q.W-1) > 1e-12 {
		t.Fatalf("normalized quaternion = %+v", q)
	}
	if _, err := QuatFromComponents(0, 0, 0, 0); err == nil {
		t.Fatal("expected zero quaternion to fail")
	}
	if _, err := QuatFromAxisAngle(Vec3{}, 1); err == nil {
		t.Fatal("expected zero axis to fail")
	}
}
