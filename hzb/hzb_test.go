package hzb

import (
	"math"
	"testing"
)

func testCam(id string, pos Vec3, fwd Vec3) *Camera {
	return &Camera{
		ID: id, Position: pos, Forward: fwd, Up: Vec3{0, 1, 0},
		FovYDeg: 60, Near: 0.1, Far: 100, Width: 64, Height: 64,
	}
}

func boxAt(cx, cy, cz, hx, hy, hz float64) AABB {
	return AABB{
		Min: Vec3{cx - hx, cy - hy, cz - hz},
		Max: Vec3{cx + hx, cy + hy, cz + hz},
	}
}

func TestDownsampleMaxPool(t *testing.T) {
	db := NewDepthBuffer(4, 4)
	db.Depth[0] = 5     // (0,0)
	db.Depth[3*4+3] = 7 // (3,3)
	db.Depth[1*4+2] = 3 // (2,1)
	p := NewPyramid(db, "c", 0, 0)
	if len(p.Levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(p.Levels))
	}
	l1 := p.Levels[1]
	if got := l1.Data[0]; got != 5 {
		t.Errorf("l1(0,0) = %v, want 5", got)
	}
	if got := l1.Data[1*2+1]; got != 7 {
		t.Errorf("l1(1,1) = %v, want 7", got)
	}
	if got := l1.Data[0*2+1]; got != 3 {
		t.Errorf("l1(1,0) = %v, want 3", got)
	}
	if got := p.Levels[2].Data[0]; got != 7 {
		t.Errorf("l2(0,0) = %v, want 7 (global max)", got)
	}
}

func TestEmptyCellNeverOccludes(t *testing.T) {
	db := NewDepthBuffer(8, 8) // all +Inf
	p := NewPyramid(db, "c", 0, 0)
	for i, lv := range p.Levels[1:] {
		for j, v := range lv.Data {
			if v != 0 {
				t.Fatalf("level %d cell %d = %v, want 0 for empty", i+1, j, v)
			}
		}
	}
}

func TestQueryOccludedAndVisible(t *testing.T) {
	cam := testCam("c", Vec3{0, 0, 0}, Vec3{0, 0, 1})
	wall := BoxTriangles(Vec3{-1, -1, 4.9}, Vec3{1, 1, 5.1})
	db := RenderDepth(cam, wall)
	p := NewPyramid(db, "c", 0, 0)

	behind := boxAt(0, 0, 6.5, 0.3, 0.3, 0.3)
	if res := p.Query(cam, behind); !res.Occluded {
		t.Errorf("box fully behind wall should be occluded: %+v", res)
	}
	front := boxAt(0, 0, 4.0, 0.3, 0.3, 0.3)
	if res := p.Query(cam, front); res.Occluded {
		t.Errorf("box in front of wall should be visible: %+v", res)
	}
	aside := boxAt(3.0, 0, 6.5, 0.3, 0.3, 0.3)
	if res := p.Query(cam, aside); res.Occluded {
		t.Errorf("box beside wall should be visible: %+v", res)
	}
	behindCam := boxAt(0, 0, -5, 0.3, 0.3, 0.3)
	if res := p.Query(cam, behindCam); res.Occluded {
		t.Errorf("box behind camera must not be culled: %+v", res)
	}
}

func TestGroundTruth(t *testing.T) {
	cam := testCam("c", Vec3{0, 0, 0}, Vec3{0, 0, 1})
	wall := BoxTriangles(Vec3{-1, -1, 4.9}, Vec3{1, 1, 5.1})
	db := RenderDepth(cam, wall)

	if GroundTruthVisible(cam, db, boxAt(0, 0, 6.5, 0.3, 0.3, 0.3)) {
		t.Error("box behind wall should be ground-truth occluded")
	}
	if !GroundTruthVisible(cam, db, boxAt(3.0, 0, 6.5, 0.3, 0.3, 0.3)) {
		t.Error("box beside wall should be ground-truth visible")
	}
	if !GroundTruthVisible(cam, db, boxAt(0, 0, 4.0, 0.3, 0.3, 0.3)) {
		t.Error("box in front of wall should be ground-truth visible")
	}
}

func TestNearPlaneClipping(t *testing.T) {
	cam := testCam("c", Vec3{0, 0, 0}, Vec3{0, 0, 1})
	cam.Near = 1.0
	// Triangle straddling the near plane.
	tri := []Triangle{{Vec3{-1, -1, 0.5}, Vec3{1, -1, 0.5}, Vec3{0, 1, 3}}}
	db := RenderDepth(cam, tri)
	finite := 0
	for _, v := range db.Depth {
		if !math.IsInf(v, 1) {
			finite++
			if v < cam.Near-1e-9 {
				t.Fatalf("depth %v below near plane", v)
			}
		}
	}
	if finite == 0 {
		t.Fatal("clipped triangle should still cover pixels")
	}
}
