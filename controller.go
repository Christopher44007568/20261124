package foveq

import "fmt"

// Reason tags explain why a decision was taken. They are stable strings so
// external loggers can aggregate decisions without importing internals.
const (
	ReasonHealthy         = "healthy"
	ReasonGPUOverrun      = "gpu_overrun"
	ReasonHeadroomLack    = "insufficient_headroom"
	ReasonCenterError     = "center_error_demand"
	ReasonPeripheralError = "peripheral_error_demand"
	ReasonLoadSpike       = "load_spike_freeze"
	ReasonFastGaze        = "fast_gaze_freeze"
	ReasonCooldown        = "shift_cooldown"
	ReasonUpshiftHeld     = "upshift_hysteresis_hold"
	ReasonTrial           = "trial_confirmation"
	ReasonTrialPromote    = "trial_confirmed_good"
	ReasonTrialRollback   = "trial_failed_rollback"
	ReasonFrameFailure    = "frame_failure_downshift"
	ReasonTierBlocked     = "tier_in_failure_cooldown"
	ReasonBoundaryClamp   = "ladder_boundary_clamp"
	ReasonNoQualityDemand = "no_quality_demand"
)

// Config parameterizes the Controller. Zero values are replaced with sane
// defaults by NewController.
type Config struct {
	Ladder *Ladder
	// StartTier is the tier selected at startup; must be inside the ladder.
	StartTier int
	// EMAAlpha in (0,1] controls smoothing of GPU/error/gaze signals.
	EMAAlpha float64
	// EMAWarmup is the number of frames averaged before EMA kicks in.
	EMAWarmup int
	// OverrunHoldFrames: consecutive over-budget frames before a downshift.
	OverrunHoldFrames int
	// UpshiftHoldFrames: consecutive eligible frames before an upshift.
	UpshiftHoldFrames int
	// ShiftCooldownFrames: minimum frames between any two tier changes.
	ShiftCooldownFrames int64
	// HeadroomRatio: upshift needs gpu <= budget*(1-HeadroomRatio).
	HeadroomRatio float64
	// Error thresholds (on smoothed 0..1 errors) that justify more quality.
	CenterErrorThreshold     float64
	PeripheralErrorThreshold float64
	// FastGazeDps: smoothed gaze speed above this blocks upshifts.
	FastGazeDps float64
	// SpikeCooldownFrames: extra frames an upshift stays blocked after a spike.
	SpikeCooldownFrames int64
	// TrialFrames: frames a newly selected higher tier must survive cleanly.
	TrialFrames int64
	// FailureCooldownFrames: frames a failed tier is blocked from reselection.
	FailureCooldownFrames int64
	// Telemetry, may be nil.
	Telemetry *Telemetry
}

// Controller is the joint quality governor. It is safe for concurrent use.
type Controller struct {
	cfg Config
	mu  chan struct{} // token-based mutex, zero allocation per Observe

	frame     int64
	tier      int
	lastGood  int
	gpuEMA    ema
	centerEMA ema
	periphEMA ema
	gazeEMA   ema

	upCounter   int
	overCounter int
	lastShift   int64

	spikeUntil  int64
	wasSpike    bool
	wasFastGaze bool

	inTrial    bool
	trialUntil int64
	trialFrom  int
	trialTo    int

	blockedUntil map[int]int64
}

// DefaultConfig returns a conservative configuration for a ladder.
func DefaultConfig(ladder *Ladder) Config {
	return Config{
		Ladder:                   ladder,
		StartTier:                2,
		EMAAlpha:                 0.25,
		EMAWarmup:                4,
		OverrunHoldFrames:        3,
		UpshiftHoldFrames:        6,
		ShiftCooldownFrames:      8,
		HeadroomRatio:            0.15,
		CenterErrorThreshold:     0.12,
		PeripheralErrorThreshold: 0.20,
		FastGazeDps:              120,
		SpikeCooldownFrames:      10,
		TrialFrames:              12,
		FailureCooldownFrames:    60,
	}
}

// NewController validates the config and builds a governor.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Ladder == nil {
		return nil, fmt.Errorf("foveq: config requires a ladder")
	}
	if cfg.StartTier < cfg.Ladder.Min() || cfg.StartTier > cfg.Ladder.Max() {
		return nil, fmt.Errorf("%w: start tier %d", errTierOutOfRange, cfg.StartTier)
	}
	if cfg.EMAAlpha <= 0 {
		cfg.EMAAlpha = 0.25
	}
	if cfg.EMAWarmup <= 0 {
		cfg.EMAWarmup = 4
	}
	if cfg.OverrunHoldFrames <= 0 {
		cfg.OverrunHoldFrames = 3
	}
	if cfg.UpshiftHoldFrames <= 0 {
		cfg.UpshiftHoldFrames = 6
	}
	if cfg.ShiftCooldownFrames < 0 {
		cfg.ShiftCooldownFrames = 0
	}
	if cfg.HeadroomRatio <= 0 || cfg.HeadroomRatio >= 1 {
		cfg.HeadroomRatio = 0.15
	}
	if cfg.CenterErrorThreshold <= 0 {
		cfg.CenterErrorThreshold = 0.12
	}
	if cfg.PeripheralErrorThreshold <= 0 {
		cfg.PeripheralErrorThreshold = 0.20
	}
	if cfg.FastGazeDps <= 0 {
		cfg.FastGazeDps = 120
	}
	if cfg.SpikeCooldownFrames < 0 {
		cfg.SpikeCooldownFrames = 0
	}
	if cfg.TrialFrames <= 0 {
		cfg.TrialFrames = 12
	}
	if cfg.FailureCooldownFrames < 0 {
		cfg.FailureCooldownFrames = 0
	}
	token := make(chan struct{}, 1)
	token <- struct{}{}
	return &Controller{
		cfg:          cfg,
		mu:           token,
		tier:         cfg.StartTier,
		lastGood:     cfg.StartTier,
		gpuEMA:       newEMA(cfg.EMAAlpha, cfg.EMAWarmup),
		centerEMA:    newEMA(cfg.EMAAlpha, cfg.EMAWarmup),
		periphEMA:    newEMA(cfg.EMAAlpha, cfg.EMAWarmup),
		gazeEMA:      newEMA(cfg.EMAAlpha, cfg.EMAWarmup),
		blockedUntil: make(map[int]int64),
	}, nil
}

// CurrentTier returns the active tier index.
func (c *Controller) CurrentTier() int {
	<-c.mu
	defer func() { c.mu <- struct{}{} }()
	return c.tier
}

// LastGoodTier returns the most recent tier confirmed as healthy.
func (c *Controller) LastGoodTier() int {
	<-c.mu
	defer func() { c.mu <- struct{}{} }()
	return c.lastGood
}
