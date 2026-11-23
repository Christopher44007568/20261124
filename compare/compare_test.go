package compare

import (
	"math"
	"testing"

	"stereowarp/raster"
)

func TestIdenticalFrames(t *testing.T) {
	a := raster.NewFramebuffer(8, 8, raster.RGBA{R: 10, G: 20, B: 30, A: 255})
	b := raster.NewFramebuffer(8, 8, raster.RGBA{R: 10, G: 20, B: 30, A: 255})
	m := Frames(a, b)
	if m.MeanAbsErr != 0 || m.Mismatch != 0 || !math.IsInf(m.PSNR, 1) {
		t.Fatalf("identical frames must score zero error, got %+v", m)
	}
}

func TestKnownDifference(t *testing.T) {
	a := raster.NewFramebuffer(2, 1, raster.RGBA{R: 0, G: 0, B: 0, A: 255})
	b := raster.NewFramebuffer(2, 1, raster.RGBA{R: 0, G: 0, B: 0, A: 255})
	b.Color[0] = raster.RGBA{R: 30, G: 30, B: 30, A: 255}
	m := Frames(a, b)
	if m.MeanAbsErr != 15 {
		t.Fatalf("MAE = %v, want 15", m.MeanAbsErr)
	}
	if m.Mismatch != 1 {
		t.Fatalf("one pixel exceeds the mismatch threshold, got %d", m.Mismatch)
	}
	if img := Heatmap(a, b); img.Bounds().Dx() != 2 {
		t.Fatal("heatmap dimensions must match input")
	}
}
