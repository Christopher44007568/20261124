// Command fidelitycheck runs the dependency-free synthetic quality suite.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"fidelitycheck/internal/eval"
	"fidelitycheck/internal/inpaint"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg := eval.DefaultConfig()
	fs := flag.NewFlagSet("fidelitycheck", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	width := fs.Int("width", cfg.Width, "frame width")
	height := fs.Int("height", cfg.Height, "frame height")
	frames := fs.Int("frames", cfg.Frames, "frames per scenario (minimum 3)")
	radii := fs.String("radii", "30,45,60", "comma-separated mask radii as percent of half the shorter side")
	algorithms := fs.String("algorithms", "mean,nearest,blur,diffusion", "comma-separated reconstructors")
	baseline := fs.String("baseline", cfg.Baseline, "reference reconstructor for quality guardrails")
	jsonPath := fs.String("json", "", "optional path for the full JSON report")
	gate := fs.Bool("gate", true, "return a non-zero exit code when a quality guardrail fails")
	quiet := fs.Bool("quiet", false, "suppress the human-readable table")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintln(fs.Output(), "Measures center detail, peripheral completion, color-edge retention,")
		fmt.Fprintln(fs.Output(), "and frame-to-frame flicker across mask radii and reconstructors.")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	radiusValues, err := parseFloats(*radii)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fidelitycheck: invalid -radii: %v\n", err)
		return 2
	}
	algorithmsList, err := parseAlgorithms(*algorithms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fidelitycheck: %v\n", err)
		return 2
	}

	cfg.Width = *width
	cfg.Height = *height
	cfg.Frames = *frames
	cfg.RadiusPercents = radiusValues
	cfg.Algorithms = algorithmsList
	cfg.Baseline = strings.TrimSpace(*baseline)

	report, err := eval.Run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fidelitycheck: %v\n", err)
		return 2
	}
	if *jsonPath != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "fidelitycheck: encode report: %v\n", err)
			return 2
		}
		data = append(data, '\n')
		if err := os.WriteFile(*jsonPath, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "fidelitycheck: write report: %v\n", err)
			return 2
		}
	}
	if !*quiet {
		printReport(report)
	}
	if *gate && !report.Passed {
		return 1
	}
	return 0
}

func parseFloats(raw string) ([]float64, error) {
	parts := strings.Split(raw, ",")
	values := make([]float64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", part)
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("no radii provided")
	}
	return values, nil
}

func parseAlgorithms(raw string) ([]inpaint.Algorithm, error) {
	all := inpaint.Defaults()
	var selected []inpaint.Algorithm
	seen := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" || seen[name] {
			continue
		}
		alg, err := inpaint.Find(all, name)
		if err != nil {
			return nil, err
		}
		seen[name] = true
		selected = append(selected, alg)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no reconstructors selected")
	}
	return selected, nil
}

func printReport(report *eval.Report) {
	fmt.Printf(
		"fidelitycheck: %d cases, %d reconstructors, %d frames per sequence\n",
		report.Summary.Cases,
		len(report.Config.Algorithms),
		report.Summary.FramesPerCase,
	)
	fmt.Println("Spatial errors and flicker are lower-is-better; edge retention is higher-is-better.")
	fmt.Println()

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SCENE\tRADIUS\tALGO\tGLOBAL\tCENTER\tCEN-P95\tPERIPH\tEDGE-RET\tFLICK-P95\tVERDICT")
	for _, c := range report.Cases {
		for _, r := range c.Results {
			verdict := "PASS"
			if !r.Guard.Passed {
				verdict = "FAIL"
			}
			fmt.Fprintf(
				tw,
				"%s\t%.0f%% (%.1f px)\t%s\t%.5f\t%.5f\t%.5f\t%.5f\t%.3f\t%.5f\t%s\n",
				c.Scene,
				c.RadiusPercent,
				c.RadiusPixels,
				r.Algorithm,
				r.Metrics.GlobalMAE,
				r.Metrics.CenterMAE,
				r.Metrics.CenterP95,
				r.Metrics.PeripheryMAE,
				r.Metrics.EdgeRetention,
				r.Metrics.FlickerP95,
				verdict,
			)
		}
	}
	tw.Flush()

	if report.Summary.FailedRuns > 0 {
		fmt.Println()
		fmt.Println("Guardrail failures:")
		for _, c := range report.Cases {
			for _, r := range c.Results {
				for _, violation := range r.Guard.Violations {
					fmt.Printf(
						"- %s, radius %.0f%% (%.1f px), %s: %s: %s\n",
						c.Scene,
						c.RadiusPercent,
						c.RadiusPixels,
						r.Algorithm,
						violation.Code,
						violation.Message,
					)
				}
			}
		}
	}
	fmt.Println()
	if report.Passed {
		fmt.Println("RESULT: PASS")
	} else {
		fmt.Printf("RESULT: FAIL (%d of %d runs violated a guardrail)\n", report.Summary.FailedRuns, report.Summary.AlgorithmRuns)
	}
}
