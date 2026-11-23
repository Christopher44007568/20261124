package inpaint

import (
	"math"
	"testing"

	"fidelitycheck/internal/imaging"
)

func TestDefaultsFillMaskAndPreserveVisiblePixels(t *testing.T) {
	frame := imaging.NewFrame(24, 20)
	for y := 0; y < frame.H; y++ {
		for x := 0; x < frame.W; x++ {
			v := float64((x*7+y*3)%17) / 16
			frame.Set(x, y, imaging.RGB{R: v, G: 1 - v, B: v * 0.5})
		}
	}
	mask := imaging.CircleMask(frame.W, frame.H, 6)

	for _, alg := range Defaults() {
		out := alg.Reconstruct(frame, mask)
		if out.W != frame.W || out.H != frame.H {
			t.Fatalf("%s: wrong dimensions %dx%d", alg.Name, out.W, out.H)
		}
		for i, missing := range mask.Bits {
			if !missing {
				if out.Pix[i] != frame.Pix[i] {
					t.Fatalf("%s: changed visible pixel %d", alg.Name, i)
				}
				continue
			}
			c := out.Pix[i]
			if math.IsNaN(c.R) || math.IsNaN(c.G) || math.IsNaN(c.B) {
				t.Fatalf("%s: NaN in missing pixel %d", alg.Name, i)
			}
			if c.R < 0 || c.R > 1 || c.G < 0 || c.G > 1 || c.B < 0 || c.B > 1 {
				t.Fatalf("%s: out-of-range pixel %d: %+v", alg.Name, i, c)
			}
		}
	}
}

func TestNearestFillPropagatesSingleColor(t *testing.T) {
	frame := imaging.NewFrame(5, 5)
	frame.Fill(imaging.RGB{R: 0.9, G: 0.1, B: 0.2})
	mask := imaging.NewMask(5, 5)
	for i := range mask.Bits {
		mask.Bits[i] = true
	}
	mask.Set(2, 2, false)

	out := NearestFill(frame, mask)
	for i, c := range out.Pix {
		if c != frame.At(2, 2) {
			t.Fatalf("pixel %d = %+v, want source color", i, c)
		}
	}
}

func TestFindRejectsUnknownAlgorithm(t *testing.T) {
	if _, err := Find(Defaults(), "telea"); err == nil {
		t.Fatal("Find accepted an unknown algorithm")
	}
}
