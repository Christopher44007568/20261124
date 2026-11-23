package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"rollingshutter"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("rollingshutter", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	directionName := flags.String("direction", "top-to-bottom", "top-to-bottom or bottom-to-top")
	rows := flags.Int("rows", 480, "image/readout row count")
	rowPeriod := flags.Duration("row-period", 8*time.Microsecond, "exposure start spacing")
	exposure := flags.Duration("exposure", time.Millisecond, "per-row exposure duration")
	poseOffset := flags.Duration("pose-offset", 500*time.Microsecond, "pose sample offset from exposure start")
	frameStart := flags.Duration("frame-start", 0, "exposure start of first row in readout order")

	translationX := flags.Float64("translation-x", -0.18, "translation segment x displacement in metres")
	pitch := flags.Float64("pitch", 0.06, "pitch segment rotation in radians")
	turn := flags.Float64("turn", 0.16, "sudden turn segment yaw in radians")
	dropStart := flags.Duration("drop-pose-start", 0, "start of deliberate missing-pose interval")
	dropEnd := flags.Duration("drop-pose-end", 0, "end of deliberate missing-pose interval")
	summaryOnly := flags.Bool("summary-only", false, "omit per-row projections")
	pretty := flags.Bool("pretty", true, "indent JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *rows <= 0 {
		return fmt.Errorf("rows must be positive: %d", *rows)
	}

	direction, err := parseDirection(*directionName)
	if err != nil {
		return err
	}
	timing, err := rollingshutter.NewRowTiming(rollingshutter.RowTimingConfig{
		FirstExposureStart: *frameStart,
		RowPeriod:          *rowPeriod,
		ExposureDuration:   *exposure,
		PoseSampleOffset:   *poseOffset,
		Direction:          direction,
	})
	if err != nil {
		return err
	}

	trajectory := rollingshutter.ThreeSegmentTrajectory{
		Start:          0,
		TranslationEnd: 1500 * time.Microsecond,
		PitchEnd:       3 * time.Millisecond,
		TurnEnd:        4800 * time.Microsecond,
		Translation:    rollingshutter.Vec3{X: *translationX},
		Pitch:          *pitch,
		Turn:           *turn,
	}
	var poseSource rollingshutter.PoseSource = trajectory
	if *dropStart != 0 || *dropEnd != 0 {
		if *dropEnd <= *dropStart {
			return fmt.Errorf("drop-pose-end must be after drop-pose-start")
		}
		poseSource = rollingshutter.DropoutPoseSource{
			Source: trajectory,
			Start:  *dropStart,
			End:    *dropEnd,
		}
	}

	camera := rollingshutter.DefaultCamera()
	camera.Height = *rows
	camera.CY = float64(*rows) / 2
	analysis, err := rollingshutter.Analyze(rollingshutter.AnalysisConfig{
		Timing: timing,
		Camera: camera,
		Line: rollingshutter.LineSegment{
			A: rollingshutter.Vec3{X: -0.8, Y: -2, Z: 2.4},
			B: rollingshutter.Vec3{X: 0.8, Y: 2, Z: 2.4},
		},
		PoseSource: poseSource,
	})
	if err != nil {
		return err
	}
	if *summaryOnly {
		analysis.Rows = nil
	}

	encoder := json.NewEncoder(output)
	if *pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(analysis)
}

func parseDirection(value string) (rollingshutter.ReadoutDirection, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "top-to-bottom", "top-down", "forward":
		return rollingshutter.TopToBottom, nil
	case "bottom-to-top", "bottom-up", "reversed", "reverse":
		return rollingshutter.BottomToTop, nil
	default:
		return 0, fmt.Errorf("unknown readout direction %q", value)
	}
}
