package tex

import "testing"

func TestSeamWrapsWithoutDoubleImage(t *testing.T) {
	m := NewMarker()
	// Marker centered on the U=0/1 seam, repeating in U, clamped in V.
	p := Placement{CenterU: 0, CenterV: 0.5, HalfU: 0.1, HalfV: 0.2,
		WrapU: WrapRepeat, WrapV: WrapClamp}

	hits := 0
	const steps = 4096
	for i := 0; i < steps; i++ {
		u := float64(i) / steps
		if _, _, _, a, ok := m.SamplePlacement(u, 0.5, p); ok && a > 0.5 {
			hits++
		}
	}
	// Footprint width is 2*HalfU of the period, plus one bilinear texel.
	wantMax := int(2*p.HalfU*steps) + 4
	if hits > wantMax {
		t.Fatalf("marker hit %d columns, want <= %d (duplicated across seam)", hits, wantMax)
	}
	if hits == 0 {
		t.Fatal("marker never sampled across the seam")
	}

	// The two sides of the seam must return the same opaque coverage.
	_, _, _, a0, ok0 := m.SamplePlacement(0.995, 0.5, p)
	_, _, _, a1, ok1 := m.SamplePlacement(0.005, 0.5, p)
	if !ok0 || !ok1 || a0 < 0.9 || a1 < 0.9 {
		t.Fatalf("seam discontinuity: a0=%v ok0=%v a1=%v ok1=%v", a0, ok0, a1, ok1)
	}
}

func TestClampStaysInside(t *testing.T) {
	m := NewMarker()
	p := Placement{CenterU: 0.5, CenterV: 0.5, HalfU: 0.1, HalfV: 0.2,
		WrapU: WrapClamp, WrapV: WrapClamp}
	if _, _, _, _, ok := m.SamplePlacement(0.01, 0.5, p); ok {
		t.Fatal("clamp placement leaked outside its footprint")
	}
	if _, _, _, _, ok := m.SamplePlacement(0.5, 0.5, p); !ok {
		t.Fatal("clamp placement missing at its own center")
	}
}

func TestPoleClampPaintDoesNotRepeat(t *testing.T) {
	m := NewMarker()
	p := Placement{CenterU: 0.5, CenterV: 0.9, HalfU: 0.1, HalfV: 0.12,
		WrapU: WrapRepeat, WrapV: WrapClampPaint}
	// Right at the edge the boundary texel is frozen, not wrapped.
	_, _, _, aEdge, okEdge := m.SamplePlacement(0.5, 1.001, p)
	if !okEdge || aEdge <= 0 {
		t.Fatal("clamp-paint should freeze the pole edge, got empty sample")
	}
	// Well above the marker footprint the sample is empty: no edge smear.
	if _, _, _, _, ok := m.SamplePlacement(0.5, 1.2, p); ok {
		t.Fatal("clamp-paint leaked far beyond the marker footprint")
	}
	// Longitude far from the marker center stays empty even near the pole.
	if _, _, _, _, ok := m.SamplePlacement(0.8, 0.95, p); ok {
		t.Fatal("longitude repeat produced a second marker copy near the pole")
	}
}
