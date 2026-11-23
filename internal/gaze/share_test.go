package gaze

import "testing"

func TestIdenticalGazeSharesInteriorButNotBoundaries(t *testing.T) {
	gazePoint := Point{X: 0.5, Y: 0.5}
	left := mustMask(t, LeftEye, gazePoint, false, 32)
	right := mustMask(t, RightEye, gazePoint, false, 32)

	report, err := AnalyzeSharing(left, right)
	if err != nil {
		t.Fatalf("AnalyzeSharing: %v", err)
	}
	if report.OverlapCount != left.SampledCount() {
		t.Fatalf("overlap = %d, want %d sampled cells", report.OverlapCount, left.SampledCount())
	}
	if report.SharedCount == 0 {
		t.Fatal("identical gaze should share interior cells")
	}
	if report.SharedCount >= report.OverlapCount {
		t.Fatalf("shared %d should exclude boundary cells from overlap %d", report.SharedCount, report.OverlapCount)
	}

	for row := 0; row < report.Size; row++ {
		for col := 0; col < report.Size; col++ {
			if !report.Shared[row][col] {
				continue
			}
			if left.Boundary[row][col] || right.Boundary[row][col] {
				t.Fatalf("shared cell (%d,%d) sits on a quality boundary", row, col)
			}
		}
	}

	if report.SharedWork >= report.StandaloneWork {
		t.Fatal("shared pipeline must reduce but not eliminate all work")
	}
}

func TestOcclusionDisablesSharing(t *testing.T) {
	gazePoint := Point{X: 0.5, Y: 0.5}
	left := mustMask(t, LeftEye, gazePoint, true, 24)
	right := mustMask(t, RightEye, gazePoint, false, 24)

	report, err := AnalyzeSharing(left, right)
	if err != nil {
		t.Fatalf("AnalyzeSharing: %v", err)
	}
	if !report.EitherOccluded {
		t.Fatal("occlusion flag not recorded")
	}
	if report.SharedCount != 0 {
		t.Fatalf("occluded frame has %d shared cells, want 0", report.SharedCount)
	}
}

func TestReverseSaccadeDoesNotCopyFoveaAcrossEyes(t *testing.T) {
	left := mustMask(t, LeftEye, Point{X: 0.12, Y: 0.5}, false, 32)
	right := mustMask(t, RightEye, Point{X: 0.88, Y: 0.5}, false, 32)

	report, err := AnalyzeSharing(left, right)
	if err != nil {
		t.Fatalf("AnalyzeSharing: %v", err)
	}
	for row := 0; row < report.Size; row++ {
		for col := 0; col < report.Size; col++ {
			if report.Shared[row][col] &&
				(left.Tiers[row][col] == Foveal || right.Tiers[row][col] == Foveal) {
				t.Fatalf("foveal cell (%d,%d) incorrectly shared", row, col)
			}
		}
	}

	rebuiltLeft, rebuiltRight := ApplySharing(left, right, report)
	if !left.Equals(rebuiltLeft) || !right.Equals(rebuiltRight) {
		t.Fatal("sharing changed a monocular mask")
	}
}

func TestWorkPlanMatchesReportCounts(t *testing.T) {
	left := mustMask(t, LeftEye, Point{X: 0.45, Y: 0.5}, false, 28)
	right := mustMask(t, RightEye, Point{X: 0.55, Y: 0.5}, false, 28)
	report, err := AnalyzeSharing(left, right)
	if err != nil {
		t.Fatalf("AnalyzeSharing: %v", err)
	}
	plan := report.PlanWork()

	if len(plan.Shared) != report.SharedCount {
		t.Fatalf("planned shared %d, want %d", len(plan.Shared), report.SharedCount)
	}
	if len(plan.LeftPrivate) != report.OverlapCount-report.SharedCount {
		t.Fatalf("left private %d, want %d", len(plan.LeftPrivate), report.OverlapCount-report.SharedCount)
	}
	if len(plan.RightPrivate) != len(plan.LeftPrivate) {
		t.Fatal("both eyes need private evaluations for every non-shared overlap cell")
	}
}

func TestAnalyzeSharingRejectsMismatchedGrids(t *testing.T) {
	left := mustMask(t, LeftEye, Point{X: 0.5, Y: 0.5}, false, 16)
	right := mustMask(t, RightEye, Point{X: 0.5, Y: 0.5}, false, 20)
	if _, err := AnalyzeSharing(left, right); err == nil {
		t.Fatal("expected error for mismatched grid sizes")
	}
}

func mustMask(t *testing.T, id Eye, point Point, occluded bool, size int) *Mask {
	t.Helper()
	mask, err := BuildMask(EyeConfig{
		Eye:      id,
		Gaze:     point,
		Occluded: occluded,
		Params:   DefaultParams(),
	}, size)
	if err != nil {
		t.Fatalf("BuildMask: %v", err)
	}
	return mask
}
