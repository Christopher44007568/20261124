package imaging

import "math"

// Mask marks pixels that must be reconstructed. Bits are stored row-major.
type Mask struct {
	W    int
	H    int
	Bits []bool
}

func NewMask(w, h int) *Mask {
	if w <= 0 || h <= 0 {
		panic("imaging: mask dimensions must be positive")
	}
	return &Mask{W: w, H: h, Bits: make([]bool, w*h)}
}

func (m *Mask) Index(x, y int) int {
	return y*m.W + x
}

func (m *Mask) At(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.Bits[m.Index(x, y)]
}

func (m *Mask) Set(x, y int, missing bool) {
	m.Bits[m.Index(x, y)] = missing
}

func (m *Mask) Count() int {
	n := 0
	for _, missing := range m.Bits {
		if missing {
			n++
		}
	}
	return n
}

func (m *Mask) Clone() *Mask {
	out := &Mask{W: m.W, H: m.H, Bits: make([]bool, len(m.Bits))}
	copy(out.Bits, m.Bits)
	return out
}

func (m *Mask) Apply(f *Frame, fill RGB) {
	for i, missing := range m.Bits {
		if missing {
			f.Pix[i] = fill
		}
	}
}

// CircleMask returns a centered circular mask. The center uses the pixel
// coordinate ((W-1)/2, (H-1)/2), and a pixel is masked when its center is
// within the radius.
func CircleMask(w, h int, radius float64) *Mask {
	m := NewMask(w, h)
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= radius*radius {
				m.Set(x, y, true)
			}
		}
	}
	return m
}

// Disk returns a mask for a disk. It is independent of any source mask, which
// keeps the metric regions explicit in reports.
func Disk(w, h int, radius float64) *Mask {
	return CircleMask(w, h, radius)
}

// Ring returns a mask for pixels whose distance from the image center lies in
// [inner, outer].
func Ring(w, h int, inner, outer float64) *Mask {
	m := NewMask(w, h)
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := math.Hypot(dx, dy)
			if d >= inner && d <= outer {
				m.Set(x, y, true)
			}
		}
	}
	return m
}

// Intersect returns the pixels present in both masks.
func Intersect(a, b *Mask) *Mask {
	if a.W != b.W || a.H != b.H {
		panic("imaging: mask dimensions differ")
	}
	out := NewMask(a.W, a.H)
	for i := range out.Bits {
		out.Bits[i] = a.Bits[i] && b.Bits[i]
	}
	return out
}

// And is a convenience wrapper for Intersect.
func And(a, b *Mask) *Mask {
	return Intersect(a, b)
}
