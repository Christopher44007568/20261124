package lensdistortion_test

import (
	"math"
	"testing"

	lens "lensdistortion"
)

func TestStereoUsesIndependentDistortionCenters(t *testing.T) {
	leftLens := sampleLens(lens.Point{X: 45, Y: 50})
	rightLens := sampleLens(lens.Point{X: 55, Y: 50})
	left, err := lens.NewAnalyticMap(leftLens)
	if err != nil {
		t.Fatal(err)
	}
	right, err := lens.NewAnalyticMap(rightLens)
	if err != nil {
		t.Fatal(err)
	}
	rig, err := lens.NewStereoRig(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if center, _ := rig.Center(lens.LeftEye); center != leftLens.Center {
		t.Fatalf("left center = %+v, want %+v", center, leftLens.Center)
	}
	if center, _ := rig.Center(lens.RightEye); center != rightLens.Center {
		t.Fatalf("right center = %+v, want %+v", center, rightLens.Center)
	}
	for _, eye := range []lens.Eye{lens.LeftEye, lens.RightEye} {
		center, _ := rig.Center(eye)
		got, ok := rig.SourceFromDisplay(eye, center)
		if !ok || got != center {
			t.Fatalf("%s center shifted to %+v", eye, got)
		}
	}
	leftSource, ok := rig.SourceFromDisplay(lens.LeftEye, lens.Point{X: 50, Y: 50})
	if !ok {
		t.Fatal("left lookup failed")
	}
	rightSource, ok := rig.SourceFromDisplay(lens.RightEye, lens.Point{X: 50, Y: 50})
	if !ok {
		t.Fatal("right lookup failed")
	}
	if leftSource == rightSource {
		t.Fatal("stereo eyes used the same center")
	}
	if leftSource.X >= 50 || rightSource.X <= 50 {
		t.Fatalf("barrel compression went toward the wrong eye center: left=%+v right=%+v", leftSource, rightSource)
	}
	if _, ok := rig.SourceFromDisplay(lens.Eye(99), lens.Point{}); ok {
		t.Fatal("invalid eye was accepted")
	}
}

func TestStereoRigAllMappingKinds(t *testing.T) {
	for _, kind := range []lens.MapKind{lens.KindAnalytic, lens.KindPolynomial, lens.KindLUT} {
		leftLens := sampleLens(lens.Point{X: 45, Y: 50})
		rightLens := sampleLens(lens.Point{X: 55, Y: 50})
		rig, err := lens.NewStereoRigFromCalibrations(
			lens.EyeCalibration{Kind: kind, Lens: leftLens},
			lens.EyeCalibration{Kind: kind, Lens: rightLens},
		)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		for _, eye := range []lens.Eye{lens.LeftEye, lens.RightEye} {
			center, _ := rig.Center(eye)
			got, ok := rig.SourceFromDisplay(eye, center)
			if !ok || distance(got, center) > 1e-9 {
				t.Fatalf("%s/%s: center maps to %+v", kind, eye, got)
			}
		}
		if rig.Mapper(lens.LeftEye).Kind() != kind {
			t.Fatalf("%s: left mapper kind = %s", kind, rig.Mapper(lens.LeftEye).Kind())
		}
	}
}

type offsetMapper struct {
	center lens.Point
	delta  lens.Point
}

func (m offsetMapper) SourceFromDisplay(display lens.Point) (lens.Point, bool) {
	if !display.IsFinite() {
		return lens.Point{}, false
	}
	return lens.Point{X: display.X + m.delta.X, Y: display.Y + m.delta.Y}, true
}

func (m offsetMapper) DisplayFromSource(source lens.Point) (lens.Point, bool) {
	return lens.Point{X: source.X - m.delta.X, Y: source.Y - m.delta.Y}, source.IsFinite()
}

func (m offsetMapper) Center() lens.Point { return m.center }
func (m offsetMapper) Kind() lens.MapKind { return lens.KindAnalytic }

func TestCoverageFullFrameAndNonRectangularMask(t *testing.T) {
	mapper := offsetMapper{center: lens.Point{X: 4, Y: 4}}
	report, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 10, Height: 10, SourceWidth: 10, SourceHeight: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RegionPixels != 100 || report.SampledPixels != 100 || report.HolePixels != 0 {
		t.Fatalf("full frame coverage = %+v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("full frame validation: %v", err)
	}

	polygon, err := lens.NewPolygonRegion([]lens.Point{
		{X: 4, Y: 0}, {X: 9, Y: 4}, {X: 4, Y: 9}, {X: 0, Y: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !polygon.Contains(lens.Point{X: 4, Y: 0}) {
		t.Fatal("polygon edge must be inside the region")
	}
	if polygon.Contains(lens.Point{X: 0, Y: 0}) {
		t.Fatal("polygon corner outside mask was accepted")
	}
	report, err = lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 10, Height: 10, SourceWidth: 10, SourceHeight: 10,
		Region: polygon,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RegionPixels <= 10 || report.RegionPixels >= 100 {
		t.Fatalf("unexpected masked pixel count %d", report.RegionPixels)
	}
	if report.At(0, 0) != lens.PixelOutsideRegion {
		t.Fatalf("outside mask status = %s", report.At(0, 0))
	}
	if report.At(4, 4) != lens.PixelMapped {
		t.Fatalf("inside mask status = %s", report.At(4, 4))
	}
	if report.HolePixels != 0 {
		t.Fatalf("masked display created %d holes", report.HolePixels)
	}
}

func TestCoverageReportsOutOfSourceAndBoundaryHoles(t *testing.T) {
	mapper := offsetMapper{center: lens.Point{X: 0, Y: 0}}
	report, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 12, Height: 10, SourceWidth: 8, SourceHeight: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceOutPixels != 40 || report.HolePixels != 40 {
		t.Fatalf("out-of-source counts: %+v", report)
	}
	if !report.HoleAt(11, 0) || report.HoleAt(7, 0) {
		t.Fatal("hole positions are wrong")
	}
	if report.Validate() == nil {
		t.Fatal("expected validation failure")
	}
}

func TestCoverageNearestVersusBilinearAtSourceBoundary(t *testing.T) {
	mapper := offsetMapper{center: lens.Point{X: 2, Y: 0}, delta: lens.Point{X: 0.4}}
	nearest, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 5, Height: 1, SourceWidth: 5, SourceHeight: 1,
		Sampling: lens.SampleNearest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if nearest.SampledPixels != 5 || nearest.HolePixels != 0 {
		t.Fatalf("nearest coverage = %+v", nearest)
	}
	bilinear, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 5, Height: 1, SourceWidth: 5, SourceHeight: 1,
		Sampling: lens.SampleBilinear,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bilinear.SampledPixels != 4 || bilinear.HolePixels != 1 || bilinear.At(4, 0) != lens.PixelOutOfSource {
		t.Fatalf("bilinear coverage = %+v, status at edge = %s", bilinear, bilinear.At(4, 0))
	}
}

func TestCoverageClampAvoidsReadsButStillReportsBoundary(t *testing.T) {
	mapper := offsetMapper{center: lens.Point{X: 2, Y: 0}, delta: lens.Point{X: 0.4}}
	report, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 5, Height: 1, SourceWidth: 5, SourceHeight: 1,
		Sampling: lens.SampleClamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ClampedPixels != 1 || report.SourceOutPixels != 1 || report.HolePixels != 0 {
		t.Fatalf("clamp coverage = %+v", report)
	}
	if report.At(4, 0) != lens.PixelClamped {
		t.Fatalf("clamped edge status = %s", report.At(4, 0))
	}
	if report.Validate() == nil {
		t.Fatal("clamped boundary should remain visible to validation")
	}
}

func TestCoverageModelDomainFailure(t *testing.T) {
	model := sampleLens(lens.Point{X: 0, Y: 0})
	model.MaxRadius = math.Sqrt2
	mapper, err := lens.NewAnalyticMap(model)
	if err != nil {
		t.Fatal(err)
	}
	report, err := lens.AnalyzeCoverage(mapper, lens.CoverageOptions{
		Width: 101, Height: 101, SourceWidth: 101, SourceHeight: 101,
		Sampling: lens.SampleNearest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ModelOutPixels == 0 || report.HolePixels == 0 {
		t.Fatalf("expected corner domain failures, got %+v", report)
	}
}
