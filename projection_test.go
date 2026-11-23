package rollingshutter

import (
	"math"
	"testing"
)

func TestProjectLineAtRowUsesRequestedPose(t *testing.T) {
	camera := DefaultCamera()
	line := LineSegment{
		A: Vec3{X: 0.3, Y: -10, Z: 2},
		B: Vec3{X: 0.3, Y: 10, Z: 2},
	}
	pose := Pose{Translation: Vec3{X: 0.1}, Yaw: -0.01}
	points := make([]ProjectedPoint, 0, 3)
	for _, row := range []int{20, 240, 460} {
		projection, err := ProjectLineAtRow(camera, pose, line, row)
		if err != nil {
			t.Fatalf("row %d: %v", row, err)
		}
		if math.Abs(projection.Image.Y-(float64(row)+0.5)) > 1e-7 {
			t.Fatalf("row %d projected to y=%g", row, projection.Image.Y)
		}
		points = append(points, ProjectedPoint{
			Row: row,
			X:   projection.Image.X,
			Y:   projection.Image.Y,
		})
	}
	if math.Abs(points[0].X-points[1].X) > 1e-9 || math.Abs(points[1].X-points[2].X) > 1e-9 {
		t.Fatalf("static vertical line changed x: %+v", points)
	}
	if curvature := StraightLineCurvature(points); curvature.MaxPixels > 1e-9 {
		t.Fatalf("straight projection curvature = %g", curvature.MaxPixels)
	}
}

func TestStraightLineCurvatureDetectsBend(t *testing.T) {
	straight := StraightLineCurvature([]ProjectedPoint{
		{Row: 0, X: 100, Y: 0.5},
		{Row: 1, X: 105, Y: 1.5},
		{Row: 2, X: 110, Y: 2.5},
	})
	if straight.MaxPixels > 1e-9 {
		t.Fatalf("straight line curvature = %g", straight.MaxPixels)
	}
	bent := StraightLineCurvature([]ProjectedPoint{
		{Row: 0, X: 100, Y: 0.5},
		{Row: 1, X: 140, Y: 1.5},
		{Row: 2, X: 110, Y: 2.5},
	})
	if !bent.Computed || bent.MaxPixels < 4 {
		t.Fatalf("bent curvature = %+v", bent)
	}
	if bent.AtRow != 1 {
		t.Fatalf("curvature row = %d, want 1", bent.AtRow)
	}
}
