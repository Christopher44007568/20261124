// Command demo exercises the foveated temporal denoiser on four scenarios:
// static convergence, a fast gaze saccade, a dynamic specular highlight and a
// local disocclusion. It prints per-scenario statistics and writes PPM frames
// (P6, no external dependencies) into the given output directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fovealtemporal/foveal"
)

const width, height = 96, 72

func main() {
	outDir := "artifacts/frames"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	d, err := foveal.New(foveal.DefaultParams(width, height))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scene := foveal.NewScene(width, height)
	scene.Gaze = foveal.Vec2{X: width * 0.3, Y: height * 0.5}

	runStatic(scene, d, outDir)
	runSaccade(scene, d, outDir)
	runHighlight(scene, d, outDir)
	runDisocclusion(scene, d, outDir)

	fmt.Printf("\nPPM frames written to %s\n", outDir)
}

func runStatic(s *foveal.Scene, d *foveal.Denoiser, outDir string) {
	fmt.Println("== scenario 1: static convergence ==")
	var last *foveal.Result
	for i := 0; i < 40; i++ {
		f, _, _ := s.Render()
		last, _ = d.Process(f)
		s.Step()
	}
	writePPM(filepath.Join(outDir, "1_static.ppm"), last.RGB, width, height)
	printStats(last.Stats)
}

func runSaccade(s *foveal.Scene, d *foveal.Denoiser, outDir string) {
	fmt.Println("== scenario 2: fast head turn / saccade ==")
	s.Gaze = foveal.Vec2{X: width * 0.85, Y: height * 0.2}
	f, _, _ := s.Render()
	res, _ := d.Process(f)
	writePPM(filepath.Join(outDir, "2_saccade.ppm"), res.RGB, width, height)
	printStats(res.Stats)
	s.Step()
}

func runHighlight(s *foveal.Scene, d *foveal.Denoiser, outDir string) {
	fmt.Println("== scenario 3: dynamic specular highlight ==")
	s.Spot.Enabled = true
	s.Spot.X, s.Spot.Y = width*0.75, height*0.3
	f, _, _ := s.Render()
	res, _ := d.Process(f)
	writePPM(filepath.Join(outDir, "3_highlight.ppm"), res.RGB, width, height)
	printStats(res.Stats)
	s.Step()
}

func runDisocclusion(s *foveal.Scene, d *foveal.Denoiser, outDir string) {
	fmt.Println("== scenario 4: local occlusion reveal ==")
	s.Spot.Enabled = false
	s.Object.X = width * 0.2
	s.Object.Y = height * 0.4
	s.Object.PrevX = s.Object.X
	s.Object.PrevY = s.Object.Y
	s.Object.SpeedX = 3

	// Converge on this layout first.
	for i := 0; i < 20; i++ {
		s.Object.SpeedX = 0
		f, _, _ := s.Render()
		d.Process(f)
	}
	s.Object.SpeedX = 3
	s.Step()
	f, _, _ := s.Render()
	res, _ := d.Process(f)
	writePPM(filepath.Join(outDir, "4_disocclusion.ppm"), res.RGB, width, height)
	printStats(res.Stats)
}

func printStats(st foveal.Stats) {
	fmt.Printf("  resets: eye=%d motion=%d disocclusion=%d exposure=%d highlight=%d border=%d zone=%d\n",
		st.ResetEye, st.ResetMotion, st.ResetDisocclusion, st.ResetExposure,
		st.ResetHighlight, st.ResetBorder, st.ResetZone)
	if st.FovealPixels > 0 {
		fmt.Printf("  mean age: fovea=%.2f periphery=%.2f\n",
			st.MeanFovealAge/float64(st.FovealPixels),
			st.MeanPeriphAge/float64(max(1, st.PeripheralPixels)))
	}
}

func writePPM(path string, rgb []float32, w, h int) {
	var buf []byte
	buf = append(buf, []byte(fmt.Sprintf("P6\n%d %d\n255\n", w, h))...)
	for i := 0; i < w*h; i++ {
		for c := 0; c < 3; c++ {
			v := rgb[i*3+c]
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			buf = append(buf, byte(v*255+0.5))
		}
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
