package motion

import (
	"math"

	"posebench/internal/geom"
)

// Axis identifies a rotational or translational degree of freedom.
type Axis int

const (
	Yaw Axis = iota
	Pitch
	Roll
	TX
	TY
	TZ
)

type Term struct {
	Axis   Axis
	Amp    float64
	FreqHz float64
	Phase  float64
}

// Profile is an analytic, periodic head-motion trajectory. All rotational
// amplitudes are degrees, translational amplitudes are millimetres, and
// phases are radians.
type Profile struct {
	Name        string
	Description string
	PeriodSec   float64
	BaseT       geom.Vec3
	Terms       []Term
}

func (p Profile) PoseAt(t float64) geom.Pose {
	var yaw, pitch, roll float64
	translation := p.BaseT
	for _, term := range p.Terms {
		value := term.Amp * math.Sin(2*math.Pi*term.FreqHz*t+term.Phase)
		switch term.Axis {
		case Yaw:
			yaw += value
		case Pitch:
			pitch += value
		case Roll:
			roll += value
		case TX:
			translation.X += value
		case TY:
			translation.Y += value
		case TZ:
			translation.Z += value
		}
	}
	return geom.Pose{
		Q: geom.QuatFromEuler(degToRad(yaw), degToRad(pitch), degToRad(roll)),
		T: translation,
	}
}

// AngularSpeedDegPerSec returns a finite-difference estimate of the angular
// speed at t.
func (p Profile) AngularSpeedDegPerSec(t float64) float64 {
	const dt = 1e-5
	a := p.PoseAt(t - dt)
	b := p.PoseAt(t + dt)
	return geom.AngularErrorDeg(a, b) / (2 * dt)
}

func (p Profile) PeakAngularSpeedDegPerSec() float64 {
	const samples = 2048
	var peak float64
	for i := 0; i <= samples; i++ {
		t := p.PeriodSec * float64(i) / samples
		if speed := p.AngularSpeedDegPerSec(t); speed > peak {
			peak = speed
		}
	}
	return peak
}

func Catalog() []Profile {
	return []Profile{
		{
			Name:        "fast-turn",
			Description: "fast yaw turn with small pitch/roll coupling",
			PeriodSec:   1,
			BaseT:       geom.Vec3{Z: 550},
			Terms: []Term{
				{Axis: Yaw, Amp: 30, FreqHz: 2, Phase: -math.Pi / 2},
				{Axis: Pitch, Amp: 6, FreqHz: 1, Phase: 0.3},
				{Axis: Roll, Amp: 4, FreqHz: 1, Phase: -0.2},
				{Axis: TX, Amp: 4, FreqHz: 1, Phase: 0.5},
			},
		},
		{
			Name:        "side-tilt",
			Description: "lateral roll and yaw sweep with side-to-side translation",
			PeriodSec:   1,
			BaseT:       geom.Vec3{Z: 550},
			Terms: []Term{
				{Axis: Roll, Amp: 22, FreqHz: 2, Phase: -math.Pi / 2},
				{Axis: Yaw, Amp: 12, FreqHz: 1, Phase: 0.4},
				{Axis: Pitch, Amp: 5, FreqHz: 1, Phase: -0.4},
				{Axis: TX, Amp: 14, FreqHz: 1, Phase: math.Pi / 2},
				{Axis: TY, Amp: 5, FreqHz: 2, Phase: 0},
			},
		},
		{
			Name:        "combined",
			Description: "fast turn, side tilt, nod, and lateral translation",
			PeriodSec:   1,
			BaseT:       geom.Vec3{Z: 550},
			Terms: []Term{
				{Axis: Yaw, Amp: 34, FreqHz: 2, Phase: -math.Pi / 2},
				{Axis: Pitch, Amp: 9, FreqHz: 3, Phase: 0.2},
				{Axis: Roll, Amp: 16, FreqHz: 2, Phase: 0.7},
				{Axis: TX, Amp: 18, FreqHz: 1, Phase: math.Pi / 2},
				{Axis: TY, Amp: 8, FreqHz: 2, Phase: -0.3},
			},
		},
	}
}

func degToRad(v float64) float64 {
	return v * math.Pi / 180
}
