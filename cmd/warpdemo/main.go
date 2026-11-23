package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Christopher44007568/20261124/timewarp"
)

type namedConfig struct {
	key    string
	config timewarp.ScenarioConfig
}

func main() {
	scenario := flag.String("scenario", "all", "all, constant, stop, or drop")
	flag.Parse()

	configs, err := selectScenarios(strings.ToLower(*scenario))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, item := range configs {
		report, err := timewarp.RunScenario(item.config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", item.key, err)
			os.Exit(1)
		}
		printReport(report)
	}
	if *scenario == "all" {
		printGuards()
	}
}

func selectScenarios(key string) ([]namedConfig, error) {
	constant := timewarp.ScenarioConfig{
		Name:            "constant-turn",
		Duration:        timewarp.Millis(240),
		DisplayInterval: timewarp.Micros(8333),
		FrameInterval:   timewarp.Millis(33),
		PoseInterval:    timewarp.Millis(4),
		Motion: timewarp.MotionProfile{
			StartYaw: -45,
			YawRate:  90,
		},
	}
	stop := constant
	stop.Name = "sudden-stop"
	stop.Duration = timewarp.Millis(260)
	stop.Motion.HasStop = true
	stop.Motion.StopAt = timewarp.Millis(120)

	drop := constant
	drop.Name = "frame-drop-recovery"
	drop.Duration = timewarp.Millis(310)
	dropInterval := timewarp.Interval{
		Start: timewarp.Millis(100),
		End:   timewarp.Millis(180),
	}
	drop.FrameDrop = &dropInterval

	switch key {
	case "constant":
		return []namedConfig{{key: key, config: constant}}, nil
	case "stop":
		return []namedConfig{{key: key, config: stop}}, nil
	case "drop":
		return []namedConfig{{key: key, config: drop}}, nil
	case "all":
		return []namedConfig{
			{key: "constant", config: constant},
			{key: "stop", config: stop},
			{key: "drop", config: drop},
		}, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q", key)
	}
}

func printReport(report timewarp.ScenarioReport) {
	fmt.Printf("\n== %s ==\n", report.Name)
	fmt.Printf("frames=%d duplicate_frames=%d poses=%d scanouts=%d warps=%d reused=%d\n",
		report.Stats.FramesAccepted, report.Stats.DuplicateFrames,
		report.Stats.PosesAccepted, report.Stats.Scanouts,
		report.Stats.Warps, report.Stats.ReusedFrameWarps)
	fmt.Printf("error_deg: max=%.6f final=%.6f sum_abs=%.6f rms=%.6f\n",
		report.MaxAbsYawError, report.FinalAbsYawError,
		report.SumAbsYawError, report.RMSYawError)

	if len(report.Samples) > 0 {
		final := report.Samples[len(report.Samples)-1]
		fmt.Printf("final_scanout: at=%dus frame=%d image=%s chain_depth=%d pose_version=%d\n",
			final.At, final.SourceFrameID, final.ImageID,
			final.ChainDepth, final.TargetPoseVersion)
	}
	printChain("source chain (source -> final)", report.FinalChain)
	if len(report.LongestChain) > len(report.FinalChain) {
		printChain(fmt.Sprintf("longest observed chain (scanout %dus)", report.LongestChainAt),
			report.LongestChain)
	}
	printRecentWarps(report.Records)
}

func printChain(label string, chain []timewarp.Image) {
	fmt.Printf("%s:\n", label)
	for i, image := range chain {
		parent := image.ParentID
		if parent == "" {
			parent = "-"
		}
		fmt.Printf("  [%02d] %-10s kind=%-12s frame=%-2d pose=v%-2d yaw=%9.4f parent=%s\n",
			i, image.ID, image.Kind, image.SourceFrameID,
			image.Pose.Version, image.Pose.Yaw, parent)
	}
}

func printRecentWarps(records []timewarp.WarpRecord) {
	const limit = 4
	start := len(records) - limit
	if start < 0 {
		start = 0
	}
	fmt.Println("last warps:")
	for _, record := range records[start:] {
		fmt.Printf("  #%d at=%dus frame=%d pose=v%d->v%d history=%s out=%s delta=%.4f\n",
			record.Sequence, record.ScanoutAt, record.SourceFrameID,
			record.HistoryPoseVersion, record.TargetPoseVersion,
			record.HistorySourceID, record.IntermediateImageID, record.DeltaYaw)
	}
}

func printGuards() {
	p := timewarp.NewPipeline()
	first := timewarp.Pose{Version: 1, At: timewarp.Millis(10), Yaw: 0}
	_, _ = p.SubmitPose(first)
	frame := timewarp.Frame{ID: 1, RenderTime: timewarp.Millis(10), Pose: first}
	_, _ = p.SubmitFrame(frame)
	accepted, _ := p.SubmitFrame(frame)

	second := timewarp.Pose{Version: 2, At: timewarp.Millis(20), Yaw: 2}
	_, _ = p.SubmitPose(second)
	_, _ = p.Scanout(timewarp.Millis(24))
	_, regression := p.SubmitFrame(timewarp.Frame{
		ID:         2,
		RenderTime: timewarp.Millis(15),
		Pose:       first,
	})
	_, future := p.Scanout(timewarp.Millis(32))

	fmt.Println("\n== guards ==")
	fmt.Printf("duplicate_frame_accepted=%t accepted_frames=%d duplicate_frames=%d\n",
		accepted, p.Stats().FramesAccepted, p.Stats().DuplicateFrames)
	fmt.Printf("clock_regression=%v\nfuture_scanout=%v\nstopped=%t\n",
		regression, future, p.Stopped())
}
