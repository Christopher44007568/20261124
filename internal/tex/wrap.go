package tex

// WrapMode controls how texture coordinates outside [0,1] are resolved.
type WrapMode int

const (
	// WrapClamp discards samples that leave [0,1] (transparent outside).
	WrapClamp WrapMode = iota
	// WrapRepeat wraps around; used across cylinder/sphere longitude seams.
	WrapRepeat
	// WrapClampPaint freezes the edge texel, used at sphere poles.
	WrapClampPaint
)

// Sample is one bilinear-filtered texture fetch. It never projects the same
// marker twice: on a repeat axis the shortest wrapped delta decides membership,
// so a marker whose footprint is narrower than one period is hit exactly once.
func (m *Marker) Sample(u, v float64, wrapU, wrapV WrapMode) (r, g, b, a float64, ok bool) {
	fx, insideX := resolve(u, m.width, wrapU)
	fy, insideY := resolve(v, m.height, wrapV)
	if !insideX || !insideY {
		return 0, 0, 0, 0, false
	}
	x0 := int(fx)
	y0 := int(fy)
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	x1, y1 := x0+1, y0+1
	if wrapU != WrapRepeat {
		x1 = clampInt(x1, 0, m.width-1)
	}
	if wrapV != WrapRepeat {
		y1 = clampInt(y1, 0, m.height-1)
	}
	c00 := m.at(x0, y0, wrapU, wrapV)
	c10 := m.at(x1, y0, wrapU, wrapV)
	c01 := m.at(x0, y1, wrapU, wrapV)
	c11 := m.at(x1, y1, wrapU, wrapV)
	w00 := (1 - tx) * (1 - ty)
	w10 := tx * (1 - ty)
	w01 := (1 - tx) * ty
	w11 := tx * ty
	r = c00.r*w00 + c10.r*w10 + c01.r*w01 + c11.r*w11
	g = c00.g*w00 + c10.g*w10 + c01.g*w01 + c11.g*w11
	b = c00.b*w00 + c10.b*w10 + c01.b*w01 + c11.b*w11
	a = c00.a*w00 + c10.a*w10 + c01.a*w01 + c11.a*w11
	return r, g, b, a, true
}

type texel struct{ r, g, b, a float64 }

func (m *Marker) at(x, y int, wrapU, wrapV WrapMode) texel {
	if wrapU == WrapRepeat {
		x = ((x % m.width) + m.width) % m.width
	} else {
		x = clampInt(x, 0, m.width-1)
	}
	if wrapV == WrapRepeat {
		y = ((y % m.height) + m.height) % m.height
	} else {
		y = clampInt(y, 0, m.height-1)
	}
	idx := (y*m.width + x) * 4
	return texel{
		r: float64(m.pix[idx]) / 255,
		g: float64(m.pix[idx+1]) / 255,
		b: float64(m.pix[idx+2]) / 255,
		a: float64(m.pix[idx+3]) / 255,
	}
}

// resolve maps a normalized coordinate to a continuous texel position and
// reports whether the position belongs to the visible marker region.
func resolve(u float64, size int, wrap WrapMode) (float64, bool) {
	switch wrap {
	case WrapRepeat:
		// Single shortest-period representation; membership is always valid.
		return u * float64(size), true
	case WrapClampPaint:
		// Freeze the boundary texel only up to one texel past the edge;
		// beyond the marker's footprint the sample is empty so the pole
		// does not smear the marker edge across the whole latitude ring.
		if u < 0 || u > 1+1/float64(size) {
			return 0, false
		}
		if u > 1 {
			return float64(size - 1), true
		}
		return u * float64(size), true
	default: // WrapClamp: outside the unit interval there is no marker.
		if u < 0 || u > 1 {
			return 0, false
		}
		return u * float64(size), true
	}
}

func clampInt(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
