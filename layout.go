package eyebuffer

import (
	"fmt"
	"image"
	"math"
)

// BytesPerPixel is fixed because the atlas is stored as RGBA8. The format is
// intentionally kept separate from the layout so row-stride bugs are visible.
const BytesPerPixel = 4

// EyeSpec describes one eye. Width and Height are logical pixels; Scale in
// LayoutOptions turns them into physical atlas pixels.
type EyeSpec struct {
	Name    string
	Width   int
	Height  int
	Mipmaps bool
}

// LayoutOptions controls packing. A zero value means RGBA8, a 4-byte row
// alignment, no gutter, and a scale of one.
type LayoutOptions struct {
	Gutter    int
	Alignment int
	Scale     float64
}

// MipLevel is one eye image and mip level placed in the atlas.
type MipLevel struct {
	Eye          string
	EyeIndex     int
	Level        int
	Width        int
	Height       int
	Rect         image.Rectangle
	PackedStride int
	ByteOffset   int
}

// EyeLayout is an independent viewport for one eye. Slot includes the gutter
// reserved around Base and all mip levels.
type EyeLayout struct {
	Name           string
	Index          int
	LogicalWidth   int
	LogicalHeight  int
	PixelWidth     int
	PixelHeight    int
	Slot           image.Rectangle
	Levels         []MipLevel
	BaseRowStride  int
	MipmapsEnabled bool
}

// AtlasLayout contains the atlas dimensions and every viewport. Width excludes
// row padding; Stride is the physical number of bytes between adjacent rows.
type AtlasLayout struct {
	Width     int
	Height    int
	Stride    int
	Alignment int
	Gutter    int
	Scale     float64
	Eyes      []EyeLayout
}

// AlignUp rounds value up to the next multiple of alignment.
func AlignUp(value, alignment int) int {
	if alignment <= 0 {
		return value
	}
	return (value + alignment - 1) / alignment * alignment
}

// RowStride returns the aligned packed byte width for a pixel row.
func RowStride(width, alignment int) int {
	return AlignUp(width*BytesPerPixel, alignment)
}

// MipChain returns the full mip dimensions including the base level.
func MipChain(width, height int, enabled bool) []image.Point {
	if width <= 0 || height <= 0 {
		return nil
	}
	levels := []image.Point{{X: width, Y: height}}
	if !enabled {
		return levels
	}
	for width > 1 || height > 1 {
		width = max(1, width/2)
		height = max(1, height/2)
		levels = append(levels, image.Pt(width, height))
	}
	return levels
}

// BuildLayout allocates a horizontal slot for every eye. Mip levels are
// stacked below their base level, never packed into the neighbouring eye.
func BuildLayout(eyes []EyeSpec, opts LayoutOptions) (AtlasLayout, error) {
	if len(eyes) == 0 {
		return AtlasLayout{}, fmt.Errorf("at least one eye is required")
	}
	if opts.Gutter < 0 {
		return AtlasLayout{}, fmt.Errorf("gutter must be non-negative, got %d", opts.Gutter)
	}
	if opts.Alignment == 0 {
		opts.Alignment = 4
	}
	if opts.Alignment < 1 {
		return AtlasLayout{}, fmt.Errorf("alignment must be positive, got %d", opts.Alignment)
	}
	if opts.Alignment%4 != 0 {
		return AtlasLayout{}, fmt.Errorf("alignment must be a multiple of 4 bytes for RGBA8, got %d", opts.Alignment)
	}
	if opts.Scale == 0 {
		opts.Scale = 1
	}
	if opts.Scale < 0 || math.IsNaN(opts.Scale) || math.IsInf(opts.Scale, 0) {
		return AtlasLayout{}, fmt.Errorf("scale must be finite and positive, got %v", opts.Scale)
	}

	type pending struct {
		eye   EyeLayout
		sizes []image.Point
		slotW int
		slotH int
	}
	items := make([]pending, 0, len(eyes))
	seen := make(map[string]bool, len(eyes))
	for i, spec := range eyes {
		if spec.Name == "" {
			return AtlasLayout{}, fmt.Errorf("eye %d has an empty name", i)
		}
		if seen[spec.Name] {
			return AtlasLayout{}, fmt.Errorf("duplicate eye name %q", spec.Name)
		}
		seen[spec.Name] = true
		if spec.Width <= 0 || spec.Height <= 0 {
			return AtlasLayout{}, fmt.Errorf("eye %q has invalid size %dx%d", spec.Name, spec.Width, spec.Height)
		}

		pw := max(1, int(math.Ceil(float64(spec.Width)*opts.Scale)))
		ph := max(1, int(math.Ceil(float64(spec.Height)*opts.Scale)))
		sizes := MipChain(pw, ph, spec.Mipmaps)
		maxW := 0
		sumH := 0
		for _, size := range sizes {
			maxW = max(maxW, size.X)
			sumH += size.Y
		}
		slotW := maxW + opts.Gutter*2
		slotH := sumH + opts.Gutter*(len(sizes)+1)
		items = append(items, pending{
			eye: EyeLayout{
				Name:           spec.Name,
				Index:          i,
				LogicalWidth:   spec.Width,
				LogicalHeight:  spec.Height,
				PixelWidth:     pw,
				PixelHeight:    ph,
				BaseRowStride:  RowStride(pw, opts.Alignment),
				MipmapsEnabled: spec.Mipmaps,
			},
			sizes: sizes,
			slotW: slotW,
			slotH: slotH,
		})
	}

	atlasW, atlasH := 0, 0
	for _, item := range items {
		atlasW += item.slotW
		atlasH = max(atlasH, item.slotH)
	}
	stride := RowStride(atlasW, opts.Alignment)

	layout := AtlasLayout{
		Width:     atlasW,
		Height:    atlasH,
		Stride:    stride,
		Alignment: opts.Alignment,
		Gutter:    opts.Gutter,
		Scale:     opts.Scale,
		Eyes:      make([]EyeLayout, 0, len(items)),
	}
	x := 0
	for _, item := range items {
		eye := item.eye
		eye.Slot = image.Rect(x, 0, x+item.slotW, item.slotH)
		contentX := eye.Slot.Min.X + opts.Gutter
		y := eye.Slot.Min.Y + opts.Gutter
		eye.Levels = make([]MipLevel, 0, len(item.sizes))
		for level, size := range item.sizes {
			rect := image.Rect(contentX, y, contentX+size.X, y+size.Y)
			eye.Levels = append(eye.Levels, MipLevel{
				Eye:          eye.Name,
				EyeIndex:     eye.Index,
				Level:        level,
				Width:        size.X,
				Height:       size.Y,
				Rect:         rect,
				PackedStride: RowStride(size.X, opts.Alignment),
				ByteOffset:   rect.Min.Y*stride + rect.Min.X*BytesPerPixel,
			})
			y += size.Y + opts.Gutter
		}
		layout.Eyes = append(layout.Eyes, eye)
		x += item.slotW
	}
	return layout, nil
}

// Level returns an eye mip level, if it exists.
func (l AtlasLayout) Level(eyeIndex, level int) (MipLevel, bool) {
	if eyeIndex < 0 || eyeIndex >= len(l.Eyes) {
		return MipLevel{}, false
	}
	levels := l.Eyes[eyeIndex].Levels
	if level < 0 || level >= len(levels) {
		return MipLevel{}, false
	}
	return levels[level], true
}
