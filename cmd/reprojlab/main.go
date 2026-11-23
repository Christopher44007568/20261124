package main

import (
	"flag"
	"fmt"
	"os"

	"reprojlab"
)

func main() {
	out := flag.String("out", "artifacts/orderlab", "directory for report.json, report.md and PNG artifacts")
	maxProbeError := flag.Float64("max-probe-depth-error", 0.05, "absolute depth error tolerated by probe checks")
	flag.Parse()

	report, err := reprojlab.WriteExperimentArtifacts(*out, reprojlab.DefaultConfig(), *maxProbeError)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(report.String())
	for _, o := range report.Orders {
		fmt.Printf("%s selected=%v holes=%d (%.3f%%) reads=%d writes=%d colorRMSE=%.5f depthRMSE=%.5f\n",
			o.Order, o.Selected, o.Stats.Holes, o.Stats.HolePercent, o.Stats.SourceSamples,
			o.Stats.Writes, o.Reference.ColorRMSE, o.Reference.DepthRMSE)
	}
	fmt.Printf("wrote %s\n", *out)
}
