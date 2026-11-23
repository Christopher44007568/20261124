package xr

import (
	"math"
	"testing"
)

func TestQuaternionRoundTrip(t *testing.T) {
	q := QuatFromAxisAngle(Vec3{0, 1, 0}, math.Pi/2)
	v := Vec3{X: 0.65, Y: 1.5, Z: 0.12}
	inv := Quat{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
	got := inv.Rotate(q.Rotate(v))
	if d := got.Sub(v).Norm(); d > 1e-12 {
		t.Fatalf("round trip drift: %v (norm %g)", got, d)
	}
}

func TestQuaternionYaw90(t *testing.T) {
	q := QuatFromAxisAngle(Vec3{0, 1, 0}, math.Pi/2)
	got := q.Rotate(Vec3{X: 1, Z: 0})
	want := Vec3{X: 0, Z: -1}
	if d := got.Sub(want).Norm(); d > 1e-12 {
		t.Fatalf("yaw 90 of (1,0,0) = %+v, want %+v", got, want)
	}
}

func TestRigidTransformRoundTrip(t *testing.T) {
	rt := RigidTransform{
		Q: QuatFromAxisAngle(Vec3{0, 1, 0}, math.Pi/2),
		T: Vec3{X: 6, Y: 0, Z: -2},
	}
	p := Pose{P: Vec3{X: 0.65, Y: 1.5, Z: 0.12}, Q: QuatFromAxisAngle(Vec3{0, 1, 0}, 0.3)}
	back := p.Transform(rt).Transform(rt.Invert())
	if d := PositionError(back, p); d > 1e-12 {
		t.Fatalf("rigid transform round trip drift %g", d)
	}
}

func TestRelocDeltaRoundTrip(t *testing.T) {
	old := IdentityTransform
	next := RigidTransform{
		Q: QuatFromAxisAngle(Vec3{0, 1, 0}, math.Pi/2),
		T: Vec3{X: 6, Y: 0, Z: -2},
	}
	delta := composeBInvA(next, old)
	poseA := Pose{P: Vec3{X: 0.65, Y: 1.5, Z: 0.12}}
	poseB := poseA.Transform(delta)
	// Re-projecting back through the inverse delta must recover the A-frame pose.
	back := poseB.Transform(delta.Invert())
	if d := PositionError(back, poseA); d > 1e-12 {
		t.Fatalf("anchor reproject drift %g", d)
	}
	// And the B-frame pose must equal the direct canonical -> B mapping.
	want := poseA.Transform(next)
	if d := PositionError(poseB, want); d > 1e-12 {
		t.Fatalf("B-frame pose %+v != direct mapping %+v (drift %g)", poseB.P, want.P, d)
	}
}
