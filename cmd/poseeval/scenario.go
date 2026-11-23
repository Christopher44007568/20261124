package main

import (
	"math/rand"

	"posebuffer/pose"
)

const (
	period90  = pose.Timestamp(11_111_111) // ~11.11 ms
	period120 = pose.Timestamp(8_333_333)  // ~8.33 ms
	lostSpan  = pose.Timestamp(200_000_000)
	rngSeed   = 20261124
)

// Scenario describes one input-stream condition applied to the same truth.
type Scenario struct {
	Name   string
	Domain pose.Domain
	Apply  func([]pose.Sample) []pose.Sample
}

// baseStream emits mixed 90/120 Hz samples for duration ns: even-numbered
// segments run at 90 Hz and odd-numbered segments at 120 Hz, exercising the
// resampler against both input rates on one clock domain.
func baseStream(duration pose.Timestamp, jitterNS int64) []pose.Sample {
	rng := rand.New(rand.NewSource(rngSeed))
	tr := NewTrajectory()
	var out []pose.Sample
	var t pose.Timestamp
	segment := 0
	for t < duration {
		period := period90
		if segment%2 == 1 {
			period = period120
		}
		stamp := t
		if jitterNS > 0 {
			stamp += pose.Timestamp(rng.Int63n(2*jitterNS+1) - jitterNS)
		}
		out = append(out, tr.at(stamp))
		t += period
		if t > pose.Timestamp(segment+1)*500_000_000 {
			segment++
		}
	}
	return out
}

func dropWindow(samples []pose.Sample, start, span pose.Timestamp) []pose.Sample {
	out := samples[:0:0]
	for _, s := range samples {
		if s.Time >= start && s.Time < start+span {
			continue
		}
		out = append(out, s)
	}
	return out
}

// rotateThree reverses every block of three consecutive samples, reproducing
// continuous three-frame out-of-order arrival without changing timestamps.
func rotateThree(samples []pose.Sample) []pose.Sample {
	out := append([]pose.Sample(nil), samples...)
	for i := 0; i+2 < len(out); i += 3 {
		out[i], out[i+2] = out[i+2], out[i]
	}
	return out
}

// rollback injects one stale sample whose timestamp is 60 ms older than the
// newest sample already delivered.
func rollback(samples []pose.Sample) []pose.Sample {
	tr := NewTrajectory()
	out := append([]pose.Sample(nil), samples...)
	anchor := out[len(out)*3/4].Time
	stale := tr.at(anchor - 60_000_000)
	idx := len(out)*3/4 + 1
	return append(out[:idx], append([]pose.Sample{stale}, out[idx:]...)...)
}

func buildScenarios(duration pose.Timestamp) []Scenario {
	return []Scenario{
		{Name: "baseline_mixed_90_120", Domain: "clockA", Apply: func(s []pose.Sample) []pose.Sample {
			return s
		}},
		{Name: "duplicate_samples", Domain: "clockA", Apply: func(s []pose.Sample) []pose.Sample {
			out := make([]pose.Sample, 0, len(s)+2)
			for i, smp := range s {
				out = append(out, smp)
				if i == len(s)/3 || i == 2*len(s)/3 {
					out = append(out, smp) // repeated arrival, same timestamp
				}
			}
			return out
		}},
		{Name: "loss_gap_200ms", Domain: "clockA", Apply: func(s []pose.Sample) []pose.Sample {
			return dropWindow(s, duration/2, lostSpan)
		}},
		{Name: "out_of_order_3frames", Domain: "clockA", Apply: rotateThree},
		{Name: "clock_rollback", Domain: "clockA", Apply: rollback},
		{Name: "all_stressors_combined", Domain: "clockA", Apply: func(s []pose.Sample) []pose.Sample {
			s = dropWindow(s, duration/2, lostSpan)
			s = rotateThree(s)
			s = rollback(s)
			return s
		}},
	}
}
