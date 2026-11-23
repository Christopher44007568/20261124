package main

import (
	"math"
	"time"
)

const (
	// DefaultHistoryWindow is how long samples are retained. Samples older
	// than this relative to the newest accepted sample cannot be accepted or
	// queried anymore.
	DefaultHistoryWindow = 100 * time.Millisecond
	// DefaultHorizon is the furthest distance a prediction target is allowed
	// to lie ahead of the newest accepted sample.
	DefaultHorizon = 50 * time.Millisecond
	// DefaultVelocityTau is the EMA time constant used to estimate angular
	// velocity from the quantized 1 kHz yaw samples (rad/s of yaw).
	DefaultVelocityTau = 8 * time.Millisecond
	// DefaultMaxAngularVelocity is the limiter applied by ModeClamped.
	DefaultMaxAngularVelocity = 6.0
)

// Mode selects the pose prediction strategy.
type Mode int

const (
	// ModeZOH is zero-order hold: the newest yaw is reused without rotation.
	ModeZOH Mode = iota
	// ModeVelocity extrapolates with the EMA-estimated angular velocity.
	ModeVelocity
	// ModeClamped extrapolates like ModeVelocity but limits |omega|.
	ModeClamped
)

func (m Mode) String() string {
	switch m {
	case ModeZOH:
		return "zoh"
	case ModeVelocity:
		return "velocity"
	case ModeClamped:
		return "clamped"
	default:
		return "unknown"
	}
}

// RejectReason explains why a sample or query was refused.
type RejectReason string

const (
	RejectNaNYaw          RejectReason = "nan_yaw"
	RejectTimestampRolled RejectReason = "timestamp_rolled_back"
	RejectBeforeWindow    RejectReason = "before_history_window"
	RejectNoSamples       RejectReason = "no_samples"
	RejectBeyondHorizon   RejectReason = "beyond_prediction_horizon"
)

// Reject is one invalid timestamp/value record.
type Reject struct {
	At     time.Duration
	Target time.Duration
	Kind   string // "sample" or "query"
	Mode   Mode
	Reason RejectReason
}

// Config holds the predictor tuning knobs.
type Config struct {
	HistoryWindow time.Duration
	Horizon       time.Duration
	VelocityTau   time.Duration
	MaxAngularVel float64
}

type sample struct {
	t   time.Duration
	yaw float64
}

// Predictor keeps a bounded yaw history and an EMA angular-velocity estimate.
type Predictor struct {
	cfg      Config
	mode     Mode
	history  []sample
	lastT    time.Duration
	oldestT  time.Duration
	omega    float64
	haveInit bool
	rejects  []Reject
}

// NewPredictor builds a predictor for the given mode.
func NewPredictor(mode Mode, cfg Config) *Predictor {
	return &Predictor{cfg: cfg, mode: mode}
}

func (p *Predictor) logReject(kind string, at, target time.Duration, reason RejectReason) {
	p.rejects = append(p.rejects, Reject{
		At: at, Target: target, Kind: kind, Mode: p.mode, Reason: reason,
	})
}

// Rejects returns a copy of every rejected sample/query record.
func (p *Predictor) Rejects() []Reject {
	out := make([]Reject, len(p.rejects))
	copy(out, p.rejects)
	return out
}

// Len reports the number of retained samples.
func (p *Predictor) Len() int { return len(p.history) }

// LastT reports the timestamp of the newest accepted sample.
func (p *Predictor) LastT() (time.Duration, bool) {
	if !p.haveInit {
		return 0, false
	}
	return p.lastT, true
}

// AddSample ingests one 1 kHz head-tracking sample.
func (p *Predictor) AddSample(t time.Duration, yaw float64) bool {
	if math.IsNaN(yaw) || math.IsInf(yaw, 0) {
		p.logReject("sample", t, t, RejectNaNYaw)
		return false
	}
	if p.haveInit && t < p.oldestT {
		p.logReject("sample", t, t, RejectBeforeWindow)
		return false
	}
	if p.haveInit && t <= p.lastT {
		// Monotonic clock required: a regression (duplicate or backwards)
		// timestamp inside the retained window must never overwrite history.
		p.logReject("sample", t, t, RejectTimestampRolled)
		return false
	}

	if p.haveInit {
		dt := (t - p.lastT).Seconds()
		inst := wrapAngle(yaw-p.history[len(p.history)-1].yaw) / dt
		alpha := 1 - math.Exp(-dt/p.cfg.VelocityTau.Seconds())
		p.omega = (1-alpha)*p.omega + alpha*inst
	}
	p.history = append(p.history, sample{t: t, yaw: yaw})
	p.lastT = t
	if !p.haveInit {
		p.oldestT = t
		p.haveInit = true
	}
	p.trim()
	return true
}

func (p *Predictor) trim() {
	cutoff := p.lastT - p.cfg.HistoryWindow
	keep := 0
	for keep < len(p.history) && p.history[keep].t < cutoff {
		keep++
	}
	if keep > 0 {
		p.history = append(p.history[:0], p.history[keep:]...)
		p.oldestT = p.history[0].t
	}
}

// PredictAt returns the predicted yaw for display scanout at target.
func (p *Predictor) PredictAt(now, target time.Duration) (float64, bool) {
	if !p.haveInit {
		p.logReject("query", now, target, RejectNoSamples)
		return 0, false
	}
	if target < p.oldestT {
		p.logReject("query", now, target, RejectBeforeWindow)
		return 0, false
	}
	if target-p.lastT > p.cfg.Horizon {
		p.logReject("query", now, target, RejectBeyondHorizon)
		return 0, false
	}

	latest := p.history[len(p.history)-1]
	if p.mode == ModeZOH || target <= latest.t {
		return latest.yaw, true
	}
	omega := p.omega
	if p.mode == ModeClamped {
		omega = clampFloat(omega, -p.cfg.MaxAngularVel, p.cfg.MaxAngularVel)
	}
	dt := (target - latest.t).Seconds()
	return latest.yaw + omega*dt, true
}
