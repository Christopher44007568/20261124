package offaxis

import "testing"

func TestPlaneDerivationsAgree(t *testing.T) {
	b := Bounds{Left: -2.1, Right: 0.4, Bottom: -0.8, Top: 1.3, Near: 1.5, Far: 20}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	diff, err := PlaneSetAgreement(b, m)
	if err != nil {
		t.Fatal(err)
	}
	if diff > 1e-9 {
		t.Fatalf("analytic and extracted planes diverge: %v", diff)
	}
}

func TestCrossCheckMasksAndDepth(t *testing.T) {
	b := Bounds{Left: -1.2, Right: 0.8, Bottom: -0.7, Top: 0.9, Near: 1, Far: 9}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	probes, err := ProbeGrid(m, b, 4, 13, 11)
	if err != nil {
		t.Fatal(err)
	}
	mismatches, depth := CrossCheck(probes)
	if len(mismatches) != 0 {
		t.Fatalf("pipeline disagreement: %+v", mismatches)
	}
	if depth.Count == 0 || depth.MaxAbsError > 1e-9 {
		t.Fatalf("depth reconciliation failed: %+v", depth)
	}
	// Overscan guarantees at least one outside probe on every side.
	outside := 0
	for _, p := range probes {
		if !p.Inside {
			outside++
		}
	}
	if outside == 0 {
		t.Fatal("expected outside probes from overscan")
	}
}

func TestRasterizerDepthMatchesAnalytic(t *testing.T) {
	b := Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 1, Far: 10}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	vp := Viewport{Width: 16, Height: 16}
	fb := NewFramebuffer(vp)
	// Two triangles forming a quad placed inside the frustum at z = -4.
	z := -4.0
	quad := [4]Vec3{{-0.5, -0.5, z}, {0.5, -0.5, z}, {0.5, 0.5, z}, {-0.5, 0.5, z}}
	fb.DrawTriangle(m, vp, quad[0], quad[1], quad[2])
	fb.DrawTriangle(m, vp, quad[0], quad[2], quad[3])
	planes, _ := b.AnalyticPlanes()
	eyeAt := func(x, y int) (Vec3, bool) {
		ndcX := (float64(x)+0.5-float64(vp.X))/float64(vp.Width)*2 - 1
		ndcY := 1 - (float64(y)+0.5-float64(vp.Y))/float64(vp.Height)*2
		return m.Unproject(ndcX, ndcY, 4)
	}
	covered, count, maxErr := CompareFramebufferDepth(fb, eyeAt, planes)
	if count == 0 {
		t.Fatal("no covered pixels reconciled")
	}
	if maxErr > 1e-9 {
		t.Fatalf("rasterized depth diverges: %v", maxErr)
	}
	if covered == 0 {
		t.Fatal("rasterizer produced no fragments")
	}
}

func TestNearPlanePenetrationIsClipped(t *testing.T) {
	b := Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 2, Far: 10}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	// Triangle straddles the near plane: one vertex behind the camera.
	behind := Vec3{X: -0.4, Y: 0.4, Z: 1.5}
	in1 := Vec3{X: -0.4, Y: -0.4, Z: -3}
	in2 := Vec3{X: 0.4, Y: -0.4, Z: -3}
	clip := MarkEdges(m.TransformPoint(behind))
	if clip&(1<<PlaneNear) == 0 {
		t.Fatal("behind-camera vertex must carry the near edge marker")
	}
	vp := Viewport{Width: 12, Height: 12}
	fb := NewFramebuffer(vp)
	fb.DrawTriangle(m, vp, behind, in1, in2)
	covered := 0
	for _, ok := range fb.Covered {
		if ok {
			covered++
		}
	}
	if covered == 0 {
		t.Fatal("clipped triangle should still rasterize a fragment")
	}
}

func TestStereoAcceptsParallelAndRejectsToeIn(t *testing.T) {
	// Parallel-axis: each eye-space center ray is exactly -Z; the head-space
	// separation is carried by the view translation alone. Asymmetric
	// (off-axis) bounds are exercised by the single-eye tests above.
	le := Eye{Name: "L", Bounds: Bounds{-1, 1, -1, 1, 1, 10},
		Pose: Pose{Position: Vec3{X: -0.063}}}
	re := Eye{Name: "R", Bounds: Bounds{-1, 1, -1, 1, 1, 10},
		Pose: Pose{Position: Vec3{X: 0.063}}}
	if _, _, err := BuildStereo(le, re); err != nil {
		t.Fatalf("parallel pair rejected: %v", err)
	}
	// Rotate the right eye inward: center rays converge and must be refused.
	yawed := Identity()
	yawed[0][0] = 0.999
	yawed[0][2] = 0.044
	yawed[2][0] = -0.044
	yawed[2][2] = 0.999
	re.Pose.Rotation = yawed
	if _, _, err := BuildStereo(le, re); err != ErrCenterRay {
		t.Fatalf("toe-in pair should fail with ErrCenterRay, got %v", err)
	}
}
