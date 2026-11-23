package imaging

import (
	"math"
	"testing"
)

func TestFrameAccessAndClone(t *testing.T) {
	f := NewFrame(3, 2)
	c := RGB{R: 0.2, G: 0.4, B: 0.6}
	f.Set(1, 1, c)
	if got := f.At(1, 1); got != c {
		t.Fatalf("At = %+v, want %+v", got, c)
	}
	clone := f.Clone()
	clone.Set(0, 0, RGB{R: 1})
	if f.At(0, 0) == clone.At(0, 0) {
		t.Fatal("Clone shares backing pixel storage")
	}
}

func TestNRGBARoundTrip(t *testing.T) {
	f := NewFrame(2, 2)
	f.Set(0, 0, RGB{R: 1, G: 0.5, B: 0})
	f.Set(1, 0, RGB{R: 0.25, G: 0.75, B: 1})
	f.Set(0, 1, RGB{R: -1, G: 2, B: 0.5})
	f.Set(1, 1, RGB{R: 0.1, G: 0.2, B: 0.3})

	got := FromImage(f.NRGBA())
	if got.W != 2 || got.H != 2 {
		t.Fatalf("round-trip size = %dx%d", got.W, got.H)
	}
	if math.Abs(got.At(0, 0).R-1) > 0.01 || math.Abs(got.At(0, 0).G-0.5) > 0.01 {
		t.Fatalf("round-trip color = %+v", got.At(0, 0))
	}
	if got.At(0, 1).R != 0 || got.At(0, 1).G != 1 {
		t.Fatalf("values were not clamped before conversion: %+v", got.At(0, 1))
	}
}

func TestMaskRegions(t *testing.T) {
	disk := CircleMask(5, 5, 2)
	if !disk.At(2, 2) || disk.At(0, 0) {
		t.Fatal("CircleMask membership is wrong")
	}
	ring := Ring(5, 5, 1, 2)
	if ring.At(2, 2) {
		t.Fatal("Ring included its center")
	}
	if got := Intersect(disk, ring).Count(); got == 0 || got > ring.Count() {
		t.Fatalf("Intersect count = %d", got)
	}
}
