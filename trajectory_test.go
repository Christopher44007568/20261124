package rollingshutter

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestThreeSegmentTrajectoryPoseProgression(t *testing.T) {
	trajectory := ThreeSegmentTrajectory{
		Start:          0,
		TranslationEnd: time.Millisecond,
		PitchEnd:       2 * time.Millisecond,
		TurnEnd:        3 * time.Millisecond,
		Translation:    Vec3{X: 1},
		Pitch:          0.1,
		Turn:           0.2,
	}
	cases := []struct {
		at    time.Duration
		x     float64
		pitch float64
		yaw   float64
	}{
		{500 * time.Microsecond, 0.5, 0, 0},
		{1500 * time.Microsecond, 1, 0.05, 0},
		{2500 * time.Microsecond, 1, 0.1, 0.1},
		{3 * time.Millisecond, 1, 0.1, 0.2},
	}
	for _, test := range cases {
		pose, err := trajectory.PoseAt(test.at)
		if err != nil {
			t.Fatalf("at %s: %v", test.at, err)
		}
		if math.Abs(pose.Translation.X-test.x) > 1e-12 ||
			math.Abs(pose.Pitch-test.pitch) > 1e-12 ||
			math.Abs(pose.Yaw-test.yaw) > 1e-12 {
			t.Fatalf("at %s pose = %+v", test.at, pose)
		}
	}
	if _, err := trajectory.PoseAt(3*time.Millisecond + time.Nanosecond); !errors.Is(err, ErrPoseMissing) {
		t.Fatalf("after trajectory error = %v, want ErrPoseMissing", err)
	}
}

func TestThreeSegmentTrajectorySampling(t *testing.T) {
	trajectory := ThreeSegmentTrajectory{
		Start:          0,
		TranslationEnd: time.Millisecond,
		PitchEnd:       2 * time.Millisecond,
		TurnEnd:        3 * time.Millisecond,
	}
	samples, err := trajectory.Sample(500 * time.Microsecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 7 || samples[len(samples)-1].At != 3*time.Millisecond {
		t.Fatalf("samples = %+v", samples)
	}
}
