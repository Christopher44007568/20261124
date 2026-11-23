package foveal

import "fmt"

// DefaultParams returns a sensible configuration for the given frame size.
// The fovea converges with an effective history of 32 frames; the periphery
// only keeps 4 equivalent samples so moving/aging content is dropped quickly.
func DefaultParams(width, height int) Params {
	return Params{
		Width:                      width,
		Height:                     height,
		FoveaRadius:                float32(max(width, height)) * 0.12,
		GuardWidth:                 float32(max(width, height)) * 0.02,
		FovealHistory:              32,
		PeripheralHistory:          4,
		SaccadeSpeed:               float32(max(width, height)) * 0.10,
		SaccadeCorridor:            float32(max(width, height)) * 0.03,
		MotionThreshold:            0.75,
		DivergenceThreshold:        1.5,
		DepthDisocclusionThreshold: 0.05,
		ExposureResetThreshold:     2.0,
		HighlightThreshold:         1.2,
		HighlightTolerance:         0.35,
		ClampExpand:                0.10,
	}
}

func (p *Params) normalize() error {
	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("foveal: non-positive frame size %dx%d", p.Width, p.Height)
	}
	if p.FoveaRadius <= 0 {
		p.FoveaRadius = float32(max(p.Width, p.Height)) * 0.12
	}
	if p.GuardWidth < 0 {
		p.GuardWidth = 0
	}
	if p.FovealHistory < 1 {
		p.FovealHistory = 1
	}
	if p.PeripheralHistory < 1 {
		p.PeripheralHistory = 1
	}
	if p.SaccadeSpeed <= 0 {
		p.SaccadeSpeed = float32(max(p.Width, p.Height)) * 0.10
	}
	if p.SaccadeCorridor < 0 {
		p.SaccadeCorridor = 0
	}
	if p.MotionThreshold < 0 {
		p.MotionThreshold = 0
	}
	if p.DivergenceThreshold < 0 {
		p.DivergenceThreshold = 0
	}
	if p.ExposureResetThreshold <= 0 {
		p.ExposureResetThreshold = 2
	}
	if p.HighlightThreshold < 0 {
		p.HighlightThreshold = 0
	}
	if p.HighlightTolerance < 0 {
		p.HighlightTolerance = 0
	}
	if p.ClampExpand < 0 {
		p.ClampExpand = 0
	}
	return nil
}
