package headpose

import (
	"math"
	"testing"
)

func mustQuat(t *testing.T, axis Vec3, degrees float64) Quat {
	t.Helper()
	q, err := QuatFromAxisAngle(axis, degrees*math.Pi/180)
	if err != nil {
		t.Fatalf("QuatFromAxisAngle: %v", err)
	}
	return q
}

func mustRigid(t *testing.T, rotation Quat, translation Vec3) Transform {
	t.Helper()
	transform, err := NewRigid(rotation, translation)
	if err != nil {
		t.Fatalf("NewRigid: %v", err)
	}
	return transform
}

func mustSimilarity(t *testing.T, rotation Quat, translation Vec3, scale float64) Transform {
	t.Helper()
	transform, err := NewSimilarity(rotation, translation, scale)
	if err != nil {
		t.Fatalf("NewSimilarity: %v", err)
	}
	return transform
}

func requireVec(t *testing.T, got, want Vec3, tolerance float64) {
	t.Helper()
	if !got.AlmostEqual(want, tolerance) {
		t.Fatalf("vector mismatch: got %+v, want %+v (tol %g)", got, want, tolerance)
	}
}

func testScene(t *testing.T) *Scene {
	t.Helper()
	devicePose := mustRigid(t, mustQuat(t, Vec3{Y: 1}, 90), Vec3{X: 0.25, Y: 1.6, Z: 0.1})
	cameraPose := mustRigid(t, mustQuat(t, Vec3{Y: 1}, 20), Vec3{X: 0.4, Y: 1.65, Z: 0.3})
	scene, err := NewScene(devicePose, cameraPose, ScreenParams{
		PixelsPerMeter: 1200,
		PrincipalX:     640,
		PrincipalY:     360,
	})
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	return scene
}
