package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"text/tabwriter"

	"stereoholes"
)

func main() {
	out := flag.String("out", "out", "output directory")
	width := flag.Int("width", 480, "image width")
	height := flag.Int("height", 320, "image height")
	baseline := flag.Float64("baseline", 0.36, "stereo baseline")
	flag.Parse()

	if *width < 96 || *height < 64 {
		fatal(fmt.Errorf("width and height must be at least 96x64"))
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}

	rig := stereoholes.NewStereoRig(*width, *height, *baseline, 46)
	scene := stereoholes.NewDemoScene()
	target := scene.Render(rig.Left)
	source := scene.Render(rig.Right)
	holes := stereoholes.DemoHoles(target)

	original := stereoholes.OriginalEye(target, holes.Mask)
	guarded := stereoholes.CrossEyeFill(target, source, holes.Mask,
		stereoholes.DefaultFillOptions())
	noCross := stereoholes.NoCrossEyeFill(target, holes.Mask)
	reference := stereoholes.ReferenceFill(target, holes.Mask)
	unguarded := stereoholes.UnguardedCrossEyeFill(target, source, holes.Mask)

	files := []struct {
		name   string
		title  string
		accent color.RGBA
		res    stereoholes.FillResult
	}{
		{"01-original.png", "A  ORIGINAL EYE", color.RGBA{95, 107, 128, 255}, original},
		{"02-cross-eye-guarded.png", "B  CROSS-EYE (GUARDED)", color.RGBA{47, 171, 130, 255}, guarded},
		{"03-no-cross-eye.png", "C  NO CROSS-EYE", color.RGBA{201, 148, 55, 255}, noCross},
		{"04-reference.png", "D  COMPLETE REFERENCE", color.RGBA{84, 137, 224, 255}, reference},
		{"05-naive-unguarded.png", "E  UNGUARDED (GHOSTS)", color.RGBA{208, 75, 92, 255}, unguarded},
	}
	panels := make([]stereoholes.Panel, 0, 4)
	for i, f := range files {
		img := stereoholes.RGBImage(*width, *height, f.res.RGB)
		if err := stereoholes.WritePNG(filepath.Join(*out, f.name), img); err != nil {
			fatal(err)
		}
		if i < 4 {
			panels = append(panels, stereoholes.Panel{
				Title: f.title, Image: img, Accent: f.accent,
			})
		}
	}
	sheet := stereoholes.ComparisonSheet(panels, 2)
	if err := stereoholes.WritePNG(filepath.Join(*out, "comparison.png"), sheet); err != nil {
		fatal(err)
	}
	guardMap := stereoholes.GuardImage(*width, *height, holes.Mask, guarded)
	if err := stereoholes.WritePNG(filepath.Join(*out, "06-guard-map.png"), guardMap); err != nil {
		fatal(err)
	}

	fills := []stereoholes.NamedFill{
		{Name: "original", Result: original},
		{Name: "cross-eye-guarded", Result: guarded},
		{Name: "no-cross-eye", Result: noCross},
		{Name: "reference", Result: reference},
		{Name: "unguarded-diagnostic", Result: unguarded},
	}
	meanDisp := stereoholes.MeanDisparity(target, source, holes.Mask)
	report := stereoholes.BuildReport(target, holes, *baseline, meanDisp, fills)
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "report.json"), append(raw, '\n'), 0o644); err != nil {
		fatal(err)
	}

	printSummary(report)
	fmt.Printf("\nwrote comparison and diagnostics to %s\n", *out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func printSummary(r stereoholes.Report) {
	fmt.Printf("stereo holes  %dx%d  baseline=%.3f  mean disparity=%.1f px\n",
		r.Width, r.Height, r.Baseline, r.MeanDisparity)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "MODE\tFILLED\tMEAN ERR\tGHOST RATE")
	for _, m := range r.Global {
		fmt.Fprintf(w, "%s\t%d/%d (%.0f%%)\t%.4f\t%.0f%%\n",
			m.Name, m.Filled, m.Holes, 100*m.FillRate, m.MeanError, 100*m.GhostRate)
	}
	fmt.Fprintln(w, "\nREGION\tMODE\tFILLED\tMEAN ERR\tGHOST RATE")
	for _, g := range r.Groups {
		for _, m := range g.Modes {
			fmt.Fprintf(w, "%s\t%s\t%d/%d\t%.4f\t%.0f%%\n",
				g.Name, m.Name, m.Filled, m.Holes, m.MeanError, 100*m.GhostRate)
		}
	}
	w.Flush()
}
