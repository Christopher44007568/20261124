package main

import (
	"flag"
	"fmt"
	"log"

	"ipdscene/internal/stereo"
)

func main() {
	outDir := flag.String("out", "artifacts", "output directory")
	flag.Parse()

	report, err := stereo.Run(*outDir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d synchronized stereo runs and 1 viewport-only failure run to %s\n",
		len(report.Runs), *outDir)
	for _, run := range report.Runs {
		fmt.Printf(
			"IPD %4.0f mm  valid %5.1f%%  median err %7.2f mm  p95 %7.2f mm  back-delta %4d px  OOB %d\n",
			run.IPDMM,
			100*run.Match.ValidRatio,
			run.Match.MedianAbsDepthErrorMM,
			run.Match.P95AbsDepthErrorMM,
			run.Occlusion.BackVisibleDelta,
			run.OutOfBoundsPixels,
		)
	}
	fmt.Printf("broken    valid %5.1f%%  median err %7.2f mm  p95 %7.2f mm  back-delta %4d px  OOB %d\n",
		100*report.Broken.Match.ValidRatio,
		report.Broken.Match.MedianAbsDepthErrorMM,
		report.Broken.Match.P95AbsDepthErrorMM,
		report.Broken.Occlusion.BackVisibleDelta,
		report.Broken.OutOfBoundsPixels,
	)
	fmt.Println("failure checks:")
	for _, check := range report.FailureChecks {
		fmt.Printf(
			"  %-22s correct=%-5t broken-detected=%-5t\n",
			check.Name,
			check.CorrectPass,
			check.BrokenDetected,
		)
	}
}
