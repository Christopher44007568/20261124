package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"aehist/ae"
)

func main() {
	outDir := flag.String("out", "artifacts", "directory for CSV and Markdown reports")
	windowFrames := flag.Int("window", 4, "temporal histogram window length")
	flag.Parse()

	meterCfg := ae.DefaultMeterConfig()
	meterCfg.WindowFrames = *windowFrames
	controlCfg := ae.DefaultControlConfig()
	modes := []ae.MeterMode{
		ae.ModeWholeFrame,
		ae.ModeZonal,
		ae.ModeTemporalWindow,
	}
	traces, err := ae.Compare(ae.Scenarios(), modes, meterCfg, controlCfg)
	if err != nil {
		log.Fatalf("simulation failed: %v", err)
	}
	if err := writeReports(*outDir, traces, meterCfg, controlCfg); err != nil {
		log.Fatalf("write reports: %v", err)
	}
	printSummary(traces)
}

func printSummary(traces []ae.Trace) {
	fmt.Printf("%-16s %-16s %8s %10s %12s %10s\n",
		"scenario", "meter", "finalEV", "convFrames", "overshootEV", "settled")
	for _, trace := range traces {
		perf := trace.Performance
		fmt.Printf("%-16s %-16s %8.4f %10d %12.5f %10t\n",
			trace.Scenario.Name,
			trace.Mode,
			perf.FinalEV,
			perf.ConvergenceFrames,
			perf.OvershootEV,
			perf.Settled,
		)
	}
	fmt.Println("\nAverage by meter:")
	fmt.Printf("%-16s %10s %12s %10s\n", "meter", "conv", "overshoot", "peak")
	for _, mode := range []ae.MeterMode{
		ae.ModeWholeFrame,
		ae.ModeZonal,
		ae.ModeTemporalWindow,
	} {
		conv, overshoot, peak, count := 0.0, 0.0, 0.0, 0
		for _, trace := range traces {
			if trace.Mode != mode || trace.Performance.ConvergenceFrames < 0 {
				continue
			}
			conv += float64(trace.Performance.ConvergenceFrames)
			overshoot += trace.Performance.OvershootEV
			peak += trace.Performance.PeakExcursionEV
			count++
		}
		if count == 0 {
			fmt.Printf("%-16s %10s %12s %10s\n", mode, "-", "-", "-")
			continue
		}
		n := float64(count)
		fmt.Printf("%-16s %10.2f %12.5f %10.5f\n", mode, conv/n, overshoot/n, peak/n)
	}
}

func openOutput(path string) (*os.File, error) {
	return os.Create(path)
}
