package checker

import (
	"image"
	"image/png"
	"io"
)

// ToNRGBA converts the float RGB raster to an 8-bit image for encoding.
func (m *Image) ToNRGBA() *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, m.W, m.H))
	for i, c := range m.Px {
		p := i * 4
		out.Pix[p+0] = color8(c.R)
		out.Pix[p+1] = color8(c.G)
		out.Pix[p+2] = color8(c.B)
		out.Pix[p+3] = 255
	}
	return out
}

// MaskImage renders the shading mask: white = shaded, red = per-pixel center,
// black = skipped peripheral pixel.
func (m *Mask) MaskImage() *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, m.W, m.H))
	for i := range m.Shaded {
		p := i * 4
		switch {
		case m.InCenter[i]:
			out.Pix[p+0], out.Pix[p+1], out.Pix[p+2] = 220, 40, 40
		case m.Shaded[i]:
			out.Pix[p+0], out.Pix[p+1], out.Pix[p+2] = 235, 235, 235
		default:
			out.Pix[p+0], out.Pix[p+1], out.Pix[p+2] = 20, 20, 20
		}
		out.Pix[p+3] = 255
	}
	return out
}

// WritePNG encodes an NRGBA image to w as a standard-library PNG.
func WritePNG(w io.Writer, img *image.NRGBA) error {
	return png.Encode(w, img)
}

func color8(v float64) uint8 {
	v = clamp01(v)
	return uint8(v*255 + 0.5)
}
