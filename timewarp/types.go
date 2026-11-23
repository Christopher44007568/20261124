// Package timewarp models late-latched display reprojection without depending
// on a graphics API. Images are symbolic frames; the pipeline tracks their
// pose, parent, and provenance rather than pixel storage.
package timewarp

import (
	"errors"
	"fmt"
	"math"
)

// Timestamp is a monotonic simulation time in microseconds. It is deliberately
// not a wall clock: the pipeline needs a single ordered time domain.
type Timestamp int64

// Millis converts milliseconds to a Timestamp.
func Millis(ms int64) Timestamp { return Timestamp(ms * 1000) }

// Micros converts microseconds to a Timestamp.
func Micros(us int64) Timestamp { return Timestamp(us) }

// Seconds returns the timestamp as seconds.
func (t Timestamp) Seconds() float64 { return float64(t) / 1e6 }

// Pose is the head orientation used by the reprojection model. Angles are in
// degrees. Version must increase whenever the pose snapshot changes.
type Pose struct {
	Version uint64
	At      Timestamp
	Yaw     float64
	Pitch   float64
	Roll    float64
}

// NormalizeYaw maps an angle in degrees into [-180, 180).
func NormalizeYaw(deg float64) float64 {
	deg = math.Mod(deg+180, 360)
	if deg < 0 {
		deg += 360
	}
	return deg - 180
}

// YawDelta returns the shortest signed yaw rotation from a to b.
func YawDelta(a, b float64) float64 {
	return NormalizeYaw(b - a)
}

// YawError returns the absolute angular error between two poses.
func YawError(a, b Pose) float64 {
	return math.Abs(YawDelta(a.Yaw, b.Yaw))
}

const poseEpsilon = 1e-9

func sameOrientation(a, b Pose) bool {
	return math.Abs(YawDelta(a.Yaw, b.Yaw)) <= poseEpsilon &&
		math.Abs(a.Pitch-b.Pitch) <= poseEpsilon &&
		math.Abs(a.Roll-b.Roll) <= poseEpsilon
}

// Frame is one rendered image delivered by the renderer. The pose is the pose
// version against which the image was rendered.
type Frame struct {
	ID         uint64
	RenderTime Timestamp
	Pose       Pose
}

// Validate checks the frame's local invariants.
func (f Frame) Validate() error {
	if f.ID == 0 {
		return fmt.Errorf("%w: frame ID must be non-zero", ErrInvalidFrame)
	}
	if f.Pose.At > f.RenderTime {
		return fmt.Errorf("%w: frame %d pose is newer than its render time", ErrInvalidFrame, f.ID)
	}
	return nil
}

// ImageKind distinguishes a renderer source image from a reprojection result.
type ImageKind string

const (
	ImageSource       ImageKind = "source"
	ImageIntermediate ImageKind = "intermediate"
)

// Image is a symbolic image in a provenance chain. Every warp output points
// to the image it consumed; source images have no parent.
type Image struct {
	ID            string
	Kind          ImageKind
	ParentID      string
	SourceFrameID uint64
	SourceImageID string
	Pose          Pose
	CreatedAt     Timestamp
	WarpSequence  uint64
	ChainDepth    int
}

// WarpRecord is the audit entry for one reprojection. HistorySourceID names
// the exact image consumed by this warp, which may itself be an intermediate.
type WarpRecord struct {
	Sequence               uint64
	ScanoutAt              Timestamp
	SourceFrameID          uint64
	SourceFramePoseVersion uint64
	HistorySourceID        string
	HistoryPoseVersion     uint64
	IntermediateImageID    string
	TargetPoseVersion      uint64
	FromPose               Pose
	TargetPose             Pose
	DeltaYaw               float64
	FrameAge               Timestamp
	ReusedFrame            bool
	ChainDepth             int
}

// Scanout describes the image selected for one display refresh.
type Scanout struct {
	At            Timestamp
	SourceFrameID uint64
	ImageID       string
	Warped        bool
	Record        *WarpRecord
	Provenance    []Image
}

// Stats are counters for accepted work. Duplicates are counted separately and
// never increase the accepted frame, pose, scanout, or warp totals.
type Stats struct {
	FramesAccepted   uint64
	DuplicateFrames  uint64
	PosesAccepted    uint64
	DuplicatePoses   uint64
	Scanouts         uint64
	NoFrameScanouts  uint64
	Warps            uint64
	ReusedFrameWarps uint64
	ClockStops       uint64
}

var (
	// ErrClockWentBackwards is terminal: a single time-domain regression stops
	// all future warps to avoid mixing incompatible histories.
	ErrClockWentBackwards = errors.New("timewarp: clock moved backwards; warp stopped")
	// ErrWarpStopped is returned by every operation after a clock regression.
	ErrWarpStopped = errors.New("timewarp: warp pipeline is stopped")
	// ErrNoFrame means a scanout happened before any source image was available.
	ErrNoFrame = errors.New("timewarp: no rendered frame available")
	// ErrNoPose means a scanout happened before any pose was available.
	ErrNoPose = errors.New("timewarp: no pose available")
	// ErrInvalidFrame reports malformed frame input.
	ErrInvalidFrame = errors.New("timewarp: invalid frame")
	// ErrUnknownImage reports a provenance lookup for an unknown image.
	ErrUnknownImage = errors.New("timewarp: unknown image")
	// ErrProvenanceCycle reports corrupt parent links.
	ErrProvenanceCycle = errors.New("timewarp: provenance cycle")
)
