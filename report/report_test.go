package report

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"timewarp/geom"
	"timewarp/metrics"
	"timewarp/scene"
	"timewarp/warp"
)

func tinyFrame(w, h int) *scene.Frame {
	n := w * h
	f := &scene.Frame{
		W: w, H: h,
		Pix:        make([]color.RGBA, n),
		Depth:      make([]float64, n),
		DepthValid: make([]bool, n),
		Region:     make([]scene.Region, n),
	}
	for i := 0; i < n; i++ {
		f.Pix[i] = color.RGBA{R: uint8(i), G: 90, B: 140, A: 255}
		f.Depth[i] = 2
		f.DepthValid[i] = true
	}
	for y := 0; y < h/2; y++ {
		for x := 0; x < w/3; x++ {
			f.Region[y*w+x] = scene.RegionPole
		}
	}
	for y := h / 2; y < h; y++ {
		for x := w / 2; x < w/2+w/8; x++ {
			f.Region[y*w+x] = scene.RegionSpecular
		}
	}
	return f
}

func tinyResult(w, h int) *warp.Result {
	out := &warp.Result{
		W: w, H: h,
		Pix:        make([]color.RGBA, w*h),
		Depth:      make([]float64, w*h),
		DepthValid: make([]bool, w*h),
		Reject:     make([]warp.RejectReason, w*h),
	}
	for i := range out.Pix {
		out.Pix[i] = color.RGBA{R: uint8(i), G: 90, B: 140, A: 255}
		out.Depth[i] = 2
		out.DepthValid[i] = true
	}
	out.Stats = warp.Stats{Evaluated: w * h, Valid: w * h}
	return out
}

func TestBuildWritesReport(t *testing.T) {
	const w, h = 96, 54
	gt := tinyFrame(w, h)
	rot := tinyResult(w, h)
	dep := tinyResult(w, h)
	rot.Stats.Valid = w*h - 7
	rot.Stats.Rejected = 7
	for i := 0; i < 7; i++ {
		rot.Reject[i] = warp.RejectOutOfBounds
	}
	rs := metrics.Compute("pure rotation", rot, gt)
	ds := metrics.Compute("depth", dep, gt)
	data := Data{
		In:        geom.NewIntrinsics(w, h, 0.9),
		FOVYDeg:   52,
		Source:    gt,
		PoleWorld: geom.Vec3{X: 0, Y: 0, Z: 2},
		Cases: []Case{
			{ID: "a", Title: "A", Motion: "m", Note: "n", GT: gt, Rotation: rot, Depth: dep, RotStats: rs, DepthStat: ds},
			{ID: "b", Title: "B", Motion: "m", Note: "n", GT: gt, Rotation: rot, Depth: dep, RotStats: rs, DepthStat: ds},
		},
	}
	dir := t.TempDir()
	if err := Build(dir, data); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"report.html", "metrics.json", "source.png", "source_depth.png", "source_regions.png",
		"a_gt.png", "a_rotation.png", "a_depth.png", "a_rotation_mask.png",
		"a_depth_mask.png", "a_rotation_heat.png", "a_depth_heat.png",
		"a_pole_focus.png", "a_specular_focus.png",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	html, err := os.ReadFile(filepath.Join(dir, "report.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(html) < 1000 {
		t.Fatalf("report too short: %d", len(html))
	}
}

func TestLargestRegionBounds(t *testing.T) {
	f := &scene.Frame{W: 8, H: 4, Region: make([]scene.Region, 32)}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			f.Region[y*8+x] = scene.RegionPole
		}
	}
	for y := 0; y < 3; y++ {
		for x := 4; x < 8; x++ {
			f.Region[y*8+x] = scene.RegionPole
		}
	}
	got := LargestRegionBounds(f, scene.RegionPole)
	want := image.Rect(4, 0, 8, 3)
	if got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}
