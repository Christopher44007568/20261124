package gazedenoise

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// RenderOptions controls PNG presentation. The linear colour buffers are
// gamma-encoded and missing samples are shown in magenta.
type RenderOptions struct {
	MaskOutline bool
	Amplify     float64
}

// WriteFramePNG renders f to an 8-bit PNG.
func WriteFramePNG(path string, f *Frame, opts RenderOptions) error {
	if f == nil {
		return fmt.Errorf("nil frame")
	}
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	amplify := opts.Amplify
	if amplify <= 0 {
		amplify = 1
	}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := f.Index(x, y)
			var c color.RGBA
			if !f.Valid[i] {
				c = color.RGBA{R: 230, G: 24, B: 170, A: 255}
			} else {
				v := f.Color[i].Mul(amplify)
				c = color.RGBA{
					R: toSRGB(v.X),
					G: toSRGB(v.Y),
					B: toSRGB(v.Z),
					A: 255,
				}
			}
			if opts.MaskOutline && isMaskBoundary(f, x, y) {
				c = color.RGBA{R: 30, G: 230, B: 240, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return writePNG(path, img)
}

// WriteDifferencePNG renders signed luminance error, amplified for halo
// inspection. Bright red is above truth, blue is below truth.
func WriteDifferencePNG(path string, truth, got *Frame, scale float64) error {
	if truth == nil || got == nil {
		return fmt.Errorf("nil difference frame")
	}
	if truth.Width != got.Width || truth.Height != got.Height {
		return fmt.Errorf("difference frames have different dimensions")
	}
	if scale <= 0 {
		scale = 12
	}
	img := image.NewRGBA(image.Rect(0, 0, truth.Width, truth.Height))
	for y := 0; y < truth.Height; y++ {
		for x := 0; x < truth.Width; x++ {
			i := truth.Index(x, y)
			e := 0.0
			if got.Valid[i] {
				e = (got.Color[i].Luma() - truth.Color[i].Luma()) * scale
			}
			e = Clamp(e, -1, 1)
			var c color.RGBA
			if e >= 0 {
				c = color.RGBA{
					R: uint8(255 * Clamp01(e)),
					G: uint8(255 * Clamp01(1-e)),
					B: uint8(255 * Clamp01(1-e)),
					A: 255,
				}
			} else {
				c = color.RGBA{
					R: uint8(255 * Clamp01(1+e)),
					G: uint8(255 * Clamp01(1+e)),
					B: uint8(255 * Clamp01(-e)),
					A: 255,
				}
			}
			if isMaskBoundary(truth, x, y) {
				c = color.RGBA{R: 40, G: 220, B: 250, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return writePNG(path, img)
}

// WriteNormalPNG maps normal components from [-1,1] to RGB.
func WriteNormalPNG(path string, f *Frame, opts RenderOptions) error {
	if err := f.Validate(); err != nil {
		return err
	}
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := f.Index(x, y)
			n := f.Normal[i].Normalized()
			c := color.RGBA{
				R: uint8(math.Round(255 * Clamp01(0.5+0.5*n.X))),
				G: uint8(math.Round(255 * Clamp01(0.5+0.5*n.Y))),
				B: uint8(math.Round(255 * Clamp01(0.5+0.5*n.Z))),
				A: 255,
			}
			if opts.MaskOutline && isMaskBoundary(f, x, y) {
				c = color.RGBA{R: 30, G: 230, B: 240, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return writePNG(path, img)
}

// WriteScalarPNG renders a scalar guide with a blue-cyan-yellow-red map.
func WriteScalarPNG(path string, f *Frame, values []float64, minValue, maxValue float64, opts RenderOptions) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if len(values) != len(f.Color) {
		return fmt.Errorf("scalar buffer has %d values, want %d", len(values), len(f.Color))
	}
	if maxValue <= minValue {
		minValue, maxValue = scalarRange(values)
	}
	span := maxValue - minValue
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := f.Index(x, y)
			t := 0.5
			if span > 0 {
				t = Clamp01((values[i] - minValue) / span)
			}
			c := heatColour(t)
			if opts.MaskOutline && isMaskBoundary(f, x, y) {
				c = color.RGBA{R: 30, G: 230, B: 240, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return writePNG(path, img)
}

func scalarRange(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 1
	}
	lo, hi := values[0], values[0]
	for _, value := range values[1:] {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		if value < lo {
			lo = value
		}
		if value > hi {
			hi = value
		}
	}
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

func heatColour(t float64) color.RGBA {
	t = Clamp01(t)
	return color.RGBA{
		R: uint8(math.Round(255 * Clamp01(1.5-math.Abs(4*t-3)))),
		G: uint8(math.Round(255 * Clamp01(1.5-math.Abs(4*t-2)))),
		B: uint8(math.Round(255 * Clamp01(1.5-math.Abs(4*t-1)))),
		A: 255,
	}
}

// WriteProvenancePNG visualizes hole targets, their recorded source pixels,
// and the gaze boundary. Targets are magenta; sources are green.
func WriteProvenancePNG(path string, scene *Scene, result *Result) error {
	if scene == nil || result == nil || result.Output == nil {
		return fmt.Errorf("incomplete provenance input")
	}
	out := result.Output
	img := image.NewRGBA(image.Rect(0, 0, out.Width, out.Height))
	for y := 0; y < out.Height; y++ {
		for x := 0; x < out.Width; x++ {
			i := out.Index(x, y)
			v := 40 + 150*Clamp01(out.Color[i].Luma())
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(v * 0.38),
				G: uint8(v * 0.42),
				B: uint8(v * 0.48),
				A: 255,
			})
		}
	}
	for _, prov := range result.Provenance {
		for _, source := range prov.Sources {
			mark(img, source.X, source.Y, 1, color.RGBA{R: 40, G: 245, B: 90, A: 255})
		}
	}
	for _, prov := range result.Provenance {
		mark(img, prov.X, prov.Y, 1, color.RGBA{R: 250, G: 40, B: 210, A: 255})
	}
	for y := 0; y < out.Height; y++ {
		for x := 0; x < out.Width; x++ {
			if isMaskBoundary(out, x, y) {
				img.SetRGBA(x, y, color.RGBA{R: 30, G: 220, B: 250, A: 255})
			}
		}
	}
	return writePNG(path, img)
}

func mark(img *image.RGBA, x, y, radius int, c color.RGBA) {
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			px, py := x+dx, y+dy
			if image.Pt(px, py).In(img.Bounds()) {
				img.SetRGBA(px, py, c)
			}
		}
	}
}

func isMaskBoundary(f *Frame, x, y int) bool {
	i := f.Index(x, y)
	inside := f.Mask[i]
	for _, offset := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		nx, ny := x+offset[0], y+offset[1]
		if f.InBounds(nx, ny) && f.Mask[f.Index(nx, ny)] != inside {
			return true
		}
	}
	return false
}

func toSRGB(v float64) uint8 {
	v = Clamp01(v)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(Clamp01(v) * 255))
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}
