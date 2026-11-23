package warp

import (
	"image/color"
	"math"
	"testing"

	"timewarp/geom"
)

func testSource(w, h int) *Source {
	s := &Source{
		W: w, H: h,
		Pix:        make([]color.RGBA, w*h),
		Depth:      make([]float64, w*h),
		DepthValid: make([]bool, w*h),
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			s.Pix[i] = color.RGBA{R: uint8(20 + x), G: uint8(40 + y), B: uint8(70 + (x+y)%100), A: 255}
			s.Depth[i] = 2
			s.DepthValid[i] = true
		}
	}
	return s
}

func TestRotationIdentityCopiesColorAndNeverWritesDepth(t *testing.T) {
	in := geom.NewIntrinsics(80, 48, 0.9)
	src := testSource(80, 48)
	pose := geom.NewPose(0.2, 0.1, geom.Vec3{X: 1, Y: 2, Z: 3})
	out := RotationWarp(src, in, pose, pose)
	for i := range src.Pix {
		if out.Pix[i] != src.Pix[i] {
			t.Fatalf("pixel %d = %v, want %v", i, out.Pix[i], src.Pix[i])
		}
		if out.DepthValid[i] || out.Depth[i] != 0 {
			t.Fatalf("rotation fabricated depth at %d", i)
		}
	}
	if out.Stats.Valid != 80*48 {
		t.Fatalf("valid = %d", out.Stats.Valid)
	}
}

func TestRotationIgnoresTranslationAndDepthBuffer(t *testing.T) {
	in := geom.NewIntrinsics(64, 36, 0.9)
	src := testSource(64, 36)
	src.Depth = nil
	src.DepthValid = nil
	from := geom.NewPose(0.1, 0, geom.Vec3{})
	toA := geom.NewPose(0.3, 0.02, geom.Vec3{})
	toB := geom.NewPose(0.3, 0.02, geom.Vec3{X: 5, Y: -2, Z: 8})
	a := RotationWarp(src, in, from, toA)
	b := RotationWarp(src, in, from, toB)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] || a.Reject[i] != b.Reject[i] {
			t.Fatalf("translation changed rotation warp at %d", i)
		}
	}
}

func TestRotationRejectsOutOfBounds(t *testing.T) {
	in := geom.NewIntrinsics(80, 48, 0.9)
	src := testSource(80, 48)
	from := geom.NewPose(0, 0, geom.Vec3{})
	to := geom.NewPose(1.5, 0, geom.Vec3{})
	out := RotationWarp(src, in, from, to)
	if out.Stats.OutOfBounds == 0 {
		t.Fatal("expected out-of-bounds rejections")
	}
	for i, why := range out.Reject {
		if why == RejectOutOfBounds && out.Pix[i] != (color.RGBA{}) {
			t.Fatalf("out-of-bounds pixel %d was filled", i)
		}
	}
	if out.Stats.Valid+out.Stats.OutOfBounds+out.Stats.BehindCamera != out.Stats.Evaluated {
		t.Fatalf("stats do not add up: %+v", out.Stats)
	}
}

func TestDepthWarpMissingDepthProducesHolesNotSamples(t *testing.T) {
	in := geom.NewIntrinsics(64, 36, 0.9)
	src := testSource(64, 36)
	for i := range src.DepthValid {
		src.DepthValid[i] = false
	}
	from := geom.NewPose(0, 0, geom.Vec3{})
	to := geom.NewPose(0, 0, geom.Vec3{Z: 0.5})
	out := DepthWarp(src, in, from, to)
	if out.Stats.Valid != 0 || out.Stats.OutOfBounds != 0 || out.Stats.NoCoverage != 64*36 {
		t.Fatalf("unexpected stats: %+v", out.Stats)
	}
	if out.Stats.SourceNoDepth != 64*36 {
		t.Fatalf("source missing depth = %d", out.Stats.SourceNoDepth)
	}
	for i, why := range out.Reject {
		if why != RejectNoCoverage {
			t.Fatalf("pixel %d reason %v", i, why)
		}
		if out.Pix[i] != (color.RGBA{}) || out.DepthValid[i] {
			t.Fatalf("missing-depth pixel %d was fabricated", i)
		}
	}
}

func TestDepthWarpFrontoParallelPlane(t *testing.T) {
	in := geom.NewIntrinsics(96, 54, 0.9)
	src := testSource(96, 54)
	for i := range src.Pix {
		src.Pix[i] = color.RGBA{R: 100, G: 150, B: 200, A: 255}
	}
	from := geom.NewPose(0, 0, geom.Vec3{})
	to := geom.NewPose(0, 0, geom.Vec3{Z: 0.2})
	out := DepthWarp(src, in, from, to)
	if out.Stats.OutOfBounds != 0 {
		t.Fatalf("out-of-bounds samples = %d", out.Stats.OutOfBounds)
	}
	if out.Stats.Coverage() < 0.98 {
		t.Fatalf("coverage = %f, stats %+v", out.Stats.Coverage(), out.Stats)
	}
	for i := range out.Pix {
		if !out.DepthValid[i] {
			continue
		}
		if math.Abs(out.Depth[i]-1.8) > 0.02 {
			t.Fatalf("depth[%d] = %g, want ~1.8", i, out.Depth[i])
		}
		if out.Pix[i] != (color.RGBA{R: 100, G: 150, B: 200, A: 255}) {
			t.Fatalf("color[%d] = %v", i, out.Pix[i])
		}
	}
}

func TestDepthWarpBehindCameraIsRejected(t *testing.T) {
	in := geom.NewIntrinsics(48, 32, 0.9)
	src := testSource(48, 32)
	from := geom.NewPose(0, 0, geom.Vec3{})
	to := geom.NewPose(0, 0, geom.Vec3{Z: 3})
	out := DepthWarp(src, in, from, to)
	if out.Stats.Valid != 0 || out.Stats.OutOfBounds != 0 {
		t.Fatalf("unexpected stats: %+v", out.Stats)
	}
	if out.Stats.SourceOffscreen == 0 {
		t.Fatal("expected source projections behind target camera")
	}
}

func TestBilinearSamplerNeverLeavesFrame(t *testing.T) {
	pix := make([]color.RGBA, 4*3)
	for i := range pix {
		pix[i] = color.RGBA{R: 255, A: 255}
	}
	for _, tc := range []struct{ u, v float64 }{
		{-1, 0}, {0, -1}, {4, 0}, {0, 3}, {math.NaN(), 1}, {1, math.NaN()},
		{math.Inf(-1), 1}, {1, math.Inf(1)},
	} {
		if _, ok := sampleBilinear(pix, 4, 3, tc.u, tc.v); ok {
			t.Fatalf("sampled outside frame at (%v,%v)", tc.u, tc.v)
		}
	}
	if _, ok := sampleBilinear(pix, 4, 3, 3, 2); !ok {
		t.Fatal("last pixel should be sampleable")
	}
}
