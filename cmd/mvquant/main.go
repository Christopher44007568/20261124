package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mvquant"
)

const frames = 30

func main() {
	var report string
	header := fmt.Sprintf("%-12s %-10s %12s %12s %14s %14s %14s",
		"field", "encoder", "maxErr(px)", "zeroRate", "ghostMax(px)", "ghostAvg(px)", "ghostPeak(px)")
	fmt.Println(header)
	report = header + "\n"

	for _, kind := range mvquant.AllKinds() {
		results, err := mvquant.RunExperiment(kind, frames)
		if err != nil {
			fmt.Fprintf(os.Stderr, "experiment failed for %s: %v\n", kind, err)
			os.Exit(1)
		}
		for _, r := range results {
			line := fmt.Sprintf("%-12s %-10s %12.4f %11.2f%% %14.4f %14.4f %14.4f",
				r.Kind, r.Encoder,
				r.Metrics.MaxError,
				r.Metrics.ZeroVectorRate*100,
				r.Metrics.Ghost.FinalMaxDrift,
				r.Metrics.Ghost.FinalAvgDrift,
				r.Metrics.Ghost.PeakDrift)
			fmt.Println(line)
			report += line + "\n"
		}
	}

	if err := demonstrateRejection(); err != nil {
		fmt.Fprintf(os.Stderr, "rejection check failed: %v\n", err)
		os.Exit(1)
	}
	note := "\nout-of-range components are rejected with mvquant.ErrOutOfRange (no silent clamping)\n"
	fmt.Print(note)
	report += note

	path := filepath.Join("artifacts", "quant_report.txt")
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nreport written to %s\n", path)
}

func demonstrateRejection() error {
	illegal := [][]mvquant.Vec2{{{X: mvquant.CodeMax + 0.5, Y: 0}}}
	encoders := []mvquant.Encoder{
		mvquant.TruncEncoder{},
		mvquant.RoundEncoder{},
		mvquant.NewDiffusionEncoder(1, 1),
	}
	for _, encoder := range encoders {
		if _, err := encoder.Encode(illegal); !errors.Is(err, mvquant.ErrOutOfRange) {
			return fmt.Errorf("%s accepted out-of-range vector", encoder.Name())
		}
	}
	return nil
}
