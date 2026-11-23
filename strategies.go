package main

import "math"

const (
	RateLow  = 0.25
	RateBase = 0.5
	RateHigh = 1.0
)

type Decision struct {
	Rate     float64
	CenterX  float64
	CenterY  float64
	Boost    bool
	Fallback bool
}

type BoostEvent struct {
	Start, End     int
	PredictedOnset int
	MatchedOnset   int
}

type Strategy interface {
	Name() string
	Init(x, y float64, total int)
	Step(i int, f, prev Frame) Decision
	Events() []BoostEvent
	Switches() int
	RevertMax() int
}

// saccadeDetector fires onset when displacement accumulated over a
// 3-frame window is large and every step moved, so a single-frame
// observation spike (jump out, jump back) is ignored.
type saccadeDetector struct {
	inSacc bool
	nhist  int
	px, py float64
	qx, qy float64
}

func (d *saccadeDetector) step(f Frame) (onset, offset bool) {
	if !f.ObsValid {
		return false, false
	}
	if d.nhist >= 2 {
		d2 := math.Hypot(f.ObsX-d.px, f.ObsY-d.py)
		d1 := math.Hypot(f.ObsX-d.qx, f.ObsY-d.qy)
		d0 := math.Hypot(d.qx-d.px, d.qy-d.py)
		switch {
		case !d.inSacc && d2 > 30 && d1 > 10 && d0 > 10:
			d.inSacc = true
			onset = true
		case d.inSacc && d1 <= 20:
			d.inSacc = false
			offset = true
		}
	}
	d.px, d.py = d.qx, d.qy
	d.qx, d.qy = f.ObsX, f.ObsY
	if d.nhist < 2 {
		d.nhist++
	}
	return
}

// NoPredict: fixed sampling, center follows reactively.
type NoPredict struct {
	tr *CenterTracker
}

func NewNoPredict() *NoPredict { return &NoPredict{} }

func (s *NoPredict) Name() string { return "nopredict" }

func (s *NoPredict) Init(x, y float64, total int) { s.tr = NewCenterTracker(x, y) }

func (s *NoPredict) Step(i int, f, prev Frame) Decision {
	s.tr.Update(f.ObsX, f.ObsY, f.ObsValid)
	return Decision{Rate: RateBase, CenterX: s.tr.CX, CenterY: s.tr.CY}
}

func (s *NoPredict) Events() []BoostEvent { return nil }
func (s *NoPredict) Switches() int        { return s.tr.Switches }
func (s *NoPredict) RevertMax() int       { return 0 }

// ConstantDelay: boost a fixed number of frames after detection.
type ConstantDelay struct {
	tr        *CenterTracker
	det       saccadeDetector
	delay     int
	boostLen  int
	boostFrom int
	boostTo   int
	events    []BoostEvent
}

func NewConstantDelay(delay, boostLen int) *ConstantDelay {
	return &ConstantDelay{delay: delay, boostLen: boostLen, boostTo: -1}
}

func (s *ConstantDelay) Name() string { return "constdelay" }

func (s *ConstantDelay) Init(x, y float64, total int) { s.tr = NewCenterTracker(x, y) }

func (s *ConstantDelay) Step(i int, f, prev Frame) Decision {
	s.tr.Update(f.ObsX, f.ObsY, f.ObsValid)
	onset, _ := s.det.step(f)
	if onset && i >= s.boostTo {
		s.boostFrom = i + s.delay
		s.boostTo = s.boostFrom + s.boostLen
		s.events = append(s.events, BoostEvent{Start: s.boostFrom, End: s.boostTo, PredictedOnset: -1, MatchedOnset: -1})
	}
	rate := RateBase
	boost := false
	if i >= s.boostFrom && i < s.boostTo {
		rate = RateHigh
		boost = true
	}
	return Decision{Rate: rate, CenterX: s.tr.CX, CenterY: s.tr.CY, Boost: boost}
}

func (s *ConstantDelay) Events() []BoostEvent { return s.events }
func (s *ConstantDelay) Switches() int        { return s.tr.Switches }
func (s *ConstantDelay) RevertMax() int       { return 0 }
