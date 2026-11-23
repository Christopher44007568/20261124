// Command stereoval runs the dependency-free stereo re-projection validator.
// It renders a synthetic stereo sequence along a continuous camera path and
// evaluates four re-projection policies independently, recording per-eye
// error, disparity behavior, ghosting and edge misalignment.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"stereoval/internal/harness"
	"stereoval/internal/render"
	"stereoval/internal/scene"
)

type runOutput struct {
	Mode     string                   `json:"mode"`
	Pass     bool                     `json:"pass"`
	Failures []harness.Failure        `json:"failures"`
	Agg      map[string]interface{}   `json:"aggregate"`
	Frames   []map[string]interface{} `json:"frames"`
}

func main() {
	outDir := flag.String("out", "artifacts/reports", "directory for JSON reports")
	dump := flag.Bool("dump", false, "write PGM images for the first frame of each mode")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	sc := scene.New()
	modes := []harness.Mode{
		harness.ModeHealthy,
		harness.ModeRejectHist,
		harness.ModeCrossFill,
		harness.ModePoseSkew,
	}

	allPass := true
	for _, mode := range modes {
		cfg := harness.DefaultConfig()
		cfg.Mode = mode
		frames, k := harness.Run(cfg, sc)
		reports := harness.Evaluate(frames, k, cfg.Rig, sc)
		agg := harness.AggregateReports(reports)
		verdict := harness.Judge(mode, agg, harness.DefaultThresholds())
		allPass = allPass && verdict.Pass

		out := runOutput{
			Mode: string(mode), Pass: verdict.Pass, Failures: verdict.Failures,
			Agg: encodeAgg(agg), Frames: encodeFrames(reports),
		}
		path := filepath.Join(*outDir, string(mode)+".json")
		if err := writeJSON(path, out); err != nil {
			fatal(err)
		}
		printSummary(verdict)

		if *dump {
			if err := dumpFirst(harness.Export(frames), *outDir, mode); err != nil {
				fatal(err)
			}
		}
	}
	if !allPass {
		fmt.Println("\nOVERALL: FAIL (at least one rejected policy)")
		os.Exit(1)
	}
	fmt.Println("\nOVERALL: PASS")
}

func dumpFirst(pairs []harness.FramePair, dir string, mode harness.Mode) error {
	if len(pairs) == 0 {
		return nil
	}
	p := pairs[0]
	files := map[string]*render.Frame{
		string(mode) + "_left_gt.pgm":    p.LeftGT,
		string(mode) + "_right_gt.pgm":   p.RightGT,
		string(mode) + "_left_warp.pgm":  &p.LeftWarped.Frame,
		string(mode) + "_right_warp.pgm": &p.RightWarped.Frame,
	}
	for name, f := range files {
		if err := render.DumpPGM(f, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "stereoval:", err)
	os.Exit(2)
}
