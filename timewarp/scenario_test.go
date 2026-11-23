package timewarp_test

import (
	"math"
	"testing"

	"github.com/Christopher44007568/20261124/timewarp"
)

func baseScenario(name string) timewarp.ScenarioConfig {
	return timewarp.ScenarioConfig{
		Name:            name,
		Duration:        timewarp.Millis(300),
		DisplayInterval: timewarp.Micros(8333),
		FrameInterval:   timewarp.Millis(33),
		PoseInterval:    timewarp.Millis(4),
		Motion: timewarp.MotionProfile{
			StartYaw: -45,
			YawRate:  90,
		},
	}
}

func TestConstantTurnAccumulatesNoPoseDrift(t *testing.T) {
	config := baseScenario("constant-turn")
	config.Duration = timewarp.Millis(240)
	report, err := timewarp.RunScenario(config)
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}
	if report.Stats.FramesAccepted < 3 {
		t.Fatalf("accepted frames = %d, want at least 3", report.Stats.FramesAccepted)
	}
	if report.Stats.Warps <= report.Stats.FramesAccepted {
		t.Fatalf("warps = %d, expected repeated reprojection of frames", report.Stats.Warps)
	}
	if report.MaxAbsYawError > 0.37 {
		t.Fatalf("max yaw error = %.6f deg, want <= 0.37", report.MaxAbsYawError)
	}
	if report.FinalAbsYawError > 0.37 {
		t.Fatalf("final yaw error = %.6f deg, want <= 0.37", report.FinalAbsYawError)
	}
	if len(report.FinalChain) < 2 {
		t.Fatalf("final chain has %d images, want intermediate -> final", len(report.FinalChain))
	}
	if len(report.LongestChain) < 3 {
		t.Fatalf("longest chain has %d images, want multiple intermediates", len(report.LongestChain))
	}
	for i, record := range report.Records {
		if record.TargetPoseVersion <= record.HistoryPoseVersion {
			t.Fatalf("record %d has non-increasing pose version %d->%d", i,
				record.HistoryPoseVersion, record.TargetPoseVersion)
		}
		if record.ChainDepth < 2 {
			t.Fatalf("record %d chain depth = %d, want at least 2", i, record.ChainDepth)
		}
	}
}

func TestSuddenStopSettlesWithoutFurtherWarps(t *testing.T) {
	config := baseScenario("sudden-stop")
	config.Duration = timewarp.Millis(260)
	config.Motion.HasStop = true
	config.Motion.StopAt = timewarp.Millis(120)
	report, err := timewarp.RunScenario(config)
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}
	if report.MaxAbsYawError > 0.37 {
		t.Fatalf("max yaw error = %.6f deg, want <= 0.37", report.MaxAbsYawError)
	}

	warpsAfterStop := 0
	for _, record := range report.Records {
		if record.ScanoutAt > config.Motion.StopAt {
			warpsAfterStop++
			if math.Abs(record.DeltaYaw) > 0.37 {
				t.Fatalf("post-stop warp delta = %.6f deg", record.DeltaYaw)
			}
		}
	}
	if warpsAfterStop > 1 {
		t.Fatalf("warps after stop = %d, want at most the settling warp", warpsAfterStop)
	}
	for _, sample := range report.Samples {
		if sample.At > config.Motion.StopAt && sample.AbsYawError > 1e-9 {
			t.Fatalf("post-stop sample at %dus still drifts by %.6f deg",
				sample.At, sample.AbsYawError)
		}
	}
}

func TestFrameDropRecoversFreshSourceAndChain(t *testing.T) {
	config := baseScenario("frame-drop-recovery")
	config.Duration = timewarp.Millis(310)
	drop := timewarp.Interval{Start: timewarp.Millis(100), End: timewarp.Millis(180)}
	config.FrameDrop = &drop
	report, err := timewarp.RunScenario(config)
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}
	if report.Stats.ReusedFrameWarps < 5 {
		t.Fatalf("reused-frame warps = %d, want repeated warps during drop",
			report.Stats.ReusedFrameWarps)
	}
	if report.MaxAbsYawError > 0.37 {
		t.Fatalf("max yaw error = %.6f deg, want <= 0.37", report.MaxAbsYawError)
	}
	if report.FinalAbsYawError > 0.37 {
		t.Fatalf("final yaw error after recovery = %.6f deg, want <= 0.37",
			report.FinalAbsYawError)
	}

	var recovery *timewarp.ErrorSample
	var postDropMax float64
	for i := range report.Samples {
		sample := &report.Samples[i]
		if sample.At >= drop.End && sample.AbsYawError > postDropMax {
			postDropMax = sample.AbsYawError
		}
		if recovery == nil && sample.At >= drop.End && sample.NewSourceFrame {
			recovery = sample
		}
	}
	if postDropMax > 0.37 {
		t.Fatalf("post-drop max yaw error = %.6f deg, want <= 0.37", postDropMax)
	}
	if recovery == nil {
		t.Fatal("no fresh source frame after the drop")
	}
	if recovery.ChainDepth > 2 {
		t.Fatalf("recovery chain depth = %d, want a fresh source chain", recovery.ChainDepth)
	}
	if len(report.FinalChain) < 2 {
		t.Fatal("final chain should contain post-drop intermediates")
	}
	base := report.FinalChain[0]
	final := report.FinalChain[len(report.FinalChain)-1]
	if base.Kind != timewarp.ImageSource {
		t.Fatalf("chain base kind = %q, want source", base.Kind)
	}
	if base.SourceFrameID != report.Samples[len(report.Samples)-1].SourceFrameID {
		t.Fatalf("final chain source frame = %d, current source frame = %d",
			base.SourceFrameID, report.Samples[len(report.Samples)-1].SourceFrameID)
	}
	if final.Pose.Version != report.Samples[len(report.Samples)-1].TargetPoseVersion {
		t.Fatalf("final chain pose version = %d, sample version = %d",
			final.Pose.Version, report.Samples[len(report.Samples)-1].TargetPoseVersion)
	}
}

func TestScenarioRejectsInvalidConfiguration(t *testing.T) {
	config := baseScenario("invalid")
	config.DisplayInterval = 0
	if _, err := timewarp.RunScenario(config); err == nil {
		t.Fatal("expected invalid interval error")
	}
}
