package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"stereowarp/compare"
	"stereowarp/geom"
	"stereowarp/raster"
	"stereowarp/warp"
)

const (
	width   = 320
	height  = 240
	near    = 0.1
	far     = 30.0
	fovY    = 1.15
	iod     = 0.064 // inter-ocular distance, 6.4 cm
	baseCam = 0.0
	baseDep = 0.0
)

// Scenario defines how the right-eye camera differs from the left-eye one.
type Scenario struct {
	Name    string
	Left    geom.Camera
	Right   geom.Camera
	Comment string
}

func eye(pos geom.Vec3, yaw, pitch float64) geom.Camera {
	return geom.Camera{
		Pos: pos, Yaw: yaw, Pitch: pitch,
		Near: near, Far: far, FovY: fovY, Width: width, Height: height,
	}
}

func scenarios() []Scenario {
	center := geom.V(0, 0, 0)
	right := geom.V(iod/2, 0, 0)
	left := geom.V(-iod/2, 0, 0)
	return []Scenario{
		{
			Name:    "nominal",
			Left:    eye(center.Add(left), baseCam, 0),
			Right:   eye(center.Add(right), baseCam, 0),
			Comment: "pure horizontal inter-ocular offset",
		},
		{
			Name:    "small-head-turn",
			Left:    eye(center.Add(left), -0.045, 0.01),
			Right:   eye(center.Add(right), 0.045, -0.01),
			Comment: "small yaw/pitch head rotation between eyes",
		},
		{
			Name:    "large-forward-move",
			Left:    eye(geom.V(-iod/2, 0, 1.1), 0, 0),
			Right:   eye(geom.V(iod/2, 0, 1.9), 0, 0),
			Comment: "right eye moves 0.8 m forward, depth scale changes strongly",
		},
		{
			Name:    "edge-glance",
			Left:    eye(center.Add(left), 0.16, 0),
			Right:   eye(center.Add(right), 0.16, 0),
			Comment: "sideways glance pushing scene content toward frustum edges",
		},
	}
}

// RunResult bundles everything produced for one scenario comparison.
type RunResult struct {
	Scenario Scenario
	Metrics  compare.Metrics
	Warp     *warp.Result
	LeftFB   *raster.Framebuffer
	RefFB    *raster.Framebuffer
	WarpFB   *raster.Framebuffer
}

func runScenario(s Scenario, meshes []*raster.Mesh) (*RunResult, error) {
	rend := raster.Renderer{}
	leftFB := raster.NewFramebuffer(width, height, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
	refFB := raster.NewFramebuffer(width, height, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
	rend.Render(leftFB, s.Left, meshes, raster.FullScissor(width, height), nil)
	rend.Render(refFB, s.Right, meshes, raster.FullScissor(width, height), nil)

	cfg := warp.Config{
		Source: warp.EyeResource{
			Color:       toColorVecs(leftFB),
			Depth:       append([]float64(nil), leftFB.Depth...),
			Camera:      s.Left,
			CameraEpoch: 1,
			DepthEpoch:  1,
		},
		Target:      s.Right,
		CameraEpoch: 1,
		DepthEpoch:  1,
	}
	warpFB, warpStats, err := warp.Reproject(cfg, meshes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Name, err)
	}

	return &RunResult{
		Scenario: s,
		Metrics:  compare.Frames(warpFB, refFB),
		Warp:     warpStats,
		LeftFB:   leftFB,
		RefFB:    refFB,
		WarpFB:   warpFB,
	}, nil
}

func toColorVecs(fb *raster.Framebuffer) []geom.Vec3 {
	out := make([]geom.Vec3, len(fb.Color))
	for i, c := range fb.Color {
		out[i] = geom.V(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	}
	return out
}

func writePNG(dir, name string, img image.Image) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
