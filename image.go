package binocular

import (
	"fmt"
	"io"
)

// RGB is an 8-bit color channel triplet.
type RGB struct {
	R, G, B uint8
}

// Image is a tightly packed RGB framebuffer.
type Image struct {
	Width  int
	Height int
	Pixels []RGB
}

// NewImage allocates a framebuffer and fills it with clear.
func NewImage(width, height int, clear RGB) Image {
	if width < 0 || height < 0 {
		panic("binocular: negative image dimensions")
	}
	pixels := make([]RGB, width*height)
	for i := range pixels {
		pixels[i] = clear
	}
	return Image{Width: width, Height: height, Pixels: pixels}
}

// At returns the pixel at (x,y). It panics when the coordinate is outside the
// image.
func (im Image) At(x, y int) RGB {
	return im.Pixels[y*im.Width+x]
}

// Set writes a pixel. It panics when the coordinate is outside the image.
func (im Image) Set(x, y int, color RGB) {
	im.Pixels[y*im.Width+x] = color
}

// Equal reports whether two images have the same dimensions and bytes.
func (im Image) Equal(other Image) bool {
	if im.Width != other.Width || im.Height != other.Height {
		return false
	}
	if len(im.Pixels) != len(other.Pixels) {
		return false
	}
	for i := range im.Pixels {
		if im.Pixels[i] != other.Pixels[i] {
			return false
		}
	}
	return true
}

// Difference summarizes pixel-level differences against another image.
type Difference struct {
	DifferentPixels int
	MaxChannelDelta int
}

// CompareImages counts differing pixels and the largest channel delta.
func CompareImages(a, b Image) Difference {
	diff := Difference{}
	if a.Width != b.Width || a.Height != b.Height {
		diff.DifferentPixels = max(a.Width*a.Height, b.Width*b.Height)
		diff.MaxChannelDelta = 255
		return diff
	}
	for i := range a.Pixels {
		pa, pb := a.Pixels[i], b.Pixels[i]
		if pa != pb {
			diff.DifferentPixels++
		}
		diff.MaxChannelDelta = max(diff.MaxChannelDelta,
			absChannelDelta(pa.R, pb.R),
			absChannelDelta(pa.G, pb.G),
			absChannelDelta(pa.B, pb.B))
	}
	return diff
}

func absChannelDelta(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// WritePPM writes a binary P6 image without any third-party dependency.
func (im Image) WritePPM(w io.Writer) error {
	if im.Width < 0 || im.Height < 0 {
		return fmt.Errorf("binocular: invalid image dimensions %dx%d", im.Width, im.Height)
	}
	if len(im.Pixels) != im.Width*im.Height {
		return fmt.Errorf("binocular: pixel buffer does not match %dx%d image", im.Width, im.Height)
	}
	if _, err := fmt.Fprintf(w, "P6\n%d %d\n255\n", im.Width, im.Height); err != nil {
		return err
	}
	row := make([]byte, im.Width*3)
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			pixel := im.At(x, y)
			row[x*3] = pixel.R
			row[x*3+1] = pixel.G
			row[x*3+2] = pixel.B
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
