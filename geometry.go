package main

// Coordinates are in logical-pixel units. The origin is the top-left corner
// of logical pixel (0,0); x grows right, y grows down.

type vec2 struct{ x, y float64 }

type rect struct{ min, max vec2 }

func (r rect) contains(p vec2) bool {
	return p.x >= r.min.x && p.x < r.max.x && p.y >= r.min.y && p.y < r.max.y
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// tentIntegral returns the integral of the 1-D triangle kernel centered at
// center with half-width radius, evaluated over [a,b]. The kernel integrates
// to 1 over the whole line, so a uniform full field reconstructs exactly 1.
func tentIntegral(center, radius, a, b float64) float64 {
	antideriv := func(x float64) float64 {
		d := x - center
		switch {
		case d <= -radius:
			return 0
		case d < 0:
			u := d + radius
			return 0.5 * u * u / radius
		case d < radius:
			u := radius - d
			return 1 - 0.5*u*u/radius
		default:
			return 1
		}
	}
	v := antideriv(b) - antideriv(a)
	if v < 0 {
		return 0
	}
	return v
}

// overlapWeight is the fraction of a rectangular aperture covered by the
// eye's point-spread function, normalized by the PSF's total mass. The kernel
// is mildly anisotropic: a wider horizontal lobe models display optical
// cross-talk between neighboring subpixel columns, while the vertical lobe
// stays one logical pixel wide. Both eyes use identical widths; only the
// center (phase) differs.
func overlapWeight(center vec2, rx, ry float64, ap rect) float64 {
	wx := tentIntegral(center.x, rx, ap.min.x, ap.max.x)
	wy := tentIntegral(center.y, ry, ap.min.y, ap.max.y)
	return wx * wy
}
