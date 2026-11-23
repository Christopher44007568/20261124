package checker

import "math"

// Scene is a procedural ground-truth image used to exercise the pipeline.
// Coverage returns the binary foreground mask at a pixel, used for the
// coverage/leakage metric. Wellposed is false when the pattern aliases under
// 2:2 sampling, in which case perfect recovery is information-theoretically
// impossible and only aperiodicity is enforced.
type Scene struct {
	Name      string
	Wellposed bool
	Render    func(w, h int) *Image
	Coverage  func(x, y int) bool
	// Foreground classifies a ground-truth/reconstructed color as foreground,
	// keeping the coverage metric independent of absolute brightness.
	Foreground func(RGB) bool
}

// Scenes returns the full benchmark set: analytic shapes, procedural texture,
// thin diagonal lines, high-contrast checkerboards and a hue sweep.
func Scenes() []Scene {
	return []Scene{
		{
			Name:       "analytic_blob",
			Wellposed:  true,
			Render:     renderAnalytic,
			Coverage:   analyticCover,
			Foreground: func(c RGB) bool { return Luma(c) > 0.55 },
		},
		{
			Name:       "procedural_fbm",
			Wellposed:  true,
			Render:     renderFBM,
			Coverage:   fbmCover,
			Foreground: func(c RGB) bool { return Luma(c) > 0.5 },
		},
		{
			Name:       "thin_diagonal_lines",
			Wellposed:  true,
			Render:     renderLines,
			Coverage:   linesCover,
			Foreground: func(c RGB) bool { return c.R > 0.5 && c.B < 0.5 },
		},
		{
			Name:       "checker_16px",
			Wellposed:  true,
			Render:     func(w, h int) *Image { return renderChecker(w, h, 16) },
			Coverage:   func(x, y int) bool { return checkerCover(x, y, 16) },
			Foreground: func(c RGB) bool { return Luma(c) > 0.5 },
		},
		{
			Name:       "hue_sweep",
			Wellposed:  true,
			Render:     renderHueSweep,
			Coverage:   hueCover,
			Foreground: func(c RGB) bool { _, ch := Hue(c); return ch > 0.35 },
		},
		{
			Name:       "checker_8px_aliased",
			Wellposed:  false,
			Render:     func(w, h int) *Image { return renderChecker(w, h, 8) },
			Coverage:   func(x, y int) bool { return checkerCover(x, y, 8) },
			Foreground: func(c RGB) bool { return Luma(c) > 0.5 },
		},
		{
			Name:       "checker_2px_aliased",
			Wellposed:  false,
			Render:     func(w, h int) *Image { return renderChecker(w, h, 2) },
			Coverage:   func(x, y int) bool { return checkerCover(x, y, 2) },
			Foreground: func(c RGB) bool { return Luma(c) > 0.5 },
		},
		{
			Name:       "checker_1px_aliased",
			Wellposed:  false,
			Render:     func(w, h int) *Image { return renderChecker(w, h, 1) },
			Coverage:   func(x, y int) bool { return checkerCover(x, y, 1) },
			Foreground: func(c RGB) bool { return Luma(c) > 0.5 },
		},
	}
}

func norm(x, y, w, h int) (float64, float64) {
	return float64(x) / float64(w-1), float64(y) / float64(h-1)
}

// renderAnalytic combines smooth radial and sinusoidal terms so the exact
// value is well defined at every sub-pixel position.
func renderAnalytic(w, h int) *Image {
	img := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u, v := norm(x, y, w, h)
			dx, dy := u-0.5, v-0.42
			r := math.Hypot(dx, dy)
			blob := 0.5 + 0.5*math.Cos(r*math.Pi*3.2)
			wave := 0.5 + 0.5*math.Sin(u*9.0+v*5.0+math.Sin(v*7.0))
			ring := 0.5 + 0.5*math.Sin(math.Hypot(u-0.72, v-0.7)*18)
			img.Set(x, y, RGB{
				R: clamp01(0.55*blob + 0.30*wave),
				G: clamp01(0.45*wave + 0.35*ring),
				B: clamp01(0.60*ring + 0.25*blob),
			})
		}
	}
	return img
}

func analyticCover(x, y int) bool {
	u := float64(x) / 159.0
	v := float64(y) / 159.0
	return math.Hypot(u-0.5, v-0.42) < 0.26
}

// valueHash is a deterministic 2D hash in [0,1), reused by the textures.
func valueHash(ix, iy int) float64 {
	h := uint32(ix)*374761393 + uint32(iy)*668265263 + 0x9e3779b9
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h) / 4294967296.0
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func valueNoise(x, y float64) float64 {
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	fx := smooth(x - x0)
	fy := smooth(y - y0)
	ix, iy := int(x0), int(y0)
	v00 := valueHash(ix, iy)
	v10 := valueHash(ix+1, iy)
	v01 := valueHash(ix, iy+1)
	v11 := valueHash(ix+1, iy+1)
	top := v00 + (v10-v00)*fx
	bot := v01 + (v11-v01)*fx
	return top + (bot-top)*fy
}

func fbm(x, y float64) float64 {
	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for o := 0; o < 5; o++ {
		sum += amp * valueNoise(x*freq, y*freq)
		norm += amp
		amp *= 0.5
		freq *= 2.05
	}
	return sum / norm
}

func renderFBM(w, h int) *Image {
	img := NewImage(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u, v := norm(x, y, w, h)
			n1 := fbm(u*6.0, v*6.0)
			n2 := fbm(u*13.0+11.0, v*13.0-4.0)
			img.Set(x, y, RGB{
				R: clamp01(0.25 + 0.75*n1),
				G: clamp01(0.20 + 0.70*n2),
				B: clamp01(0.30 + 0.60*(n1*0.5+n2*0.5)),
			})
		}
	}
	return img
}

func fbmCover(x, y int) bool {
	u := float64(x) / 159.0
	v := float64(y) / 159.0
	return fbm(u*6.0, v*6.0) > 0.52
}
