package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"example.com/reprojection"
)

type report struct {
	Width          int                    `json:"width"`
	Height         int                    `json:"height"`
	Frames         int                    `json:"frames"`
	StepMS         float64                `json:"step_ms"`
	MaxYawDegPerS  float64                `json:"max_yaw_deg_per_s"`
	WorldLatencyMS float64                `json:"world_latency_ms"`
	UIInputMS      float64                `json:"ui_input_latency_ms"`
	Summaries      []reprojection.Summary `json:"summaries"`
	Checks         map[string]bool        `json:"checks"`
}

func main() {
	frames := flag.Int("frames", 90, "number of scanout frames")
	fps := flag.Float64("fps", 90, "scanout rate")
	startMS := flag.Float64("start-ms", 250, "first scanout time in milliseconds")
	width := flag.Int("width", 480, "display width")
	height := flag.Int("height", 270, "display height")
	vfov := flag.Float64("vfov-deg", 60, "vertical field of view in degrees")
	worldLatencyMS := flag.Float64("world-latency-ms", 11, "render-to-scanout world latency")
	uiInputMS := flag.Float64("ui-input-ms", 4, "latest UI input sample age")
	pngDir := flag.String("png-dir", "", "optional directory for one sample frame per mode")
	pngAtMS := flag.Float64("png-at-ms", 300, "scanout time of exported PNGs")
	asJSON := flag.Bool("json", false, "write the report as JSON")
	flag.Parse()

	if *frames <= 0 || *fps <= 0 || *width <= 0 || *height <= 0 {
		fmt.Fprintln(os.Stderr, "frames, fps, width and height must be positive")
		os.Exit(2)
	}
	step := time.Duration(float64(time.Second) / *fps)
	start := time.Duration(*startMS * float64(time.Millisecond))

	s := reprojection.DefaultScenario()
	s.Camera = reprojection.NewCamera(*width, *height, *vfov*math.Pi/180)
	s.WorldLatency = time.Duration(*worldLatencyMS * float64(time.Millisecond))
	s.UIInputLatency = time.Duration(*uiInputMS * float64(time.Millisecond))

	summaries, err := s.Compare(start, step, *frames)
	if err != nil {
		fmt.Fprintln(os.Stderr, "compare:", err)
		os.Exit(1)
	}
	checks := make(map[string]bool)
	for _, sum := range summaries {
		switch sum.Mode {
		case reprojection.Layered:
			checks["layered_no_world_depth_occlusion"] = sum.Max.DepthRejectedRate == 0
			checks["layered_fixed_ui_zero_drift"] = sum.Max.FixedUIDriftPx == 0
			checks["layered_same_scanout"] = sum.Max.ScanoutSkewMs == 0
			checks["layered_matches_reference"] = sum.Max.ReferenceMismatch == 0
		case reprojection.MergedDepth:
			checks["naive_merge_gets_occluded"] = sum.Max.DepthRejectedRate > 0
		case reprojection.InheritedWorldCamera:
			checks["inherited_camera_drifts"] = sum.Max.FixedUIDriftPx > 1
			checks["inherited_camera_skews_scanout"] = sum.Max.ScanoutSkewMs > 0
			checks["inherited_camera_adds_follow_latency"] = sum.Max.FollowLatencyMs > float64(*uiInputMS)
		}
	}

	rep := report{
		Width:          *width,
		Height:         *height,
		Frames:         *frames,
		StepMS:         float64(step) / float64(time.Millisecond),
		MaxYawDegPerS:  maxYawRate(s, start, step, *frames),
		WorldLatencyMS: *worldLatencyMS,
		UIInputMS:      *uiInputMS,
		Summaries:      summaries,
		Checks:         checks,
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, "json:", err)
			os.Exit(1)
		}
	} else {
		printReport(rep)
	}

	if *pngDir != "" {
		at := time.Duration(*pngAtMS * float64(time.Millisecond))
		if err := writePNGs(*pngDir, s, at); err != nil {
			fmt.Fprintln(os.Stderr, "png:", err)
			os.Exit(1)
		}
		fmt.Printf("\nPNG frames written to %s\n", *pngDir)
	}
}

func maxYawRate(s reprojection.Scenario, start, step time.Duration, frames int) float64 {
	if frames == 0 {
		return 0
	}
	end := start + time.Duration(frames-1)*step
	const dt = 250 * time.Microsecond
	maxRate := 0.0
	prev := s.HeadAt(start).Yaw
	for t := start + dt; t <= end; t += dt {
		yaw := s.HeadAt(t).Yaw
		rate := math.Abs(yaw-prev) / dt.Seconds()
		if rate > maxRate {
			maxRate = rate
		}
		prev = yaw
	}
	return maxRate * 180 / math.Pi
}

func printReport(rep report) {
	fmt.Printf("camera %dx%d, %d frames @ %.1f Hz, max yaw %.0f deg/s\n",
		rep.Width, rep.Height, rep.Frames, 1000/rep.StepMS, rep.MaxYawDegPerS)
	fmt.Printf("world latency %.1f ms, latest UI input age %.1f ms\n\n",
		rep.WorldLatencyMS, rep.UIInputMS)
	fmt.Printf("%-23s %13s %13s %13s %11s %11s %10s %12s\n",
		"mode", "depth-occl", "presence-err", "frame-diff", "drift-px", "cursor-px", "follow-ms", "skew-ms")
	for _, s := range rep.Summaries {
		fmt.Printf("%-23s %6.2f/%5.2f %6.2f/%5.2f %6.2f/%5.2f %5.1f/%4.1f %5.2f/%4.2f %4.1f/%4.1f %5.1f/%4.1f\n",
			s.Mode,
			100*s.Mean.DepthRejectedRate, 100*s.Max.DepthRejectedRate,
			100*s.Mean.UIPresenceError, 100*s.Max.UIPresenceError,
			100*s.Mean.ReferenceMismatch, 100*s.Max.ReferenceMismatch,
			s.Mean.FixedUIDriftPx, s.Max.FixedUIDriftPx,
			s.Mean.CursorErrorPx, s.Max.CursorErrorPx,
			s.Mean.FollowLatencyMs, s.Max.FollowLatencyMs,
			s.Mean.ScanoutSkewMs, s.Max.ScanoutSkewMs)
	}
	fmt.Printf("\n(values are mean/max; depth-occl, presence-err and frame-diff are percent)\n")
	fmt.Printf("world reprojection mean hole rate %.2f%%, max %.2f%%; near-field coverage %.1f%%\n",
		100*rep.Summaries[0].Mean.WorldHoleRate,
		100*rep.Summaries[0].Max.WorldHoleRate,
		100*rep.Summaries[0].Mean.WorldNearCoverage)
	fmt.Println("\nchecks:")
	keys := []string{
		"layered_no_world_depth_occlusion",
		"layered_fixed_ui_zero_drift",
		"layered_same_scanout",
		"layered_matches_reference",
		"naive_merge_gets_occluded",
		"inherited_camera_drifts",
		"inherited_camera_skews_scanout",
		"inherited_camera_adds_follow_latency",
	}
	for _, k := range keys {
		fmt.Printf("  %-42s %v\n", k, rep.Checks[k])
	}
}

func writePNGs(dir string, s reprojection.Scenario, at time.Duration) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, mode := range reprojection.AllModes() {
		r, err := s.Frame(at, mode)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, string(mode)+".png")
		if err := savePNG(path, r.Buffer); err != nil {
			return err
		}
	}
	return nil
}

func savePNG(path string, fb *reprojection.FrameBuffer) error {
	img := image.NewRGBA(image.Rect(0, 0, fb.Width, fb.Height))
	for y := 0; y < fb.Height; y++ {
		for x := 0; x < fb.Width; x++ {
			c := fb.Pixels[y*fb.Width+x]
			img.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, c.A})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
