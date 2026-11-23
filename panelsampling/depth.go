package panelsampling

import (
	"errors"
	"math"
)

// DepthBuffer stores only explicitly valid depth samples. An invalid sample
// can neither create a new depth nor overwrite an existing one.
type DepthBuffer struct {
	width  int
	height int
	depth  []float32
	valid  []bool
}

// NewDepthBuffer creates an empty destination depth buffer. Smaller depth
// values are considered nearer.
func NewDepthBuffer(width, height int) (*DepthBuffer, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("panelsampling: depth dimensions must be positive")
	}
	if width > math.MaxInt/height {
		return nil, errors.New("panelsampling: depth dimensions overflow")
	}
	size := width * height
	return &DepthBuffer{
		width:  width,
		height: height,
		depth:  make([]float32, size),
		valid:  make([]bool, size),
	}, nil
}

// Width returns the destination width.
func (d *DepthBuffer) Width() int {
	if d == nil {
		return 0
	}
	return d.width
}

// Height returns the destination height.
func (d *DepthBuffer) Height() int {
	if d == nil {
		return 0
	}
	return d.height
}

// Store writes a sample if and only if both the sample and its depth are
// valid. It returns true only when a new nearest depth was written.
func (d *DepthBuffer) Store(x, y int, sample Sample) bool {
	if d == nil || !sample.Valid || !sample.HasDepth {
		return false
	}
	value := float64(sample.Depth)
	if math.IsNaN(value) || math.IsInf(value, 0) || sample.Depth < 0 {
		return false
	}
	if x < 0 || y < 0 || x >= d.width || y >= d.height {
		return false
	}

	index := y*d.width + x
	if d.valid[index] && sample.Depth >= d.depth[index] {
		return false
	}
	d.depth[index] = sample.Depth
	d.valid[index] = true
	return true
}

// Lookup returns a stored depth and whether that depth is valid.
func (d *DepthBuffer) Lookup(x, y int) (float32, bool) {
	if d == nil || x < 0 || y < 0 || x >= d.width || y >= d.height {
		return 0, false
	}
	index := y*d.width + x
	if !d.valid[index] {
		return 0, false
	}
	return d.depth[index], true
}

// Clear removes all valid depth values.
func (d *DepthBuffer) Clear() {
	if d == nil {
		return
	}
	for index := range d.valid {
		d.depth[index] = 0
		d.valid[index] = false
	}
}
