package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aehist/ae"
)

func writeReports(
	dir string,
	traces []ae.Trace,
	meterCfg ae.MeterConfig,
	controlCfg ae.ControlConfig,
) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeResponseCSV(filepath.Join(dir, "exposure_response.csv"), traces); err != nil {
		return err
	}
	if err := writeSummaryCSV(filepath.Join(dir, "summary.csv"), traces); err != nil {
		return err
	}
	return writeMarkdown(filepath.Join(dir, "report.md"), traces, meterCfg, controlCfg)
}

func writeResponseCSV(path string, traces []ae.Trace) error {
	file, err := openOutput(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"scenario", "meter", "frame", "scene_cut", "ev", "control",
		"mean", "high_fraction", "generation", "window_frames", "stale_zone_reads",
	}); err != nil {
		return err
	}
	for _, trace := range traces {
		for _, sample := range trace.Samples {
			if err := writer.Write([]string{
				trace.Scenario.Name,
				string(trace.Mode),
				strconv.Itoa(sample.Frame),
				strconv.FormatBool(sample.SceneCut),
				formatFloat(sample.EV),
				formatFloat(sample.Control),
				formatFloat(sample.Mean),
				formatFloat(sample.HighFraction),
				strconv.FormatUint(sample.Generation, 10),
				strconv.Itoa(sample.WindowFrames),
				strconv.FormatUint(sample.StaleZoneReads, 10),
			}); err != nil {
				return err
			}
		}
	}
	return writer.Error()
}

func writeSummaryCSV(path string, traces []ae.Trace) error {
	file, err := openOutput(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"scenario", "meter", "start_ev", "final_ev", "convergence_frames",
		"overshoot_ev", "overshoot_percent", "peak_excursion_ev",
		"settled", "stale_zone_reads",
	}); err != nil {
		return err
	}
	for _, trace := range traces {
		perf := trace.Performance
		stale := uint64(0)
		if len(trace.Samples) > 0 {
			stale = trace.Samples[len(trace.Samples)-1].StaleZoneReads
		}
		if err := writer.Write([]string{
			trace.Scenario.Name,
			string(trace.Mode),
			formatFloat(perf.StartEV),
			formatFloat(perf.FinalEV),
			strconv.Itoa(perf.ConvergenceFrames),
			formatFloat(perf.OvershootEV),
			formatFloat(perf.OvershootPercent),
			formatFloat(perf.PeakExcursionEV),
			strconv.FormatBool(perf.Settled),
			strconv.FormatUint(stale, 10),
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeMarkdown(
	path string,
	traces []ae.Trace,
	meterCfg ae.MeterConfig,
	controlCfg ae.ControlConfig,
) error {
	var report strings.Builder
	report.WriteString("# Automatic exposure histogram comparison\n\n")
	report.WriteString("Meter configuration: ")
	report.WriteString(fmt.Sprintf("%dx%d zones, %d-frame window, high threshold %.2f. ",
		meterCfg.ZonesX, meterCfg.ZonesY, meterCfg.WindowFrames, meterCfg.HighThreshold))
	report.WriteString(fmt.Sprintf("Controller: target %.3f, gain %.2f, max step %.1f EV.\n\n",
		controlCfg.Target, controlCfg.Gain, controlCfg.MaxStepEV))
	report.WriteString("Convergence counts the transition frame; a value of `-1` means ")
	report.WriteString("the trace did not settle. Overshoot is measured beyond the final EV.\n\n")
	report.WriteString("| Scenario | Meter | Final EV | Converged frames | Overshoot EV | ")
	report.WriteString("Overshoot % | Peak excursion EV | Stale zone reads |\n")
	report.WriteString("|---|---|---:|---:|---:|---:|---:|---:|\n")
	for _, trace := range traces {
		perf := trace.Performance
		stale := uint64(0)
		if len(trace.Samples) > 0 {
			stale = trace.Samples[len(trace.Samples)-1].StaleZoneReads
		}
		report.WriteString(fmt.Sprintf(
			"| %s | %s | %.4f | %d | %.5f | %.2f | %.5f | %d |\n",
			trace.Scenario.Name,
			trace.Mode,
			perf.FinalEV,
			perf.ConvergenceFrames,
			perf.OvershootEV,
			perf.OvershootPercent,
			perf.PeakExcursionEV,
			stale,
		))
	}
	return os.WriteFile(path, []byte(report.String()), 0o644)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}
