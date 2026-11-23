package headpose

import (
	"math"
	"testing"
)

func TestCameraToScreenUnitAxesAndRoundTrip(t *testing.T) {
	transform, err := CameraToScreenTransform(ScreenParams{
		PixelsPerMeter: 1200,
		PrincipalX:     640,
		PrincipalY:     360,
	})
	if err != nil {
		t.Fatalf("CameraToScreenTransform: %v", err)
	}
	cameraPoint := Vec3{X: 1, Y: 2, Z: -3}
	screenPoint := transform.ApplyPoint(cameraPoint)
	requireVec(t, screenPoint, Vec3{X: 1840, Y: -2040, Z: 3600}, 1e-9)
	requireVec(t, transform.Inverse().ApplyPoint(screenPoint), cameraPoint, 1e-9)

	requireVec(t, transform.ApplyDirection(Vec3{X: 1}), Vec3{X: 1}, 1e-12)
	requireVec(t, transform.ApplyDirection(Vec3{Y: 1}), Vec3{Y: -1}, 1e-12)
	requireVec(t, transform.ApplyDirection(Vec3{Z: 1}), Vec3{Z: -1}, 1e-12)
	if transform.Determinant() <= 0 {
		t.Fatal("screen mapping must preserve a positive determinant")
	}
}

func TestNinetyDegreeAxisMismatch(t *testing.T) {
	scene := testScene(t)
	tests := []struct {
		name string
		from Frame
		to   Frame
		in   Vec3
		want Vec3
	}{
		{"device +X -> world -Z", Device, World, Vec3{X: 1}, Vec3{Z: -1}},
		{"device +Y -> world +Y", Device, World, Vec3{Y: 1}, Vec3{Y: 1}},
		{"device +Z -> world +X", Device, World, Vec3{Z: 1}, Vec3{X: 1}},
		{"world +X -> device +Z", World, Device, Vec3{X: 1}, Vec3{Z: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := scene.ConvertDirection(test.from, test.to, test.in)
			if err != nil {
				t.Fatalf("ConvertDirection: %v", err)
			}
			requireVec(t, got, test.want, 1e-12)
		})
	}
}

func TestAllOrderedPairsRoundTrip(t *testing.T) {
	scene := testScene(t)
	samples := map[Frame]Vec3{
		World:  {X: 1.1, Y: 1.75, Z: -2.2},
		Device: {X: -0.3, Y: 0.25, Z: 0.6},
		Camera: {X: 0.15, Y: -0.4, Z: -1.8},
		Screen: {X: 980, Y: 720, Z: 1600},
	}
	for _, from := range FrameOrder {
		for _, to := range FrameOrder {
			result, err := scene.RoundTripPoint(from, to, samples[from], 1e-8)
			if err != nil {
				t.Fatalf("%s -> %s: %v", from, to, err)
			}
			if !result.Pass {
				t.Errorf("%s -> %s error %g exceeds tolerance", from, to, result.MaxError)
			}
		}
	}
}

func TestNestedTransformsMatchDirectPath(t *testing.T) {
	scene := testScene(t)
	graph := scene.Graph()
	direct, err := graph.Transform(Device, Screen)
	if err != nil {
		t.Fatalf("direct transform: %v", err)
	}
	deviceToWorld := scene.DevicePoseInWorld()
	worldToCamera := scene.CameraPoseInWorld().Inverse()
	cameraToScreen, err := CameraToScreenTransform(scene.Screen())
	if err != nil {
		t.Fatalf("camera-to-screen: %v", err)
	}
	nested := deviceToWorld.Compose(worldToCamera).Compose(cameraToScreen)
	if !direct.AlmostEqual(nested, 1e-10) {
		t.Fatalf("direct %+v != nested %+v", direct, nested)
	}
	if math.Abs(direct.Scale()-1200) > 1e-9 || math.Abs(nested.Scale()-1200) > 1e-9 {
		t.Fatalf("device-to-screen scales = %g, %g", direct.Scale(), nested.Scale())
	}

	path, err := graph.Path(Device, Screen)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	wantPath := []Frame{Device, World, Camera, Screen}
	if len(path) != len(wantPath) {
		t.Fatalf("path = %v, want %v", path, wantPath)
	}
	for i := range path {
		if path[i] != wantPath[i] {
			t.Fatalf("path = %v, want %v", path, wantPath)
		}
	}

	point := Vec3{X: 0.2, Y: -0.1, Z: 0.5}
	stepwise := cameraToScreen.ApplyPoint(worldToCamera.ApplyPoint(deviceToWorld.ApplyPoint(point)))
	requireVec(t, direct.ApplyPoint(point), stepwise, 1e-9)
}

func TestSceneScaleIsReciprocalAcrossScreen(t *testing.T) {
	scene := testScene(t)
	toScreen, err := scene.Graph().Transform(World, Screen)
	if err != nil {
		t.Fatalf("world-to-screen: %v", err)
	}
	fromScreen, err := scene.Graph().Transform(Screen, World)
	if err != nil {
		t.Fatalf("screen-to-world: %v", err)
	}
	if math.Abs(toScreen.Scale()-1200) > 1e-12 {
		t.Fatalf("world-to-screen scale = %g", toScreen.Scale())
	}
	if math.Abs(fromScreen.Scale()-1.0/1200) > 1e-15 {
		t.Fatalf("screen-to-world scale = %g", fromScreen.Scale())
	}

	point := Vec3{X: 0.4, Y: 1.2, Z: -0.5}
	current := point
	for i := 0; i < 100; i++ {
		current, err = scene.ConvertPoint(World, Screen, current)
		if err != nil {
			t.Fatalf("forward conversion %d: %v", i, err)
		}
		current, err = scene.ConvertPoint(Screen, World, current)
		if err != nil {
			t.Fatalf("backward conversion %d: %v", i, err)
		}
	}
	requireVec(t, current, point, 1e-9)
}

func TestNegativeQuaternionSceneEquivalence(t *testing.T) {
	q := mustQuat(t, Vec3{X: 1, Y: 1}, 63)
	translation := Vec3{X: -0.2, Y: 1.55, Z: 0.7}
	camera := mustRigid(t, mustQuat(t, Vec3{Y: 1}, -10), Vec3{X: 0.1, Y: 1.6, Z: 0.2})
	params := ScreenParams{PixelsPerMeter: 900, PrincipalX: 320, PrincipalY: 240}

	positive := mustRigid(t, q, translation)
	negative := mustRigid(t, q.Negate(), translation)
	positiveScene, err := NewScene(positive, camera, params)
	if err != nil {
		t.Fatalf("positive scene: %v", err)
	}
	negativeScene, err := NewScene(negative, camera, params)
	if err != nil {
		t.Fatalf("negative scene: %v", err)
	}

	point := Vec3{X: 0.7, Y: -0.2, Z: 0.4}
	for _, to := range FrameOrder {
		gotPositive, err := positiveScene.ConvertPoint(Device, to, point)
		if err != nil {
			t.Fatalf("positive %s: %v", to, err)
		}
		gotNegative, err := negativeScene.ConvertPoint(Device, to, point)
		if err != nil {
			t.Fatalf("negative %s: %v", to, err)
		}
		requireVec(t, gotNegative, gotPositive, 1e-9)
	}
}

func TestSceneRejectsInvalidConfiguration(t *testing.T) {
	rigid := mustRigid(t, IdentityQuat(), Vec3{})
	scaled := mustSimilarity(t, IdentityQuat(), Vec3{}, 2)
	validScreen := ScreenParams{PixelsPerMeter: 1000, PrincipalX: 320, PrincipalY: 240}
	if _, err := NewScene(scaled, rigid, validScreen); err == nil {
		t.Error("expected scaled device pose to fail")
	}
	if _, err := NewScene(rigid, scaled, validScreen); err == nil {
		t.Error("expected scaled camera pose to fail")
	}
	for _, params := range []ScreenParams{
		{PixelsPerMeter: 0},
		{PixelsPerMeter: -1},
		{PixelsPerMeter: 1000, PrincipalX: math.NaN()},
		{PixelsPerMeter: 1000, PrincipalY: math.Inf(1)},
	} {
		if _, err := NewScene(rigid, rigid, params); err == nil {
			t.Errorf("expected screen params %+v to fail", params)
		}
	}
}
