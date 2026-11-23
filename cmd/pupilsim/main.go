// Command pupilsim simulates virtual-image drift caused by pupil
// movement behind a lens and compares three compensation strategies.
// It writes CSV metrics and PNG charts to the artifacts directory.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"pupilsim/internal/optics"
	"pupilsim/internal/sim"
)

var palette = []color.RGBA{
	{0x1F, 0x77, 0xB4, 0xFF}, // left eye
	{0xFF, 0x7F, 0x0E, 0xFF}, // right eye
	{0x2C, 0xA0, 0x2C, 0xFF}, // disparity
	{0xD6, 0x27, 0x28, 0xFF}, // loss bands
}

func main() {
	outDir := flag.String("out", "artifacts", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	lens := optics.DefaultLens()
	comps := map[string]func() optics.Compensator{
		"fixed":    func() optics.Compensator { return optics.Fixed{} },
		"per-eye":  func() optics.Compensator { return optics.PerEye{} },
		"smoothed": func() optics.Compensator { return optics.NewSmoothed() },
	}
	order := []string{"fixed", "per-eye", "smoothed"}

	var summaries []sim.Summary
	for _, sc := range sim.Scenarios() {
		for _, name := range order {
			frames, sum := sim.Run(sc, lens, comps[name])
			summaries = append(summaries, sum)
			base := filepath.Join(*outDir, fmt.Sprintf("%s_%s", sc.Name, name))
			if err := writeCSV(base+".csv", frames); err != nil {
				fatal(err)
			}
			if err := writeChart(base+".png", sc, name, frames); err != nil {
				fatal(err)
			}
		}
	}
	if err := writeSummary(filepath.Join(*outDir, "summary.csv"), summaries); err != nil {
		fatal(err)
	}
	printTable(summaries)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pupilsim:", err)
	os.Exit(1)
}
