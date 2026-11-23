package stereo

import (
	"math"
	"testing"
)

func TestSynchronizedProjectionTracksIPD(t *testing.T) {
	point := Vec3{X: 10, Y: 15, Z: 800}
	var previous float64
	for _, ipd := range IPDValues {
		left := NewCamera(ipd, LeftEye, Synchronized)
		right := NewCamera(ipd, RightEye, Synchronized)
		if math.Abs(left.EyeCenter().X+ipd/2) > 1e-9 {
			t.Fatalf("IPD %.0f: left eye is %v", ipd, left.EyeCenter())
		}
		disparity, ok := left.ProjectedDisparity(point)
		if !ok {
			t.Fatalf("IPD %.0f: point did not project", ipd)
		}
		want := FocalPixels * ipd / point.Z
		if math.Abs(disparity-want) > 1e-9 {
			t.Fatalf("IPD %.0f: disparity %.6f, want %.6f", ipd, disparity, want)
		}
		if disparity <= previous {
			t.Fatalf("IPD %.0f: disparity did not increase", ipd)
		}
		previous = disparity
		if left.ViewportOffset() != 0 || right.ViewportOffset() != 0 {
			t.Fatal("synchronized camera must not use a fake viewport offset")
		}
		near := left.ProjectionOffsetAt(FrontPlateZ)
		far := left.ProjectionOffsetAt(WallZ)
		if math.Abs(near-far) < 1 {
			t.Fatalf("IPD %.0f: projection offset does not vary with depth", ipd)
		}
	}
}

func TestViewportOffsetIsDepthIndependent(t *testing.T) {
	camera := NewCamera(72, LeftEye, ViewportOnly)
	near := camera.ProjectionOffsetAt(FrontPlateZ)
	far := camera.ProjectionOffsetAt(WallZ)
	if near != far {
		t.Fatalf("failure camera offset changed with depth: near %.3f far %.3f", near, far)
	}
}

func TestOcclusionChangesWithIPD(t *testing.T) {
	delta := func(ipd float64) int {
		left := Render(NewCamera(ipd, LeftEye, Synchronized))
		right := Render(NewCamera(ipd, RightEye, Synchronized))
		return occlusionStats(left, right).BackVisibleDelta
	}
	d55 := delta(55)
	d72 := delta(72)
	if d55 <= 0 || d72 <= d55 {
		t.Fatalf("back-plate visibility deltas do not grow with IPD: 55=%d 72=%d", d55, d72)
	}
}

func TestViewportOnlyFailsAllDiagnostics(t *testing.T) {
	report, err := Run(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.FailureChecks) < 4 {
		t.Fatalf("expected at least 4 failure checks, got %d", len(report.FailureChecks))
	}
	for _, check := range report.FailureChecks {
		if !check.CorrectPass {
			t.Errorf("%s: synchronized model failed sanity check: %s", check.Name, check.CorrectDetail)
		}
		if !check.BrokenDetected {
			t.Errorf("%s: viewport-only failure was not detected: %s", check.Name, check.BrokenDetail)
		}
	}
	last := report.Runs[len(report.Runs)-1]
	if last.Match.P95AbsDepthErrorMM > 20 {
		t.Errorf("p95 depth error %.2f mm exceeds 20 mm", last.Match.P95AbsDepthErrorMM)
	}
}
