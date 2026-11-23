package relight

import "math"

func ReprojectUnifiedWarp(scene Scene, oldFrame *Frame, oldCamera, currentCamera Camera, currentDepth *DepthImage, validateDepth bool, tolerance float64) *Frame {
	result := NewFrame(currentCamera.Width, currentCamera.Height)
	for y := 0; y < currentCamera.Height; y++ {
		for x := 0; x < currentCamera.Width; x++ {
			pixelX := float64(x) + 0.5
			pixelY := float64(y) + 0.5
			origin, direction := currentCamera.Ray(pixelX, pixelY)
			currentHit, currentHitOK := scene.Trace(origin, direction, currentCamera.Far)
			receiverPoint, oldX, oldY, ok := locateReceiver(scene, oldFrame, oldCamera, currentCamera, currentDepth, pixelX, pixelY, validateDepth, tolerance, currentHit, currentHitOK)
			if !ok {
				continue
			}
			index := y*currentCamera.Width + x
			result.Depth[index] = projectedDepth(currentCamera, receiverPoint)
			result.Primary[index], _ = SampleFrame(oldFrame, "primary", oldX, oldY)
			result.Shadow[index], _ = SampleFrame(oldFrame, "shadow", oldX, oldY)
			result.Reflection[index], _ = SampleFrame(oldFrame, "reflection", oldX, oldY)
		}
	}
	return result
}

func ReprojectSeparated(scene Scene, oldFrame *Frame, oldCamera, currentCamera Camera, currentDepth *DepthImage, light Light, tolerance float64) *Frame {
	lightCamera := LightCamera(light)
	shadowMap := RenderDepth(scene, lightCamera)
	result := NewFrame(currentCamera.Width, currentCamera.Height)
	for y := 0; y < currentCamera.Height; y++ {
		for x := 0; x < currentCamera.Width; x++ {
			pixelX := float64(x) + 0.5
			pixelY := float64(y) + 0.5
			index := y*currentCamera.Width + x
			origin, rayDirection := currentCamera.Ray(pixelX, pixelY)
			hit, currentHitOK := scene.Trace(origin, rayDirection, currentCamera.Far)
			receiverPoint, oldX, oldY, ok := locateReceiver(scene, oldFrame, oldCamera, currentCamera, currentDepth, pixelX, pixelY, true, tolerance, hit, currentHitOK)
			if !ok {
				continue
			}
			result.Depth[index] = projectedDepth(currentCamera, receiverPoint)

			historyPrimary, ok := SampleFrame(oldFrame, "primary", oldX, oldY)
			if !ok {
				continue
			}
			result.Primary[index] = historyPrimary

			if !hit.Surface.Mirror {
				result.Shadow[index] = currentShadowTerm(scene, hit, light, lightCamera, shadowMap)
			}

			historyReflection, ok := SampleFrame(oldFrame, "reflection", oldX, oldY)
			if !ok {
				continue
			}
			currentReflection := RGB{}
			if hit.Surface.Mirror {
				currentReflection = reflectionColor(scene, hit, rayDirection, light, lightCamera, shadowMap)
			}
			clamped := MinRGB(historyReflection, currentReflection)
			result.Reflection[index] = clamped
		}
	}
	return result
}

func locateReceiver(scene Scene, oldFrame *Frame, oldCamera, currentCamera Camera, currentDepth *DepthImage, pixelX, pixelY float64, validateDepth bool, tolerance float64, currentHit Hit, currentHitOK bool) (Vec3, float64, float64, bool) {
	depth, ok := SampleDepth(currentDepth, pixelX, pixelY)
	if !ok || depth == 0 || !currentHitOK {
		return Vec3{}, 0, 0, false
	}
	receiverPoint := currentCamera.Unproject(pixelX, pixelY, depth)
	oldX, oldY, oldDepth, ok := oldCamera.Project(receiverPoint)
	if !ok {
		return Vec3{}, 0, 0, false
	}
	if validateDepth {
		recordedDepth, ok := SampleDepth(&DepthImage{Width: oldFrame.Width, Height: oldFrame.Height, Depth: oldFrame.Depth}, oldX, oldY)
		recordedSurface, surfaceOK := SampleSurfaceID(oldFrame, oldX, oldY)
		if !ok || recordedDepth == 0 || !surfaceOK || recordedSurface != currentHit.Surface.ID || math.Abs(recordedDepth-oldDepth) > tolerance {
			return Vec3{}, 0, 0, false
		}
		oldOrigin, oldDirection := oldCamera.Ray(oldX, oldY)
		oldHit, oldHitOK := scene.Trace(oldOrigin, oldDirection, oldCamera.Far)
		if !oldHitOK || oldHit.Surface.ID != recordedSurface {
			return Vec3{}, 0, 0, false
		}
	}
	return receiverPoint, oldX, oldY, true
}

func projectedDepth(camera Camera, point Vec3) float64 {
	_, _, depth, ok := camera.Project(point)
	if !ok {
		return 0
	}
	return depth
}

func currentShadowTerm(scene Scene, hit Hit, light Light, lightCamera Camera, shadowMap *DepthImage) RGB {
	diffuse := diffuseColor(scene, hit, light)
	if inShadow(hit.Point, lightCamera, shadowMap) {
		return ScaleRGB(diffuse, -1)
	}
	return RGB{}
}
