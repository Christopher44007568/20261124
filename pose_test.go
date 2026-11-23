package rollingshutter

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestKeyframePoseSourceInterpolatesButFlagsWideGaps(t *testing.T) {
	source, err := NewKeyframePoseSource([]PoseSample{
		{At: 0, Pose: Pose{Translation: Vec3{}}},
		{At: 10 * time.Millisecond, Pose: Pose{Translation: Vec3{X: 1}, Pitch: 0.2}},
	}, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	pose, err := source.PoseAt(5 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(pose.Translation.X-0.5) > 1e-12 || math.Abs(pose.Pitch-0.1) > 1e-12 {
		t.Fatalf("interpolated pose = %+v", pose)
	}

	sparse, err := NewKeyframePoseSource([]PoseSample{
		{At: 0, Pose: Pose{}},
		{At: 20 * time.Millisecond, Pose: Pose{}},
	}, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sparse.PoseAt(5 * time.Millisecond); !errors.Is(err, ErrPoseMissing) {
		t.Fatalf("wide gap error = %v, want ErrPoseMissing", err)
	}
	if _, err := sparse.PoseAt(20 * time.Millisecond); err != nil {
		t.Fatalf("exact keyframe should remain available: %v", err)
	}
}

func TestDropoutPoseSourceMarksOnlyConfiguredInterval(t *testing.T) {
	source := DropoutPoseSource{
		Source: StaticPoseSource{Pose: Pose{Translation: Vec3{X: 3}}},
		Start:  2 * time.Millisecond,
		End:    3 * time.Millisecond,
	}
	if pose, err := source.PoseAt(2500 * time.Microsecond); !errors.Is(err, ErrPoseMissing) || pose.Translation.X != 0 {
		t.Fatalf("dropout pose=%+v err=%v", pose, err)
	}
	if pose, err := source.PoseAt(3500 * time.Microsecond); err != nil || pose.Translation.X != 3 {
		t.Fatalf("outside dropout pose=%+v err=%v", pose, err)
	}
}

func TestKeyframePoseSourceRejectsDuplicateTimes(t *testing.T) {
	_, err := NewKeyframePoseSource([]PoseSample{
		{At: time.Millisecond, Pose: Pose{}},
		{At: time.Millisecond, Pose: Pose{}},
	}, time.Millisecond)
	if err == nil {
		t.Fatal("expected duplicate timestamp error")
	}
}
