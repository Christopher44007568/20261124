package rollingshutter

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// ErrPoseMissing marks a requested pose sample that cannot be supplied. The
// analyzer preserves the row and reports this condition instead of silently
// substituting a neighbouring pose.
var ErrPoseMissing = errors.New("pose sample missing")

// Vec3 is a world-space position in metres.
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v Vec3) Scale(factor float64) Vec3 {
	return Vec3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

// Pose is the camera position and orientation in world space. Angles are in
// radians. Camera axes are x right, y down and z forward, matching image rows.
type Pose struct {
	Translation Vec3    `json:"translation"`
	Pitch       float64 `json:"pitch"`
	Yaw         float64 `json:"yaw"`
	Roll        float64 `json:"roll"`
}

// WorldToCamera converts a world point into camera coordinates.
func (p Pose) WorldToCamera(world Vec3) Vec3 {
	camera := world.Sub(p.Translation)
	camera = rotateY(camera, -p.Yaw)
	camera = rotateX(camera, -p.Pitch)
	camera = rotateZ(camera, -p.Roll)
	return camera
}

func rotateX(v Vec3, angle float64) Vec3 {
	sine, cosine := math.Sincos(angle)
	return Vec3{
		X: v.X,
		Y: cosine*v.Y - sine*v.Z,
		Z: sine*v.Y + cosine*v.Z,
	}
}

func rotateY(v Vec3, angle float64) Vec3 {
	sine, cosine := math.Sincos(angle)
	return Vec3{
		X: cosine*v.X + sine*v.Z,
		Y: v.Y,
		Z: -sine*v.X + cosine*v.Z,
	}
}

func rotateZ(v Vec3, angle float64) Vec3 {
	sine, cosine := math.Sincos(angle)
	return Vec3{
		X: cosine*v.X - sine*v.Y,
		Y: sine*v.X + cosine*v.Y,
		Z: v.Z,
	}
}

// PoseSource supplies the pose used by one row. It is separate from both the
// exposure clock and the readout direction.
type PoseSource interface {
	PoseAt(at time.Duration) (Pose, error)
}

// StaticPoseSource returns one pose for every requested time.
type StaticPoseSource struct {
	Pose Pose
}

func (s StaticPoseSource) PoseAt(time.Duration) (Pose, error) {
	return s.Pose, nil
}

// PoseSample is a timestamped pose keyframe.
type PoseSample struct {
	At   time.Duration `json:"atNs"`
	Pose Pose          `json:"pose"`
}

// KeyframePoseSource interpolates between timestamped samples. A time outside
// the samples, or an interval wider than MaxGap, is reported as ErrPoseMissing.
type KeyframePoseSource struct {
	samples []PoseSample
	maxGap  time.Duration
}

// NewKeyframePoseSource copies and sorts samples. Duplicate timestamps and a
// non-positive maximum gap are rejected.
func NewKeyframePoseSource(samples []PoseSample, maxGap time.Duration) (*KeyframePoseSource, error) {
	if maxGap <= 0 {
		return nil, fmt.Errorf("maximum pose gap must be positive: %s", maxGap)
	}
	copied := append([]PoseSample(nil), samples...)
	sort.SliceStable(copied, func(i, j int) bool {
		return copied[i].At < copied[j].At
	})
	for i := 1; i < len(copied); i++ {
		if copied[i].At == copied[i-1].At {
			return nil, fmt.Errorf("duplicate pose sample at %s", copied[i].At)
		}
	}
	return &KeyframePoseSource{samples: copied, maxGap: maxGap}, nil
}

func (k *KeyframePoseSource) PoseAt(at time.Duration) (Pose, error) {
	if len(k.samples) == 0 {
		return Pose{}, fmt.Errorf("%w at %s: no keyframes", ErrPoseMissing, at)
	}
	index := sort.Search(len(k.samples), func(i int) bool {
		return k.samples[i].At >= at
	})
	if index < len(k.samples) && k.samples[index].At == at {
		return k.samples[index].Pose, nil
	}
	if index == 0 || index == len(k.samples) {
		return Pose{}, fmt.Errorf("%w at %s: outside keyframe range", ErrPoseMissing, at)
	}
	before, after := k.samples[index-1], k.samples[index]
	span := after.At - before.At
	if span > k.maxGap {
		return Pose{}, fmt.Errorf(
			"%w at %s: keyframe gap %s exceeds %s",
			ErrPoseMissing, at, span, k.maxGap,
		)
	}
	amount := float64(at-before.At) / float64(span)
	return interpolatePose(before.Pose, after.Pose, amount), nil
}

// DropoutPoseSource hides samples in [Start, End), making a deliberate gap for
// testing and for replaying real missing-data intervals.
type DropoutPoseSource struct {
	Source PoseSource
	Start  time.Duration
	End    time.Duration
}

func (d DropoutPoseSource) PoseAt(at time.Duration) (Pose, error) {
	if d.Source == nil {
		return Pose{}, fmt.Errorf("%w at %s: nil source", ErrPoseMissing, at)
	}
	if at >= d.Start && at < d.End {
		return Pose{}, fmt.Errorf("%w at %s: dropped by configured interval", ErrPoseMissing, at)
	}
	return d.Source.PoseAt(at)
}

func interpolatePose(before, after Pose, amount float64) Pose {
	return Pose{
		Translation: Vec3{
			X: before.Translation.X + (after.Translation.X-before.Translation.X)*amount,
			Y: before.Translation.Y + (after.Translation.Y-before.Translation.Y)*amount,
			Z: before.Translation.Z + (after.Translation.Z-before.Translation.Z)*amount,
		},
		Pitch: before.Pitch + (after.Pitch-before.Pitch)*amount,
		Yaw:   before.Yaw + (after.Yaw-before.Yaw)*amount,
		Roll:  before.Roll + (after.Roll-before.Roll)*amount,
	}
}
