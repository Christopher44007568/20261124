package main

import (
	"math"

	"posebuffer/pose"
)

// Trajectory is a smooth, analytically defined pose path so that the "true"
// pose at *any* query time is known exactly, without resampling error.
//
//	pos(t) = (cos(wt), sin(wt), 0.4 t)
//	rot(t) = Rz(w1 t) * Ry(w2 t)   (two-axis rotation, Hamilton product)
type Trajectory struct {
	w  float64 // orbital and Z-axis angular rate
	w2 float64 // secondary Y-axis angular rate
}

func NewTrajectory() *Trajectory {
	return &Trajectory{w: 1.5, w2: 0.7}
}

func (tr *Trajectory) at(t pose.Timestamp) pose.Sample {
	sec := float64(t) / 1e9
	a := tr.w * sec
	b := tr.w2 * sec
	return pose.Sample{
		Time: t,
		Pos:  pose.Vec3{X: math.Cos(a), Y: math.Sin(a), Z: 0.4 * sec},
		Rot:  pose.RotAngleZ(a).Mul(pose.RotAngleY(b)),
	}
}
