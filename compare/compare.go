package compare

import (
	"image"
	"image/color"
	"math"

	"stereowarp/raster"
)

// Metrics quantifies a reuse+repaint image against the full right-eye render.
type Metrics struct {
	Width, Height int
	MeanAbsErr    float64
	MaxAbsErr     int
	Mismatch      int
	MSE           float64
	PSNR          float64
	DepthMAE      float64
	DepthMax      float64
}

const mismatchThreshold = 12

// Frames evaluates color and depth differences pixel by pixel.
func Frames(got, ref *raster.Framebuffer) Metrics {
	if got.Width != ref.Width || got.Height != ref.Height {
		panic("compare: framebuffer dimensions differ")
	}
	m := Metrics{Width: got.Width, Height: got.Height}
	n := got.Width * got.Height
	var sumAbs, sumSq, depthSum float64
	for i := 0; i < n; i++ {
		a, b := got.Color[i], ref.Color[i]
		dr := int(a.R) - int(b.R)
		dg := int(a.G) - int(b.G)
		db := int(a.B) - int(b.B)
		abs := float64(absI(dr)+absI(dg)+absI(db)) / 3.0
		sumAbs += abs
		if int(abs) > m.MaxAbsErr {
			m.MaxAbsErr = int(abs)
		}
		if abs > mismatchThreshold {
			m.Mismatch++
		}
		sumSq += float64(dr*dr+dg*dg+db*db) / 3.0

		var depthErr float64
		if got.Depth[i] == 0 && ref.Depth[i] == 0 {
			depthErr = 0
		} else {
			depthErr = math.Abs(got.Depth[i] - ref.Depth[i])
		}
		depthSum += depthErr
		if depthErr > m.DepthMax {
			m.DepthMax = depthErr
		}
	}
	m.MeanAbsErr = sumAbs / float64(n)
	m.MSE = sumSq / float64(n)
	if m.MSE > 0 {
		m.PSNR = 10 * math.Log10((255*255)/m.MSE)
	} else {
		m.PSNR = math.Inf(1)
	}
	m.DepthMAE = depthSum / float64(n)
	return m
}

// Heatmap returns an NRGBA image where red intensity encodes color error and
// blue marks reference pixels whose depth differed materially.
func Heatmap(got, ref *raster.Framebuffer) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, got.Width, got.Height))
	n := got.Width * got.Height
	for i := 0; i < n; i++ {
		a, b := got.Color[i], ref.Color[i]
		err := (absI(int(a.R)-int(b.R)) + absI(int(a.G)-int(b.G)) + absI(int(a.B)-int(b.B))) / 3
		depthErr := math.Abs(got.Depth[i] - ref.Depth[i])
		c := color.NRGBA{
			R: uint8(min(err*2, 255)),
			G: uint8(max(0, 60-err)),
			B: uint8(min(int(depthErr*40), 255)),
			A: 255,
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = c.A
	}
	return img
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
