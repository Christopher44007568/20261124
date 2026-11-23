// Package chroma implements RGB-channel-aware radial lens warping without
// depending on a graphics SDK. It is intended for diagnosing and calibrating
// chromatic aberration in software-rendered images.
package chroma

import (
	"fmt"
	"math"
)

// Channel identifies one of the three color channels.
type Channel uint8

const (
	Red Channel = iota
	Green
	Blue
)

func (c Channel) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	case Blue:
		return "blue"
	default:
		return fmt.Sprintf("channel(%d)", c)
	}
}

func (c Channel) valid() bool {
	return c <= Blue
}

// Point is a position in normalized field-of-view coordinates. The optical
// center is (0, 0), and the unshifted image corners are at (+/-1, +/-1).
type Point struct {
	X float64
	Y float64
}

// Aberration holds one independent radial sampling coefficient per color
// channel. A positive coefficient samples farther from the optical center, so
// rendered content moves inward; a negative coefficient samples closer and
// moves content outward.
type Aberration struct {
	Red   float64
	Green float64
	Blue  float64
}

// For returns the coefficient for ch.
func (a Aberration) For(ch Channel) float64 {
	switch ch {
	case Red:
		return a.Red
	case Green:
		return a.Green
	case Blue:
		return a.Blue
	default:
		return 0
	}
}

// IsZero reports whether all three channel coefficients are zero.
func (a Aberration) IsZero() bool {
	return a.Red == 0 && a.Green == 0 && a.Blue == 0
}

// Add returns the component-wise sum of a and b.
func (a Aberration) Add(b Aberration) Aberration {
	return Aberration{
		Red:   a.Red + b.Red,
		Green: a.Green + b.Green,
		Blue:  a.Blue + b.Blue,
	}
}

// Validate rejects non-finite coefficients. Large but finite coefficients are
// allowed because calibration fixtures sometimes intentionally overshoot.
func (a Aberration) Validate() error {
	values := [...]struct {
		name  string
		value float64
	}{
		{"red", a.Red},
		{"green", a.Green},
		{"blue", a.Blue},
	}
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) {
			return fmt.Errorf("%s aberration coefficient is not finite", item.name)
		}
	}
	return nil
}

// Eye identifies the monocular calibration slot.
type Eye uint8

const (
	LeftEye Eye = iota
	RightEye
)

func (e Eye) String() string {
	switch e {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	default:
		return fmt.Sprintf("eye(%d)", e)
	}
}

func (e Eye) valid() bool {
	return e <= RightEye
}

// EyeCalibration is the per-eye manufacturing or user calibration for one
// monocular view. CenterOffset is expressed in normalized field-of-view units.
// RadialBias is added to the base channel coefficients for that eye only.
type EyeCalibration struct {
	CenterOffset Point
	RadialBias   Aberration
}

// Validate rejects non-finite calibration values.
func (e EyeCalibration) Validate() error {
	if math.IsNaN(e.CenterOffset.X) || math.IsInf(e.CenterOffset.X, 0) ||
		math.IsNaN(e.CenterOffset.Y) || math.IsInf(e.CenterOffset.Y, 0) {
		return fmt.Errorf("calibration center offset is not finite")
	}
	if err := e.RadialBias.Validate(); err != nil {
		return fmt.Errorf("calibration radial bias: %w", err)
	}
	return nil
}

// Calibration stores independent calibration data for each eye. This keeps a
// bias measured on one eye from leaking into the other eye or being reused for
// all three color channels.
type Calibration struct {
	Left  EyeCalibration
	Right EyeCalibration
}

// For returns the calibration for eye.
func (c Calibration) For(eye Eye) EyeCalibration {
	if eye == RightEye {
		return c.Right
	}
	return c.Left
}

// EdgeMode selects how samples outside the source image are handled.
type EdgeMode uint8

const (
	// EdgeBlack treats out-of-bounds taps as opaque black. This is useful for
	// inspecting the black-background result of an over-expanded channel.
	EdgeBlack EdgeMode = iota
	// EdgeClamp clamps taps to the nearest source pixel.
	EdgeClamp
	// EdgeTransparent treats out-of-bounds taps as transparent black.
	EdgeTransparent
)

func (m EdgeMode) String() string {
	switch m {
	case EdgeBlack:
		return "black"
	case EdgeClamp:
		return "clamp"
	case EdgeTransparent:
		return "transparent"
	default:
		return fmt.Sprintf("edge-mode(%d)", m)
	}
}

func (m EdgeMode) valid() bool {
	return m <= EdgeTransparent
}

// Config describes a single render pass for one eye. Base aberration
// coefficients are per-channel, before the selected eye's calibration bias is
// added.
type Config struct {
	Aberration  Aberration
	Eye         Eye
	Calibration Calibration
	Edge        EdgeMode
}

// Validate checks the configuration for invalid values.
func (c Config) Validate() error {
	if !c.Eye.valid() {
		return fmt.Errorf("invalid eye %d", c.Eye)
	}
	if !c.Edge.valid() {
		return fmt.Errorf("invalid edge mode %d", c.Edge)
	}
	if err := c.Aberration.Validate(); err != nil {
		return fmt.Errorf("aberration: %w", err)
	}
	if err := c.Calibration.Left.Validate(); err != nil {
		return fmt.Errorf("left eye: %w", err)
	}
	if err := c.Calibration.Right.Validate(); err != nil {
		return fmt.Errorf("right eye: %w", err)
	}
	return nil
}

// EffectiveAberration returns the base coefficients plus the selected eye's
// calibration bias.
func (c Config) EffectiveAberration() Aberration {
	return c.Aberration.Add(c.Calibration.For(c.Eye).RadialBias)
}

// OpticalCenter returns the selected eye's calibrated optical center.
func (c Config) OpticalCenter() Point {
	return c.Calibration.For(c.Eye).CenterOffset
}
