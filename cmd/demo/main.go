// Command demo runs the 2:2 checkerboard shading/Reconstruction benchmark and
// optionally writes the ground-truth, shaded, reconstructed and mask images as
// PNG files for visual inspection. It depends only on the Go standard library.
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"checkerrb/checker"
)

func main() {
	outDir := flag.String("out", "", "directory for PNG outputs (optional)")
	flag.Parse()

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			fatal("create output dir: %v", err)
		}
	}

	fmt.Printf("%-22s %-9s %8s %8s %8s %8s %8s %8s %8s\n",
		"scene", "posed", "lumaRMSE", "lumaMAE", "hueMAE", "covMAE", "dice", "periodAC", "phaseΔ")
	for _, sc := range checker.Scenes() {
		m, rec, truth, mask := checker.RunScene(sc)
		posed := "yes"
		if !sc.Wellposed {
			posed = "alias"
		}
		fmt.Printf("%-22s %-9s %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f\n",
			sc.Name, posed, m.LumaRMSE, m.LumaMAE, m.HueMAE,
			m.CoverageMAE, m.CoverageDice, m.PeriodAC, m.PhaseDelta)

		if *outDir == "" {
			continue
		}
		writePNG(filepath.Join(*outDir, sc.Name+"_truth.png"), truth.ToNRGBA())
		writePNG(filepath.Join(*outDir, sc.Name+"_recon.png"), rec.ToNRGBA())
		writePNG(filepath.Join(*outDir, sc.Name+"_shaded.png"), checker.Shade(truth, mask).ToNRGBA())
		writePNG(filepath.Join(*outDir, sc.Name+"_mask.png"), mask.MaskImage())
	}

	if *outDir != "" {
		fmt.Printf("\nPNG frames written to %s\n", *outDir)
	}
}

func writePNG(path string, img *image.NRGBA) {
	f, err := os.Create(path)
	if err != nil {
		fatal("create %s: %v", path, err)
	}
	defer f.Close()
	if err := checker.WritePNG(f, img); err != nil {
		fatal("encode %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "demo: "+format+"\n", args...)
	os.Exit(1)
}
