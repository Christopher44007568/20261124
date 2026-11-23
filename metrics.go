package foveq

// Sample is the per-frame decision input. Every field that can drive a tier
// change is collected here so the controller decision is fully explainable.
type Sample struct {
	// Frame is a monotonically increasing frame counter.
	Frame int64
	// GPUMicros is the measured GPU time for the frame in microseconds.
	GPUMicros float64
	// CenterError is the measured foveal/central visual error (0..1).
	CenterError float64
	// PeripheralError is the measured peripheral error (0..1).
	PeripheralError float64
	// GazeVelocity is angular gaze speed in degrees per second.
	GazeVelocity float64
	// LoadSpike marks an externally detected transient load burst.
	LoadSpike bool
	// Failed marks a failed or aborted frame (e.g. GPU timeout, submit error).
	Failed bool
}

// ema is an exponential moving average with a warm-up phase that simply
// averages the first few observations so the filter starts unbiased.
type ema struct {
	value  float64
	alpha  float64
	count  int
	warmup int
}

func newEMA(alpha float64, warmup int) ema {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.25
	}
	if warmup < 1 {
		warmup = 1
	}
	return ema{alpha: alpha, warmup: warmup}
}

func (e *ema) update(v float64) float64 {
	e.count++
	if e.count == 1 {
		e.value = v
		return e.value
	}
	if e.count <= e.warmup {
		e.value += (v - e.value) / float64(e.count)
		return e.value
	}
	e.value += e.alpha * (v - e.value)
	return e.value
}

func (e *ema) get() float64 { return e.value }

// smoothedInputs are the filtered signals the rule engine actually consumes.
// Raw samples are kept on the Decision record for audit purposes.
type smoothedInputs struct {
	GPU        float64 `json:"gpu_micros"`
	Center     float64 `json:"center_error"`
	Peripheral float64 `json:"peripheral_error"`
	Gaze       float64 `json:"gaze_velocity"`
}

// Decision captures one full controller evaluation, including the raw inputs,
// the filtered inputs, every rule that fired and the resulting tier. It is the
// "decision input" record emitted to the telemetry sink.
type Decision struct {
	Frame    int64
	Raw      Sample
	Smoothed smoothedInputs
	Budget   float64
	Headroom float64
	FromTier int
	ToTier   int
	Reasons  []string
	Spike    bool
	FastGaze bool
	InTrial  bool
	LastGood int
}

// DecisionLog returns the reasons as a stable string slice for external
// loggers that do not wish to depend on the reason constants.
func (d Decision) DecisionLog() map[string]any {
	reasons := make([]string, len(d.Reasons))
	copy(reasons, d.Reasons)
	return map[string]any{
		"frame":           d.Frame,
		"gpu_micros":      d.Raw.GPUMicros,
		"gpu_smoothed":    d.Smoothed.GPU,
		"center_error":    d.Raw.CenterError,
		"center_smoothed": d.Smoothed.Center,
		"periph_error":    d.Raw.PeripheralError,
		"periph_smoothed": d.Smoothed.Peripheral,
		"gaze_velocity":   d.Raw.GazeVelocity,
		"gaze_smoothed":   d.Smoothed.Gaze,
		"budget_micros":   d.Budget,
		"headroom_micros": d.Headroom,
		"load_spike":      d.Spike,
		"fast_gaze":       d.FastGaze,
		"in_trial":        d.InTrial,
		"from_tier":       d.FromTier,
		"to_tier":         d.ToTier,
		"last_good_tier":  d.LastGood,
		"reasons":         reasons,
	}
}
