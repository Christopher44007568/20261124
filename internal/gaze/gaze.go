// Package gaze generates synthetic pupil trajectories behind a lens:
// natural fixational eye movement, rapid saccades, return-to-center
// sequences, and intermittent tracking loss.
package gaze

import "math/rand"

// Vec2 is a 2D offset in millimetres on the lens plane.
type Vec2 struct {
	X, Y float64
}

// Sample is one frame of synthetic eye-tracking data.
type Sample struct {
	T        float64 // seconds
	Pupil    Vec2    // true pupil centre offset from the optical axis (mm)
	Measured Vec2    // tracker-reported pupil offset (noisy); invalid when Lost
	Lost     bool    // tracker lost the pupil on this frame
}

// Config controls the synthetic gaze generator.
type Config struct {
	DT            float64 // frame interval (s)
	NoiseMM       float64 // tracker measurement noise stddev (mm)
	DriftAmpMM    float64 // fixational drift amplitude (mm)
	MicroSaccMM   float64 // microsaccade amplitude (mm)
	LossProb      float64 // per-frame probability of entering a loss burst
	LossMinFrames int
	LossMaxFrames int
	Seed          int64
}

// DefaultConfig returns a 240 Hz tracker configuration.
func DefaultConfig() Config {
	return Config{
		DT:            1.0 / 240.0,
		NoiseMM:       0.08,
		DriftAmpMM:    0.15,
		MicroSaccMM:   0.12,
		LossProb:      0.002,
		LossMinFrames: 4,
		LossMaxFrames: 30,
		Seed:          42,
	}
}

// Event is a scripted gaze displacement.
type Event struct {
	AtFrame  int
	Target   Vec2 // absolute fixation target (mm)
	Duration int  // frames over which the saccade executes (>=1)
}

// Generator produces pupil samples frame by frame.
type Generator struct {
	cfg       Config
	rng       *rand.Rand
	frame     int
	from      Vec2 // saccade start centre
	to        Vec2 // saccade end centre (also the resting centre)
	saccStart int
	saccEnd   int
	events    []Event
	lostFor   int // remaining frames of tracking loss
}

// New builds a generator with scripted saccade events (sorted by AtFrame).
func New(cfg Config, events []Event) *Generator {
	return &Generator{cfg: cfg, rng: rand.New(rand.NewSource(cfg.Seed)), events: events}
}

// Next advances one frame and returns the sample.
func (g *Generator) Next() Sample {
	f := g.frame
	for len(g.events) > 0 && g.events[0].AtFrame <= f {
		ev := g.events[0]
		g.events = g.events[1:]
		g.from = g.centre()
		g.to = ev.Target
		d := ev.Duration
		if d < 1 {
			d = 1
		}
		g.saccStart = f
		g.saccEnd = f + d
	}
	centre := g.centre()

	// Fixational drift plus occasional microsaccade.
	drift := Vec2{
		X: g.cfg.DriftAmpMM * (2*g.rng.Float64() - 1),
		Y: g.cfg.DriftAmpMM * (2*g.rng.Float64() - 1),
	}
	if g.rng.Float64() < 0.02 {
		drift.X += g.cfg.MicroSaccMM * (2*g.rng.Float64() - 1)
		drift.Y += g.cfg.MicroSaccMM * (2*g.rng.Float64() - 1)
	}
	truePupil := Vec2{X: centre.X + drift.X, Y: centre.Y + drift.Y}

	// Tracking loss bursts.
	lost := false
	if g.lostFor > 0 {
		g.lostFor--
		lost = true
	} else if g.rng.Float64() < g.cfg.LossProb {
		span := g.cfg.LossMinFrames
		if g.cfg.LossMaxFrames > g.cfg.LossMinFrames {
			span += g.rng.Intn(g.cfg.LossMaxFrames - g.cfg.LossMinFrames + 1)
		}
		g.lostFor = span - 1
		lost = true
	}
	meas := truePupil
	if !lost {
		meas.X += g.rng.NormFloat64() * g.cfg.NoiseMM
		meas.Y += g.rng.NormFloat64() * g.cfg.NoiseMM
	}
	g.frame++
	return Sample{
		T:        float64(f) * g.cfg.DT,
		Pupil:    truePupil,
		Measured: meas,
		Lost:     lost,
	}
}

// centre returns the fixation centre at the current frame, executing any
// active saccade with a smoothstep (ease-in-out) profile.
func (g *Generator) centre() Vec2 {
	if g.frame >= g.saccEnd {
		return g.to
	}
	total := float64(g.saccEnd - g.saccStart)
	progress := float64(g.frame-g.saccStart) / total
	s := progress * progress * (3 - 2*progress)
	return Vec2{
		X: g.from.X + (g.to.X-g.from.X)*s,
		Y: g.from.Y + (g.to.Y-g.from.Y)*s,
	}
}
