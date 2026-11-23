package scan

import (
	"math"
	"testing"
)

func TestRowTimeEndpointsAndInverse(t *testing.T) {
	sh := DefaultShutter()
	start := 2.5
	h := sh.Camera.Height
	if got, want := sh.RowTime(start, 0), start; math.Abs(got-want) > 1e-12 {
		t.Fatalf("top row time = %g, want %g", got, want)
	}
	if got, want := sh.RowTime(start, h-1), start+sh.ReadoutSec; math.Abs(got-want) > 1e-12 {
		t.Fatalf("bottom row time = %g, want %g", got, want)
	}
	for _, row := range []int{0, 17, 239, h - 1} {
		got := sh.RowAtTime(start, sh.RowTime(start, row))
		if math.Abs(got-float64(row)) > 1e-9 {
			t.Fatalf("row inverse for row %d = %g", row, got)
		}
	}
}

func TestBottomToTopDirection(t *testing.T) {
	sh := DefaultShutter()
	sh.Direction = BottomToTop
	start := 0.0
	if got, want := sh.RowTime(start, 0), start+sh.ReadoutSec; math.Abs(got-want) > 1e-12 {
		t.Fatalf("first row time = %g, want %g", got, want)
	}
	if got, want := sh.RowTime(start, sh.Camera.Height-1), start; math.Abs(got-want) > 1e-12 {
		t.Fatalf("last row time = %g, want %g", got, want)
	}
}

func TestAffineFitIdentity(t *testing.T) {
	src := []Point2{{X: 1, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 7}}
	dst := append([]Point2(nil), src...)
	a, ok := FitAffine(src, dst)
	if !ok {
		t.Fatal("affine fit failed")
	}
	if math.Abs(a.A-1) > 1e-12 || math.Abs(a.D-1) > 1e-12 ||
		math.Abs(a.B) > 1e-12 || math.Abs(a.C) > 1e-12 {
		t.Fatalf("identity fit = %+v", a)
	}
}
