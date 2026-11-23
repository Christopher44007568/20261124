package chroma

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

type rgbaSample struct {
	R float64
	G float64
	B float64
	A float64
}

func (s rgbaSample) forChannel(ch Channel) float64 {
	switch ch {
	case Red:
		return s.R
	case Green:
		return s.G
	case Blue:
		return s.B
	default:
		return 0
	}
}

// Result is one per-eye output and the three independently warped coverage
// masks. Mask images must not be copied from one channel to another: a mask
// that is aligned for red is already misaligned for green and blue.
type Result struct {
	Image *image.RGBA
	Masks [3]*image.Gray
}

// Warp samples src through the per-channel radial model. mask may be nil, in
// which case every channel has full coverage. When mask is non-nil it is
// sampled at each channel's own source coordinate and emitted three times in
// Result.Masks.
func Warp(src *image.RGBA, mask *image.Gray, cfg Config) (Result, error) {
	if src == nil {
		return Result{}, fmt.Errorf("source image is nil")
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	mapper, err := NewMapper(width, height, cfg)
	if err != nil {
		return Result{}, err
	}
	if mask != nil {
		maskBounds := mask.Bounds()
		if maskBounds.Dx() != width || maskBounds.Dy() != height {
			return Result{}, fmt.Errorf("mask size %dx%d does not match source %dx%d",
				maskBounds.Dx(), maskBounds.Dy(), width, height)
		}
	}

	out := image.NewRGBA(image.Rect(0, 0, width, height))
	result := Result{Image: out}
	for i := range result.Masks {
		result.Masks[i] = image.NewGray(image.Rect(0, 0, width, height))
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var channelValues [3]float64
			var effectiveAlpha [3]float64

			for ch := Red; ch <= Blue; ch++ {
				point := mapper.MapPixel(x, y, ch)
				sampleX := point.X + float64(bounds.Min.X)
				sampleY := point.Y + float64(bounds.Min.Y)
				sampled := sampleRGBA(src, sampleX, sampleY, cfg.Edge)
				coverage := 1.0
				if mask != nil {
					maskBounds := mask.Bounds()
					coverage = sampleGray(mask,
						point.X+float64(maskBounds.Min.X),
						point.Y+float64(maskBounds.Min.Y),
						cfg.Edge,
					)
				}

				maskOffset := result.Masks[ch].PixOffset(x, y)
				result.Masks[ch].Pix[maskOffset] = toByte(coverage)

				channelValues[ch] = sampled.forChannel(ch) * coverage
				effectiveAlpha[ch] = sampled.A * coverage
			}

			offset := out.PixOffset(x, y)
			out.Pix[offset+0] = toByte(channelValues[Red])
			out.Pix[offset+1] = toByte(channelValues[Green])
			out.Pix[offset+2] = toByte(channelValues[Blue])
			out.Pix[offset+3] = 255
			if cfg.Edge == EdgeTransparent {
				out.Pix[offset+3] = toByte(max3(effectiveAlpha[0], effectiveAlpha[1], effectiveAlpha[2]))
			}
		}
	}

	return result, nil
}

// WarpImage is Warp without a coverage mask.
func WarpImage(src *image.RGBA, cfg Config) (*image.RGBA, error) {
	result, err := Warp(src, nil, cfg)
	if err != nil {
		return nil, err
	}
	return result.Image, nil
}

// SampleRGBA exposes the sampler used by Warp. The boolean reports whether the
// sample position was inside source bounds; edge mode still determines the
// returned value when it is false.
func SampleRGBA(src *image.RGBA, x, y float64, edge EdgeMode) (color.RGBA, bool) {
	sample := sampleRGBA(src, x, y, edge)
	return color.RGBA{
		R: toByte(sample.R),
		G: toByte(sample.G),
		B: toByte(sample.B),
		A: toByte(sample.A),
	}, src != nil && inBounds(src.Bounds(), x, y)
}

func sampleRGBA(src *image.RGBA, x, y float64, edge EdgeMode) rgbaSample {
	if src == nil {
		return rgbaSample{}
	}
	bounds := src.Bounds()
	if !inBounds(bounds, x, y) {
		switch edge {
		case EdgeClamp:
			x = clampFloat(x, 0, float64(bounds.Dx()-1))
			y = clampFloat(y, 0, float64(bounds.Dy()-1))
		default:
			return rgbaSample{}
		}
	}

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)

	var result rgbaSample
	for dy := 0; dy <= 1; dy++ {
		for dx := 0; dx <= 1; dx++ {
			weight := (1 - fx) * (1 - fy)
			if dx == 1 {
				weight = fx * (1 - fy)
			}
			if dy == 1 {
				weight = (1 - fx) * fy
			}
			if dx == 1 && dy == 1 {
				weight = fx * fy
			}
			if weight == 0 {
				continue
			}
			tap, ok := rgbaTap(src, x0+dx, y0+dy, edge)
			if !ok {
				continue
			}
			result.R += tap.R * weight
			result.G += tap.G * weight
			result.B += tap.B * weight
			result.A += tap.A * weight
		}
	}
	return result
}

func rgbaTap(src *image.RGBA, x, y int, edge EdgeMode) (rgbaSample, bool) {
	bounds := src.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		if edge != EdgeClamp {
			return rgbaSample{}, false
		}
		x = int(clampFloat(float64(x), float64(bounds.Min.X), float64(bounds.Max.X-1)))
		y = int(clampFloat(float64(y), float64(bounds.Min.Y), float64(bounds.Max.Y-1)))
	}
	offset := src.PixOffset(x, y)
	scale := 1.0 / 255.0
	return rgbaSample{
		R: float64(src.Pix[offset+0]) * scale,
		G: float64(src.Pix[offset+1]) * scale,
		B: float64(src.Pix[offset+2]) * scale,
		A: float64(src.Pix[offset+3]) * scale,
	}, true
}

func sampleGray(mask *image.Gray, x, y float64, edge EdgeMode) float64 {
	bounds := mask.Bounds()
	if !inBounds(bounds, x, y) {
		switch edge {
		case EdgeClamp:
			x = clampFloat(x, 0, float64(bounds.Dx()-1))
			y = clampFloat(y, 0, float64(bounds.Dy()-1))
		default:
			return 0
		}
	}

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	var result float64
	for dy := 0; dy <= 1; dy++ {
		for dx := 0; dx <= 1; dx++ {
			weight := (1 - fx) * (1 - fy)
			if dx == 1 {
				weight = fx * (1 - fy)
			}
			if dy == 1 {
				weight = (1 - fx) * fy
			}
			if dx == 1 && dy == 1 {
				weight = fx * fy
			}
			if weight == 0 {
				continue
			}
			tap, ok := grayTap(mask, x0+dx, y0+dy, edge)
			if ok {
				result += tap * weight
			}
		}
	}
	return result
}

func grayTap(mask *image.Gray, x, y int, edge EdgeMode) (float64, bool) {
	bounds := mask.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		if edge != EdgeClamp {
			return 0, false
		}
		x = int(clampFloat(float64(x), float64(bounds.Min.X), float64(bounds.Max.X-1)))
		y = int(clampFloat(float64(y), float64(bounds.Min.Y), float64(bounds.Max.Y-1)))
	}
	return float64(mask.Pix[mask.PixOffset(x, y)]) / 255.0, true
}

func inBounds(bounds image.Rectangle, x, y float64) bool {
	return x >= float64(bounds.Min.X) && x <= float64(bounds.Max.X-1) &&
		y >= float64(bounds.Min.Y) && y <= float64(bounds.Max.Y-1)
}

func clampFloat(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func toByte(v float64) uint8 {
	v = clampFloat(v, 0, 1)
	return uint8(math.Round(v * 255))
}

func max3(a, b, c float64) float64 {
	return math.Max(a, math.Max(b, c))
}
