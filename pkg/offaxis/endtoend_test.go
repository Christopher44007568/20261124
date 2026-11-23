package offaxis

import "testing"

func TestEndToEndOffAxisWideAspect(t *testing.T) {
	b := Bounds{Left: -2.4, Right: 2.0, Bottom: -0.6, Top: 0.8, Near: 1, Far: 20}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	// Every frustum corner must project onto its expected NDC box face.
	corners, err := b.FrustumCorners()
	if err != nil {
		t.Fatal(err)
	}
	planes, _ := b.AnalyticPlanes()
	for _, c := range corners {
		ndc := m.TransformPoint(c).NDC()
		for _, axis := range [3]float64{ndc.X, ndc.Y, ndc.Z} {
			if axis < -1-1e-9 || axis > 1+1e-9 {
				t.Fatalf("corner %+v projects outside NDC box: %+v", c, ndc)
			}
		}
		if !Inside(planes, c, 1e-9) {
			t.Fatalf("frustum corner reported outside its own frustum: %+v", c)
		}
	}

	vp := Viewport{Width: 28, Height: 8}
	fb := NewFramebuffer(vp)
	fb.DrawTriangle(m, vp,
		Vec3{X: -2, Y: -0.4, Z: -6},
		Vec3{X: 1.8, Y: -0.4, Z: -6},
		Vec3{X: 0, Y: 0.6, Z: -6})
	covered := 0
	for _, ok := range fb.Covered {
		if ok {
			covered++
		}
	}
	if covered == 0 {
		t.Fatal("wide-aspect triangle produced no fragments")
	}
	eyeAt := func(x, y int) (Vec3, bool) {
		ndcX := (float64(x)+0.5)/float64(vp.Width)*2 - 1
		ndcY := 1 - (float64(y)+0.5)/float64(vp.Height)*2
		return m.Unproject(ndcX, ndcY, 6)
	}
	cov, reconciled, maxErr := CompareFramebufferDepth(fb, eyeAt, planes)
	if cov == 0 || reconciled == 0 || maxErr > 1e-9 {
		t.Fatalf("depth reconciliation failed: cov=%d rec=%d err=%v", cov, reconciled, maxErr)
	}
}

func TestReversedAndDegenerateAreSentinelErrors(t *testing.T) {
	if _, err := (Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 5, Far: 5}).ProjectionMatrix(); err != ErrReversedDepth {
		t.Fatalf("equal near/far must be ErrReversedDepth, got %v", err)
	}
	if _, err := (Bounds{Left: 2, Right: 2, Bottom: -1, Top: 1, Near: 1, Far: 5}).ProjectionMatrix(); err != ErrDegenerate {
		t.Fatalf("zero aperture must be ErrDegenerate, got %v", err)
	}
}
