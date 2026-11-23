package posetimeline

// All timestamps are int64 nanoseconds. They are opaque tick values inside a
// single clock domain; comparison across domains is only legal through a
// ClockMap registered on the Validator.

// Eye identifies which render eye a sample belongs to.
type Eye uint8

const (
	EyeLeft Eye = iota
	EyeRight
)

// Vec3 is a translation in the tracker's metric frame.
type Vec3 struct {
	X, Y, Z float64
}

// Quat is a unit quaternion (W real part, X/Y/Z imaginary parts).
type Quat struct {
	W, X, Y, Z float64
}

// Pose is a rigid-body pose at one timestamp.
type Pose struct {
	Position    Vec3
	Orientation Quat
}

// Sample is one incoming pose observation.
type Sample struct {
	Eye       Eye
	Clock     string // clock domain id, "" means the primary/default domain
	FrameID   uint64 // tracker frame id; 0 disables the frame-duplicate rule
	Timestamp int64  // raw tick inside Clock, nanoseconds
	Pose      Pose
}

// RejectReason is a bitmask of every reason a sample was refused.
type RejectReason uint32

const (
	RejNone RejectReason = 0
	// RejUnknownClock: no translation is registered for this clock domain.
	RejUnknownClock RejectReason = 1 << iota
	// RejDuplicateTimestamp: another sample for this eye carries the same
	// translated timestamp.
	RejDuplicateTimestamp
	// RejClockBackwards: translated timestamp is older than the previously
	// accepted sample (a non-monotonic / regressing clock).
	RejClockBackwards
	// RejExpired: pose is older than the configured max pose age relative to
	// the reference time.
	RejExpired
	// RejFuture: pose lies in the future relative to the reference time.
	RejFuture
	// RejEyeMismatch: left/right pair timestamps differ by more than the
	// configured tolerance.
	RejEyeMismatch
	// RejDuplicateFrame: two samples for the same eye reuse one frame id at
	// different timestamps (more than one pose per frame).
	RejDuplicateFrame
	// RejTimelineBroken: an earlier fault invalidated the timeline, and the
	// sample cannot re-anchor it by itself.
	RejTimelineBroken
)

// Decision records the acceptance outcome for one submitted sample.
type Decision struct {
	// Index is the sample's position in the Submit batch.
	Index int
	// Sample is a copy of the submitted sample.
	Sample Sample
	// Accepted is true only when Reasons == RejNone.
	Accepted bool
	// Reasons is the OR-ed rejection bitmask.
	Reasons RejectReason
	// GlobalTime is the timestamp translated onto the primary clock domain.
	// Zero when translation failed.
	GlobalTime int64
	// ReferenceTime is the reference tick (primary domain) used for the
	// expired/future evaluation.
	ReferenceTime int64
}

// String renders a rejection bitmask as a stable, pipe-separated label list.
func (r RejectReason) String() string {
	if r == RejNone {
		return "none"
	}
	labels := []struct {
		bit   RejectReason
		label string
	}{
		{RejUnknownClock, "unknown_clock"},
		{RejDuplicateTimestamp, "duplicate_timestamp"},
		{RejClockBackwards, "clock_backwards"},
		{RejExpired, "expired"},
		{RejFuture, "future_pose"},
		{RejEyeMismatch, "eye_time_mismatch"},
		{RejDuplicateFrame, "duplicate_frame"},
		{RejTimelineBroken, "timeline_broken"},
	}
	out := ""
	for _, item := range labels {
		if r&item.bit != 0 {
			if out != "" {
				out += "|"
			}
			out += item.label
		}
	}
	return out
}
