package geom

import (
	"math"
	"testing"
)

func TestBasisIsOrthonormal(t *testing.T) {
	m := Basis(0.37, -0.21)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			dot := m[r][0]*m[c][0] + m[r][1]*m[c][1] + m[r][2]*m[c][2]
			want := 0.0
			if r == c {
				want = 1
			}
			if math.Abs(dot-want) > 1e-12 {
				t.Fatalf("row %d dot row %d = %g, want %g", r, c, dot, want)
			}
		}
	}
}

func TestPoseRoundTrip(t *testing.T) {
	p := NewPose(0.4, -0.15, Vec3{X: 0.3, Y: -0.2, Z: 1.1})
	w := Vec3{X: -0.7, Y: 0.9, Z: 4.2}
	got := p.CamToWorld(p.WorldToCam(w))
	if got.Sub(w).Len() > 1e-12 {
		t.Fatalf("round trip = %+v, want %+v", got, w)
	}
}

func TestProjectUnproject(t *testing.T) {
	in := NewIntrinsics(640, 360, 0.9)
	p := Vec3{X: 0.42, Y: -0.18, Z: 2.3}
	u, v, z, ok := in.Project(p)
	if !ok || math.Abs(z-2.3) > 1e-12 {
		t.Fatalf("project = (%g,%g,%g,%v)", u, v, z, ok)
	}
	got := in.Unproject(u, v, z)
	if got.Sub(p).Len() > 1e-10 {
		t.Fatalf("unproject = %+v, want %+v", got, p)
	}
}

func TestInImageRejectsNonFinite(t *testing.T) {
	in := NewIntrinsics(32, 20, 1)
	if in.InImage(math.NaN(), 2) {
		t.Fatal("NaN u accepted")
	}
	if in.InImage(1, math.Inf(1)) {
		t.Fatal("infinite v accepted")
	}
	if !in.InImage(0, 0) || !in.InImage(31, 19) {
		t.Fatal("frame corners should be valid")
	}
}
