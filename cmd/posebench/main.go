package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"posebench/internal/analyze"
	"posebench/internal/motion"
	"posebench/internal/scan"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "posebench:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		scenario  = flag.String("scenario", "all", "comma-separated profile names, or all")
		frames    = flag.Int("frames", 120, "number of frame phases sampled per motion period")
		segments  = flag.Int("segments", 8, "number of segments for segmented strategies")
		fps       = flag.Float64("fps", 60, "frame rate")
		readoutMS = flag.Float64("readout-ms", 0, "row readout duration in ms; 0 uses one frame period")
		width     = flag.Int("width", 640, "sensor width in pixels")
		height    = flag.Int("height", 480, "sensor height in pixels")
		direction = flag.String("direction", "top-to-bottom", "scan direction: top-to-bottom or bottom-to-top")
		unifiedAt = flag.Float64("unified-at", 0.5, "unified timestamp as a fraction of readout (0,1]")
		budgetDeg = flag.Float64("boundary-budget-deg", 0.5, "maximum boundary discontinuity in degrees")
		budgetPx  = flag.Float64("boundary-budget-px", 0.5, "maximum boundary discontinuity in pixels")
		asJSON    = flag.Bool("json", false, "emit JSON instead of a table")
		outPath   = flag.String("out", "", "write output to a file instead of stdout")
		list      = flag.Bool("list", false, "list available scenarios")
	)
	flag.Parse()

	catalog := motion.Catalog()
	if *list {
		for _, p := range catalog {
			fmt.Printf("%-12s %s\n", p.Name, p.Description)
		}
		return nil
	}
	profiles, err := selectProfiles(catalog, *scenario)
	if err != nil {
		return err
	}
	if *fps <= 0 {
		return fmt.Errorf("fps must be positive")
	}
	if *height < 2 || *width < 2 {
		return fmt.Errorf("width and height must be at least 2")
	}
	if *segments < 2 || *segments > *height {
		return fmt.Errorf("segments must be in [2, height]")
	}
	if *unifiedAt <= 0 || *unifiedAt > 1 {
		return fmt.Errorf("unified-at must be in (0, 1]; the frame-start pose is a diagnostic-only baseline")
	}

	frameSec := 1 / *fps
	readout := frameSec
	if *readoutMS > 0 {
		readout = *readoutMS / 1000
	}
	if readout <= 0 || readout > frameSec+1e-9 {
		return fmt.Errorf("readout (%.3f ms) must be in (0, %.3f ms]", readout*1000, frameSec*1000)
	}
	scanDir := scan.TopToBottom
	switch strings.ToLower(*direction) {
	case "top-to-bottom", "top", "down":
	case "bottom-to-top", "bottom", "up":
		scanDir = scan.BottomToTop
	default:
		return fmt.Errorf("unknown scan direction %q", *direction)
	}

	camera := scan.DefaultCamera()
	camera.Width = *width
	camera.Height = *height
	scale := float64(*height) / 480
	camera.FX *= scale
	camera.FY *= scale
	camera.CX = float64(*width) / 2
	camera.CY = float64(*height) / 2
	shutter := scan.Shutter{
		Camera:     camera,
		ReadoutSec: readout,
		FrameSec:   frameSec,
		Direction:  scanDir,
	}

	cfg := analyze.Config{
		Frames:        *frames,
		Segments:      *segments,
		UnifiedPhase:  *unifiedAt,
		JumpBudgetDeg: *budgetDeg,
		JumpBudgetPx:  *budgetPx,
	}
	report, err := analyze.Run(profiles, shutter, cfg)
	if err != nil {
		return err
	}

	var output []byte
	if *asJSON {
		output, err = json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		output = append(output, '\n')
	} else {
		output = []byte(formatTable(report, *direction))
	}
	if *outPath == "" {
		_, err = os.Stdout.Write(output)
		return err
	}
	return os.WriteFile(*outPath, output, 0o644)
}

func selectProfiles(catalog []motion.Profile, selector string) ([]motion.Profile, error) {
	if strings.TrimSpace(selector) == "" || strings.EqualFold(selector, "all") {
		return catalog, nil
	}
	byName := make(map[string]motion.Profile, len(catalog))
	for _, p := range catalog {
		byName[p.Name] = p
	}
	var out []motion.Profile
	for _, name := range strings.Split(selector, ",") {
		name = strings.TrimSpace(name)
		p, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown scenario %q (use -list)", name)
		}
		out = append(out, p)
	}
	return out, nil
}
