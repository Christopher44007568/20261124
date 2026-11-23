package stereocull

import (
	"math"
	"testing"
)

func TestFrustumSphereClassification(t *testing.T) {
	cfg := StereoConfig{IPD: 0.064, Near: 0.1, Far: 10, VerticalFOV: 1.0, Aspect: 1.6}
	fr := BuildFrustum(cfg.LeftEye())

	cases := []struct {
		name   string
		center Vec3
		r      float64
		want   CullResult
	}{
		{"center depth", Vec3{Z: -1}, 0.05, CullInside},
		{"eye axis point", Vec3{X: 0.032, Z: -1}, 0.01, CullInside},
		{"behind camera", Vec3{Z: 1}, 0.01, CullOutside},
		{"beyond far", Vec3{Z: -11}, 0.01, CullOutside},
		{"straddle near", Vec3{Z: -0.09}, 0.02, CullIntersects},
		{"straddle far", Vec3{Z: -9.99}, 0.05, CullIntersects},
		{"far off axis", Vec3{X: 50, Z: -1}, 0.01, CullOutside},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fr.TestSphere(tc.center, tc.r); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestUnionContainsBothEyes(t *testing.T) {
	cfg := StereoConfig{IPD: 0.22, Near: 0.1, Far: 10, VerticalFOV: 1.0, Aspect: 1.6}
	left := BuildFrustum(cfg.LeftEye())
	right := BuildFrustum(cfg.RightEye())
	union := BuildUnionFrustum(cfg.LeftEye(), cfg.RightEye())

	// Dense sample points inside either eye frustum must never be rejected
	// by the union hull.
	for d := cfg.Near * 1.02; d < cfg.Far*0.99; d += 0.07 {
		halfW, halfH := cfg.HalfExtents()
		hw := halfW * d / cfg.Near
		hh := halfH * d / cfg.Near
		for _, eyeX := range []float64{+cfg.IPD / 2, -cfg.IPD / 2} {
			for gx := -0.95; gx <= 0.95; gx += 0.19 {
				for gy := -0.95; gy <= 0.95; gy += 0.19 {
					p := Vec3{X: eyeX + gx*hw, Y: gy * hh, Z: -d}
					if union.TestSphere(p, 1e-6) == CullOutside {
						t.Fatalf("union rejects eye-visible point %+v", p)
					}
				}
			}
		}
		if left.TestSphere(Vec3{X: cfg.IPD / 2, Z: -d}, 0.001) == CullOutside {
			t.Fatalf("left frustum broken at depth %v", d)
		}
		if right.TestSphere(Vec3{X: -cfg.IPD / 2, Z: -d}, 0.001) == CullOutside {
			t.Fatalf("right frustum broken at depth %v", d)
		}
	}
}

func TestTransformPreservesDistances(t *testing.T) {
	cfg := StereoConfig{IPD: 0.064, Near: 0.1, Far: 10, VerticalFOV: 1.0, Aspect: 1.6}
	local := BuildFrustum(cfg.LeftEye())
	pose := HeadPose{
		Position: Vec3{X: 1.5, Y: -0.7, Z: 2.2},
		Rotation: QuatEuler(0.3, -0.9, 0.2),
	}
	world := local.Transform(pose.Rotation, pose.Position)

	localPoint := Vec3{X: 0.02, Y: -0.05, Z: -2}
	worldPoint := pose.Position.Add(pose.Rotation.Rotate(localPoint))
	for i := range local.Planes {
		got := world.Planes[i].Distance(worldPoint)
		want := local.Planes[i].Distance(localPoint)
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("plane %d distance drift: got %.6f want %.6f", i, got, want)
		}
	}
}

func TestQuaternionRotationBasics(t *testing.T) {
	q := QuatEuler(0, math.Pi/2, 0)
	got := q.Rotate(Vec3{X: 1})
	if math.Abs(got.Z+1) > 1e-12 || math.Abs(got.X) > 1e-12 || math.Abs(got.Y) > 1e-12 {
		t.Fatalf("+X rotated by +90deg yaw = %+v, want (0,0,-1)", got)
	}
}

func TestTransformedUnionStillContainsBothEyes(t *testing.T) {
	cfg := StereoConfig{IPD: 0.22, Near: 0.1, Far: 10, VerticalFOV: 1.0, Aspect: 1.6}
	pose := HeadPose{
		Position: Vec3{X: 0.8, Y: -1.1, Z: 2.4},
		Rotation: QuatEuler(-0.3, 0.7, 0.15),
	}
	union := BuildUnionFrustum(cfg.LeftEye(), cfg.RightEye()).
		Transform(pose.Rotation, pose.Position)
	left := BuildFrustum(cfg.LeftEye()).Transform(pose.Rotation, pose.Position)
	right := BuildFrustum(cfg.RightEye()).Transform(pose.Rotation, pose.Position)

	halfW, halfH := cfg.HalfExtents()
	for d := cfg.Near * 1.02; d < cfg.Far*0.99; d += 0.25 {
		hw, hh := halfW*d/cfg.Near, halfH*d/cfg.Near
		for _, eyeX := range []float64{cfg.IPD / 2, -cfg.IPD / 2} {
			for gx := -0.9; gx <= 0.9; gx += 0.45 {
				for gy := -0.9; gy <= 0.9; gy += 0.45 {
					local := Vec3{X: eyeX + gx*hw, Y: gy * hh, Z: -d}
					world := pose.Position.Add(pose.Rotation.Rotate(local))
					if union.TestSphere(world, 1e-6) == CullOutside {
						t.Fatalf("transformed union rejects eye-visible point %+v", world)
					}
				}
			}
		}
	}
	// Sanity: a world point far behind the head is rejected by all frusta.
	behind := pose.Position.Add(pose.Rotation.Rotate(Vec3{Z: 50}))
	if union.TestSphere(behind, 0.01) != CullOutside ||
		left.TestSphere(behind, 0.01) != CullOutside ||
		right.TestSphere(behind, 0.01) != CullOutside {
		t.Fatal("behind-head point should be outside every frustum")
	}
}
