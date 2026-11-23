package relight

import (
	"math"
	"testing"
)

const testTolerance = 0.35

func testFrames(t *testing.T) (Scene, *Frame, *Frame, *Frame, Camera, Camera, DepthImage) {
	t.Helper()
	scene := BuildScene()
	oldCamera := OldCamera()
	currentCamera := CurrentCamera()
	oldFrame := RenderFrame(scene, oldCamera, OldLight())
	truth := RenderFrame(scene, currentCamera, CurrentLight())
	depth := DepthImage{Width: truth.Width, Height: truth.Height, Depth: truth.Depth}
	separated := ReprojectSeparated(scene, oldFrame, oldCamera, currentCamera, &depth, CurrentLight(), testTolerance)
	return scene, oldFrame, truth, separated, oldCamera, currentCamera, depth
}

func TestCameraRoundTripUsesLinearForwardDepth(t *testing.T) {
	camera := CurrentCamera()
	point := Vec3{X: 0.42, Y: 0.18, Z: 4.13}
	x, y, depth, ok := camera.Project(point)
	if !ok {
		t.Fatal("point should project inside the camera")
	}
	if depth <= camera.Near || depth >= camera.Far {
		t.Fatalf("depth %v is outside the linear camera range", depth)
	}
	reconstructed := camera.Unproject(x, y, depth)
	if !Approx(reconstructed, point, 1e-10) {
		t.Fatalf("round trip mismatch: got %+v want %+v", reconstructed, point)
	}
}

func TestDepthTestedWarpRejectsNearWallOcclusion(t *testing.T) {
	scene, oldFrame, truth, _, oldCamera, currentCamera, depth := testFrames(t)
	naive := ReprojectUnifiedWarp(scene, oldFrame, oldCamera, currentCamera, &depth, false, testTolerance)
	depthTested := ReprojectUnifiedWarp(scene, oldFrame, oldCamera, currentCamera, &depth, true, testTolerance)

	blockedPixels := 0
	for i, currentDepthValue := range truth.Depth {
		if currentDepthValue == 0 {
			continue
		}
		x := float64(i%truth.Width) + 0.5
		y := float64(i/truth.Width) + 0.5
		point := currentCamera.Unproject(x, y, currentDepthValue)
		origin := oldCamera.Pose.Pos
		direction := Normalize(Sub(point, origin))
		hit, ok := scene.Trace(origin, direction, oldCamera.Far)
		if !ok || Approx(hit.Point, point, testTolerance) {
			continue
		}
		blockedPixels++
		if naive.Depth[i] == 0 {
			t.Fatalf("naive warp should demonstrate a stale sample at pixel %d", i)
		}
		if depthTested.Depth[i] != 0 {
			t.Fatalf("depth-tested warp accepted an occluded receiver sample at pixel %d", i)
		}
	}
	if blockedPixels == 0 {
		t.Fatal("test scene did not contain a near-wall occlusion case")
	}
}

func TestShadowUsesCurrentLightNotHistory(t *testing.T) {
	scene, _, truth, separated, _, currentCamera, _ := testFrames(t)
	light := CurrentLight()
	lightCamera := LightCamera(light)
	shadowMap := RenderDepth(scene, lightCamera)
	currentShadowPixels := 0

	for i, shadow := range separated.Shadow {
		if shadow == (RGB{}) || truth.Depth[i] == 0 || truth.SurfaceID[i] != 0 {
			continue
		}
		pixelX := float64(i%truth.Width) + 0.5
		pixelY := float64(i/truth.Width) + 0.5
		origin, direction := currentCamera.Ray(pixelX, pixelY)
		hit, ok := scene.Trace(origin, direction, currentCamera.Far)
		if !ok || !inShadow(hit.Point, lightCamera, shadowMap) {
			t.Fatalf("pixel %d contains a shadow without current-light visibility evidence", i)
		}
		currentShadowPixels++
		if rgbDistance(separated.Shadow[i], truth.Shadow[i]) > 1e-10 {
			t.Fatalf("shadow mismatch: got %+v want %+v", separated.Shadow[i], truth.Shadow[i])
		}
	}
	if currentShadowPixels == 0 {
		t.Fatal("moving light scenario did not produce a current shadow pixel")
	}
}

func TestReflectionHistoryCannotIncreaseLuminance(t *testing.T) {
	_, oldFrame, truth, separated, oldCamera, currentCamera, _ := testFrames(t)
	if ReflectionLuminanceMax(separated) > ReflectionLuminanceMax(truth)+1e-10 {
		t.Fatal("separated reflection is brighter than the current reflection")
	}
	if ReflectionLuminanceMax(separated) >= ReflectionLuminanceMax(oldFrame) {
		t.Fatal("temporal clamp did not reduce the brighter historical reflection")
	}

	mirrorPixels := 0
	for i, value := range truth.Reflection {
		if MaxComponent(value) == 0 {
			continue
		}
		mirrorPixels++
		pixelX := float64(i%truth.Width) + 0.5
		pixelY := float64(i/truth.Width) + 0.5
		point := currentCamera.Unproject(pixelX, pixelY, truth.Depth[i])
		oldX, oldY, _, ok := oldCamera.Project(point)
		if !ok {
			t.Fatalf("mirror pixel %d is not visible in the old camera", i)
		}
		history, ok := SampleFrame(oldFrame, "reflection", oldX, oldY)
		if !ok {
			t.Fatalf("missing historical reflection for pixel %d", i)
		}
		expected := MinRGB(history, truth.Reflection[i])
		if rgbDistance(separated.Reflection[i], expected) > 1e-10 {
			t.Fatalf("reflection clamp mismatch at pixel %d: got %+v want %+v", i, separated.Reflection[i], expected)
		}
	}
	if mirrorPixels == 0 {
		t.Fatal("test scene did not contain a mirror reflection pixel")
	}
}

func TestSeparatedShadowErrorBeatsUnifiedWarp(t *testing.T) {
	scene, oldFrame, truth, separated, oldCamera, currentCamera, depth := testFrames(t)
	unified := ReprojectUnifiedWarp(scene, oldFrame, oldCamera, currentCamera, &depth, true, testTolerance)
	unifiedError := channelError(unified.Shadow, truth.Shadow)
	separatedError := channelError(separated.Shadow, truth.Shadow)
	if separatedError >= unifiedError {
		t.Fatalf("separated shadow error %v is not lower than unified warp %v", separatedError, unifiedError)
	}
}

func channelError(predicted, reference []RGB) float64 {
	total := 0.0
	for i := range predicted {
		total += math.Pow(rgbDistance(predicted[i], reference[i]), 2)
	}
	return math.Sqrt(total / float64(len(predicted)))
}
