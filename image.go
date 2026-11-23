package stereoholes

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// ToImage converts a frame to RGBA. Pixels selected by holes are left black.
func (f *Frame) ToImage(holes []bool) *image.RGBA {
	copyRGB := append([]Vec3(nil), f.RGB...)
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := f.idx(x, y)
			if holes != nil && holes[i] {
				copyRGB[i] = holeColor
			}
		}
	}
	return RGBImage(f.Width, f.Height, copyRGB)
}

// RGBImage converts linear RGB values in [0, 1] to an opaque RGBA image.
func RGBImage(width, height int, rgb []Vec3) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rgb[y*width+x]
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(clamp(c.X, 0, 1)*255 + 0.5),
				G: uint8(clamp(c.Y, 0, 1)*255 + 0.5),
				B: uint8(clamp(c.Z, 0, 1)*255 + 0.5),
				A: 255,
			})
		}
	}
	return img
}

// GuardImage visualizes accepted and rejected hole pixels by reason.
func GuardImage(width, height int, holes []bool, res FillResult) *image.RGBA {
	colors := map[RejectReason]color.RGBA{
		ReasonNoOverlap:      {125, 132, 142, 255},
		ReasonMaskSurface:    {70, 118, 220, 255},
		ReasonSourceTracking: {224, 81, 180, 255},
		ReasonDepth:          {224, 82, 78, 255},
		ReasonTargetTracking: {235, 188, 52, 255},
		ReasonSupport:        {66, 190, 212, 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			c := color.RGBA{10, 13, 19, 255}
			if holes[i] {
				if res.Status[i] == StatusFilled {
					c = color.RGBA{63, 190, 116, 255}
				} else if rc, ok := colors[res.Reason[i]]; ok {
					c = rc
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// WritePNG writes an image to path.
func WritePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
