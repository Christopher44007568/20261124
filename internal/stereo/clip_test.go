package stereo

import (
	"math"
	"testing"
)

func testCamera(t *testing.T) Camera {
	t.Helper()
	cam, err := NewCamera(
		"test",
		Vec3{0, 0, 0},
		Vec3{0, 0, 1},
		Vec3{0, 1, 0},
		1,
		10,
		Frustum{Left: -0.5, Right: 0.5, Bottom: -0.5, Top: 0.5},
		100,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}
	return cam
}

func TestClipTriangleNearCases(t *testing.T) {
	cam := testCamera(t)
	tests := []struct {
		name       string
		tri        [3]Vec3
		wantVerts  int
		wantNear   int
		wantReject bool
	}{
		{
			name:      "fully in front",
			tri:       [3]Vec3{{-0.2, -0.2, 2}, {0.2, -0.2, 2}, {0, 0.2, 2}},
			wantVerts: 3,
		},
		{
			name:      "one vertex behind",
			tri:       [3]Vec3{{-0.2, -0.2, 2}, {0.2, -0.2, 2}, {0, 0.2, 0.5}},
			wantVerts: 4,
			wantNear:  2,
		},
		{
			name:      "two vertices behind",
			tri:       [3]Vec3{{-0.2, -0.2, 0.5}, {0.2, -0.2, 0.5}, {0, 0.2, 2}},
			wantVerts: 3,
			wantNear:  2,
		},
		{
			name:       "entirely behind",
			tri:        [3]Vec3{{-0.2, -0.2, 0.5}, {0.2, -0.2, 0.5}, {0, 0.2, 0.5}},
			wantReject: true,
		},
		{
			name:      "vertex exactly on plane",
			tri:       [3]Vec3{{-0.2, -0.2, 1}, {0.2, -0.2, 2}, {0, 0.2, 2}},
			wantVerts: 3,
			wantNear:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			poly := ClipTriangleNear(cam, tc.tri)
			if tc.wantReject {
				if len(poly) != 0 {
					t.Fatalf("got %d vertices, want rejection", len(poly))
				}
				return
			}
			if len(poly) != tc.wantVerts {
				t.Fatalf("got %d vertices, want %d", len(poly), tc.wantVerts)
			}
			nearCount := 0
			for _, v := range poly {
				if v.Near {
					if math.Abs(v.View.Z-cam.Near) > 1e-9 {
						t.Errorf("near-marked vertex z=%g, want near=%g", v.View.Z, cam.Near)
					}
					nearCount++
				}
			}
			if tc.wantVerts == 3 && tc.wantNear == 0 {
				for _, v := range poly {
					if v.Near {
						t.Errorf("unexpected near flag on front-facing triangle")
					}
				}
			}
			if nearCount != tc.wantNear {
				t.Fatalf("got %d near vertices, want %d", nearCount, tc.wantNear)
			}
		})
	}
}

func TestNearClipAreaIsReduced(t *testing.T) {
	cam := testCamera(t)
	tri := [3]Vec3{{-0.3, -0.2, 2}, {0.3, -0.2, 2}, {0, 0.2, 0.4}}
	poly := ClipTriangleNear(cam, tri)
	if len(poly) != 4 {
		t.Fatalf("got %d clipped vertices", len(poly))
	}
	world := make([]Vec3, len(poly))
	for i, v := range poly {
		world[i] = v.World
	}
	before := triangleArea3D(tri)
	after := Area3D(world)
	if !(after > 0 && after < before) {
		t.Fatalf("clipped area %g should be inside (0, %g)", after, before)
	}
}

func TestProjectionHandlesAsymmetricFrustum(t *testing.T) {
	cam, err := NewCamera(
		"asymmetric",
		Vec3{},
		Vec3{0, 0, 1},
		Vec3{0, 1, 0},
		0.1,
		10,
		Frustum{Left: -0.5, Right: 0.7, Bottom: -0.4, Top: 0.6},
		200,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}
	ndc := cam.ProjectView(Vec3{0, 0, 1})
	wantX := -(0.7 - 0.5) / (0.7 + 0.5)
	wantY := -(0.6 - 0.4) / (0.6 + 0.4)
	if math.Abs(ndc.X-wantX) > 1e-12 || math.Abs(ndc.Y-wantY) > 1e-12 {
		t.Fatalf("center ndc=(%g,%g), want (%g,%g)", ndc.X, ndc.Y, wantX, wantY)
	}
}

func TestScreenClipCoverage(t *testing.T) {
	cam := testCamera(t)
	poly := []ClipVertex{
		{View: Vec3{0, 0, 1}},
		{View: Vec3{1, 0, 1}},
		{View: Vec3{1, 1, 1}},
		{View: Vec3{0, 1, 1}},
	}
	// The viewport spans x/y -0.5..0.5 at z=1, so exactly one quarter of
	// this quad survives, or 50x50 pixels.
	screen := ProjectPolygon(cam, poly)
	area := Area2D(PixelPolygon(cam, screen))
	if math.Abs(area-2500) > 1e-6 {
		t.Fatalf("visible area=%g, want 2500", area)
	}
}
