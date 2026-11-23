package main

import (
	"math"
	"time"
)

const (
	// LeftEyeOffset and RightEyeOffset express the sequential scanout: the
	// left image is scanned out one display quantum after the pose submit and
	// the right image one quantum later again. Both eyes share the 11 ms value
	// asked for in the brief; absolute offsets are 11 ms and 22 ms.
	LeftEyeOffset  = 11 * time.Millisecond
	RightEyeOffset = 22 * time.Millisecond

	// FaultAt is the tick used for invalid timestamp injections.
	FaultAt = 300 * time.Millisecond
	// LongGapStart/LongGapEnd create an 80 ms frame-drop burst, long enough
	// that queries for the far eye exceed the 50 ms prediction horizon.
	LongGapStart = 1400 * time.Millisecond
	LongGapEnd   = 1480 * time.Millisecond
)

// DropGap is one skipped interval of 1 kHz samples [start, end).
type DropGap struct {
	Start time.Duration
	End   time.Duration
}

// Record is one accepted per-eye prediction with its measured error.
type Record struct {
	Scenario  string
	Mode      Mode
	Eye       string
	SubmitAt  time.Duration
	ScanoutAt time.Duration
	TrueYaw   float64
	PredYaw   float64
	AbsErrRad float64
	SignedErr float64
	Overshoot float64
	GapAge    time.Duration
}

// Stats aggregates an error distribution.
type Stats struct {
	Count        int
	MeanAbsRad   float64
	RMSE         float64
	MaxAbsRad    float64
	MaxOvershoot float64
}

// RunResult bundles everything one scenario x mode run produced.
type RunResult struct {
	Scenario string
	Mode     Mode
	Records  []Record
	Rejects  []Reject
	Stats    map[string]Stats
}

// dropGaps defines the dropped-frame pattern for the drop scenario.
func dropGaps() []DropGap {
	return []DropGap{
		{Start: 500 * time.Millisecond, End: 512 * time.Millisecond},
		{Start: 850 * time.Millisecond, End: 866 * time.Millisecond},
		{Start: 1150 * time.Millisecond, End: 1174 * time.Millisecond},
		{Start: LongGapStart, End: LongGapEnd},
		{Start: 1700 * time.Millisecond, End: 1710 * time.Millisecond},
	}
}

func inGap(t time.Duration, gaps []DropGap) bool {
	for _, g := range gaps {
		if t >= g.Start && t < g.End {
			return true
		}
	}
	return false
}

type eyeSpec struct {
	name   string
	offset time.Duration
}

var eyes = []eyeSpec{
	{name: "left", offset: LeftEyeOffset},
	{name: "right", offset: RightEyeOffset},
}

// Run simulates one scenario with one prediction mode.
func Run(tr Trajectory, mode Mode, cfg Config) RunResult {
	p := NewPredictor(mode, cfg)
	gaps := []DropGap{}
	if tr.Name == "drop" {
		gaps = dropGaps()
	}
	res := RunResult{Scenario: tr.Name, Mode: mode, Stats: map[string]Stats{}}

	ticks := int(ScenarioDuration / SamplePeriod)
	for i := 0; i < ticks; i++ {
		now := time.Duration(i+1) * SamplePeriod

		injectSampleFaults(p, now)
		if !inGap(now, gaps) {
			p.AddSample(now, tr.Yaw(now))
		}

		for _, eye := range eyes {
			target := now + eye.offset
			queryStale := now == FaultAt && eye.name == "right"
			if queryStale {
				// Explicitly invalid query: target before the history window.
				p.PredictAt(now, 0)
			}
			pred, ok := p.PredictAt(now, target)
			if !ok {
				continue
			}
			trueYaw := tr.Yaw(target)
			signed := wrapAngle(pred - trueYaw)
			absErr := math.Abs(signed)
			direction := 1.0
			if w := tr.Omega(target); w < 0 {
				direction = -1
			}
			over := signed * direction
			if over < 0 {
				over = 0
			}
			gapAge := time.Duration(0)
			if last, has := p.LastT(); has {
				gapAge = now - last
			}
			res.Records = append(res.Records, Record{
				Scenario:  tr.Name,
				Mode:      mode,
				Eye:       eye.name,
				SubmitAt:  now,
				ScanoutAt: target,
				TrueYaw:   trueYaw,
				PredYaw:   pred,
				AbsErrRad: absErr,
				SignedErr: signed,
				Overshoot: over,
				GapAge:    gapAge,
			})
		}
	}

	res.Rejects = p.Rejects()
	res.Stats["left"] = computeStats(res.Records, "left")
	res.Stats["right"] = computeStats(res.Records, "right")
	res.Stats["both"] = computeStats(res.Records, "both")
	return res
}

// injectSampleFaults feeds deliberately invalid samples at a fixed tick.
func injectSampleFaults(p *Predictor, now time.Duration) {
	if now != FaultAt {
		return
	}
	// A timestamp behind the newest sample: regression must be refused.
	p.AddSample(now-5*time.Millisecond, 0)
	// A NaN yaw must be refused as well.
	p.AddSample(now, math.NaN())
}

func computeStats(records []Record, eye string) Stats {
	var s Stats
	var sumAbs, sumSq float64
	for _, r := range records {
		if eye != "both" && r.Eye != eye {
			continue
		}
		s.Count++
		sumAbs += r.AbsErrRad
		sumSq += r.SignedErr * r.SignedErr
		if r.AbsErrRad > s.MaxAbsRad {
			s.MaxAbsRad = r.AbsErrRad
		}
		if r.Overshoot > s.MaxOvershoot {
			s.MaxOvershoot = r.Overshoot
		}
	}
	if s.Count == 0 {
		return s
	}
	s.MeanAbsRad = sumAbs / float64(s.Count)
	s.RMSE = math.Sqrt(sumSq / float64(s.Count))
	return s
}

// DefaultConfig returns the tuning used by the reproducible experiment.
func DefaultConfig() Config {
	return Config{
		HistoryWindow: DefaultHistoryWindow,
		Horizon:       DefaultHorizon,
		VelocityTau:   DefaultVelocityTau,
		MaxAngularVel: DefaultMaxAngularVelocity,
	}
}
