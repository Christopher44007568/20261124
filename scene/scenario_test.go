package scene_test

import (
	"testing"

	"stereowarp/compare"
	"stereowarp/geom"
	"stereowarp/raster"
	"stereowarp/scene"
	"stereowarp/warp"
)

const (
	w, h = 320, 240
)

func eye(pos geom.Vec3, yaw, pitch float64) geom.Camera {
	return geom.Camera{Pos: pos, Yaw: yaw, Pitch: pitch,
		Near: 0.1, Far: 30, FovY: 1.15, Width: w, Height: h}
}

type spec struct {
	name        string
	left, right geom.Camera
	minPSNR     float64
}

func colorVecs(fb *raster.Framebuffer) []geom.Vec3 {
	out := make([]geom.Vec3, len(fb.Color))
	for i, c := range fb.Color {
		out[i] = geom.V(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	}
	return out
}

func TestScenarioQuality(t *testing.T) {
	meshes := scene.Build()
	specs := []spec{
		{"nominal",
			eye(geom.V(-0.032, 0, 0), 0, 0), eye(geom.V(0.032, 0, 0), 0, 0), 34},
		{"small-head-turn",
			eye(geom.V(-0.032, 0, 0), -0.045, 0.01),
			eye(geom.V(0.032, 0, 0), 0.045, -0.01), 33},
		{"large-forward-move",
			eye(geom.V(-0.032, 0, 1.1), 0, 0),
			eye(geom.V(0.032, 0, 1.9), 0, 0), 28},
		{"edge-glance",
			eye(geom.V(-0.032, 0, 0), 0.16, 0),
			eye(geom.V(0.032, 0, 0), 0.16, 0), 34},
	}
	rend := raster.Renderer{}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			leftFB := raster.NewFramebuffer(w, h, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
			refFB := raster.NewFramebuffer(w, h, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
			rend.Render(leftFB, s.left, meshes, raster.FullScissor(w, h), nil)
			rend.Render(refFB, s.right, meshes, raster.FullScissor(w, h), nil)

			outFB, stats, err := warp.Reproject(warp.Config{
				Source: warp.EyeResource{
					Color: colorVecs(leftFB), Depth: append([]float64(nil), leftFB.Depth...),
					Camera: s.left, CameraEpoch: 1, DepthEpoch: 1,
				},
				Target: s.right, CameraEpoch: 1, DepthEpoch: 1,
			}, meshes)
			if err != nil {
				t.Fatalf("reproject: %v", err)
			}
			if stats.Reused == 0 || stats.Tiles == 0 {
				t.Fatalf("expected reuse and repaint, got %+v", stats)
			}
			metrics := compare.Frames(outFB, refFB)
			if metrics.PSNR < s.minPSNR {
				t.Fatalf("%s PSNR %.2f below %.2f (mismatch=%d depthMAE=%.4f)",
					s.name, metrics.PSNR, s.minPSNR, metrics.Mismatch, metrics.DepthMAE)
			}
		})
	}
}
