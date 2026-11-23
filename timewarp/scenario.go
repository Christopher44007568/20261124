package timewarp

import (
	"fmt"
	"math"
	"sort"
)

// MotionProfile is the ground-truth head motion used by a scenario. YawRate is
// in degrees per second; motion freezes at StopAt when HasStop is true.
type MotionProfile struct {
	StartYaw float64
	YawRate  float64
	StopAt   Timestamp
	HasStop  bool
}

// YawAt returns the true yaw at t.
func (m MotionProfile) YawAt(t Timestamp) float64 {
	if t < 0 {
		t = 0
	}
	effective := t
	if m.HasStop && effective > m.StopAt {
		effective = m.StopAt
	}
	return m.StartYaw + m.YawRate*effective.Seconds()
}

// Interval is a half-open frame-drop range [Start, End).
type Interval struct {
	Start Timestamp
	End   Timestamp
}

func (i Interval) contains(t Timestamp) bool {
	return i.Start <= t && t < i.End
}

// ScenarioConfig describes a deterministic simulation. All rates are implied
// by interval lengths, so scanout and render events can be merged exactly.
type ScenarioConfig struct {
	Name            string
	Duration        Timestamp
	DisplayInterval Timestamp
	FrameInterval   Timestamp
	PoseInterval    Timestamp
	Motion          MotionProfile
	FrameDrop       *Interval
}

// ErrorSample compares the final image for one scanout with ground truth.
type ErrorSample struct {
	At                    Timestamp
	ImageID               string
	SourceFrameID         uint64
	Warped                bool
	ReusedFrame           bool
	NewSourceFrame        bool
	ChainDepth            int
	TargetPoseVersion     uint64
	GroundTruthYaw        float64
	TargetYaw             float64
	AbsYawError           float64
	CumulativeAbsYawError float64
}

// ScenarioReport contains both the audit trail and the error measurements for
// one run. FinalChain is the source-to-final provenance chain of the last
// scanout.
type ScenarioReport struct {
	Name             string
	Config           ScenarioConfig
	Stats            Stats
	Samples          []ErrorSample
	Records          []WarpRecord
	FinalChain       []Image
	LongestChain     []Image
	LongestChainAt   Timestamp
	MaxAbsYawError   float64
	SumAbsYawError   float64
	RMSYawError      float64
	FinalAbsYawError float64
}

type scenarioEvent struct {
	at       Timestamp
	priority int
}

const (
	posePriority = iota
	framePriority
	scanoutPriority
)

// RunScenario simulates frames, pose snapshots, and scanouts on one monotonic
// time axis. Frames submitted during a drop interval are absent; scanouts
// continue to consume the latest available source image.
func RunScenario(config ScenarioConfig) (ScenarioReport, error) {
	if err := validateScenario(config); err != nil {
		return ScenarioReport{}, err
	}

	events := buildEvents(config)
	pipeline := NewPipeline()
	report := ScenarioReport{Name: config.Name, Config: config}
	var (
		nextFrameID     uint64
		nextPoseVersion uint64
		latestPose      Pose
		havePose        bool
		lastSourceID    uint64
		lastScanout     Scanout
	)

	for _, event := range events {
		switch event.priority {
		case posePriority:
			nextPoseVersion++
			latestPose = Pose{
				Version: nextPoseVersion,
				At:      event.at,
				Yaw:     config.Motion.YawAt(event.at),
			}
			if _, err := pipeline.SubmitPose(latestPose); err != nil {
				return report, err
			}
			havePose = true

		case framePriority:
			if config.FrameDrop != nil && config.FrameDrop.contains(event.at) {
				continue
			}
			if !havePose {
				continue
			}
			nextFrameID++
			frame := Frame{
				ID:         nextFrameID,
				RenderTime: event.at,
				Pose:       latestPose,
			}
			if _, err := pipeline.SubmitFrame(frame); err != nil {
				return report, err
			}

		case scanoutPriority:
			scanout, err := pipeline.Scanout(event.at)
			if err != nil {
				return report, fmt.Errorf("scanout at %dus: %w", event.at, err)
			}
			if len(scanout.Provenance) == 0 {
				return report, fmt.Errorf("scanout at %dus has empty provenance", event.at)
			}
			finalImage := scanout.Provenance[len(scanout.Provenance)-1]
			sample := ErrorSample{
				At:                event.at,
				ImageID:           scanout.ImageID,
				SourceFrameID:     scanout.SourceFrameID,
				Warped:            scanout.Warped,
				NewSourceFrame:    scanout.SourceFrameID != lastSourceID,
				ChainDepth:        finalImage.ChainDepth,
				TargetPoseVersion: finalImage.Pose.Version,
				GroundTruthYaw:    config.Motion.YawAt(event.at),
				TargetYaw:         finalImage.Pose.Yaw,
			}
			sample.AbsYawError = math.Abs(YawDelta(sample.GroundTruthYaw, sample.TargetYaw))
			if scanout.Record != nil {
				sample.ReusedFrame = scanout.Record.ReusedFrame
			}
			report.SumAbsYawError += sample.AbsYawError
			sample.CumulativeAbsYawError = report.SumAbsYawError
			report.Samples = append(report.Samples, sample)
			lastSourceID = scanout.SourceFrameID
			lastScanout = scanout
			if len(scanout.Provenance) > len(report.LongestChain) {
				report.LongestChain = append([]Image(nil), scanout.Provenance...)
				report.LongestChainAt = event.at
			}
		}
	}

	report.Stats = pipeline.Stats()
	report.Records = pipeline.Records()
	report.FinalChain = append([]Image(nil), lastScanout.Provenance...)
	report.MaxAbsYawError, report.RMSYawError, report.FinalAbsYawError =
		summarizeErrors(report.Samples)
	return report, nil
}

func validateScenario(config ScenarioConfig) error {
	if config.Duration <= 0 {
		return fmt.Errorf("timewarp: scenario duration must be positive")
	}
	if config.DisplayInterval <= 0 || config.FrameInterval <= 0 || config.PoseInterval <= 0 {
		return fmt.Errorf("timewarp: scenario intervals must be positive")
	}
	if config.FrameDrop != nil {
		if config.FrameDrop.Start < 0 || config.FrameDrop.End <= config.FrameDrop.Start {
			return fmt.Errorf("timewarp: invalid frame drop interval")
		}
	}
	if config.Motion.HasStop && (config.Motion.StopAt < 0 || config.Motion.StopAt > config.Duration) {
		return fmt.Errorf("timewarp: stop time outside scenario")
	}
	return nil
}

func buildEvents(config ScenarioConfig) []scenarioEvent {
	events := make([]scenarioEvent, 0, 256)
	seenPoseTimes := make(map[Timestamp]struct{})

	addPose := func(at Timestamp) {
		if at > config.Duration {
			return
		}
		if _, exists := seenPoseTimes[at]; exists {
			return
		}
		seenPoseTimes[at] = struct{}{}
		events = append(events, scenarioEvent{at: at, priority: posePriority})
	}
	for at := Timestamp(0); at <= config.Duration; at += config.PoseInterval {
		addPose(at)
	}
	if config.Motion.HasStop {
		addPose(config.Motion.StopAt)
	}
	for at := Timestamp(0); at <= config.Duration; at += config.FrameInterval {
		events = append(events, scenarioEvent{at: at, priority: framePriority})
	}
	for at := Timestamp(0); at <= config.Duration; at += config.DisplayInterval {
		events = append(events, scenarioEvent{at: at, priority: scanoutPriority})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].at == events[j].at {
			return events[i].priority < events[j].priority
		}
		return events[i].at < events[j].at
	})
	return events
}

func summarizeErrors(samples []ErrorSample) (max, rms, final float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	var sumSquares float64
	for _, sample := range samples {
		if sample.AbsYawError > max {
			max = sample.AbsYawError
		}
		sumSquares += sample.AbsYawError * sample.AbsYawError
	}
	return max, math.Sqrt(sumSquares / float64(len(samples))), samples[len(samples)-1].AbsYawError
}
