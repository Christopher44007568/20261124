package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Christopher44007568/20261124/transparency"
)

func main() {
	width := flag.Int("width", 96, "render width in pixels")
	height := flag.Int("height", 64, "render height in pixels")
	frames := flag.Int("frames", 24, "number of temporal frames")
	spp := flag.Int("spp", 6, "reference samples per axis")
	scale := flag.Int("scale", 4, "PNG integer scale")
	outDir := flag.String("out", filepath.Join("artifacts", "temporal-transparency"), "output directory")
	flag.Parse()

	config := transparency.Config{
		Width:            *width,
		Height:           *height,
		Frames:           *frames,
		ReferenceSamples: *spp,
	}
	if config.Width < 16 || config.Height < 16 || config.Frames < 1 || config.ReferenceSamples < 1 {
		fmt.Fprintln(os.Stderr, "width/height must be >= 16 and frames/spp must be >= 1")
		os.Exit(2)
	}

	scene := transparency.NewDemoScene(config.Width, config.Height)
	evaluation := transparency.Evaluate(scene, config)
	transparency.WriteTextReport(os.Stdout, evaluation)

	last := evaluation.Runs[len(evaluation.Runs)-1]
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := transparency.WriteComparisonImages(*outDir, last, evaluation.Config, *scale); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := transparency.WriteJSON(filepath.Join(*outDir, "report.json"), evaluation); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("\nlast frame: %d\nwrote %s\n", last.Frame, *outDir)
}
