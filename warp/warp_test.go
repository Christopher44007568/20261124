package warp_test

import (
	"errors"
	"testing"

	"stereowarp/geom"
	"stereowarp/raster"
	"stereowarp/scene"
	"stereowarp/warp"
)

const (
	tw, th = 160, 120
)

func camAt(x, z float64) geom.Camera {
	return geom.Camera{Pos: geom.V(x, 0, z), Near: 0.1, Far: 30, FovY: 1.15, Width: tw, Height: th}
}

func render(cam geom.Camera, meshes []*raster.Mesh) *raster.Framebuffer {
	fb := raster.NewFramebuffer(tw, th, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
	raster.Renderer{}.Render(fb, cam, meshes, raster.FullScissor(tw, th), nil)
	return fb
}

func baseConfig(meshes []*raster.Mesh) warp.Config {
	left := camAt(-0.032, 0)
	fb := render(left, meshes)
	colors := make([]geom.Vec3, len(fb.Color))
	for i, c := range fb.Color {
		colors[i] = geom.V(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	}
	return warp.Config{
		Source: warp.EyeResource{
			Color: colors, Depth: append([]float64(nil), fb.Depth...),
			Camera: left, CameraEpoch: 5, DepthEpoch: 7,
		},
		Target: camAt(0.032, 0), CameraEpoch: 5, DepthEpoch: 7,
	}
}

func TestReprojectReusesAndRepaints(t *testing.T) {
	meshes := scene.Build()
	cfg := baseConfig(meshes)
	fb, stats, err := warp.Reproject(cfg, meshes)
	if err != nil {
		t.Fatalf("reproject failed: %v", err)
	}
	if fb == nil || stats == nil {
		t.Fatal("expected framebuffer and stats")
	}
	if stats.Reused == 0 {
		t.Fatal("nominal stereo offset must reuse source pixels")
	}
	if stats.Tiles == 0 {
		t.Fatal("disocclusions and glass require repaint tiles")
	}
	ref := render(cfg.Target, meshes)
	var mismatch int
	for i := range ref.Color {
		a, b := fb.Color[i], ref.Color[i]
		e := abs(int(a.R)-int(b.R)) + abs(int(a.G)-int(b.G)) + abs(int(a.B)-int(b.B))
		if e > 36 {
			mismatch++
		}
	}
	if mismatch > tw*th/50 {
		t.Fatalf("too many mismatching pixels vs full right eye: %d", mismatch)
	}
}

func TestResourceFailureGuards(t *testing.T) {
	meshes := scene.Build()

	staleCamera := baseConfig(meshes)
	staleCamera.CameraEpoch++
	if _, _, err := warp.Reproject(staleCamera, meshes); !errors.Is(err, warp.ErrStaleCamera) {
		t.Fatalf("stale camera: got %v want ErrStaleCamera", err)
	}

	staleDepth := baseConfig(meshes)
	staleDepth.DepthEpoch++
	if _, _, err := warp.Reproject(staleDepth, meshes); !errors.Is(err, warp.ErrStaleDepth) {
		t.Fatalf("stale depth: got %v want ErrStaleDepth", err)
	}

	nilRes := baseConfig(meshes)
	nilRes.Source.Color = nil
	if _, _, err := warp.Reproject(nilRes, meshes); !errors.Is(err, warp.ErrNilResource) {
		t.Fatalf("nil resource: got %v want ErrNilResource", err)
	}

	corrupt := baseConfig(meshes)
	corrupt.Source.Depth[10] = -1
	if _, _, err := warp.Reproject(corrupt, meshes); !errors.Is(err, warp.ErrDepthCorrupt) {
		t.Fatalf("corrupt depth: got %v want ErrDepthCorrupt", err)
	}
}

func TestExpiredDataNeverReachesOutput(t *testing.T) {
	meshes := scene.Build()
	cfg := baseConfig(meshes)
	cfg.CameraEpoch = 99
	fb, _, err := warp.Reproject(cfg, meshes)
	if err == nil {
		t.Fatal("expected rejection of an expired camera epoch")
	}
	if fb != nil {
		t.Fatal("no framebuffer may be produced from expired data")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
