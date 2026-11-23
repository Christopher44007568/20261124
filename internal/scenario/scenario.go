package scenario

import (
	"math"

	"stereogaze/internal/gaze"
)

// Frame is one timed eye configuration in a scenario.
type Frame struct {
	Index int            `json:"index"`
	Left  gaze.EyeConfig `json:"left"`
	Right gaze.EyeConfig `json:"right"`
}

// Scenario is a deterministic sequence of binocular eye states.
type Scenario struct {
	Name   string  `json:"name"`
	Frames []Frame `json:"frames"`
}

// ReverseSaccades drives the eyes through mirrored horizontal saccades.
// The left fovea repeatedly lands where the right eye has only periphery.
func ReverseSaccades(frames int) Scenario {
	return build("reverse_saccades", frames, func(progress float64) (gaze.EyeConfig, gaze.EyeConfig) {
		sweep := 0.5 + 0.38*math.Cos(2*math.Pi*progress)
		left := eye(gaze.LeftEye, gaze.Point{X: sweep, Y: 0.5}, false)
		right := eye(gaze.RightEye, gaze.Point{X: 1 - sweep, Y: 0.5}, false)
		return left, right
	})
}

// BriefOcclusion keeps the right eye tracking while the left eye briefly
// loses tracking. Sharing must stop for every occluded frame.
func BriefOcclusion(frames int) Scenario {
	return build("brief_occlusion", frames, func(progress float64) (gaze.EyeConfig, gaze.EyeConfig) {
		phase := 2 * math.Pi * progress
		leftGaze := gaze.Point{
			X: 0.5 + 0.32*math.Cos(phase),
			Y: 0.5 + 0.22*math.Sin(phase),
		}
		rightGaze := gaze.Point{
			X: 0.5 + 0.32*math.Cos(phase+0.9),
			Y: 0.5 + 0.22*math.Sin(phase+0.9),
		}
		occluded := progress >= 0.35 && progress < 0.65
		return eye(gaze.LeftEye, leftGaze, occluded), eye(gaze.RightEye, rightGaze, false)
	})
}

// IdenticalGaze places both gaze points on the same location every frame.
// This is the maximum-sharing case; boundaries still stay independently defined.
func IdenticalGaze(frames int) Scenario {
	return build("identical_gaze", frames, func(progress float64) (gaze.EyeConfig, gaze.EyeConfig) {
		phase := 2 * math.Pi * progress
		sharedGaze := gaze.Point{
			X: 0.5 + 0.30*math.Sin(phase),
			Y: 0.5 + 0.24*math.Sin(2*phase),
		}
		return eye(gaze.LeftEye, sharedGaze, false), eye(gaze.RightEye, sharedGaze, false)
	})
}

// All returns every mandatory scenario in execution order.
func All(frames int) []Scenario {
	return []Scenario{
		ReverseSaccades(frames),
		BriefOcclusion(frames),
		IdenticalGaze(frames),
	}
}

func build(name string, frames int, states func(float64) (gaze.EyeConfig, gaze.EyeConfig)) Scenario {
	if frames <= 0 {
		frames = 1
	}
	result := Scenario{Name: name}
	for index := 0; index < frames; index++ {
		progress := float64(index) / float64(frames)
		left, right := states(progress)
		result.Frames = append(result.Frames, Frame{
			Index: index,
			Left:  left,
			Right: right,
		})
	}
	return result
}

func eye(id gaze.Eye, point gaze.Point, occluded bool) gaze.EyeConfig {
	return gaze.EyeConfig{
		Eye:      id,
		Gaze:     point,
		Occluded: occluded,
		Params:   gaze.DefaultParams(),
	}
}
