package main

import (
	"errors"
	"fmt"
	"os"

	"stereowarp/compare"
	"stereowarp/raster"
	"stereowarp/scene"
	"stereowarp/warp"
)

func main() {
	outDir := "artifacts/renders"
	meshes := scene.Build()

	fmt.Println("== stereo reuse + repaint scenarios ==")
	var failed int
	for _, s := range scenarios() {
		res, err := runScenario(s, meshes)
		if err != nil {
			fmt.Printf("[FAIL] %-18s %v\n", s.Name, err)
			failed++
			continue
		}
		printReport(res)
		if err := saveImages(outDir, s.Name, res); err != nil {
			fmt.Printf("[FAIL] write %s: %v\n", s.Name, err)
			failed++
			continue
		}
	}

	fmt.Println("\n== single-eye resource failure guards ==")
	for _, c := range failureCases(meshes) {
		_, _, err := warp.Reproject(c.cfg, meshes)
		ok := errors.Is(err, c.want)
		status := "ok"
		if !ok {
			status = "FAIL"
			failed++
		}
		fmt.Printf("[%s] %-18s err=%v\n", status, c.name, err)
	}

	fmt.Printf("\nimages written to %s\n", outDir)
	if failed > 0 {
		fmt.Printf("%d check(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Println("all checks passed")
}

func printReport(r *RunResult) {
	m := r.Metrics
	fmt.Printf("[run ] %-18s %s\n", r.Scenario.Name, r.Scenario.Comment)
	fmt.Printf("       reused=%d holes=%d dilated=%d repaint=%d tiles=%d glass=%d outside=%d rejected=%d\n",
		r.Warp.Reused, r.Warp.Holes, r.Warp.Dilated, r.Warp.Repaint,
		r.Warp.Tiles, r.Warp.Glass, r.Warp.Outside, r.Warp.Rejected)
	fmt.Printf("       MAE=%.3f maxErr=%d mismatch=%d MSE=%.3f PSNR=%.2f dB depthMAE=%.4f depthMax=%.4f\n",
		m.MeanAbsErr, m.MaxAbsErr, m.Mismatch, m.MSE, m.PSNR, m.DepthMAE, m.DepthMax)
}

func saveImages(dir, name string, r *RunResult) error {
	if err := writePNG(dir, name+"_1_left.png", r.LeftFB.Image()); err != nil {
		return err
	}
	if err := writePNG(dir, name+"_2_reuse_repaint.png", r.WarpFB.Image()); err != nil {
		return err
	}
	if err := writePNG(dir, name+"_3_right_reference.png", r.RefFB.Image()); err != nil {
		return err
	}
	return writePNG(dir, name+"_4_diff.png", compare.Heatmap(r.WarpFB, r.RefFB))
}

type failureCase struct {
	name string
	cfg  warp.Config
	want error
}

func failureCases(meshes []*raster.Mesh) []failureCase {
	s := scenarios()[0]
	base := func() warp.Config {
		fb := raster.NewFramebuffer(width, height, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
		raster.Renderer{}.Render(fb, s.Left, meshes, raster.FullScissor(width, height), nil)
		return warp.Config{
			Source: warp.EyeResource{
				Color:       toColorVecs(fb),
				Depth:       append([]float64(nil), fb.Depth...),
				Camera:      s.Left,
				CameraEpoch: 7,
				DepthEpoch:  9,
			},
			Target:      s.Right,
			CameraEpoch: 7,
			DepthEpoch:  9,
		}
	}

	staleCamera := base()
	staleCamera.CameraEpoch = 8 // frame moved on; source camera is expired

	staleDepth := base()
	staleDepth.DepthEpoch = 10 // depth buffer was regenerated elsewhere

	nilResource := base()
	nilResource.Source.Color = nil

	corruptDepth := base()
	corruptDepth.Source.Depth = append([]float64(nil), corruptDepth.Source.Depth...)
	corruptDepth.Source.Depth[100] = -3

	mismatched := base()
	mismatched.Target = s.Right
	mismatched.Target.Width = width / 2

	return []failureCase{
		{"stale-camera", staleCamera, warp.ErrStaleCamera},
		{"stale-depth", staleDepth, warp.ErrStaleDepth},
		{"nil-resource", nilResource, warp.ErrNilResource},
		{"corrupt-depth", corruptDepth, warp.ErrDepthCorrupt},
		{"size-mismatch", mismatched, warp.ErrBufferSizeMismatch},
	}
}
