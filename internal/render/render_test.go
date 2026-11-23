package render

import (
	"math"
	"testing"

	"stereoval/internal/geo"
	"stereoval/internal/scene"
)

func testCamera() (geo.Intrinsics, geo.Pose) {
	k := geo.Intrinsics{W: 320, H: 240, Fx: 360, Fy: 360, Cx: 160, Cy: 120}
	head := geo.HeadSample{Rot: geo.RotYX(0, 0)}
	pose := head.EyePose(geo.Rig{Baseline: 0.12}, geo.Left)
	return k, pose
}

func TestRenderFullyCovered(t *testing.T) {
	sc := scene.New()
	k, pose := testCamera()
	f := Render(sc, k, pose)
	holes := 0
	for _, g := range f.Gray {
		if math.IsNaN(g) {
			holes++
		}
	}
	if holes != 0 {
		t.Fatalf("background wall leaves %d holes", holes)
	}
}

func TestZeroMotionWarpHasNoHolesAndIsClose(t *testing.T) {
	sc := scene.New()
	k, pose := testCamera()
	gt := Render(sc, k, pose)
	wp := ForwardWarpDepth(gt, gt, k, pose, pose, "v0")
	var sum, n float64
	borderHoles := 0
	for y := 0; y < k.H; y++ {
		for x := 0; x < k.W; x++ {
			i := y*k.W + x
			if wp.Hole[i] {
				// A 1px image border cannot be bilinearly sampled; allow it.
				if x == 0 || y == 0 || x == k.W-1 || y == k.H-1 {
					borderHoles++
					continue
				}
				t.Fatalf("zero-motion warp produced an interior hole at pixel %d", i)
			}
			sum += math.Abs(wp.Gray[i] - gt.Gray[i])
			n++
		}
	}
	if borderHoles > 2*(k.W+k.H) {
		t.Fatalf("too many border holes: %d", borderHoles)
	}
	if mean := sum / n; mean > 6 {
		t.Fatalf("zero-motion mean gray diff %v too large", mean)
	}
}

func TestMotionOpensHoles(t *testing.T) {
	sc := scene.New()
	k, pose := testCamera()
	src := Render(sc, k, pose)
	moved := pose
	moved.C = geo.Add(pose.C, geo.Vec3{X: 0.4})
	dstGeom := Render(sc, k, moved)
	wp := ForwardWarpDepth(src, dstGeom, k, pose, moved, "v1")
	if wp.HoleRatio() <= 0 {
		t.Fatalf("expected disocclusion holes under lateral motion, got 0")
	}
}
