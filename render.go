package stereoholes

import (
	"math"
)

// Frame is one eye's shaded observation plus geometry metadata.
type Frame struct {
	Camera    Camera
	RGB       []Vec3
	Depth     []float64
	ObjectID  []int
	Category  []string
	Stable    []bool
	Ambiguous []bool
	Width     int
	Height    int
}

func NewFrame(c Camera) *Frame {
	n := c.Width * c.Height
	return &Frame{
		Camera: c, RGB: make([]Vec3, n), Depth: make([]float64, n),
		ObjectID: make([]int, n), Category: make([]string, n),
		Stable: make([]bool, n), Ambiguous: make([]bool, n),
		Width: c.Width, Height: c.Height,
	}
}

func (f *Frame) idx(x, y int) int { return y*f.Width + x }

func (f *Frame) At(x, y int) Vec3 { return f.RGB[f.idx(x, y)] }

func (s *Scene) Render(c Camera) *Frame {
	f := NewFrame(c)
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			r := c.Ray(x, y)
			i := f.idx(x, y)
			f.RGB[i], f.Depth[i], f.ObjectID[i], f.Category[i],
				f.Stable[i], f.Ambiguous[i] = s.trace(r, c)
		}
	}
	return f
}

func (s *Scene) trace(r Ray, cam Camera) (Vec3, float64, int, string, bool, bool) {
	hits := s.intersections(r)
	color := background(r)
	transmittance := 1.0
	stable := true
	ambiguous := false
	firstDepth := math.Inf(1)
	firstID := 0
	firstCat := ""

	for _, h := range hits {
		m := h.quad.Material
		if math.IsInf(firstDepth, 1) {
			firstDepth = h.t * r.Dir.Dot(cam.Forward)
			firstID = h.quad.ID
			firstCat = m.Category
		}
		if !m.Stable {
			stable = false
		}
		if m.Alpha < 1 {
			ambiguous = true
		}
		a := clamp(m.Alpha, 0, 1)
		layer := s.shade(h, r)
		color = layer.Mul(a * transmittance).Add(color.Mul(1 - a*transmittance))
		transmittance *= 1 - a
		if transmittance < 1e-4 {
			break
		}
	}
	if math.IsInf(firstDepth, 1) {
		firstDepth = 0
	}
	return color.Clamp01(), firstDepth, firstID, firstCat, stable, ambiguous
}
