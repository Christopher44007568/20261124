package stereo

import (
	"math"
	"testing"
)

func TestBuiltInScenesUseIndependentEyes(t *testing.T) {
	scenes, err := Scenes()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	for _, scene := range scenes {
		t.Run(scene.ID, func(t *testing.T) {
			report, err := Analyze(scene)
			if err != nil {
				t.Fatal(err)
			}
			left, right := report.Eyes[0], report.Eyes[1]
			if left.Eye != "left" || right.Eye != "right" {
				t.Fatalf("unexpected eye order: %q, %q", left.Eye, right.Eye)
			}
			if left.Camera.Frustum == right.Camera.Frustum {
				t.Fatal("left and right frustums must be distinct")
			}
			if left.NearPlaneCenter.Sub(right.NearPlaneCenter).Len() < 1e-6 {
				t.Fatal("near-plane centers must be calculated per eye")
			}
			if !report.Comparison.DistinctNearCenters {
				t.Fatal("comparison did not detect distinct near centers")
			}
			if left.Signature == right.Signature {
				t.Fatal("left and right clip results are identical")
			}
			if !report.Comparison.DistinctClipResults {
				t.Fatal("comparison did not detect distinct clip results")
			}
			for _, eye := range report.Eyes {
				if eye.Totals.SourceTriangles == 0 {
					t.Fatal("scene has no triangles")
				}
				if eye.Totals.ScreenAreaPx <= 0 {
					t.Fatal("scene is not visible")
				}
				if eye.Totals.ClippedArea3D > eye.Totals.SourceArea3D+1e-7 {
					t.Fatalf("clipped area %g exceeds source area %g",
						eye.Totals.ClippedArea3D, eye.Totals.SourceArea3D)
				}
				for _, object := range eye.Objects {
					if object.Metrics.ScreenAreaPx > 0 && object.Metrics.CoveragePct <= 0 {
						t.Fatalf("object %q has screen area but zero coverage", object.Mesh)
					}
				}
			}
		})
	}
}

func TestLargePlaneNearClipOnlyForRightEye(t *testing.T) {
	scene, err := FindScene("large-plane")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Analyze(scene)
	if err != nil {
		t.Fatal(err)
	}
	left, right := report.Eyes[0].Totals, report.Eyes[1].Totals
	if left.RejectedTriangles != 0 {
		t.Fatalf("left eye rejected %d triangles, want 0", left.RejectedTriangles)
	}
	if math.Abs(left.ClippedArea3D-left.SourceArea3D) > 1e-7 {
		t.Fatalf("left clipped area %g should equal source area %g", left.ClippedArea3D, left.SourceArea3D)
	}
	if right.ClippedArea3D >= right.SourceArea3D {
		t.Fatalf("right clipped area %g should be less than source area %g",
			right.ClippedArea3D, right.SourceArea3D)
	}
	if right.NearEdgeLengthPx <= 0 {
		t.Fatal("right eye should expose a near-plane edge")
	}
	if left.NearEdgeLengthPx != 0 {
		t.Fatalf("left eye near edge length=%g, want 0", left.NearEdgeLengthPx)
	}
}

func TestLongRodExposesDifferentNearEdges(t *testing.T) {
	scene, err := FindScene("long-rod")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Analyze(scene)
	if err != nil {
		t.Fatal(err)
	}
	if report.Eyes[0].Totals.NearEdgeLengthPx <= 0 {
		t.Fatal("left eye should show part of the rod's near cut ring")
	}
	if report.Eyes[1].Totals.NearEdgeLengthPx != 0 {
		t.Fatal("right eye near cut should be outside the viewport")
	}
	if report.Comparison.NearEdgeDeltaPx <= 0 {
		t.Fatal("rod near-edge results should differ by eye")
	}
}
