package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"curvelayers/internal/geom"
	"curvelayers/internal/render"
	"curvelayers/internal/tex"
)

const (
	width    = 480
	height   = 360
	focal    = 520.0
	baseline = 0.18
)

func main() {
	outDir := "artifacts"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	layers, _ := render.DefaultScene()
	marker := tex.NewMarker()
	baseCam := geom.Camera{Focal: focal, WidthPx: width, HeightPx: height}
	leftCam := baseCam.EyeX(-baseline / 2)
	rightCam := baseCam.EyeX(+baseline / 2)

	// Per-layer single references, rendered alone for each eye.
	names := []string{"plane", "cylinder", "sphere"}
	for i, name := range names {
		solo := []render.Layer{layers[i]}
		l := render.RenderEye(leftCam, solo, marker)
		r := render.RenderEye(rightCam, solo, marker)
		writePNG(filepath.Join(outDir, "ref_"+name+".png"), render.StereoCanvas(l, r))
	}

	// Full scene with correct nearest-surface ordering.
	fullL := render.RenderEye(leftCam, layers, marker)
	fullR := render.RenderEye(rightCam, layers, marker)
	writePNG(filepath.Join(outDir, "overlay_sorted.png"), render.StereoCanvas(fullL, fullR))

	// Ordering-error diagnostic against fixed painter order.
	writePNG(filepath.Join(outDir, "ordering_error.png"), render.OrderingErrorCanvas(fullL, fullR))

	// Binocular disparity false-color map.
	disp, maxDisp := render.Disparity(fullL, focal, baseline)
	writePNG(filepath.Join(outDir, "disparity.png"), render.DisparityCanvas(disp, width, height, maxDisp))

	// Wrong-approach reference: planar four-corner bilinear warp on the
	// cylinder, showing duplicated seam markers and flattened curvature.
	quad := render.QuadWarpEye(leftCam, layers[1], marker)
	writePNG(filepath.Join(outDir, "quadwarp_cylinder_wrong.png"), quad)

	fmt.Println("wrote artifacts: ref_{plane,cylinder,sphere}.png, overlay_sorted.png,")
	fmt.Println("ordering_error.png, disparity.png, quadwarp_cylinder_wrong.png")
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
