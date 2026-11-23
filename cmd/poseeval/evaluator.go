package main

import (
	"math"

	"posebuffer/pose"
)

const cameraPeriod = pose.Timestamp(8_333_333) // 120 Hz camera query rate

// Errors aggregates position (metres) and orientation (radians) deviations
// from the analytic truth.
type Errors struct {
	N           int
	Unavailable int
	PosSumSq    float64
	PosMax      float64
	RotSumSq    float64
	RotMax      float64
	DeltaSumAbs pose.Timestamp
	DeltaMaxAbs pose.Timestamp
}

func (e *Errors) add(posErr, rotErr float64, delta pose.Timestamp) {
	e.N++
	e.PosSumSq += posErr * posErr
	if posErr > e.PosMax {
		e.PosMax = posErr
	}
	e.RotSumSq += rotErr * rotErr
	if rotErr > e.RotMax {
		e.RotMax = rotErr
	}
	d := delta
	if d < 0 {
		d = -d
	}
	e.DeltaSumAbs += d
	if d > e.DeltaMaxAbs {
		e.DeltaMaxAbs = d
	}
}

func (e Errors) PosRMSE() float64 {
	if e.N == 0 {
		return math.NaN()
	}
	return math.Sqrt(e.PosSumSq / float64(e.N))
}

func (e Errors) RotRMSE() float64 {
	if e.N == 0 {
		return math.NaN()
	}
	return math.Sqrt(e.RotSumSq / float64(e.N))
}

func (e Errors) MeanDeltaMS() float64 {
	if e.N == 0 {
		return 0
	}
	return float64(e.DeltaSumAbs) / float64(e.N) / 1e6
}

// Estimator labels.
const (
	EstNearest  = "nearest_neighbor"
	EstResample = "history_resample"
	EstTruth    = "analytic_truth"
)

type EstimatorResult struct {
	Name string
	Err  Errors
}

// Run feeds one arrival stream into a domain buffer and queries it at every
// camera exposure instant, comparing all three pose sources against truth.
func Run(sc Scenario, stream []pose.Sample, duration pose.Timestamp) ([]EstimatorResult, pose.Stats) {
	cfg := pose.Config{
		Capacity:    1024,        // covers the full 4 s window at 90/120 Hz (~420)
		Extrapolate: 25_000_000,  // 25 ms tolerance for late arrivals
		MaxGap:      150_000_000, // the 200 ms outage must refuse interpolation
	}
	hub := pose.NewHub(cfg)
	buf := hub.Buffer(sc.Domain)

	for _, s := range stream { // arrival order, not timestamp order
		hub.Insert(sc.Domain, s)
	}

	tr := NewTrajectory()
	nn, rs := Errors{}, Errors{}
	var t pose.Timestamp = cameraPeriod
	for ; t < duration; t += cameraPeriod {
		truth := tr.at(t)

		r := buf.QueryNearest(t)
		if r.Available() {
			nn.add(pose.Distance(r.Sample.Pos, truth.Pos),
				pose.AngleBetween(r.Sample.Rot, truth.Rot), r.NearestDelta)
		} else {
			nn.Unavailable++
		}

		q := buf.Query(t)
		if q.Available() {
			rs.add(pose.Distance(q.Sample.Pos, truth.Pos),
				pose.AngleBetween(q.Sample.Rot, truth.Rot), q.NearestDelta)
		} else {
			rs.Unavailable++
		}
	}

	// The analytic estimator has zero error by construction; it is reported
	// alongside the other two as the reference row.
	return []EstimatorResult{
		{Name: EstNearest, Err: nn},
		{Name: EstResample, Err: rs},
		{Name: EstTruth, Err: Errors{N: rs.N + rs.Unavailable}},
	}, buf.Stats()
}
