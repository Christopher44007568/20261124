package lensdistortion

import (
	"fmt"
	"math"
)

// SampleMode controls how mapped source coordinates are validated.
type SampleMode uint8

const (
	// SampleNearest needs one valid source pixel.
	SampleNearest SampleMode = iota
	// SampleBilinear needs the full four-pixel interpolation footprint. A
	// mapped point whose fractional footprint crosses the source edge is a
	// boundary hole even if the point itself is nominally in bounds.
	SampleBilinear
	// SampleClamp clamps a finite source coordinate to the source edge before
	// bilinear sampling. It never reads outside the source, but any point that
	// needed clamping is still reported as a source-boundary failure.
	SampleClamp
)

func (m SampleMode) String() string {
	switch m {
	case SampleNearest:
		return "nearest"
	case SampleBilinear:
		return "bilinear"
	case SampleClamp:
		return "clamp"
	default:
		return "invalid"
	}
}

// PixelStatus describes whether an output pixel could be sampled.
type PixelStatus uint8

const (
	PixelOutsideRegion PixelStatus = iota
	PixelMapped
	PixelClamped
	PixelModelOutOfDomain
	PixelOutOfSource
)

func (s PixelStatus) String() string {
	switch s {
	case PixelOutsideRegion:
		return "outside-region"
	case PixelMapped:
		return "mapped"
	case PixelClamped:
		return "clamped"
	case PixelModelOutOfDomain:
		return "model-out-of-domain"
	case PixelOutOfSource:
		return "out-of-source"
	default:
		return "invalid"
	}
}

type CoverageOptions struct {
	// Width and Height are output pixel counts. Pixel centers are integer
	// coordinates 0..Width-1 and 0..Height-1.
	Width, Height int
	// SourceWidth and SourceHeight define valid source pixel indices
	// 0..SourceWidth-1 and 0..SourceHeight-1.
	SourceWidth, SourceHeight int
	// Region masks the output. nil means the full output rectangle.
	Region Region
	// Sampling defaults to SampleNearest.
	Sampling SampleMode
}

// CoverageReport records one status per output pixel.
type CoverageReport struct {
	Width, Height int
	Status        []PixelStatus

	RegionPixels    int
	SampledPixels   int
	ClampedPixels   int
	ModelOutPixels  int
	SourceOutPixels int
	HolePixels      int
}

func (r CoverageReport) At(x, y int) PixelStatus {
	if x < 0 || x >= r.Width || y < 0 || y >= r.Height {
		return PixelOutsideRegion
	}
	return r.Status[y*r.Width+x]
}

func (r CoverageReport) HoleAt(x, y int) bool {
	status := r.At(x, y)
	return status == PixelModelOutOfDomain || status == PixelOutOfSource
}

// Validate fails when any masked output pixel has no sample source. With
// SampleClamp, clamped points still fail because the raw mapping was outside
// the source; callers that intentionally use edge replication can inspect the
// counters instead.
func (r CoverageReport) Validate() error {
	if r.HolePixels == 0 && r.ModelOutPixels == 0 && r.SourceOutPixels == 0 {
		return nil
	}
	return &CoverageError{
		ModelOut:  r.ModelOutPixels,
		SourceOut: r.SourceOutPixels,
		Holes:     r.HolePixels,
	}
}

type CoverageError struct {
	ModelOut  int
	SourceOut int
	Holes     int
}

func (e *CoverageError) Error() string {
	return fmt.Sprintf("lensdistortion: coverage has %d model-domain failures, %d source-boundary failures and %d holes", e.ModelOut, e.SourceOut, e.Holes)
}

func AnalyzeCoverage(mapper Mapper, options CoverageOptions) (CoverageReport, error) {
	if mapper == nil || options.Width <= 0 || options.Height <= 0 ||
		options.SourceWidth <= 0 || options.SourceHeight <= 0 ||
		options.Sampling > SampleClamp {
		return CoverageReport{}, ErrInvalidCoverage
	}
	region := options.Region
	if region == nil {
		region = RectRegion{Rect: Rect{
			Min: Point{X: 0, Y: 0},
			Max: Point{X: float64(options.Width - 1), Y: float64(options.Height - 1)},
		}}
	}
	report := CoverageReport{
		Width:  options.Width,
		Height: options.Height,
		Status: make([]PixelStatus, options.Width*options.Height),
	}
	for y := 0; y < options.Height; y++ {
		for x := 0; x < options.Width; x++ {
			display := Point{X: float64(x), Y: float64(y)}
			index := y*options.Width + x
			if !region.Contains(display) {
				report.Status[index] = PixelOutsideRegion
				continue
			}
			report.RegionPixels++
			source, ok := mapper.SourceFromDisplay(display)
			if !ok || !source.IsFinite() {
				report.Status[index] = PixelModelOutOfDomain
				report.ModelOutPixels++
				report.HolePixels++
				continue
			}
			status := sourceStatus(source, options)
			report.Status[index] = status
			switch status {
			case PixelMapped:
				report.SampledPixels++
			case PixelClamped:
				report.SampledPixels++
				report.ClampedPixels++
				report.SourceOutPixels++
			case PixelOutOfSource:
				report.SourceOutPixels++
				report.HolePixels++
			}
		}
	}
	return report, nil
}

func sourceStatus(source Point, options CoverageOptions) PixelStatus {
	mode := options.Sampling
	if mode == SampleClamp {
		if sampleable(source, options.SourceWidth, options.SourceHeight, SampleBilinear) {
			return PixelMapped
		}
		return PixelClamped
	}
	if sampleable(source, options.SourceWidth, options.SourceHeight, mode) {
		return PixelMapped
	}
	return PixelOutOfSource
}

func sampleable(p Point, width, height int, mode SampleMode) bool {
	if !p.IsFinite() {
		return false
	}
	if mode == SampleClamp {
		return true
	}
	roundX, roundY := math.Round(p.X), math.Round(p.Y)
	if mode == SampleNearest {
		return roundX >= 0 && roundX <= float64(width-1) &&
			roundY >= 0 && roundY <= float64(height-1)
	}
	return bilinearFootprintWithin(p.X, width) && bilinearFootprintWithin(p.Y, height)
}

func bilinearFootprintWithin(value float64, count int) bool {
	maxIndex := float64(count - 1)
	if value < 0 || value > maxIndex {
		return false
	}
	base := math.Floor(value)
	if value == base {
		return base >= 0 && base <= maxIndex
	}
	return base >= 0 && base+1 <= maxIndex
}
