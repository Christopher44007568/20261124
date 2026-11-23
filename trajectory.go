package rollingshutter

import (
	"fmt"
	"time"
)

// ThreeSegmentTrajectory is a continuous camera path with three consecutive
// phases: translation, pitch and a short sudden turn (yaw). Each phase starts
// from the state reached by the previous phase.
type ThreeSegmentTrajectory struct {
	Start          time.Duration
	TranslationEnd time.Duration
	PitchEnd       time.Duration
	TurnEnd        time.Duration

	Translation Vec3
	Pitch       float64
	Turn        float64
}

func (t ThreeSegmentTrajectory) Validate() error {
	if !(t.Start < t.TranslationEnd && t.TranslationEnd < t.PitchEnd && t.PitchEnd < t.TurnEnd) {
		return fmt.Errorf(
			"trajectory boundaries must increase: start=%s translation=%s pitch=%s turn=%s",
			t.Start, t.TranslationEnd, t.PitchEnd, t.TurnEnd,
		)
	}
	return nil
}

// PoseAt samples the trajectory at an absolute time. Times outside the
// trajectory are reported as missing rather than extrapolated.
func (t ThreeSegmentTrajectory) PoseAt(at time.Duration) (Pose, error) {
	if err := t.Validate(); err != nil {
		return Pose{}, err
	}
	if at < t.Start || at > t.TurnEnd {
		return Pose{}, fmt.Errorf(
			"%w at %s: outside trajectory [%s, %s]",
			ErrPoseMissing, at, t.Start, t.TurnEnd,
		)
	}

	switch {
	case at <= t.TranslationEnd:
		fraction := fractionOf(t.Start, t.TranslationEnd, at)
		return Pose{Translation: t.Translation.Scale(fraction)}, nil
	case at <= t.PitchEnd:
		fraction := fractionOf(t.TranslationEnd, t.PitchEnd, at)
		return Pose{
			Translation: t.Translation,
			Pitch:       t.Pitch * fraction,
		}, nil
	default:
		fraction := fractionOf(t.PitchEnd, t.TurnEnd, at)
		return Pose{
			Translation: t.Translation,
			Pitch:       t.Pitch,
			Yaw:         t.Turn * fraction,
		}, nil
	}
}

// Sample returns timestamped poses from Start through TurnEnd inclusive.
func (t ThreeSegmentTrajectory) Sample(step time.Duration) ([]PoseSample, error) {
	if step <= 0 {
		return nil, fmt.Errorf("sample step must be positive: %s", step)
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	samples := make([]PoseSample, 0, int((t.TurnEnd-t.Start)/step)+2)
	for at := t.Start; at < t.TurnEnd; at += step {
		pose, err := t.PoseAt(at)
		if err != nil {
			return nil, err
		}
		samples = append(samples, PoseSample{At: at, Pose: pose})
	}
	finalPose, err := t.PoseAt(t.TurnEnd)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 || samples[len(samples)-1].At != t.TurnEnd {
		samples = append(samples, PoseSample{At: t.TurnEnd, Pose: finalPose})
	}
	return samples, nil
}

func fractionOf(start, end, at time.Duration) float64 {
	if at <= start {
		return 0
	}
	if at >= end {
		return 1
	}
	return float64(at-start) / float64(end-start)
}
