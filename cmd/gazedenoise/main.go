package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gazedenoise"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gazedenoise:", err)
		os.Exit(1)
	}
}

func run() error {
	defaults := gazedenoise.DefaultExperimentOptions()
	out := flag.String("out", defaults.OutDir, "output directory")
	width := flag.Int("width", defaults.Width, "image width")
	height := flag.Int("height", defaults.Height, "image height")
	noise := flag.Float64("noise", defaults.Noise, "Gaussian colour-noise sigma")
	seed := flag.Int64("seed", defaults.Seed, "deterministic scene seed")
	sceneList := flag.String("scenes", "all", "comma-separated scenes or all")
	radius := flag.Int("radius", defaults.Params.Radius, "filter radius")
	holeRadius := flag.Int("hole-radius", defaults.Params.HoleRadius, "hole search radius")
	sigmaSpatial := flag.Float64("sigma-spatial", defaults.Params.SigmaSpatial, "spatial sigma")
	sigmaNormal := flag.Float64("sigma-normal", defaults.Params.SigmaNormal, "normal sigma")
	sigmaDepth := flag.Float64("sigma-depth", defaults.Params.SigmaDepth, "depth sigma")
	sigmaLuma := flag.Float64("sigma-luma", defaults.Params.SigmaLuma, "luminance sigma")
	flag.Parse()

	scenes, err := parseScenes(*sceneList)
	if err != nil {
		return err
	}
	opts := gazedenoise.ExperimentOptions{
		OutDir: *out,
		Width:  *width,
		Height: *height,
		Noise:  *noise,
		Seed:   *seed,
		Scenes: scenes,
		Params: gazedenoise.Params{
			Radius:       *radius,
			HoleRadius:   *holeRadius,
			SigmaSpatial: *sigmaSpatial,
			SigmaNormal:  *sigmaNormal,
			SigmaDepth:   *sigmaDepth,
			SigmaLuma:    *sigmaLuma,
			MinWeight:    defaults.Params.MinWeight,
		},
	}
	report, err := gazedenoise.RunExperiment(opts)
	if err != nil {
		return err
	}
	for _, c := range report.Cases {
		for _, mode := range c.Modes {
			m := mode.Metrics
			fmt.Printf("%-16s %-12s centre=%.5f boundary=%.5f halo=%.5f cross=%d provenance=%t\n",
				c.Kind, mode.Mode, m.CentreMAE, m.BoundaryRMSE, m.HaloBias,
				m.CrossPartitionTaps, m.ProvenanceValid)
		}
	}
	fmt.Printf("report: %s\n", *out)
	return nil
}

func parseScenes(value string) ([]gazedenoise.SceneKind, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "all" {
		return gazedenoise.SceneKinds(), nil
	}
	parts := strings.Split(value, ",")
	scenes := make([]gazedenoise.SceneKind, 0, len(parts))
	for _, part := range parts {
		kind, err := gazedenoise.ParseSceneKind(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, kind)
	}
	return scenes, nil
}
