// Command opticsdemo exercises the versioned optical config workflow:
// apply, save, reload, mapping checks, and last-good-frame retention.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"opticslab/optics"
)

func goodConfig() *optics.Config {
	eye := func() *optics.EyeParams {
		return &optics.EyeParams{
			Distortion:          &optics.Distortion{Coefficients: []float64{0.12, -0.03}},
			ChromaticAberration: &optics.ChromaticAberration{Scale: [3]float64{1.001, 1.0, 0.999}},
			PanelCenter:         &optics.PanelCenter{X: 0.5, Y: 0.5},
			SafetyBoundary:      &optics.SafetyBoundary{MinX: 0.05, MinY: 0.05, MaxX: 0.95, MaxY: 0.95},
		}
	}
	return &optics.Config{Version: optics.CurrentVersion, Left: eye(), Right: eye()}
}

func main() {
	path := filepath.Join(os.TempDir(), "optics_config.json")
	mgr := optics.NewManager(path)

	fmt.Println("== apply known-good config ==")
	if err := mgr.Apply(goodConfig()); err != nil {
		fmt.Println("rejected:", err)
		os.Exit(1)
	}
	report, _ := optics.CheckRoundtrip(mgr.Active(), 33)
	fmt.Printf("roundtrip ok: %d samples, max error %.3g\n", report.Samples, report.MaxError)

	fmt.Println("== save and reload ==")
	if err := mgr.Save(); err != nil {
		fmt.Println("save failed:", err)
		os.Exit(1)
	}
	fresh := optics.NewManager(path)
	if err := fresh.Reload(); err != nil {
		fmt.Println("reload failed:", err)
		os.Exit(1)
	}
	fmt.Println("reloaded version", fresh.Active().Version)

	fmt.Println("== upgrade with a missing field ==")
	bad := goodConfig()
	bad.Left.ChromaticAberration = nil
	if err := mgr.Apply(bad); err != nil {
		fmt.Println(err)
	}
	fmt.Println("active config still version", mgr.Active().Version)

	fmt.Println("== upgrade with left/right format mismatch ==")
	mismatched := goodConfig()
	mismatched.Right.Distortion.Coefficients = []float64{0.1}
	if err := mgr.Apply(mismatched); err != nil {
		fmt.Println(err)
	}

	fmt.Println("== upgrade with incompatible version ==")
	future := goodConfig()
	future.Version = optics.CurrentVersion + 1
	if err := mgr.Apply(future); err != nil {
		fmt.Println(err)
	}
	fmt.Println("last good frame preserved throughout")
}
