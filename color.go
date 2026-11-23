package reprojection

// Color is an 8-bit, straight-alpha RGBA color.
type Color struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

var (
	Transparent = Color{}
	Black       = Color{0, 0, 0, 255}
	White       = Color{255, 255, 255, 255}
)

func RGB(r, g, b uint8) Color {
	return Color{r, g, b, 255}
}

// Over composites src over dst using source-over alpha blending.
func (src Color) Over(dst Color) Color {
	if src.A == 0 {
		return dst
	}
	if src.A == 255 || dst.A == 0 {
		return src
	}

	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	oa := sa + da*(1-sa)
	if oa == 0 {
		return Transparent
	}
	blend := func(s, d uint8) uint8 {
		v := (float64(s)*sa + float64(d)*da*(1-sa)) / oa
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	return Color{blend(src.R, dst.R), blend(src.G, dst.G), blend(src.B, dst.B), uint8(oa*255 + 0.5)}
}

func lerpColor(a, b Color, t float64) Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5)
	}
	return Color{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), mix(a.A, b.A)}
}
