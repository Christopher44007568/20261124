package tex

import "math"

// Marker is an RGBA texture shared by every layer.
type Marker struct {
	pix           []uint8
	width, height int
}

// Placement positions one marker inside a layer's parameter space.
type Placement struct {
	CenterU, CenterV float64
	HalfU, HalfV     float64
	WrapU, WrapV     WrapMode
}

const markerSize = 256

// NewMarker builds an asymmetric decal: cyan rounded frame, red dot in the
// upper-left, blue block in the lower-right, yellow diagonal stripe. The
// asymmetry makes any duplicated (double-image) copy immediately visible.
func NewMarker() *Marker {
	m := &Marker{width: markerSize, height: markerSize, pix: make([]uint8, markerSize*markerSize*4)}
	for y := 0; y < markerSize; y++ {
		for x := 0; x < markerSize; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			col, cov := paint(px, py)
			if cov <= 0 {
				continue
			}
			idx := (y*markerSize + x) * 4
			m.pix[idx] = clamp255(col[0] * cov)
			m.pix[idx+1] = clamp255(col[1] * cov)
			m.pix[idx+2] = clamp255(col[2] * cov)
			m.pix[idx+3] = clamp255(cov)
		}
	}
	return m
}

func paint(x, y float64) ([3]float64, float64) {
	// Rounded cyan frame.
	edge := 14.0
	outer := roundedBox(x, y, 10, 10, markerSize-10, markerSize-10, 26)
	inner := roundedBox(x, y, 10+edge, 10+edge, markerSize-10-edge, markerSize-10-edge, 16)
	frame := outer - inner
	if frame > 0 {
		return [3]float64{0.15, 0.85, 0.9}, frame
	}
	// Red orientation dot, upper-left.
	if d := math.Hypot(x-66, y-62) - 26; d < 0 {
		return [3]float64{0.95, 0.2, 0.18}, coverage(d)
	}
	// Blue solid block, lower-right.
	if bx := roundedBox(x, y, 150, 158, 214, 222, 8); bx > 0 {
		return [3]float64{0.25, 0.4, 0.95}, bx
	}
	// Yellow diagonal stripe with parallel edges.
	d1 := math.Abs((y - x) / math.Sqrt2)
	d2 := math.Abs((y + x - markerSize) / math.Sqrt2)
	if d1 < 10 || d2 < 10 {
		return [3]float64{0.98, 0.82, 0.2}, 1
	}
	return [3]float64{}, 0
}

// roundedBox returns coverage of a rounded rectangle.
func roundedBox(x, y, x0, y0, x1, y1, radius float64) float64 {
	cx := clampFloat(x, x0+radius, x1-radius)
	cy := clampFloat(y, y0+radius, y1-radius)
	d := math.Hypot(x-cx, y-cy) - radius
	return coverage(d)
}

func coverage(signedDistance float64) float64 {
	return clampFloat(0.5-signedDistance, 0, 1)
}

func clampFloat(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func clamp255(x float64) uint8 {
	v := int(math.Round(x * 255))
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// SamplePlacement evaluates the marker through a layer-space placement.
func (m *Marker) SamplePlacement(u, v float64, p Placement) (r, g, b, a float64, ok bool) {
	du := u - p.CenterU
	if p.WrapU == WrapRepeat {
		du = math.Mod(du+0.5, 1)
		if du < 0 {
			du++
		}
		du -= 0.5
		if math.Abs(du) > p.HalfU {
			return 0, 0, 0, 0, false
		}
	}
	dv := v - p.CenterV
	if p.WrapV == WrapRepeat {
		dv = math.Mod(dv+0.5, 1)
		if dv < 0 {
			dv++
		}
		dv -= 0.5
		if math.Abs(dv) > p.HalfV {
			return 0, 0, 0, 0, false
		}
	}
	localU := 0.5 + du/(2*p.HalfU)
	localV := 0.5 + dv/(2*p.HalfV)
	if p.WrapU != WrapRepeat && (localU < 0 || localU > 1) {
		return 0, 0, 0, 0, false
	}
	if p.WrapV != WrapRepeat && (localV < 0 || localV > 1) {
		return 0, 0, 0, 0, false
	}
	return m.Sample(localU, localV, p.WrapU, p.WrapV)
}
