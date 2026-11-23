package ae

import (
	"errors"
	"fmt"
)

// Frame is a single luminance frame. Luma values use a normalized [0, 1]
// range. SceneCut marks the first frame of a new scene: meters must not reuse
// metering state from an earlier scene when this flag is set.
type Frame struct {
	Width    int
	Height   int
	Luma     []float64
	SceneCut bool
	Label    string
}

// NewUniformFrame returns a frame filled with value.
func NewUniformFrame(width, height int, value float64, sceneCut bool) Frame {
	frame := Frame{
		Width:    width,
		Height:   height,
		Luma:     make([]float64, width*height),
		SceneCut: sceneCut,
	}
	for i := range frame.Luma {
		frame.Luma[i] = value
	}
	return frame
}

func (f Frame) Validate() error {
	if f.Width <= 0 || f.Height <= 0 {
		return fmt.Errorf("%w: invalid frame size %dx%d", ErrInvalidFrame, f.Width, f.Height)
	}
	if len(f.Luma) != f.Width*f.Height {
		return fmt.Errorf("%w: luma length %d does not match %dx%d",
			ErrInvalidFrame, len(f.Luma), f.Width, f.Height)
	}
	for i, value := range f.Luma {
		if !(value >= 0 && value <= 1) {
			return fmt.Errorf("%w: luma[%d]=%g is outside [0,1]", ErrInvalidFrame, i, value)
		}
	}
	return nil
}

var ErrInvalidFrame = errors.New("invalid frame")
