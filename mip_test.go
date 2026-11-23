package depthmip

import (
	"math"
	"strings"
	"testing"
)

func gradientBuffer(t *testing.T, w, h int) *Buffer {
	t.Helper()
	b, err := NewBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Deterministic non-monotonic pattern in (0,1).
			b.Data[y*w+x] = math.Mod(math.Sin(float64(y*w+x)*0.7)*0.5+0.5+0.001, 1)
		}
	}
	return b
}

func TestReduceOddDimensions(t *testing.T) {
	for _, wh := range [][2]int{{1, 1}, {5, 1}, {1, 5}, {5, 3}, {63, 37}, {2, 2}} {
		w, h := wh[0], wh[1]
		base := gradientBuffer(t, w, h)
		for _, dir := range []Direction{Min, Max} {
			chain, err := base.Chain(dir)
			if err != nil {
				t.Fatalf("%dx%d %s: %v", w, h, dir, err)
			}
			if err := VerifyChain(base, chain, dir, FullViewport(w, h)); err != nil {
				t.Errorf("%dx%d %s: %v", w, h, dir, err)
			}
		}
	}
}

func TestReversedZExtremes(t *testing.T) {
	base := gradientBuffer(t, 63, 37)
	minChain, err := base.Chain(Min)
	if err != nil {
		t.Fatal(err)
	}
	maxChain, err := base.Chain(Max)
	if err != nil {
		t.Fatal(err)
	}
	// Reversed-Z: the min chain's 1x1 root is the farthest texel (smallest
	// depth), the max chain's root the nearest (largest depth).
	wantMin, wantMax := math.Inf(1), math.Inf(-1)
	for _, v := range base.Data {
		wantMin = math.Min(wantMin, v)
		wantMax = math.Max(wantMax, v)
	}
	if got := minChain[len(minChain)-1].At(0, 0); got != wantMin {
		t.Errorf("min root = %g, want farthest %g", got, wantMin)
	}
	if got := maxChain[len(maxChain)-1].At(0, 0); got != wantMax {
		t.Errorf("max root = %g, want nearest %g", got, wantMax)
	}
}

func TestNaNDepthRejected(t *testing.T) {
	base := gradientBuffer(t, 4, 4)
	base.Data[5] = math.NaN()
	if _, err := base.Chain(Min); err == nil || !strings.Contains(err.Error(), "NaN") {
		t.Errorf("Chain(Min) with NaN = %v, want NaN error", err)
	}
	base.Data[5] = math.Inf(1)
	if _, err := base.Reduce(Max); err == nil || !strings.Contains(err.Error(), "infinite") {
		t.Errorf("Reduce(Max) with Inf = %v, want infinite-depth error", err)
	}
}

func TestEmptyLayerRejected(t *testing.T) {
	empty := &Buffer{W: 0, H: 0}
	if _, err := empty.Reduce(Min); err == nil {
		t.Error("Reduce on empty layer: want error, got nil")
	}
	if _, err := empty.Chain(Max); err == nil {
		t.Error("Chain on empty layer: want error, got nil")
	}
	if _, err := NewBuffer(0, 4); err == nil {
		t.Error("NewBuffer(0,4): want error, got nil")
	}
}

func TestInvalidDirectionRejected(t *testing.T) {
	if _, err := ParseDirection("minimum"); err == nil {
		t.Error("ParseDirection(\"minimum\"): want error, got nil")
	}
	if _, err := ParseDirection("min"); err != nil {
		t.Errorf("ParseDirection(\"min\"): %v", err)
	}
	base := gradientBuffer(t, 4, 4)
	if _, err := base.Reduce(Direction(7)); err == nil {
		t.Error("Reduce with invalid direction: want error, got nil")
	}
}
