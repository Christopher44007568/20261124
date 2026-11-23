package checker

// Mask records which pixels are actually shaded.
//
// A pixel is shaded when it either lies inside the fully-shaded center region
// (per-pixel shading) or belongs to a shaded 2:2 checkerboard block in the
// periphery. Every other peripheral pixel must be reconstructed.
type Mask struct {
	W, H   int
	Center Rect
	// Shaded and InCenter are row-major booleans of length W*H.
	Shaded   []bool
	InCenter []bool
}

// Rect is an integer half-open rectangle [X0,X1) x [Y0,Y1).
type Rect struct {
	X0, Y0, X1, Y1 int
}

func (r Rect) contains(x, y int) bool {
	return x >= r.X0 && x < r.X1 && y >= r.Y0 && y < r.Y1
}

// phase0Shaded reports the 2:2 checkerboard write rule for shading phase zero.
// Blocks of 2x2 pixels alternate, so ((x/2)+(y/2)) even means "write".
func phase0Shaded(x, y int) bool {
	return ((x/2)+(y/2))&1 == 0
}

// BuildMask constructs the shading mask for an image of size w x h with the
// given fully-shaded center rectangle. phase flips the checkerboard pattern;
// phase 1 writes the complementary blocks and is used for phase-invariance
// checks.
func BuildMask(w, h int, center Rect, phase int) *Mask {
	if center.X1 > w {
		center.X1 = w
	}
	if center.Y1 > h {
		center.Y1 = h
	}
	if center.X0 < 0 {
		center.X0 = 0
	}
	if center.Y0 < 0 {
		center.Y0 = 0
	}
	m := &Mask{
		W:        w,
		H:        h,
		Center:   center,
		Shaded:   make([]bool, w*h),
		InCenter: make([]bool, w*h),
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			inCenter := center.contains(x, y)
			m.InCenter[idx] = inCenter
			m.Shaded[idx] = inCenter || (phase0Shaded(x, y) == (phase == 0))
		}
	}
	return m
}

// At reports whether the pixel at (x,y) is shaded.
func (m *Mask) At(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.Shaded[y*m.W+x]
}

// IsCenter reports whether the pixel at (x,y) belongs to the per-pixel center.
func (m *Mask) IsCenter(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.InCenter[y*m.W+x]
}

// Shade renders a frame under the mask: shaded pixels keep their ground-truth
// color while skipped peripheral pixels are left as black and flagged absent.
func Shade(truth *Image, mask *Mask) *Image {
	out := NewImage(truth.W, truth.H)
	for i := range truth.Px {
		if mask.Shaded[i] {
			out.Px[i] = truth.Px[i]
		}
	}
	return out
}

// ShadedCount returns the number of written pixels.
func (m *Mask) ShadedCount() int {
	n := 0
	for _, s := range m.Shaded {
		if s {
			n++
		}
	}
	return n
}
