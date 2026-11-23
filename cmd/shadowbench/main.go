package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"shadowbench/shadow"
)

func main() {
	cfg := shadow.DefaultConfig()
	flag.IntVar(&cfg.Frames, "frames", cfg.Frames, "stress frames")
	flag.IntVar(&cfg.MapSize, "map", cfg.MapSize, "shadow map size")
	flag.IntVar(&cfg.TileSize, "tile", cfg.TileSize, "hybrid macro-cell size in texels")
	flag.IntVar(&cfg.Width, "width", cfg.Width, "eye render width")
	flag.IntVar(&cfg.Height, "height", cfg.Height, "eye render height")
	flag.Float64Var(&cfg.LightTurns, "light-turns", cfg.LightTurns, "light rotations during stress")
	asJSON := flag.Bool("json", false, "print JSON instead of a table")
	outPath := flag.String("out", "", "also write the JSON report to this path")
	flag.Parse()

	report := shadow.RunBenchmark(cfg)
	if *asJSON {
		if err := writeJSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, "encode report:", err)
			os.Exit(1)
		}
	} else {
		printTable(report)
	}
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create report:", err)
			os.Exit(1)
		}
		err = writeJSON(f, report)
		closeErr := f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "write report:", err)
			os.Exit(1)
		}
		if closeErr != nil {
			fmt.Fprintln(os.Stderr, "close report:", closeErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "report: %s\n", *outPath)
	}
	if !report.Invariants.SharedPreservesEyeBits ||
		!report.Invariants.FailureKeepsShadows ||
		!report.Invariants.HybridMatchesReference {
		os.Exit(1)
	}
}

func writeJSON(f *os.File, report shadow.Report) error {
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func printTable(r shadow.Report) {
	fmt.Printf("scene: %d casters, %d single-eye, %d cross-eye, IPD %.3f m\n",
		len(r.Scene.Casters), r.Scene.ExclusiveCount, r.Scene.CrossEyeCount,
		r.Scene.EyeSeparation)
	fmt.Printf("stress: %d frames, %dx%d map, %dpx tile, %dx%d eye image, %.2f light turns\n\n",
		r.Config.Frames, r.Config.MapSize, r.Config.MapSize, r.Config.TileSize,
		r.Config.Width, r.Config.Height, r.Config.LightTurns)

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "STRATEGY\tCASTERS avg/peak\tREUSE%\tPIXEL ERR%\tSHADOW KiB\tRESOURCE KiB\tFAILURES")
	for _, s := range r.Strategies {
		fail := "ok"
		for _, p := range s.Failures {
			if !p.Preserved {
				fail = "BROKEN"
			}
		}
		fmt.Fprintf(tw, "%s\t%.1f/%d\t%.1f\t%.4f\t%.1f\t%.1f\t%s\n",
			s.Name, s.AvgCasters, s.PeakCasters, s.AvgReusePct,
			s.PixelErrorPct, float64(s.PeakShadowBytes)/1024,
			float64(s.PeakResourceBytes)/1024, fail)
	}
	_ = tw.Flush()

	fmt.Printf("\ninvariants: shared-eye-bits=%v failure-preserves-shadows=%v hybrid-matches-reference=%v\n",
		r.Invariants.SharedPreservesEyeBits,
		r.Invariants.FailureKeepsShadows,
		r.Invariants.HybridMatchesReference)
	for _, s := range r.Strategies {
		for _, p := range s.Failures {
			fmt.Printf("  %-11s %s->%s: changed=%d split-texels=%d exclusive-casters=%d err-before/after=%.4f%%/%.4f%% %s\n",
				s.Name, p.DisabledEye, p.SurvivingEye, p.ChangedPixels,
				p.EyeSplitTexels, p.ExclusiveCasters,
				p.BeforeErrorPct, p.AfterErrorPct,
				strings.ToUpper(fmt.Sprint(p.Preserved)))
		}
	}
}
