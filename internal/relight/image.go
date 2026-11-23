package relight

import "math"

type RGB struct {
	R, G, B float64
}

func MaxComponent(c RGB) float64 {
	return math.Max(c.R, math.Max(c.G, c.B))
}

type Frame struct {
	Width      int
	Height     int
	Depth      []float64
	SurfaceID  []int
	Primary    []RGB
	Shadow     []RGB
	Reflection []RGB
}

func NewFrame(width, height int) *Frame {
	count := width * height
	return &Frame{
		Width:      width,
		Height:     height,
		Depth:      make([]float64, count),
		SurfaceID:  make([]int, count),
		Primary:    make([]RGB, count),
		Shadow:     make([]RGB, count),
		Reflection: make([]RGB, count),
	}
}

func (f *Frame) Index(x, y int) int {
	return y*f.Width + x
}

type DepthImage struct {
	Width  int
	Height int
	Depth  []float64
}

func NewDepthImage(width, height int) *DepthImage {
	return &DepthImage{Width: width, Height: height, Depth: make([]float64, width*height)}
}

func AddRGB(a, b RGB) RGB {
	return RGB{a.R + b.R, a.G + b.G, a.B + b.B}
}

func ScaleRGB(a RGB, s float64) RGB {
	return RGB{a.R * s, a.G * s, a.B * s}
}

func MinRGB(a, b RGB) RGB {
	return RGB{
		R: math.Min(a.R, b.R),
		G: math.Min(a.G, b.G),
		B: math.Min(a.B, b.B),
	}
}

func SampleDepth(depth *DepthImage, x, y float64) (float64, bool) {
	ix, iy, ok := pixelBounds(depth.Width, depth.Height, x, y)
	if !ok {
		return 0, false
	}
	return depth.Depth[iy*depth.Width+ix], true
}

func SampleFrame(frame *Frame, channel string, x, y float64) (RGB, bool) {
	ix, iy, ok := pixelBounds(frame.Width, frame.Height, x, y)
	if !ok {
		return RGB{}, false
	}
	switch channel {
	case "primary":
		return frame.Primary[iy*frame.Width+ix], true
	case "shadow":
		return frame.Shadow[iy*frame.Width+ix], true
	case "reflection":
		return frame.Reflection[iy*frame.Width+ix], true
	default:
		return RGB{}, false
	}
}

func SampleSurfaceID(frame *Frame, x, y float64) (int, bool) {
	ix, iy, ok := pixelBounds(frame.Width, frame.Height, x, y)
	if !ok {
		return 0, false
	}
	return frame.SurfaceID[iy*frame.Width+ix], true
}

func pixelBounds(width, height int, x, y float64) (int, int, bool) {
	ix := int(math.Floor(x + 0.5))
	iy := int(math.Floor(y + 0.5))
	if ix < 0 || ix >= width || iy < 0 || iy >= height {
		return 0, 0, false
	}
	return ix, iy, true
}
