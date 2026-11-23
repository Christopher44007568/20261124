package xr

// Clock is a simulated monotonic render clock measured in milliseconds.
// It can be made to jump, which mirrors real-world timestamp discontinuities
// (driver restart, timestamp-domain switch, PTP re-sync).
type Clock struct {
	nowMs float64
}

func NewClock(startMs float64) *Clock { return &Clock{nowMs: startMs} }

func (c *Clock) Now() float64 { return c.nowMs }

// Advance moves the clock forward by dtMs (must be non-negative).
func (c *Clock) Advance(dtMs float64) { c.nowMs += dtMs }

// Jump shifts the clock by jumpMs (negative jumps are allowed on purpose).
func (c *Clock) Jump(jumpMs float64) { c.nowMs += jumpMs }

// Sample is one tracker measurement: the device pose in the current
// tracking-origin frame, plus the monotonic timestamp of the measurement.
type Sample struct {
	TMs    float64
	Pose   Pose
	Origin string
}

// RelocEvent announces that the tracker swapped world origin. Old samples are
// expressed in FromOrigin; new samples use ToOrigin. Delta maps old-frame
// coordinates into the new frame: p_new = Delta * p_old.
type RelocEvent struct {
	AtMs       float64
	FromOrigin string
	ToOrigin   string
	Delta      RigidTransform
}

// Anchor is a persistent virtual object with a known ground-truth world
// position. Its stored pose is expressed in the tracker's current origin.
type Anchor struct {
	ID     string
	Color  string
	Pose   Pose
	Origin string
}
