package main

import (
	"math"
	"time"
)

const (
	// ScenarioDuration is the length of every synthetic trajectory.
	ScenarioDuration = 2 * time.Second
	// SamplePeriod is the 1 kHz head-tracker cadence.
	SamplePeriod = time.Millisecond
	// StopAt is when the emergency-stop trajectory halts.
	StopAt = time.Second
	// EStopSpeed is the constant yaw speed before the emergency stop.
	EStopSpeed = 10.0
	// ConstantSpeed is the constant-turn yaw speed.
	ConstantSpeed = 10.0
	// SineAmplitude and SinePeriod define the sinusoidal yaw trajectory.
	SineAmplitude = 0.8
	SinePeriod    = time.Second
)

// Trajectory is the ground-truth head motion, evaluated at arbitrary times.
type Trajectory struct {
	Name  string
	Yaw   func(t time.Duration) float64
	Omega func(t time.Duration) float64
}

// EStopTrajectory moves at EStopSpeed until StopAt, then freezes instantly.
func EStopTrajectory() Trajectory {
	stopYaw := EStopSpeed * StopAt.Seconds()
	return Trajectory{
		Name: "estop",
		Yaw: func(t time.Duration) float64 {
			if t < StopAt {
				return EStopSpeed * t.Seconds()
			}
			return stopYaw
		},
		Omega: func(t time.Duration) float64 {
			if t < StopAt {
				return EStopSpeed
			}
			return 0
		},
	}
}

// ConstantTrajectory rotates at a fixed yaw speed for the whole run.
func ConstantTrajectory() Trajectory {
	return Trajectory{
		Name:  "constant",
		Yaw:   func(t time.Duration) float64 { return ConstantSpeed * t.Seconds() },
		Omega: func(t time.Duration) float64 { return ConstantSpeed },
	}
}

// SineTrajectory sweeps yaw sinusoidally: A*sin(2*pi*t/T).
func SineTrajectory() Trajectory {
	omega0 := 2 * math.Pi / SinePeriod.Seconds()
	return Trajectory{
		Name: "sine",
		Yaw: func(t time.Duration) float64 {
			return SineAmplitude * math.Sin(omega0*t.Seconds())
		},
		Omega: func(t time.Duration) float64 {
			return SineAmplitude * omega0 * math.Cos(omega0*t.Seconds())
		},
	}
}

// DropTrajectory is the same sinusoid used as the carrier for the dropped-frame
// scenario; the drops themselves are applied by the experiment runner.
func DropTrajectory() Trajectory {
	tr := SineTrajectory()
	tr.Name = "drop"
	return tr
}

// AllTrajectories lists the scenarios in run order.
func AllTrajectories() []Trajectory {
	return []Trajectory{
		EStopTrajectory(),
		ConstantTrajectory(),
		SineTrajectory(),
		DropTrajectory(),
	}
}
