// Package optics models the apparent drift of a virtual image when the
// pupil moves behind a lens, plus three compensation strategies:
// fixed projection, per-eye offset, and smoothed compensation.
package optics

import (
	"math"

	"pupilsim/internal/gaze"
)

// Lens describes a near-eye lens and its virtual image.
type Lens struct {
	PowerD        float64 // lens power in diopters
	VirtualImageM float64 // virtual image distance in metres
}

// DefaultLens approximates a compact VR pancake/Fresnel optic.
func DefaultLens() Lens {
	return Lens{PowerD: 50, VirtualImageM: 1.5}
}

// ShiftMM converts a residual pupil decentration (mm at the lens plane)
// into apparent virtual-image displacement (mm at the virtual image
// plane) using Prentice's rule: prism[diopter] = c[cm] * F[D], and the
// deviation angle is atan(prism/100).
func (l Lens) ShiftMM(residual gaze.Vec2) gaze.Vec2 {
	scale := l.VirtualImageM * 1000 // mm at the virtual image plane
	dev := func(mm float64) float64 {
		prism := (mm / 10) * l.PowerD // cm * diopters
		return math.Tan(math.Atan(prism/100)) * scale
	}
	return gaze.Vec2{X: dev(residual.X), Y: dev(residual.Y)}
}

// ShiftArcMin converts residual decentration into apparent angular
// drift in arc-minutes as seen by the eye.
func (l Lens) ShiftArcMin(residual gaze.Vec2) gaze.Vec2 {
	dev := func(mm float64) float64 {
		prism := (mm / 10) * l.PowerD
		return math.Atan(prism/100) * (180 / math.Pi) * 60
	}
	return gaze.Vec2{X: dev(residual.X), Y: dev(residual.Y)}
}

// Compensator maps a stream of gaze samples to a render offset (mm at
// the lens plane) that counteracts the pupil decentration.
type Compensator interface {
	// Update consumes one frame and returns the offset to apply.
	Update(s gaze.Sample) gaze.Vec2
	Name() string
}

// Fixed ignores the pupil entirely: the projection stays locked to the
// optical axis.
type Fixed struct{}

func (Fixed) Name() string                 { return "fixed" }
func (Fixed) Update(gaze.Sample) gaze.Vec2 { return gaze.Vec2{} }

// PerEye applies the raw per-frame measured pupil offset for each eye
// independently. On tracking loss it falls back to zero, which causes
// an abrupt jump.
type PerEye struct{}

func (PerEye) Name() string { return "per-eye" }
func (PerEye) Update(s gaze.Sample) gaze.Vec2 {
	if s.Lost {
		return gaze.Vec2{}
	}
	return s.Measured
}

// Smoothed applies a one-pole low-pass filter with a slew-rate limit.
// During tracking loss it holds the last estimate and relaxes slowly
// toward zero, so the rendered image never jumps.
type Smoothed struct {
	Alpha     float64 // low-pass blend factor per frame (0..1)
	MaxStep   float64 // slew-rate limit (mm per frame)
	LossDecay float64 // per-frame decay toward zero while lost (0..1)
	state     gaze.Vec2
	started   bool
}

// NewSmoothed returns a Smoothed with sensible defaults for 240 Hz.
func NewSmoothed() *Smoothed {
	return &Smoothed{Alpha: 0.25, MaxStep: 0.6, LossDecay: 0.995}
}

func (s *Smoothed) Name() string { return "smoothed" }

func (s *Smoothed) Update(sample gaze.Sample) gaze.Vec2 {
	var target gaze.Vec2
	switch {
	case sample.Lost:
		// Hold and gently relax toward zero instead of snapping.
		target = gaze.Vec2{X: s.state.X * s.LossDecay, Y: s.state.Y * s.LossDecay}
	case !s.started:
		target = sample.Measured
		s.started = true
	default:
		target = gaze.Vec2{
			X: s.state.X + s.Alpha*(sample.Measured.X-s.state.X),
			Y: s.state.Y + s.Alpha*(sample.Measured.Y-s.state.Y),
		}
	}
	s.state = clampStep(s.state, target, s.MaxStep)
	return s.state
}

func clampStep(from, to gaze.Vec2, maxStep float64) gaze.Vec2 {
	d := gaze.Vec2{X: to.X - from.X, Y: to.Y - from.Y}
	mag := math.Hypot(d.X, d.Y)
	if mag <= maxStep || mag == 0 {
		return to
	}
	k := maxStep / mag
	return gaze.Vec2{X: from.X + d.X*k, Y: from.Y + d.Y*k}
}
