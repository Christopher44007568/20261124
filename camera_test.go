package stereoholes

import (
	"math"
	"testing"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestCameraProjectionRoundTrip(t *testing.T) {
	c := NewCamera(Vec3{-0.18, 0, 0}, Vec3{0, 0, 4}, 480, 320, 46)
	x, y, depth := 123.4, 210.7, 3.2
	p := c.Unproject(x, y, depth)
	gotX, gotY, gotDepth, ok := c.Project(p)
	if !ok {
		t.Fatal("projection rejected a point in front of the camera")
	}
	if !almostEqual(gotX, x, 1e-9) ||
		!almostEqual(gotY, y, 1e-9) ||
		!almostEqual(gotDepth, depth, 1e-9) {
		t.Fatalf("round trip = (%.6f, %.6f, %.6f), want (%.6f, %.6f, %.6f)",
			gotX, gotY, gotDepth, x, y, depth)
	}
}

func TestParallelStereoDisparity(t *testing.T) {
	rig := NewStereoRig(480, 320, 0.36, 46)
	p := Vec3{0, 0, 4}
	xl, _, _, okL := rig.Left.Project(p)
	xr, _, _, okR := rig.Right.Project(p)
	if !okL || !okR {
		t.Fatal("point should project into both eyes")
	}
	want := rig.Baseline * rig.Left.Focal / 4
	if got := xl - xr; !almostEqual(got, want, 1e-9) {
		t.Fatalf("disparity = %.6f, want %.6f", got, want)
	}
}
