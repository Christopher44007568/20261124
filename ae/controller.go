package ae

import (
	"fmt"
	"math"
)

type ControlConfig struct {
	Target      float64
	Gain        float64
	MaxStepEV   float64
	MinMeasured float64
}

func DefaultControlConfig() ControlConfig {
	return ControlConfig{
		Target:      0.18,
		Gain:        0.5,
		MaxStepEV:   1.0,
		MinMeasured: 1e-4,
	}
}

func (c ControlConfig) Validate() error {
	if c.Target <= 0 || c.Target > 1 {
		return fmt.Errorf("target %g is outside (0,1]", c.Target)
	}
	if c.Gain <= 0 || c.Gain > 1 {
		return fmt.Errorf("gain %g is outside (0,1]", c.Gain)
	}
	if c.MaxStepEV <= 0 {
		return fmt.Errorf("max step EV must be positive")
	}
	if c.MinMeasured <= 0 {
		return fmt.Errorf("minimum measured value must be positive")
	}
	return nil
}

type Controller struct {
	cfg ControlConfig
	ev  float64
}

func NewController(cfg ControlConfig, initialEV float64) (*Controller, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Controller{cfg: cfg, ev: initialEV}, nil
}

func (c *Controller) EV() float64 { return c.ev }

func (c *Controller) SetEV(ev float64) { c.ev = ev }

// Step advances the exposure by a bounded log-domain correction. A gain below
// one damps the loop; MaxStepEV limits the correction after a large jump.
func (c *Controller) Step(measured float64) float64 {
	if measured < c.cfg.MinMeasured {
		measured = c.cfg.MinMeasured
	}
	errorStops := math.Log2(c.cfg.Target / measured)
	step := c.cfg.Gain * errorStops
	if step > c.cfg.MaxStepEV {
		step = c.cfg.MaxStepEV
	}
	if step < -c.cfg.MaxStepEV {
		step = -c.cfg.MaxStepEV
	}
	c.ev += step
	return c.ev
}

// ApplyExposure returns a copy of frame with its scene luminance multiplied by
// the exposure gain. The result is clipped to the sensor's [0,1] range.
func ApplyExposure(frame Frame, ev float64) Frame {
	exposed := frame
	exposed.Luma = make([]float64, len(frame.Luma))
	gain := math.Exp2(ev)
	for i, value := range frame.Luma {
		value *= gain
		if value > 1 {
			value = 1
		}
		exposed.Luma[i] = value
	}
	return exposed
}
