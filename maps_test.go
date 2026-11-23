package lensdistortion_test

import (
	"errors"
	"math"
	"testing"

	lens "lensdistortion"
)

func TestPolynomialMatchesAnalyticWhenCoefficientsMatch(t *testing.T) {
	model := sampleLens(lens.Point{X: 50, Y: 50})
	analytic, err := lens.NewAnalyticMap(model)
	if err != nil {
		t.Fatal(err)
	}
	polynomial, err := lens.NewPolynomialMap(model, model.Coefficients)
	if err != nil {
		t.Fatal(err)
	}
	for name, display := range keyPoints(model.Center) {
		got, ok := polynomial.SourceFromDisplay(display)
		if !ok {
			t.Fatalf("%s: polynomial source lookup failed", name)
		}
		want, ok := analytic.SourceFromDisplay(display)
		if !ok {
			t.Fatalf("%s: analytic source lookup failed", name)
		}
		if distance(got, want) > 1e-9 {
			t.Fatalf("%s: polynomial %+v, analytic %+v", name, got, want)
		}
		back, ok := polynomial.DisplayFromSource(got)
		if !ok || distance(back, display) > 1e-8 {
			t.Fatalf("%s: polynomial inverse round trip failed: %+v", name, back)
		}
	}
}

func TestPolynomialRejectsInvalidCoefficientSets(t *testing.T) {
	model := sampleLens(lens.Point{})
	model.MaxRadius = 1
	if _, err := lens.NewPolynomialMap(model, []float64{-2}); !errors.Is(err, lens.ErrNonMonotonic) {
		t.Fatalf("expected ErrNonMonotonic, got %v", err)
	}
	if _, err := lens.NewPolynomialMap(model, []float64{math.NaN()}); !errors.Is(err, lens.ErrInvalidPolynomial) {
		t.Fatalf("expected ErrInvalidPolynomial, got %v", err)
	}
}

func TestPiecewiseLUTTracksAnalyticAndStaysInsideDomain(t *testing.T) {
	model := sampleLens(lens.Point{X: 50, Y: 50})
	analytic, err := lens.NewAnalyticMap(model)
	if err != nil {
		t.Fatal(err)
	}
	table, err := lens.NewUniformLUTFromLens(model, model.MaxRadius, 128)
	if err != nil {
		t.Fatal(err)
	}
	if got := table.MaxDisplayRadius(); math.Abs(got-model.MaxRadius) > 1e-12 {
		t.Fatalf("max display radius = %g, want %g", got, model.MaxRadius)
	}
	for name, display := range keyPoints(model.Center) {
		got, ok := table.SourceFromDisplay(display)
		if !ok {
			t.Fatalf("%s: LUT source lookup failed", name)
		}
		want, ok := analytic.SourceFromDisplay(display)
		if !ok {
			t.Fatalf("%s: analytic source lookup failed", name)
		}
		if distance(got, want) > 1e-3 {
			t.Fatalf("%s: LUT %+v, analytic %+v, error %g", name, got, want, distance(got, want))
		}
		back, ok := table.DisplayFromSource(got)
		if !ok || distance(back, display) > 1e-3 {
			t.Fatalf("%s: LUT inverse round trip failed: %+v", name, back)
		}
	}
	outside := lens.Point{
		X: model.Center.X + model.MaxRadius*model.ReferenceRadius + 1,
		Y: model.Center.Y,
	}
	if _, ok := table.SourceFromDisplay(outside); ok {
		t.Fatal("display radius beyond final LUT knot was accepted")
	}
}

func TestPiecewiseLUTValidationAndExplicitKnots(t *testing.T) {
	if _, err := lens.NewPiecewiseLUT(lens.Point{}, 1, []lens.LUTKnot{
		{DisplayRadius: 0, SourceRadius: 0},
		{DisplayRadius: 1, SourceRadius: 1},
		{DisplayRadius: 0.5, SourceRadius: 0.4},
	}); !errors.Is(err, lens.ErrInvalidLUT) {
		t.Fatalf("expected invalid unsorted LUT, got %v", err)
	}
	table, err := lens.NewPiecewiseLUT(lens.Point{}, 1, []lens.LUTKnot{
		{DisplayRadius: 0, SourceRadius: 0},
		{DisplayRadius: 1, SourceRadius: 0.5},
		{DisplayRadius: 2, SourceRadius: 1.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := table.SourceFromDisplay(lens.Point{X: 1.5})
	if !ok || math.Abs(got.X-1) > 1e-12 {
		t.Fatalf("linear interpolation = %+v, want x=1", got)
	}
	back, ok := table.DisplayFromSource(lens.Point{X: 1})
	if !ok || math.Abs(back.X-1.5) > 1e-12 {
		t.Fatalf("reverse interpolation = %+v, want x=1.5", back)
	}
}
