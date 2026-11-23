package relight

import "math"

func RenderFrame(scene Scene, camera Camera, light Light) *Frame {
	lightCamera := LightCamera(light)
	shadowMap := RenderDepth(scene, lightCamera)
	frame := NewFrame(camera.Width, camera.Height)
	for y := 0; y < camera.Height; y++ {
		for x := 0; x < camera.Width; x++ {
			pixelX := float64(x) + 0.5
			pixelY := float64(y) + 0.5
			origin, direction := camera.Ray(pixelX, pixelY)
			hit, ok := scene.Trace(origin, direction, camera.Far)
			if !ok {
				continue
			}
			index := frame.Index(x, y)
			_, _, depth, ok := camera.Project(hit.Point)
			if !ok {
				continue
			}
			frame.Depth[index] = depth
			frame.SurfaceID[index] = hit.Surface.ID
			frame.Primary[index] = primaryColor(scene, hit, light)
			frame.Shadow[index] = shadowColor(scene, hit, light, lightCamera, shadowMap)
			frame.Reflection[index] = reflectionColor(scene, hit, direction, light, lightCamera, shadowMap)
		}
	}
	return frame
}

func RenderDepth(scene Scene, camera Camera) *DepthImage {
	depthImage := NewDepthImage(camera.Width, camera.Height)
	for y := 0; y < camera.Height; y++ {
		for x := 0; x < camera.Width; x++ {
			origin, direction := camera.Ray(float64(x)+0.5, float64(y)+0.5)
			hit, ok := scene.Trace(origin, direction, camera.Far)
			if !ok {
				continue
			}
			_, _, depth, ok := camera.Project(hit.Point)
			if ok {
				depthImage.Depth[y*camera.Width+x] = depth
			}
		}
	}
	return depthImage
}

func primaryColor(scene Scene, hit Hit, light Light) RGB {
	ambient := ScaleRGB(hit.Surface.Albedo, 0.12)
	diffuse := diffuseColor(scene, hit, light)
	return AddRGB(ambient, diffuse)
}

func shadowColor(scene Scene, hit Hit, light Light, lightCamera Camera, shadowMap *DepthImage) RGB {
	diffuse := diffuseColor(scene, hit, light)
	if inShadow(hit.Point, lightCamera, shadowMap) {
		return ScaleRGB(diffuse, -1)
	}
	return RGB{}
}

func reflectionColor(scene Scene, hit Hit, viewDirection Vec3, light Light, lightCamera Camera, shadowMap *DepthImage) RGB {
	if !hit.Surface.Mirror {
		return RGB{}
	}
	reflected := reflectDirection(Scale(viewDirection, -1), hit.Normal)
	reflectionHit, ok := scene.Trace(Add(hit.Point, Scale(reflected, 1e-4)), reflected, 12)
	if !ok {
		return RGB{}
	}
	color := ScaleRGB(reflectionHit.Surface.Albedo, 0.82)
	diffuse := diffuseColor(scene, reflectionHit, light)
	if !inShadow(reflectionHit.Point, lightCamera, shadowMap) {
		color = AddRGB(color, ScaleRGB(diffuse, 0.7))
	}
	return color
}

func diffuseColor(scene Scene, hit Hit, light Light) RGB {
	lightDirection := Normalize(Sub(light.Position, hit.Point))
	attenuation := 1 / math.Pow(Length(Sub(light.Position, hit.Point)), 2)
	cosine := math.Max(0, Dot(hit.Normal, lightDirection))
	return RGB{
		R: hit.Surface.Albedo.R * light.Intensity.R * cosine * attenuation,
		G: hit.Surface.Albedo.G * light.Intensity.G * cosine * attenuation,
		B: hit.Surface.Albedo.B * light.Intensity.B * cosine * attenuation,
	}
}

func inShadow(point Vec3, lightCamera Camera, shadowMap *DepthImage) bool {
	x, y, depth, ok := lightCamera.Project(point)
	if !ok {
		return true
	}
	recorded, ok := SampleDepth(shadowMap, x, y)
	return !ok || recorded == 0 || depth > recorded+0.03
}

func reflectDirection(direction, normal Vec3) Vec3 {
	return Normalize(Sub(direction, Scale(normal, 2*Dot(direction, normal))))
}
