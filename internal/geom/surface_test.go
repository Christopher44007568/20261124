package geom

import "testing"

func cam() Camera { return Camera{Focal: 520, WidthPx: 480, HeightPx: 360} }

func TestCylinderSeamIsFrontCenter(t *testing.T) {
	s := &Surface{Kind: KindCylinder, Center: Vec3{Z: 5.7}, Radius: 2.4, HalfH: 1.95}
	o, d := cam().RayFor(240, 180)
	h, ok := s.Intersect(o, d)
	if !ok {
		t.Fatal("center ray misses the cylinder")
	}
	if h.U > 0.01 && h.U < 0.99 {
		t.Fatalf("front center must be the U=0/1 seam, got U=%v", h.U)
	}
}

func TestSphereFrontCenterAndPoles(t *testing.T) {
	s := &Surface{Kind: KindSphere, Center: Vec3{Z: 4.7}, Radius: 1.25}
	o, d := cam().RayFor(240, 180)
	h, ok := s.Intersect(o, d)
	if !ok {
		t.Fatal("center ray misses the sphere")
	}
	if h.U < 0.49 || h.U > 0.51 || h.V < 0.49 || h.V > 0.51 {
		t.Fatalf("front equator expected U=0.5,V=0.5, got U=%v V=%v", h.U, h.V)
	}
}

func TestNearestHitWinsAcrossLayers(t *testing.T) {
	plane := &Surface{ID: 0, Kind: KindPlane, Center: Vec3{Z: 6.6}, HalfW: 3, HalfH: 2}
	sphere := &Surface{ID: 2, Kind: KindSphere, Center: Vec3{Z: 4.7}, Radius: 1.25}
	o, d := cam().RayFor(240, 180)
	hp, okp := plane.Intersect(o, d)
	hs, oks := sphere.Intersect(o, d)
	if !okp || !oks {
		t.Fatal("setup ray must hit both layers")
	}
	if hs.T >= hp.T {
		t.Fatalf("sphere should be nearer: sphere T=%v plane T=%v", hs.T, hp.T)
	}
}
