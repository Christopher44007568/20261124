package gaze

import (
	"math"
	"testing"
)

func TestBuildMaskCentersFoveaOnGaze(t *testing.T) {
	cfg := EyeConfig{Eye: LeftEye, Gaze: Point{X: 0.5, Y: 0.5}, Params: DefaultParams()}
	mask, err := BuildMask(cfg, 24)
	if err != nil {
		t.Fatalf("BuildMask returned error: %v", err)
	}

	center := mask.Tiers[12][12]
	if center != Foveal {
		t.Fatalf("gaze center level = %d, want foveal (%d)", center, Foveal)
	}
	corner := mask.Tiers[0][0]
	if corner != Peripheral {
		t.Fatalf("corner level = %d, want peripheral (%d)", corner, Peripheral)
	}

	centroid, ok := mask.Centroid(Foveal)
	if !ok {
		t.Fatal("foveal centroid missing")
	}
	if math.Abs(centroid.X-0.5) > 0.06 || math.Abs(centroid.Y-0.5) > 0.06 {
		t.Fatalf("foveal centroid = %+v, want near gaze point", centroid)
	}
}

func TestBuildMaskRejectsInvalidInputs(t *testing.T) {
	valid := DefaultParams()
	cases := []EyeConfig{
		{Gaze: Point{X: 2, Y: 0}, Params: valid},
		{Gaze: Point{X: 0, Y: 0}, Params: Params{FovealRadius: -1, ParafovealRadius: 1}},
		{Gaze: Point{X: 0, Y: 0}, Params: Params{FovealRadius: 0.5, ParafovealRadius: 0.2}},
	}
	for index, cfg := range cases {
		if _, err := BuildMask(cfg, 12); err == nil {
			t.Fatalf("case %d: expected validation error", index)
		}
	}
	if _, err := BuildMask(EyeConfig{Gaze: Point{}, Params: valid}, 0); err == nil {
		t.Fatal("expected error for non-positive grid size")
	}
}

func TestOcclusionUsesFallbackEverywhere(t *testing.T) {
	cfg := EyeConfig{
		Eye:      RightEye,
		Gaze:     Point{X: 0.5, Y: 0.5},
		Occluded: true,
		Params:   DefaultParams(),
	}
	mask, err := BuildMask(cfg, 16)
	if err != nil {
		t.Fatalf("BuildMask returned error: %v", err)
	}
	for row := range mask.Tiers {
		for col, level := range mask.Tiers[row] {
			if level != Peripheral {
				t.Fatalf("occluded cell (%d,%d) level = %d, want fallback", row, col, level)
			}
		}
	}
}

func TestBoundaryIsDetected(t *testing.T) {
	cfg := EyeConfig{Gaze: Point{X: 0.5, Y: 0.5}, Params: DefaultParams()}
	mask, err := BuildMask(cfg, 24)
	if err != nil {
		t.Fatalf("BuildMask returned error: %v", err)
	}
	if mask.CountBoundary() == 0 {
		t.Fatal("expected non-empty quality boundary")
	}

	for row := range mask.Boundary {
		for col, marked := range mask.Boundary[row] {
			if !marked {
				continue
			}
			neighborLevels := neighborLevels(mask.Tiers, row, col)
			if !neighborLevels[mask.Tiers[row][col]] || len(neighborLevels) < 2 {
				t.Fatalf("cell (%d,%d) marked boundary without a different neighbor", row, col)
			}
		}
	}
}

func TestReverseGazeKeepsFoveaeSeparate(t *testing.T) {
	left, err := BuildMask(EyeConfig{
		Eye: LeftEye, Gaze: Point{X: 0.12, Y: 0.5}, Params: DefaultParams(),
	}, 32)
	if err != nil {
		t.Fatalf("left BuildMask: %v", err)
	}
	right, err := BuildMask(EyeConfig{
		Eye: RightEye, Gaze: Point{X: 0.88, Y: 0.5}, Params: DefaultParams(),
	}, 32)
	if err != nil {
		t.Fatalf("right BuildMask: %v", err)
	}

	leftCentroid, _ := left.Centroid(Foveal)
	rightCentroid, _ := right.Centroid(Foveal)
	if leftCentroid.X >= rightCentroid.X {
		t.Fatalf("left centroid %+v should stay left of right centroid %+v", leftCentroid, rightCentroid)
	}
	for row := range left.Tiers {
		for col := range left.Tiers[row] {
			if left.Tiers[row][col] == Foveal && right.Tiers[row][col] == Foveal {
				t.Fatalf("foveal sets overlap at cell (%d,%d)", row, col)
			}
		}
	}
}

func neighborLevels(tiers [][]Level, row, col int) map[Level]bool {
	result := map[Level]bool{}
	for _, neighbor := range neighbors(row, col, len(tiers)) {
		result[tiers[neighbor[0]][neighbor[1]]] = true
	}
	result[tiers[row][col]] = true
	return result
}
