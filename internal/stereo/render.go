package stereo

import (
	"image"
	"image/color"
	"math"
)

const SurfaceNone SurfaceID = -1

type Frame struct {
	Image     *image.RGBA
	Depth     []float64
	Surface   []SurfaceID
	OutOfView []bool
	OOBCount  int
}

func Render(camera Camera) Frame {
	frame := Frame{
		Image:     image.NewRGBA(image.Rect(0, 0, ImageWidth, ImageHeight)),
		Depth:     make([]float64, ImageWidth*ImageHeight),
		Surface:   make([]SurfaceID, ImageWidth*ImageHeight),
		OutOfView: make([]bool, ImageWidth*ImageHeight),
	}
	oobColor := color.RGBA{R: 255, G: 0, B: 255, A: 255}

	for y := 0; y < ImageHeight; y++ {
		for x := 0; x < ImageWidth; x++ {
			idx := y*ImageWidth + x
			frame.Surface[idx] = SurfaceNone
			frame.Depth[idx] = math.Inf(1)
			ray, oob, _ := camera.RayAt(float64(x)+0.5, float64(y)+0.5)
			if oob {
				frame.OutOfView[idx] = true
				frame.OOBCount++
				writePixel(frame.Image, idx, oobColor)
				continue
			}
			hit, ok := IntersectScene(ray)
			if !ok {
				writePixel(frame.Image, idx, color.RGBA{A: 255})
				continue
			}
			frame.Depth[idx] = hit.Point.Z
			frame.Surface[idx] = hit.Surface
			writePixel(frame.Image, idx, Shade(hit).Clamp())
		}
	}
	return frame
}

func writePixel(img *image.RGBA, idx int, c color.RGBA) {
	offset := idx * 4
	img.Pix[offset+0] = c.R
	img.Pix[offset+1] = c.G
	img.Pix[offset+2] = c.B
	img.Pix[offset+3] = 255
}

func (f Frame) IsOutOfBounds(idx int) bool {
	return idx >= 0 && idx < len(f.OutOfView) && f.OutOfView[idx]
}
