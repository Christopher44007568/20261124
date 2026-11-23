package lensdistortion_test

import (
	"errors"
	"math"
	"testing"

	lens "lensdistortion"
)

func sampleLens(center lens.Point) lens.Lens {
	return lens.Lens{
		Center:          center,
		ReferenceRadius: 50,
		Coefficients:    []float64{-0.08, 0.012},
		MaxRadius:       math.Sqrt2 * 1.2,
	}
}

func keyPoints(center lens.Point) map[string]lens.Point {
	return map[string]lens.Point{
		"center":       center,
		"left edge":    {X: center.X - 50, Y: center.Y},
		"right edge":   {X: center.X + 50, Y: center.Y},
		"top edge":     {X: center.X, Y: center.Y - 50},
		"bottom edge":  {X: center.X, Y: center.Y + 50},
		"top-left":     {X: center.X - 50, Y: center.Y - 50},
		"top-right":    {X: center.X + 50, Y: center.Y - 50},
		"bottom-left":  {X: center.X - 50, Y: center.Y + 50},
		"bottom-right": {X: center.X + 50, Y: center.Y + 50},
	}
}

func TestAnalyticMappingCenterEdgesCornersAndRoundTrip(t *testing.T) {
	model := sampleLens(lens.Point{X: 50, Y: 50})
	mapper, err := lens.NewAnalyticMap(model)
	if err != nil {
		t.Fatal(err)
	}
	for name, display := range keyPoints(model.Center) {
		source, ok := mapper.SourceFromDisplay(display)
		if !ok {
			t.Fatalf("%s: source lookup failed", name)
		}
		back, ok := mapper.DisplayFromSource(source)
		if !ok {
			t.Fatalf("%s: inverse lookup failed", name)
		}
		if got := distance(back, display); got > 1e-8 {
			t.Fatalf("%s: round trip error %.12g", name, got)
		}
		if name == "center" {
			if source != model.Center || back != model.Center {
				t.Fatalf("center must remain fixed, got %+v / %+v", source, back)
			}
		}
		if display != model.Center {
			displayRadius := distance(display, model.Center)
			sourceRadius := distance(source, model.Center)
			if sourceRadius >= displayRadius {
				t.Fatalf("%s: expected barrel compression, display=%g source=%g", name, displayRadius, sourceRadius)
			}
		}
	}
}

func TestAnalyticMappingRejectsDomainAndBadCalibration(t *testing.T) {
	model := sampleLens(lens.Point{X: 50, Y: 50})
	mapper, err := lens.NewAnalyticMap(model)
	if err != nil {
		t.Fatal(err)
	}
	outside := lens.Point{X: model.Center.X + 2.2*model.ReferenceRadius, Y: model.Center.Y}
	if _, ok := mapper.SourceFromDisplay(outside); ok {
		t.Fatal("display point beyond MaxRadius was accepted")
	}
	if _, ok := mapper.DisplayFromSource(outside); ok {
		t.Fatal("source point beyond modeled radius was accepted")
	}

	nonMonotonic := model
	nonMonotonic.Coefficients = []float64{-2}
	nonMonotonic.MaxRadius = 1
	if _, err := lens.NewAnalyticMap(nonMonotonic); !errors.Is(err, lens.ErrNonMonotonic) {
		t.Fatalf("expected ErrNonMonotonic, got %v", err)
	}

	bad := model
	bad.ReferenceRadius = 0
	if _, err := lens.NewAnalyticMap(bad); !errors.Is(err, lens.ErrInvalidLens) {
		t.Fatalf("expected ErrInvalidLens, got %v", err)
	}
}

func TestLensInverseSolvesKnownRadialValues(t *testing.T) {
	model := sampleLens(lens.Point{})
	for _, r := range []float64{0.01, 0.5, 1, math.Sqrt2} {
		display := lens.Point{X: r * model.ReferenceRadius}
		source, err := model.DistortedFromUndistorted(display)
		if err != nil {
			t.Fatalf("r=%g forward: %v", r, err)
		}
		recovered, err := model.UndistortedFromDistorted(source)
		if err != nil {
			t.Fatalf("r=%g inverse: %v", r, err)
		}
		if math.Abs(recovered.X-display.X) > 1e-9 || math.Abs(recovered.Y) > 1e-9 {
			t.Fatalf("r=%g recovered %+v, want %+v", r, recovered, display)
		}
	}
}

func distance(a, b lens.Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}
