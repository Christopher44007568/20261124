package rollingshutter

import (
	"slices"
	"testing"
	"time"
)

func testTiming(t *testing.T, direction ReadoutDirection) RowTiming {
	t.Helper()
	timing, err := NewRowTiming(RowTimingConfig{
		RowPeriod:        10 * time.Microsecond,
		ExposureDuration: 100 * time.Microsecond,
		PoseSampleOffset: 50 * time.Microsecond,
		Direction:        direction,
	})
	if err != nil {
		t.Fatal(err)
	}
	return timing
}

func testAnalysisConfig(t *testing.T, direction ReadoutDirection, source PoseSource) AnalysisConfig {
	t.Helper()
	camera := DefaultCamera()
	camera.Height = 120
	return AnalysisConfig{
		Timing: testTiming(t, direction),
		Camera: camera,
		Line: LineSegment{
			A: Vec3{X: 0.3, Y: -10, Z: 2},
			B: Vec3{X: 0.3, Y: 10, Z: 2},
		},
		PoseSource: source,
	}
}

func motionTrajectory() ThreeSegmentTrajectory {
	return ThreeSegmentTrajectory{
		Start:          0,
		TranslationEnd: 400 * time.Microsecond,
		PitchEnd:       800 * time.Microsecond,
		TurnEnd:        1500 * time.Microsecond,
		Translation:    Vec3{X: -0.18},
		Pitch:          0.06,
		Turn:           0.16,
	}
}

func TestAnalyzeStaticPoseHasNoCurvature(t *testing.T) {
	config := testAnalysisConfig(t, TopToBottom, StaticPoseSource{})
	analysis, err := Analyze(config)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.ProjectedRows != 120 || analysis.Summary.PoseMissingRows != 0 {
		t.Fatalf("summary = %+v", analysis.Summary)
	}
	if analysis.Summary.StraightLineCurvature.MaxPixels > 1e-9 {
		t.Fatalf("static curvature = %g", analysis.Summary.StraightLineCurvature.MaxPixels)
	}
	if got, want := analysis.Summary.MaxAbsTimeErrorNS, int64(1190*time.Microsecond); got != want {
		t.Fatalf("max time error = %d, want %d", got, want)
	}
	if analysis.Summary.TopRowSampleNS == analysis.Summary.BottomRowSampleNS {
		t.Fatal("top and bottom rows share a sample time")
	}
}

func TestAnalyzeMotionCurvesLineAndChangesPoseAcrossRows(t *testing.T) {
	config := testAnalysisConfig(t, TopToBottom, motionTrajectory())
	analysis, err := Analyze(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Summary.Flags) != 0 {
		t.Fatalf("unexpected flags: %v", analysis.Summary.Flags)
	}
	if analysis.Summary.StraightLineCurvature.MaxPixels < 1 {
		t.Fatalf("motion curvature = %g", analysis.Summary.StraightLineCurvature.MaxPixels)
	}
	top, bottom := analysis.Rows[0], analysis.Rows[len(analysis.Rows)-1]
	if top.Pose == nil || bottom.Pose == nil {
		t.Fatal("expected poses for top and bottom rows")
	}
	if top.Pose.Translation.X == bottom.Pose.Translation.X && top.Pose.Yaw == bottom.Pose.Yaw {
		t.Fatalf("top and bottom poses are equal: top=%+v bottom=%+v", *top.Pose, *bottom.Pose)
	}
	if bottom.Pose.Yaw <= top.Pose.Yaw {
		t.Fatalf("turn did not progress: top=%g bottom=%g", top.Pose.Yaw, bottom.Pose.Yaw)
	}
}

func TestAnalyzeFlagsReversedReadout(t *testing.T) {
	config := testAnalysisConfig(t, BottomToTop, motionTrajectory())
	analysis, err := Analyze(config)
	if err != nil {
		t.Fatal(err)
	}
	if !analysis.Summary.DirectionReversed || !analysis.Rows[0].DirectionReversed {
		t.Fatal("reversed readout was not explicitly marked")
	}
	if !slices.Contains(analysis.Summary.Flags, FlagReadoutDirectionReversed) {
		t.Fatalf("flags = %v", analysis.Summary.Flags)
	}
	if analysis.Summary.TopRowSampleNS <= analysis.Summary.BottomRowSampleNS {
		t.Fatal("bottom-up readout should sample the top row after the bottom row")
	}
	if analysis.Rows[0].ReadoutOrder != 119 || analysis.Rows[119].ReadoutOrder != 0 {
		t.Fatal("readout order is not reversed")
	}
}

func TestAnalyzeFlagsMissingPoseWithoutSubstitution(t *testing.T) {
	source := DropoutPoseSource{
		Source: motionTrajectory(),
		Start:  500 * time.Microsecond,
		End:    700 * time.Microsecond,
	}
	config := testAnalysisConfig(t, TopToBottom, source)
	analysis, err := Analyze(config)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.PoseMissingRows == 0 {
		t.Fatal("expected missing pose rows")
	}
	if !slices.Contains(analysis.Summary.Flags, FlagPoseSamplesMissing) {
		t.Fatalf("flags = %v", analysis.Summary.Flags)
	}
	for _, row := range analysis.Rows {
		if row.PoseMissing {
			if row.PoseAvailable || row.Pose != nil || row.Image != nil {
				t.Fatalf("missing row %d was filled with another pose: %+v", row.Row, row)
			}
		}
	}
	if analysis.Summary.ProjectedRows+analysis.Summary.PoseMissingRows != 120 {
		t.Fatalf("summary = %+v", analysis.Summary)
	}
}
