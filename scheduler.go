package foveq

import (
	"errors"
	"fmt"
	"sync"
)

// ErrInvalidSchedulerConfig is returned when eye weights or quantum are bad.
var ErrInvalidSchedulerConfig = errors.New("foveq: invalid scheduler config")

// eyeCount is fixed: VR rendering schedules a left and a right eye.
const eyeCount = 2

const (
	EyeLeft  = 0
	EyeRight = 1
)

// Scheduler distributes GPU frame budget between the two eyes with weighted
// Deficit Round Robin. Unlike an even split, an eye that is repeatedly
// starved jumps to the front of the queue, so neither eye can be denied
// budget indefinitely.
type Scheduler struct {
	mu      sync.Mutex
	weights [eyeCount]float64
	quantum float64

	frame       int64
	deficit     [eyeCount]float64
	served      [eyeCount]int64
	allocated   [eyeCount]float64
	lastServed  [eyeCount]int64
	starvations [eyeCount]int64

	// StarvationFrames: an eye waiting this many frames is promoted.
	starvationFrames int64
}

// SchedulerConfig configures the per-frame budget scheduler.
type SchedulerConfig struct {
	WeightLeft       float64
	WeightRight      float64
	QuantumMicros    float64
	StarvationFrames int64
}

// NewScheduler builds and validates a scheduler.
func NewScheduler(cfg SchedulerConfig) (*Scheduler, error) {
	if cfg.WeightLeft <= 0 || cfg.WeightRight <= 0 {
		return nil, fmt.Errorf("%w: weights must be positive", ErrInvalidSchedulerConfig)
	}
	if cfg.QuantumMicros <= 0 {
		return nil, fmt.Errorf("%w: quantum must be positive", ErrInvalidSchedulerConfig)
	}
	if cfg.StarvationFrames <= 0 {
		cfg.StarvationFrames = 2
	}
	return &Scheduler{
		weights:          [eyeCount]float64{cfg.WeightLeft, cfg.WeightRight},
		quantum:          cfg.QuantumMicros,
		starvationFrames: cfg.StarvationFrames,
	}, nil
}

// Allocation is the per-eye result of one scheduling round.
type Allocation struct {
	Eye      int
	Granted  float64
	Deficit  float64
	Starving bool
	Frame    int64
}

// Allocate runs one frame round. demands are requested GPU microseconds per
// eye; totalCap is the shared GPU budget available this frame. The returned
// slice has one entry per eye in left/right order. Unmet demand is retained
// as deficit for future frames, exactly like DRR.
func (s *Scheduler) Allocate(demands [eyeCount]float64, totalCap float64) [eyeCount]Allocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.frame++
	frame := s.frame
	remaining := totalCap
	if remaining < 0 {
		remaining = 0
	}

	// Credit each eye proportional to its weight at the start of the round.
	var totalWeight float64
	for _, w := range s.weights {
		totalWeight += w
	}
	for eye := range s.weights {
		s.deficit[eye] += s.quantum * s.weights[eye] / totalWeight
	}

	out := [eyeCount]Allocation{}
	for eye := range out {
		out[eye] = Allocation{Eye: eye, Frame: frame, Deficit: s.deficit[eye]}
	}

	// starving reports eyes that have not been served for too long. They are
	// visited first, which is the explicit anti-starvation guarantee.
	pending := make([]int, 0, eyeCount)
	for eye := range s.weights {
		if demands[eye] > 0 {
			pending = append(pending, eye)
		} else {
			out[eye].Deficit = s.deficit[eye]
		}
	}
	starvedFirst := s.promoteStarved(frame, pending)

	for _, eye := range starvedFirst {
		out[eye].Starving = true
		s.serveEye(eye, demands[eye], &remaining, &out[eye], frame)
	}

	// Deficit round-robin passes. The loop bound prevents pathological
	// spinning if both eyes demand far more than capacity forever.
	maxPasses := eyeCount*2 + 2
	for passes := 0; passes < maxPasses && remaining > 0; passes++ {
		progress := false
		for eye := range s.weights {
			if demands[eye] <= 0 || remaining <= 0 {
				continue
			}
			if out[eye].Granted >= demands[eye] {
				continue
			}
			if s.deficit[eye] <= 0 {
				s.deficit[eye] += s.quantum * s.weights[eye] / totalWeight
				out[eye].Deficit = s.deficit[eye]
			}
			if s.serveEye(eye, demands[eye]-out[eye].Granted, &remaining, &out[eye], frame) {
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	for eye := range out {
		out[eye].Deficit = s.deficit[eye]
	}
	return out
}

// promoteStarved moves eyes whose wait exceeds StarvationFrames to the front,
// preserving relative order among the rest.
func (s *Scheduler) promoteStarved(frame int64, pending []int) []int {
	starved := make([]int, 0, len(pending))
	rest := make([]int, 0, len(pending))
	for _, eye := range pending {
		waited := frame - s.lastServed[eye]
		// An eye with lastServed==0 is requesting budget for the first time;
		// treat that as the start of its wait so it is never silently skipped.
		if waited >= s.starvationFrames {
			s.starvations[eye]++
			starved = append(starved, eye)
			continue
		}
		rest = append(rest, eye)
	}
	return append(starved, rest...)
}

// serveEye grants as much of want as both remaining capacity and deficit
// allow. It returns whether any budget was granted.
func (s *Scheduler) serveEye(eye int, want float64, remaining *float64, a *Allocation, frame int64) bool {
	if want <= 0 || *remaining <= 0 || s.deficit[eye] <= 0 {
		return false
	}
	grant := min3(want, *remaining, s.deficit[eye])
	if grant <= 0 {
		return false
	}
	s.deficit[eye] -= grant
	*remaining -= grant
	a.Granted += grant
	a.Deficit = s.deficit[eye]
	s.allocated[eye] += grant
	if a.Granted > 0 {
		s.served[eye]++
		s.lastServed[eye] = frame
	}
	return true
}

func min3(a, b, c float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// SchedulerStats is a point-in-time snapshot for telemetry and tests.
type SchedulerStats struct {
	Frame       int64
	Served      [eyeCount]int64
	Allocated   [eyeCount]float64
	Starvations [eyeCount]int64
	Deficit     [eyeCount]float64
}

// Stats returns a copy of the scheduler counters.
func (s *Scheduler) Stats() SchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SchedulerStats{
		Frame:       s.frame,
		Served:      s.served,
		Allocated:   s.allocated,
		Starvations: s.starvations,
		Deficit:     s.deficit,
	}
}

// Shares returns the long-run granted-budget fraction per eye. Under bounded
// demand it converges to the configured weights; under overload it stays
// fair instead of letting one eye monopolize the GPU.
func (s *Scheduler) Shares() [eyeCount]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total float64
	for _, v := range s.allocated {
		total += v
	}
	var out [eyeCount]float64
	if total <= 0 {
		return out
	}
	for eye, v := range s.allocated {
		out[eye] = v / total
	}
	return out
}
