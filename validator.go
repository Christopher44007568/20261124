package posetimeline

// Config tunes a Validator. Every tolerance is in primary-domain nanoseconds.
type Config struct {
	// MaxPoseAge rejects a pose whose translated timestamp is older than
	// referenceTime - MaxPoseAge. Zero disables the expiry rule.
	MaxPoseAge int64
	// EyeTolerance rejects a left/right pair whose timestamps differ by more
	// than this value. Zero requires an exact match.
	EyeTolerance int64
}

// acceptedEntry is one sample that survived validation.
type acceptedEntry struct {
	globalTime int64
	sample     Sample
}

// eyeState holds the monotonic timeline of one eye.
type eyeState struct {
	lastTime    int64
	haveLast    bool
	lastFrameID uint64
	haveFrame   bool
	lastPose    Pose
	havePose    bool
	accepted    []acceptedEntry
	broken      bool
	integrated  int // number of samples that contributed to integration
}

func newEyeState() *eyeState {
	return &eyeState{}
}

// Validator builds and guards per-eye pose timelines. It never treats array
// position as recency: ordering is always derived from translated timestamps,
// and an invalidated timeline stops motion accumulation until Reset.
type Validator struct {
	cfg       Config
	clocks    *ClockRegistry
	eyes      [2]*eyeState
	decisions []Decision
}

// NewValidator constructs a Validator with the given config and clock
// registry. A nil registry yields one that only knows the primary domain.
func NewValidator(cfg Config, clocks *ClockRegistry) *Validator {
	if clocks == nil {
		clocks = NewClockRegistry()
	}
	return &Validator{
		cfg:    cfg,
		clocks: clocks,
		eyes:   [2]*eyeState{newEyeState(), newEyeState()},
	}
}

// Reset clears every timeline. The next accepted sample becomes the new
// integration baseline; no motion is carried across the reset.
func (v *Validator) Reset() {
	v.eyes = [2]*eyeState{newEyeState(), newEyeState()}
	v.decisions = nil
}

// TimelineBroken reports whether an eye's timeline was invalidated and is
// awaiting Reset / re-anchoring.
func (v *Validator) TimelineBroken(eye Eye) bool {
	if int(eye) >= len(v.eyes) {
		return false
	}
	return v.eyes[eye].broken
}

// AcceptedCount returns how many samples an eye has accepted so far.
func (v *Validator) AcceptedCount(eye Eye) int {
	if int(eye) >= len(v.eyes) {
		return 0
	}
	return len(v.eyes[eye].accepted)
}

// Decisions returns all decisions in batch submission order.
func (v *Validator) Decisions() []Decision {
	out := make([]Decision, len(v.decisions))
	copy(out, v.decisions)
	return out
}
