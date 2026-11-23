package geom

import (
	"math"
	"testing"
)

func TestYawRotatesForwardAxisToRight(t *testing.T) {
	q := QuatFromEuler(math.Pi/2, 0, 0)
	got := q.Rotate(Vec3{Z: 1})
	want := Vec3{X: 1}
	if got.Sub(want).Len() > 1e-9 {
		t.Fatalf("rotated vector = %+v, want %+v", got, want)
	}
}

func TestSlerpHalfAngle(t *testing.T) {
	a := QuatIdentity()
	b := QuatFromEuler(math.Pi/2, 0, 0)
	mid := Slerp(a, b, 0.5)
	if got, want := AngleDeg(a, mid), 45.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("half-angle = %.12f, want %.12f", got, want)
	}
	if got, want := AngleDeg(mid, b), 45.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("second half-angle = %.12f, want %.12f", got, want)
	}
}

func TestSlerpUsesShortestArc(t *testing.T) {
	a := QuatIdentity()
	b := Quat{W: -1}
	got := AngleDeg(a, Slerp(a, b, 0.5))
	if got > 1e-9 {
		t.Fatalf("angle to midpoint = %g, want 0", got)
	}
}
