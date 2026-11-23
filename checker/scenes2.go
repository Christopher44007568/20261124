package checker

import "math"

// renderLines draws thin one-pixel diagonal stripes over a mid-gray field.
// It stresses coverage: a reconstruction that smears must not widen or erase
// the lines.
func renderLines(w, h int) *Image {
	img := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := linesCover(x, y)
			if on {
				img.Set(x, y, RGB{R: 0.95, G: 0.92, B: 0.10})
			} else {
				img.Set(x, y, RGB{R: 0.12, G: 0.14, B: 0.18})
			}
		}
	}
	return img
}

// linesCover reports membership in two families of thin diagonal lines with
// incommensurate periods so the pattern never repeats on the 2:2 lattice.
func linesCover(x, y int) bool {
	d1 := (x - 2*y) % 17
	if d1 < 0 {
		d1 += 17
	}
	d2 := (2*x + y) % 23
	if d2 < 0 {
		d2 += 23
	}
	return d1 == 0 || d2 == 0
}

// renderChecker draws a high-contrast checkerboard with square side k.
func renderChecker(w, h, k int) *Image {
	img := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if checkerCover(x, y, k) {
				img.Set(x, y, RGB{R: 0.97, G: 0.97, B: 0.95})
			} else {
				img.Set(x, y, RGB{R: 0.03, G: 0.03, B: 0.05})
			}
		}
	}
	return img
}

func checkerCover(x, y, k int) bool {
	return ((x/k)+(y/k))&1 == 0
}

// renderHueSweep varies hue continuously while holding lightness and chroma
// high, so hue drift in the reconstruction is directly measurable.
func renderHueSweep(w, h int) *Image {
	img := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u, v := norm(x, y, w, h)
			hue := math.Mod(u*2*math.Pi+0.6*math.Sin(v*math.Pi*2), 2*math.Pi)
			if hue < 0 {
				hue += 2 * math.Pi
			}
			img.Set(x, y, hslRGB(hue, 0.85, 0.55+0.1*v))
		}
	}
	return img
}

func hueCover(x, y int) bool {
	u := float64(x) / 159.0
	return u > 0.33 && u < 0.66
}

// hslRGB converts an HSL triple (hue in radians, saturation/lightness in [0,1])
// to RGB using the standard six-sector formulation.
func hslRGB(hue, sat, light float64) RGB {
	c := (1 - math.Abs(2*light-1)) * sat
	hp := hue / (math.Pi / 3)
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r1, g1, b1 float64
	switch {
	case hp < 1:
		r1, g1, b1 = c, x, 0
	case hp < 2:
		r1, g1, b1 = x, c, 0
	case hp < 3:
		r1, g1, b1 = 0, c, x
	case hp < 4:
		r1, g1, b1 = 0, x, c
	case hp < 5:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	m := light - c/2
	return RGB{R: r1 + m, G: g1 + m, B: b1 + m}
}
