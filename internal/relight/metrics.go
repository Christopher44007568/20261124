package relight

import "math"

type Metrics struct {
	Name            string
	MeanColorError  float64
	MaxColorError   float64
	PrimaryError    float64
	ShadowError     float64
	ReflectionError float64
	StalePixels     int
	BlockedSamples  int
}

func Composite(frame *Frame) []RGB {
	colors := make([]RGB, len(frame.Primary))
	for i := range colors {
		colors[i] = AddRGB(frame.Primary[i], AddRGB(frame.Shadow[i], frame.Reflection[i]))
	}
	return colors
}

func Compare(name string, scene Scene, predicted, reference *Frame, oldCamera, currentCamera Camera) Metrics {
	predictedColor := Composite(predicted)
	referenceColor := Composite(reference)
	var colorSum float64
	var primarySum float64
	var shadowSum float64
	var reflectionSum float64
	maxColor := 0.0
	stale := 0
	blocked := 0
	for i := range predictedColor {
		colorDistance := rgbDistance(predictedColor[i], referenceColor[i])
		colorSum += colorDistance
		if colorDistance > maxColor {
			maxColor = colorDistance
		}
		if colorDistance > 0.02 {
			stale++
		}
		primaryDistance := rgbDistance(predicted.Primary[i], reference.Primary[i])
		shadowDistance := rgbDistance(predicted.Shadow[i], reference.Shadow[i])
		reflectionDistance := rgbDistance(predicted.Reflection[i], reference.Reflection[i])
		primarySum += primaryDistance * primaryDistance
		shadowSum += shadowDistance * shadowDistance
		reflectionSum += reflectionDistance * reflectionDistance
		if predicted.Depth[i] != 0 && reference.Depth[i] == 0 {
			blocked++
		}
		if reference.Depth[i] != 0 {
			pixelX := float64(i%reference.Width) + 0.5
			pixelY := float64(i/reference.Width) + 0.5
			referencePoint := currentCamera.Unproject(pixelX, pixelY, reference.Depth[i])
			_, _, _, projectOK := oldCamera.Project(referencePoint)
			if !projectOK || !scene.Visible(oldCamera.Pose.Pos, referencePoint) {
				blocked++
			}
		}
	}
	count := float64(len(predictedColor))
	return Metrics{
		Name:            name,
		MeanColorError:  colorSum / count,
		MaxColorError:   maxColor,
		PrimaryError:    math.Sqrt(primarySum / count),
		ShadowError:     math.Sqrt(shadowSum / count),
		ReflectionError: math.Sqrt(reflectionSum / count),
		StalePixels:     stale,
		BlockedSamples:  blocked,
	}
}

func ReflectionLuminanceMax(frame *Frame) float64 {
	result := 0.0
	for _, value := range frame.Reflection {
		luminance := value.R*0.2126 + value.G*0.7152 + value.B*0.0722
		if luminance > result {
			result = luminance
		}
	}
	return result
}

func rgbDistance(a, b RGB) float64 {
	dr := a.R - b.R
	dg := a.G - b.G
	db := a.B - b.B
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
