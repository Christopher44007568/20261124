package geom

import (
	"math"
	"testing"
)

func TestProjectUnprojectRoundTrip(t *testing.T) {
	cam := Camera{
		Pos: V(0.3, -0.2, 1.0), Yaw: 0.25, Pitch: -0.1,
		Near: 0.1, Far: 20, FovY: 1.1, Width: 320, Height: 240,
	}
	world := V(-0.7, 0.4, 5.4)
	u, v, z, inside := cam.Project(world)
	if !inside {
		t.Fatalf("point should be inside the frustum")
	}
	center := cam.Unproject(math.Floor(u), math.Floor(v), z)
	back := cam.Unproject(math.Floor(u), math.Floor(v), z)
	u2, v2, z2, inside2 := cam.Project(center)
	if !inside2 || math.Floor(u2) != math.Floor(u) || math.Floor(v2) != math.Floor(v) ||
		math.Abs(z2-z) > 1e-9 || back.Sub(center).Length() > 1e-12 {
		t.Fatalf("project/unproject not invertible: u=%v->%v v=%v->%v z=%v->%v",
			u, u2, v, v2, z, z2)
	}
}

func TestProjectRejectsFrustum(t *testing.T) {
	cam := Camera{
		Pos: V(0, 0, 0), Near: 0.1, Far: 20, FovY: 1.0, Width: 100, Height: 100,
	}
	if _, _, _, inside := cam.Project(V(0, 0, -2)); inside {
		t.Fatal("point behind the camera must be outside")
	}
	if _, _, _, inside := cam.Project(V(50, 0, 2)); inside {
		t.Fatal("far-off-axis point must be outside")
	}
}
